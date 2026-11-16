// Package lintops registers libkc's linter operations (internal/kcabi) on
// package analysis: a JVM linter such as detekt has kc check a module and
// reads the facts it resolved, file by file, keyed by psi node. The
// layouts below are mirrored by jvm/kc-detekt's KcFacts.
//
// OpLintAnalyze: config n:Uint path:String*n → result
//
//	config = jdkHome:String noJDK:Bool moduleName:String jobs:Uint
//	         n:Uint classPath:String*n m:Uint friendPath:String*m
//	         k:Uint optIn:String*k
//	result = handle:Int n:Uint diag*n m:Uint (nodes:Uint kinds:Uint)*m
//	diag   = path:String line:Uint col:Uint code:String message:String
//
// It parses and checks the files as one module (analysis.Analyze) and
// returns a handle to the result, its diagnostics, and for each path, in
// order, the number of its psi nodes and the hash of their kinds in
// preorder (psi.Snapshot.KindSequenceHash), which the JVM checks its PSI
// against. A result with diagnostics has no facts.
//
// OpLintFile: handle:Int file:Uint → types symbols facts
//
//	types   = n:Uint type*n
//	type    = name:String classID:String tflags:Uint owner:elem projection:Uint
//	          n:Uint arg:Uint*n m:Uint supertype:String*m
//	symbols = n:Uint symbol*n
//	symbol  = id:Uint kind:Uint name:String qualifiedName:String owner:String
//	          type:opt declaration:elem n:Uint supertype:String*n sflags:Uint
//	facts   = n:Uint fact*n
//	fact    = node:Uint mask:Uint field*
//	call    = name:String ownerClassID:String declaration:elem cflags:Uint
//	          declaredReceiver:opt receiver:opt implicitReceiver:elem n:Uint param*n
//	          m:Uint argument:Int*m returnType:opt declaredReturnType:opt
//	          typeParameters:Uint k:Uint typeParameterName:String*k
//	          annotations ownerAnnotations packageAnnotations
//	          j:Uint target:call*j
//	param   = type:Uint name:String pflags:Uint
//	elem    = file:Uint [node:Uint]
//	opt     = Uint
//
// It returns the facts of the file at index file of the analyzed paths,
// in node order, with the types they use and the symbols they name. A
// type's arguments are indices of types before it; an opt is 0 for none
// or a type's index plus one. An elem's file is 0 for none, and
// otherwise the index of a file plus one, followed by the node.
// annotations are n:Uint name:String*n.
//
// A fact's mask says which fields follow, in this order: 1 role:Uint
// (1 declaration, 2 reference, 3 expression), 2 type:Uint, 4 symbol:Uint,
// 8 call, 16 builtin:String, 32 importableName:String, 64 package:String,
// 128 when (exhaustive:Bool category:String n:Uint missing:String*n),
// 256 nonNullLeftSubtypeRight:Bool, 512 lambdaParameters (n:Uint
// symbol:Uint*n), 1024 returnTarget:elem, 2048 receiverTarget:elem,
// 4096 implicitReceiver:elem, 8192 delegateType:Uint, 16384
// delegatedCalls (n:Uint call*n), 32768 accessors (n:Uint call*n),
// 65536 deprecated:String, 131072 flags:Uint.
//
// The bit sets: tflags 1 nullable, 2 marked nullable, 4 platform,
// 8 parameter, 16 SAM, 32 suspend; projection 0 none, 1 in, 2 out, 3 star;
// a symbol's kind 1 class, 2 function, 3 constructor, 4 property, 5 local,
// 6 parameter; sflags 1 shadowed, 2 has abstract members, 4 has concrete
// members, 8 has an abstract superclass; cflags 1 extension, 2 suspend,
// 4 inline, 8 operator, 16 from Java, 32 stable names, 64 value
// invocation; pflags 1 vararg, 2 crossinline, 4 noinline; a fact's flags
// 1 classifier, 2 flow non-null, 4 definitely non-null, 8 impossible
// cast, 16 used as expression, 32 unreachable, 64 suspend operation,
// 128 implicit parameter.
//
// OpLintClose: handle:Int → nothing. It releases a result.
package lintops

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/kcabi"
	"github.com/barteo/kotlin-compiler-go/internal/kcabi/wire"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// The operations. The numbers are part of the ABI: append only.
const (
	OpLintAnalyze kcabi.Op = 20
	OpLintFile    kcabi.Op = 21
	OpLintClose   kcabi.Op = 22
)

// handles are the results the JVM holds.
var handles kcabi.Handles

func init() {
	kcabi.Register(OpLintAnalyze, analyze)
	kcabi.Register(OpLintFile, file)
	kcabi.Register(OpLintClose, closeResult)
}

// result is an analysis and its facts and symbols, split by file.
type result struct {
	facts   [][]analysis.Fact // by file
	symbols []analysis.Symbol // by ID-1
	files   map[psi.SnapshotID]int
}

func analyze(req []byte) ([]byte, error) {
	r := wire.NewReader(req)
	var cfg analysis.Config
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
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		cfg.OptIns = append(cfg.OptIns, r.String())
	}
	var paths []string
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		paths = append(paths, r.String())
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	snaps := make([]*psi.Snapshot, len(paths))
	errs := make([]error, len(paths))
	parallel(len(paths), max(cfg.Jobs, 1), func(i int) {
		b, err := os.ReadFile(paths[i])
		if err != nil {
			errs[i] = err
			return
		}
		snaps[i] = psi.Parse(paths[i], string(b))
	})
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	res, err := analysis.Analyze(context.Background(), cfg, snaps)
	if err != nil {
		return nil, err
	}
	data := res.Data()
	out := &result{facts: make([][]analysis.Fact, len(snaps)), symbols: data.Symbols, files: map[psi.SnapshotID]int{}}
	for i, s := range snaps {
		out.files[s.ID()] = i
	}
	for _, f := range data.Facts {
		if i, ok := out.files[f.Element.Snapshot]; ok {
			out.facts[i] = append(out.facts[i], f)
		}
	}
	var w wire.Writer
	w.Int(int64(handles.Put(out)))
	diags := res.Diagnostics()
	w.Uint(uint64(len(diags)))
	for _, d := range diags {
		w.String(d.Path)
		w.Uint(uint64(max(d.Line, 0)))
		w.Uint(uint64(max(d.Column, 0)))
		w.String(d.Code)
		w.String(d.Message)
	}
	w.Uint(uint64(len(snaps)))
	for _, s := range snaps {
		w.Uint(uint64(s.NodeCount()))
		w.Uint(s.KindSequenceHash())
	}
	return w.Message(), nil
}

func closeResult(req []byte) ([]byte, error) {
	r := wire.NewReader(req)
	h := kcabi.Handle(r.Int())
	if err := r.Err(); err != nil {
		return nil, err
	}
	handles.Delete(h)
	return nil, nil
}

func file(req []byte) ([]byte, error) {
	r := wire.NewReader(req)
	h := kcabi.Handle(r.Int())
	i := int(r.Uint())
	if err := r.Err(); err != nil {
		return nil, err
	}
	res, err := kcabi.Get[*result](&handles, h)
	if err != nil {
		return nil, err
	}
	if i < 0 || i >= len(res.facts) {
		return nil, errBadFile(i)
	}
	return encodeFile(res, res.facts[i]), nil
}

type errBadFile int

func (e errBadFile) Error() string { return "lintops: no file " + strconv.Itoa(int(e)) }

// encoder writes a file's facts. Types are written to their own table
// first, so the facts, calls and symbols are encoded into body and refer
// to types by index.
type encoder struct {
	res   *result
	types wire.Writer
	nType int
	index map[string]int // type key → index
	body  wire.Writer
}

func encodeFile(res *result, facts []analysis.Fact) []byte {
	e := &encoder{res: res, index: map[string]int{}}
	// The symbols the facts name.
	seen := map[uint32]bool{}
	var syms []uint32
	add := func(id uint32) {
		if id != 0 && !seen[id] {
			seen[id] = true
			syms = append(syms, id)
		}
	}
	for i := range facts {
		add(facts[i].Symbol)
		for _, id := range facts[i].LambdaParameters {
			add(id)
		}
	}
	var symw wire.Writer
	symw.Uint(uint64(len(syms)))
	for _, id := range syms {
		e.symbol(&symw, id)
	}
	e.body.Uint(uint64(len(facts)))
	for i := range facts {
		e.fact(&facts[i])
	}
	var w wire.Writer
	w.Uint(uint64(e.nType))
	out := append(w.Message(), e.types.Message()...)
	out = append(out, symw.Message()...)
	return append(out, e.body.Message()...)
}

var symbolKinds = map[string]uint64{"class": 1, "function": 2, "constructor": 3, "property": 4, "local": 5, "parameter": 6}

func (e *encoder) symbol(w *wire.Writer, id uint32) {
	var s analysis.Symbol
	if int(id) <= len(e.res.symbols) {
		s = e.res.symbols[id-1]
	}
	w.Uint(uint64(id))
	w.Uint(symbolKinds[s.Kind])
	w.String(s.Name)
	w.String(s.QualifiedName)
	w.String(s.Owner)
	if s.Kind == "" {
		w.Uint(0)
	} else {
		w.Uint(e.opt(&s.Type))
	}
	e.elem(w, s.Declaration)
	strs(w, s.Supertypes)
	w.Uint(bits(s.Shadowed, s.HasAbstractMembers, s.HasConcreteMembers, s.HasAbstractSuperclass))
}

func (e *encoder) fact(f *analysis.Fact) {
	w := &e.body
	var mask uint64
	set := func(bit int, on bool) {
		if on {
			mask |= 1 << bit
		}
	}
	role := map[string]uint64{"declaration": 1, "reference": 2, "expression": 3}[f.Role]
	flags := bits(f.Classifier, f.FlowNonNull, f.DefinitelyNonNull, f.ImpossibleCast, f.UsedAsExpression, f.Unreachable, f.SuspendOperation, f.ImplicitParameter)
	set(0, role != 0)
	set(1, f.Type != nil)
	set(2, f.Symbol != 0)
	set(3, f.Call != nil)
	set(4, f.Builtin != "")
	set(5, f.ImportableName != "")
	set(6, f.Package != "")
	set(7, f.When != nil)
	set(8, f.NonNullLeftSubtypeRight != nil)
	set(9, len(f.LambdaParameters) > 0)
	set(10, f.ReturnTarget != nil)
	set(11, f.ReceiverTarget != nil)
	set(12, f.ImplicitReceiver != nil)
	set(13, f.DelegateType != nil)
	set(14, len(f.DelegatedCalls) > 0)
	set(15, len(f.Accessors) > 0)
	set(16, f.Deprecated != nil)
	set(17, flags != 0)
	// Types first: the type table must be complete before the body
	// refers to it, and interning is idempotent, so the fact's types are
	// interned as they are written.
	w.Uint(uint64(f.Element.Node))
	w.Uint(mask)
	if role != 0 {
		w.Uint(role)
	}
	if f.Type != nil {
		w.Uint(uint64(e.typ(f.Type)))
	}
	if f.Symbol != 0 {
		w.Uint(uint64(f.Symbol))
	}
	if f.Call != nil {
		e.call(f.Call)
	}
	if f.Builtin != "" {
		w.String(f.Builtin)
	}
	if f.ImportableName != "" {
		w.String(f.ImportableName)
	}
	if f.Package != "" {
		w.String(f.Package)
	}
	if f.When != nil {
		w.Bool(f.When.Exhaustive)
		w.String(f.When.Category)
		strs(w, f.When.Missing)
	}
	if f.NonNullLeftSubtypeRight != nil {
		w.Bool(*f.NonNullLeftSubtypeRight)
	}
	if len(f.LambdaParameters) > 0 {
		w.Uint(uint64(len(f.LambdaParameters)))
		for _, id := range f.LambdaParameters {
			w.Uint(uint64(id))
		}
	}
	if f.ReturnTarget != nil {
		e.elem(w, f.ReturnTarget)
	}
	if f.ReceiverTarget != nil {
		e.elem(w, f.ReceiverTarget)
	}
	if f.ImplicitReceiver != nil {
		e.elem(w, f.ImplicitReceiver)
	}
	if f.DelegateType != nil {
		w.Uint(uint64(e.typ(f.DelegateType)))
	}
	if len(f.DelegatedCalls) > 0 {
		w.Uint(uint64(len(f.DelegatedCalls)))
		for i := range f.DelegatedCalls {
			e.call(&f.DelegatedCalls[i])
		}
	}
	if len(f.Accessors) > 0 {
		w.Uint(uint64(len(f.Accessors)))
		for i := range f.Accessors {
			e.call(&f.Accessors[i])
		}
	}
	if f.Deprecated != nil {
		w.String(*f.Deprecated)
	}
	if flags != 0 {
		w.Uint(flags)
	}
}

func (e *encoder) call(c *analysis.Call) {
	w := &e.body
	w.String(c.Name)
	w.String(c.OwnerClassID)
	e.elem(w, c.Declaration)
	w.Uint(bits(c.Extension, c.Suspend, c.Inline, c.Operator, c.FromJava, c.StableNames, c.ValueInvocation))
	w.Uint(e.opt(c.DeclaredReceiver))
	w.Uint(e.opt(c.Receiver))
	e.elem(w, c.ImplicitReceiver)
	w.Uint(uint64(len(c.Parameters)))
	for i := range c.Parameters {
		w.Uint(uint64(e.typ(&c.Parameters[i])))
		w.String(at(c.ParameterNames, i))
		w.Uint(bits(atBool(c.Varargs, i), atBool(c.Crossinline, i), atBool(c.Noinline, i)))
	}
	w.Uint(uint64(len(c.ArgumentMapping)))
	for _, p := range c.ArgumentMapping {
		w.Int(int64(p))
	}
	w.Uint(e.opt(c.ReturnType))
	w.Uint(e.opt(c.DeclaredReturnType))
	w.Uint(uint64(max(c.TypeParameters, 0)))
	strs(w, c.TypeParameterNames)
	strs(w, c.Annotations)
	strs(w, c.OwnerAnnotations)
	strs(w, c.PackageAnnotations)
	w.Uint(uint64(len(c.Targets)))
	for i := range c.Targets {
		e.call(&c.Targets[i])
	}
}

// opt returns 0 for no type, or the index of t plus one.
func (e *encoder) opt(t *analysis.Type) uint64 {
	if t == nil {
		return 0
	}
	return uint64(e.typ(t)) + 1
}

// typ returns the index of t in the type table, adding it and its
// arguments if they aren't there yet.
func (e *encoder) typ(t *analysis.Type) int {
	key := typeKey(t)
	if i, ok := e.index[key]; ok {
		return i
	}
	args := make([]int, len(t.Arguments))
	for i := range t.Arguments {
		args[i] = e.typ(&t.Arguments[i])
	}
	w := &e.types
	w.String(t.Name)
	w.String(t.ClassID)
	w.Uint(bits(t.Nullable, t.MarkedNullable, t.Platform, t.Parameter, t.SAM, t.Suspend))
	e.elem(w, t.ParameterOwner)
	w.Uint(map[string]uint64{"in": 1, "out": 2, "*": 3}[t.Projection])
	w.Uint(uint64(len(args)))
	for _, a := range args {
		w.Uint(uint64(a))
	}
	strs(w, t.Supertypes)
	i := e.nType
	e.nType++
	e.index[key] = i
	return i
}

func typeKey(t *analysis.Type) string {
	var b strings.Builder
	var walk func(t *analysis.Type)
	walk = func(t *analysis.Type) {
		b.WriteString(t.Name)
		b.WriteByte(0)
		b.WriteString(t.ClassID)
		b.WriteByte(0)
		b.WriteString(strconv.FormatUint(bits(t.Nullable, t.MarkedNullable, t.Platform, t.Parameter, t.SAM, t.Suspend), 10))
		if o := t.ParameterOwner; o != nil {
			b.WriteString("@" + strconv.Itoa(int(o.Snapshot)) + ":" + strconv.Itoa(int(o.Node)))
		}
		b.WriteString(t.Projection)
		b.WriteByte('<')
		for i := range t.Arguments {
			walk(&t.Arguments[i])
			b.WriteByte(',')
		}
		b.WriteByte('>')
		for _, s := range t.Supertypes {
			b.WriteString(s)
			b.WriteByte(';')
		}
	}
	walk(t)
	return b.String()
}

// elem writes the element id names: its file's index plus one and its
// node, or 0 for none or an element outside the analyzed files.
func (e *encoder) elem(w *wire.Writer, id *analysis.ElementID) {
	i, ok := 0, false
	if id != nil {
		i, ok = e.res.files[id.Snapshot]
	}
	if !ok {
		w.Uint(0)
		return
	}
	w.Uint(uint64(i) + 1)
	w.Uint(uint64(id.Node))
}

func strs(w *wire.Writer, ss []string) {
	w.Uint(uint64(len(ss)))
	for _, s := range ss {
		w.String(s)
	}
}

// bits returns a bit set with bit i set for each true bs[i].
func bits(bs ...bool) uint64 {
	var x uint64
	for i, b := range bs {
		if b {
			x |= 1 << i
		}
	}
	return x
}

func at(ss []string, i int) string {
	if i < len(ss) {
		return ss[i]
	}
	return ""
}

func atBool(bs []bool, i int) bool { return i < len(bs) && bs[i] }

func parallel(n, jobs int, fn func(i int)) {
	if n == 0 {
		return
	}
	jobs = min(jobs, n)
	next := make(chan int)
	done := make(chan struct{})
	for range jobs {
		go func() {
			for i := range next {
				fn(i)
			}
			done <- struct{}{}
		}()
	}
	for i := range n {
		next <- i
	}
	close(next)
	for range jobs {
		<-done
	}
}
