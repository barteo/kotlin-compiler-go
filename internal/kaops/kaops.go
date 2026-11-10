// Package kaops registers libkc's Analysis API operations (internal/kcabi)
// on package analysis/ka. Messages are in package wire's format; the
// layouts below are mirrored by jvm/kc-aa's KcModel.
//
// OpKaOpen: config → session handle
//
//	config  = jdkHome:String noJDK:Bool moduleName:String jobs:Uint
//	          n:Uint classPath:String*n m:Uint friendPath:String*m fixed:Bool
//
// OpKaUpdate: session:Int n:Uint path:String*n k:Uint kept:Uint*k b:Uint base:Uint*b → model
//
//	model   = handle:Int check:Uint java:Uint same:Bool [n:Uint file*n m:Uint diag*m]
//	file    = from:Uint path:String [package:String annots decls lookups nodes:Uint kinds:Uint
//	          text:Uint]
//	decls   = n:Uint decl*n
//	decl    = kind:Uint name:String node:Uint classID:String visibility:Uint
//	          modality:Uint flags:Uint variance:Uint type receiver:type
//	          supertypes:types bounds:types
//	          annots typeParams:decls params:decls members:decls
//	          default:value locals:decls module:String
//	          n:Uint inheritor:String*n
//	types   = n:Uint type*n
//	type    = kind:Uint [classID:String tflags:Uint param:Uint name:String
//	          contexts:Uint n:Uint (variance:Uint type)*n abbrev:type annots]
//	annots  = n:Uint annot*n
//	annot   = classID:String node:Uint target:String implied:Uint n:Uint (name:String value)*n
//	          typeArgs:types
//	value   = kind:Uint [node:Uint
//	          const: ckind:Uint [bits:Int | float:Uint | str:String]
//	          | enum: classID:String entry:String | class: type
//	          | annotation: annot | array: n:Uint value*n]
//	diag    = path:String line:Uint col:Uint message:String feature:String
//	lookups = n:Uint (scope:String kind:Uint name:String)*n
//
// check is the model's ka Model.Check, and java its Model.JavaID. kept are the checks whose models
// the caller kept what it built from; if check is one of them, same is
// set and the files and diagnostics, which are that model's, are left out.
// base are the checks whose models' files the caller kept as it read
// them: a file that is one of theirs, as an update that checks only some
// files keeps the others', has its check as from and nothing after its
// path.
// Otherwise from is 0.
//
// lookups are the names a file looks up (ka's File.Lookups), for
// incremental processing; kind is a ka LookupKind. nodes is the number of
// the file's psi nodes and kinds the hash of their kinds in preorder
// (psi.Snapshot.KindSequenceHash), which the JVM checks its PSI against.
// text is the 64-bit FNV-1a hash of the file's text as read, its bytes:
// the JVM keeps the PSI it parsed from bytes of the same hash without
// reading the file again.
//
// An unresolved annotation has classID "".
//
// A value of kind 0 is none, and has nothing after its kind: a
// declaration without a default. Otherwise kind is a ka ValueKind, and
// an unsupported value has only its node. ckind is a ka ConstKind: a
// Null has nothing after it, a Float or a Double has the bits of its
// float64, a String its str, and the others their bits.
//
// kind, visibility, modality, flags and variance are ka's Kind,
// Visibility, Modality, Flags and Variance. A type of kind 0 is none,
// and has nothing after its kind: a declaration without a type or
// receiver, or a star projection's argument. Otherwise kind is a ka
// TypeKind, and tflags a bit set: 1 nullable, 2 flexible, 4 definitely
// non-null, 8 extension function type. abbrev is the typealias type the
// type was written as, or none, and annots the type's annotations as
// written, @A T.
//
// OpKaClose: handle:Int → nothing. It releases a session or a model, and
// the class path a model reads.
//
// OpKaClass: model:Int classID:String → n:Uint decl*n absent:Bool
//
// It returns the model of a class of the class path (ka's Model.Class),
// its nested classes among its members; n is 0 if there is no such class
// and 1 otherwise. A library declaration's nodes are 0. absent is set if
// n is 0 and the class path, the sources left out, has no such class
// (ka's Model.Absent).
//
// OpKaPackage: model:Int package:String → exists:Bool n:Uint classID:String*n decls
//
// It returns the model of a package of the class path (ka's
// Model.Package): its top-level classes' class IDs, and its top-level
// functions, properties and type aliases. exists is false, and the rest
// empty, if the class path has no such package.
package kaops

import (
	"context"
	"io"
	"math"
	"slices"
	"sync"

	"github.com/barteo/kotlin-compiler-go/analysis/ka"
	"github.com/barteo/kotlin-compiler-go/internal/kcabi"
	"github.com/barteo/kotlin-compiler-go/internal/kcabi/wire"
)

// The operations. The numbers are part of the ABI: append only.
const (
	OpKaOpen    kcabi.Op = 10
	OpKaUpdate  kcabi.Op = 11
	OpKaClose   kcabi.Op = 12
	OpKaClass   kcabi.Op = 13
	OpKaPackage kcabi.Op = 14
)

// handles are the sessions and models the JVM holds.
var handles kcabi.Handles

func init() {
	kcabi.Register(OpKaOpen, open)
	kcabi.Register(OpKaUpdate, update)
	kcabi.Register(OpKaClose, closeHandle)
	kcabi.Register(OpKaClass, class)
	kcabi.Register(OpKaPackage, pkg)
}

func pkg(req []byte) ([]byte, error) {
	r := wire.NewReader(req)
	h := kcabi.Handle(r.Int())
	name := r.String()
	if err := r.Err(); err != nil {
		return nil, err
	}
	m, err := kcabi.Get[*ka.Model](&handles, h)
	if err != nil {
		return nil, err
	}
	var w wire.Writer
	p := m.Package(name)
	w.Bool(p != nil)
	if p == nil {
		p = &ka.Package{}
	}
	w.Uint(uint64(len(p.Classes)))
	for _, id := range p.Classes {
		w.String(id)
	}
	writeDecls(&w, p.Decls)
	return w.Message(), nil
}

func class(req []byte) ([]byte, error) {
	r := wire.NewReader(req)
	h := kcabi.Handle(r.Int())
	id := r.String()
	if err := r.Err(); err != nil {
		return nil, err
	}
	m, err := kcabi.Get[*ka.Model](&handles, h)
	if err != nil {
		return nil, err
	}
	var w wire.Writer
	if d := m.Class(id); d != nil {
		writeDecls(&w, []*ka.Decl{d})
		w.Bool(false)
	} else {
		writeDecls(&w, nil)
		w.Bool(m.Absent(id))
	}
	return w.Message(), nil
}

func open(req []byte) ([]byte, error) {
	r := wire.NewReader(req)
	var cfg ka.Config
	cfg.JDKHome = r.String()
	cfg.NoJDK = r.Bool()
	cfg.ModuleName = r.String()
	cfg.Jobs = int(r.Uint())
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		cfg.ClassPath = append(cfg.ClassPath, r.String())
	}
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		cfg.FriendPaths = append(cfg.FriendPaths, r.String())
	}
	cfg.FixedSources = r.Bool()
	if err := r.Err(); err != nil {
		return nil, err
	}
	var w wire.Writer
	w.Int(int64(handles.Put(ka.Open(cfg))))
	return w.Message(), nil
}

func update(req []byte) ([]byte, error) {
	r := wire.NewReader(req)
	h := kcabi.Handle(r.Int())
	var sources []ka.Source
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		sources = append(sources, ka.Source{Path: r.String()})
	}
	var kept, bases []uint64
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		kept = append(kept, r.Uint())
	}
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		bases = append(bases, r.Uint())
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	s, err := kcabi.Get[*ka.Session](&handles, h)
	if err != nil {
		return nil, err
	}
	m, err := s.Update(context.Background(), sources)
	if err != nil {
		return nil, err
	}
	tm := ka.NewTimer()
	defer tm.Print("update")
	defer tm.Mark("encode")
	var w wire.Writer
	w.Int(int64(handles.Put(m)))
	checks.add(m)
	w.Uint(m.Check)
	w.Uint(m.JavaID())
	same := slices.Contains(kept, m.Check)
	w.Bool(same)
	if same {
		return w.Message(), nil
	}
	from := checks.files(bases)
	w.Uint(uint64(len(m.Files)))
	for _, f := range m.Files {
		w.Uint(from[f])
		w.String(f.Path)
		if from[f] != 0 {
			continue
		}
		w.String(f.Package)
		writeAnnotations(&w, f.Annotations)
		writeDecls(&w, f.Decls)
		w.Uint(uint64(len(f.Lookups)))
		for _, l := range f.Lookups {
			w.String(l.Scope)
			w.Uint(uint64(l.Kind))
			w.String(l.Name)
		}
		w.Uint(uint64(f.Snapshot.NodeCount()))
		w.Uint(f.Snapshot.KindSequenceHash())
		w.Uint(textHash(f.Snapshot.Source()))
	}
	w.Uint(uint64(len(m.Diagnostics)))
	for _, d := range m.Diagnostics {
		w.String(d.Path)
		w.Uint(uint64(d.Line))
		w.Uint(uint64(d.Col))
		w.String(d.Message)
		w.String(d.Feature)
	}
	return w.Message(), nil
}

// textHash is the 64-bit FNV-1a hash of text's bytes (OpKaUpdate's
// text), which jvm/kc-aa's KcEnvironment computes the same way.
func textHash(text string) uint64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(text); i++ {
		h ^= uint64(text[i])
		h *= 1099511628211
	}
	return h
}

// checks are the files of the models with open handles, by their checks,
// for the files an update leaves out (OpKaUpdate's base). Models of the
// same check have the same files.
var checks = checkFiles{m: map[uint64]*checkEntry{}}

type checkFiles struct {
	mu sync.Mutex
	m  map[uint64]*checkEntry
}

type checkEntry struct {
	models int // the open handles of models of the check
	files  []*ka.File
}

func (c *checkFiles) add(m *ka.Model) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.m[m.Check]
	if e == nil {
		e = &checkEntry{files: m.Files}
		c.m[m.Check] = e
	}
	e.models++
}

func (c *checkFiles) remove(m *ka.Model) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.m[m.Check]; e != nil {
		if e.models--; e.models == 0 {
			delete(c.m, m.Check)
		}
	}
}

// files returns the check of each file of the models of checks.
func (c *checkFiles) files(checks []uint64) map[*ka.File]uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	from := map[*ka.File]uint64{}
	for _, check := range checks {
		if e := c.m[check]; e != nil {
			for _, f := range e.files {
				from[f] = check
			}
		}
	}
	return from
}

func writeDecls(w *wire.Writer, ds []*ka.Decl) {
	w.Uint(uint64(len(ds)))
	for _, d := range ds {
		w.Uint(uint64(d.Kind))
		w.String(d.Name)
		w.Uint(uint64(d.Node))
		w.String(d.ClassID)
		w.Uint(uint64(d.Visibility))
		w.Uint(uint64(d.Modality))
		w.Uint(uint64(d.Flags))
		w.Uint(uint64(d.Variance))
		writeType(w, d.Type)
		writeType(w, d.Receiver)
		writeTypes(w, d.Supertypes)
		writeTypes(w, d.Bounds)
		writeAnnotations(w, d.Annotations)
		writeDecls(w, d.TypeParams)
		writeDecls(w, d.Params)
		writeDecls(w, d.Members)
		writeValue(w, d.Default)
		writeDecls(w, d.Locals)
		w.String(d.Module)
		w.Uint(uint64(len(d.Inheritors)))
		for _, c := range d.Inheritors {
			w.String(c)
		}
	}
}

func writeTypes(w *wire.Writer, ts []*ka.Type) {
	w.Uint(uint64(len(ts)))
	for _, t := range ts {
		writeType(w, t)
	}
}

func writeType(w *wire.Writer, t *ka.Type) {
	if t == nil {
		w.Uint(0)
		return
	}
	w.Uint(uint64(t.Kind))
	w.String(t.ClassID)
	var flags uint64
	for i, on := range [...]bool{t.Nullable, t.Flexible, t.DefinitelyNonNull, t.Extension, t.Inferred} {
		if on {
			flags |= 1 << i
		}
	}
	w.Uint(flags)
	w.Uint(uint64(t.Param))
	w.String(t.Name)
	w.Uint(uint64(t.Contexts))
	w.Uint(uint64(len(t.Args)))
	for _, a := range t.Args {
		w.Uint(uint64(a.Variance))
		writeType(w, a.Type)
	}
	writeType(w, t.Abbrev)
	writeAnnotations(w, t.Annotations)
}

func writeAnnotations(w *wire.Writer, as []ka.Annotation) {
	w.Uint(uint64(len(as)))
	for i := range as {
		writeAnnotation(w, &as[i])
	}
}

func writeAnnotation(w *wire.Writer, a *ka.Annotation) {
	w.String(a.ClassID)
	w.Uint(uint64(a.Node))
	w.String(a.Target)
	implied := uint64(0)
	if a.Implied {
		implied = 1
	}
	w.Uint(implied)
	w.Uint(uint64(len(a.Args)))
	for _, arg := range a.Args {
		w.String(arg.Name)
		writeValue(w, arg.Value)
	}
	w.Uint(uint64(len(a.TypeArgs)))
	for _, t := range a.TypeArgs {
		writeType(w, t)
	}
}

func writeValue(w *wire.Writer, v *ka.Value) {
	if v == nil {
		w.Uint(0)
		return
	}
	w.Uint(uint64(v.Kind))
	w.Uint(uint64(v.Node))
	switch v.Kind {
	case ka.ValueConst:
		w.Uint(uint64(v.Const))
		switch v.Const {
		case ka.ConstNull:
		case ka.ConstFloat, ka.ConstDouble:
			w.Uint(math.Float64bits(v.Float))
		case ka.ConstString:
			w.String(v.Str)
		default:
			w.Int(v.Bits)
		}
	case ka.ValueEnum:
		w.String(v.ClassID)
		w.String(v.Entry)
	case ka.ValueClass:
		writeType(w, v.Type)
	case ka.ValueAnnotation:
		if v.Annotation == nil {
			writeAnnotation(w, &ka.Annotation{})
		} else {
			writeAnnotation(w, v.Annotation)
		}
	case ka.ValueArray:
		w.Uint(uint64(len(v.Elems)))
		for _, e := range v.Elems {
			writeValue(w, e)
		}
	}
}

func closeHandle(req []byte) ([]byte, error) {
	r := wire.NewReader(req)
	h := kcabi.Handle(r.Int())
	if err := r.Err(); err != nil {
		return nil, err
	}
	x, err := handles.Get(h)
	if err != nil {
		return nil, err
	}
	handles.Delete(h)
	if m, ok := x.(*ka.Model); ok {
		checks.remove(m)
	}
	if c, ok := x.(io.Closer); ok {
		return nil, c.Close()
	}
	return nil, nil
}
