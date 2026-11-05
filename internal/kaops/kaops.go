// Package kaops registers libkc's Analysis API operations (internal/kcabi)
// on package analysis/ka. Messages are in package wire's format; the
// layouts below are mirrored by jvm/kc-aa's KcModel.
//
// OpKaOpen: config → session handle
//
//	config  = jdkHome:String noJDK:Bool moduleName:String jobs:Uint
//	          n:Uint classPath:String*n
//
// OpKaUpdate: session:Int n:Uint path:String*n → model
//
//	model   = handle:Int n:Uint file*n m:Uint diag*m
//	file    = path:String package:String annots decls
//	decls   = n:Uint decl*n
//	decl    = kind:Uint name:String node:Uint classID:String visibility:Uint
//	          modality:Uint flags:Uint variance:Uint type receiver:type
//	          supertypes:types bounds:types
//	          annots typeParams:decls params:decls members:decls
//	          default:value
//	types   = n:Uint type*n
//	type    = kind:Uint [classID:String tflags:Uint param:Uint name:String
//	          contexts:Uint n:Uint (variance:Uint type)*n]
//	annots  = n:Uint annot*n
//	annot   = classID:String node:Uint target:String n:Uint (name:String value)*n
//	value   = kind:Uint [node:Uint
//	          const: ckind:Uint [bits:Int | float:Uint | str:String]
//	          | enum: classID:String entry:String | class: type
//	          | annotation: annot | array: n:Uint value*n]
//	diag    = path:String line:Uint col:Uint message:String feature:String
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
// non-null, 8 extension function type.
//
// OpKaClose: handle:Int → nothing. It releases a session or a model.
package kaops

import (
	"context"
	"math"

	"github.com/barteo/kotlin-compiler-go/analysis/ka"
	"github.com/barteo/kotlin-compiler-go/internal/kcabi"
	"github.com/barteo/kotlin-compiler-go/internal/kcabi/wire"
)

// The operations. The numbers are part of the ABI: append only.
const (
	OpKaOpen   kcabi.Op = 10
	OpKaUpdate kcabi.Op = 11
	OpKaClose  kcabi.Op = 12
)

// handles are the sessions and models the JVM holds.
var handles kcabi.Handles

func init() {
	kcabi.Register(OpKaOpen, open)
	kcabi.Register(OpKaUpdate, update)
	kcabi.Register(OpKaClose, closeHandle)
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
	var w wire.Writer
	w.Int(int64(handles.Put(m)))
	w.Uint(uint64(len(m.Files)))
	for _, f := range m.Files {
		w.String(f.Path)
		w.String(f.Package)
		writeAnnotations(&w, f.Annotations)
		writeDecls(&w, f.Decls)
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
	for i, on := range [...]bool{t.Nullable, t.Flexible, t.DefinitelyNonNull, t.Extension} {
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
	w.Uint(uint64(len(a.Args)))
	for _, arg := range a.Args {
		w.String(arg.Name)
		writeValue(w, arg.Value)
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
	handles.Delete(h)
	return nil, nil
}
