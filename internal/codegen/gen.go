// Package codegen generates JVM class files from checked syntax trees.
package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf16"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Output is one generated class.
type Output struct {
	Name  string // JVM internal name
	Bytes []byte
	// Part is set on a file's part of a multi-file class: the class, from
	// which MultifileFacade writes the facade's methods.
	Part *cf.Class
}

type loopLabels struct {
	node      syntax.Stmt // the *syntax.While or *syntax.For
	brk, cont *cf.Label
	depth     int // the stack depth in the loop: a break from an inlined lambda leaves values above it
	groups    int // the number of replace groups open around the loop, which jumps out of its body end
}

type gen struct {
	file    *syntax.File
	cls     *cf.Class
	code    *cf.Code
	owner   *types.ClassSym // class whose members are being generated
	retType types.Type
	loops   []loopLabels
	tries   []*tryCtx       // the try blocks the code is in, innermost last
	lam     *lambdaCtx      // set in a lambda's method
	inl     []*inlineLambda // the inlined lambdas being generated, innermost last
	// publicInline is set in the body of an inline function that isn't
	// private, which other files' copies call private members from
	// through their accessors.
	publicInline bool
	// inlineSingletons is set in the methods of the ComposableSingletons
	// lambdas of public inline functions: see compatSingleton.
	inlineSingletons bool
	// reifiedUse, if set, is set when the code marks an operation on a
	// reified type parameter: the lambda class being generated needs
	// copies with the call's type arguments.
	reifiedUse *bool
	// lambdaReified is set in the method of a lambda in a copy of the
	// program's inline function: the copy's type arguments for its reified
	// type parameters.
	lambdaReified types.Subst
	// valueThis is set in the static methods of a value class's members,
	// whose this is the underlying value, in local valueSelf: 0, but for
	// a secondary constructor's body.
	valueThis bool
	valueSelf int
	// valueCtor is set in the body of a value class's secondary
	// constructor, whose return returns this.
	valueCtor bool
	// objThis is set in static code whose this is the object owner: an
	// object's static initializer and @JvmStatic members.
	objThis bool
	// companionInits is set in the outer class's static initializer
	// while it runs a companion's initializers (initsInOuter), which set
	// the companion's fields directly, as its constructor would.
	companionInits bool
	// implsThis is set in the static methods of an interface's
	// DefaultImpls class, whose this, the interface owner, is in local 0,
	// and in the serialization plugin's write$Self.
	implsThis bool
	// self is the local of this in other code: 0, but in the
	// name$default method of a ComposeDefaultImpls class, which takes the
	// object after the parameters (see composeDefaults).
	self int
	// fieldParams is set in the serialization plugin's synthetic
	// constructor: the primary constructor's parameters, which default
	// values, initializers and init blocks read, are their properties'
	// fields there.
	fieldParams map[*types.VarSym]*types.VarSym
	// srcInl are the copies of the program's inline functions being
	// generated, innermost last.
	srcInl []*srcInline
	// reified are the classes of the reified type parameters of the
	// stdlib's inline function being copied, by name, and reifiedTypes
	// their types, for typeOf.
	reified      map[string]string
	reifiedTypes map[string]types.Type
	prefix       string // lambda methods are named prefix$0, prefix$1, ...
	// noLines is positive in the copy of an inline function, whose code
	// keeps the line of the call: line numbers aren't recorded.
	noLines int
	shared  *fileState
	// sm is set in a suspend function's method, and resume, before it
	// is generated, to the continuation's call of the function.
	sm     *stateMachine
	resume *resumeCall
	// tail is set in a tailrec function's method: its calls of itself
	// marked Tail jump back to start.
	tail *tailrec
	// compose is set in a composable function's method.
	compose *composeFn
	// erasedCast is the last checkcast of a value read as a type
	// parameter's erasure, which uncastErased takes back.
	erasedCast erasedCast
}

// tailrec is the state of a tailrec function: tail calls of fd store
// their arguments in its parameters and jump to start.
type tailrec struct {
	fd    *syntax.FunDecl
	start *cf.Label
}

// resumeCall is how a suspend function's continuation calls it: on the
// function's this, if this is set, with its arguments on the stack.
type resumeCall struct {
	this bool
	// thisParam is set on a static method whose first parameter is the
	// this the continuation keeps: name$suspendImpl.
	thisParam bool
	// forInline is set on the name$$forInline method of an inline
	// suspend function, which isn't a state machine.
	forInline bool
	// cont is the continuation class that the name$$forInline method of
	// an object expression's suspend member creates, between markers, and
	// public is set on the member whose continuation class it is: see
	// memberForInline.
	cont   string
	public bool
	// lambda is set on the method of a suspend fun interface that a
	// lambda implements, whose body is the method's.
	lambda *syntax.Lambda
	call   func(*gen)
}

// fileState is shared by the generators of a file's classes.
type fileState struct {
	opts        Options
	inlineDepth int       // see inlineMu
	renamed     []renamed // see nameObjects
	copies      map[*types.ClassSym]*objectCopy
	count       map[string]int // lambdas named with each class and prefix
	// lambdasIn are the lambdas of the inline functions the file copies,
	// by function: see declaredInCopy.
	lambdasIn map[*syntax.FunDecl]map[*syntax.Lambda]bool
	// classes are the file's classes, written once all are generated:
	// a class's static accessors of private properties are added when
	// its nested classes use them.
	classes map[string]*cf.Class
	// access records the private properties whose accessors other
	// classes call, class or top-level properties: 1 for the getter, 2
	// for the setter.
	access map[*types.VarSym]uint8
	// impls are the DefaultImpls classes of the interfaces compiled
	// without JVM default methods, by interface.
	impls map[*types.ClassSym]*cf.Class
	// begun are the DefaultImpls classes begun before their members, by
	// interface: those that hold its properties' $annotations methods.
	begun map[*types.ClassSym]*cf.Class
	// methods are the private methods whose accessors other classes call.
	methods map[privateMethod]bool
	// annotated are the properties with annotations for their property
	// target: the name and descriptor of their name$annotations method.
	annotated map[*types.VarSym][2]string
	// markers are the private constructors that other classes call
	// through marker constructors.
	markers map[*types.FunSym]bool
	// companions are the private companion objects that other classes
	// than their outer class get through its access$getCompanion$p.
	companions map[*types.ClassSym]bool
	// extra are classes copied from the class path for inlined calls,
	// with the calls' type arguments.
	extra []Output
	// later are the classes generated with the methods they are for:
	// the continuation classes of suspend functions, and the classes that
	// implement annotations, which annImpls names by annotation class.
	later    []string
	annImpls map[*types.ClassSym]string
	// samImpls are the classes that wrap function values as fun
	// interfaces, by interface: see sam.go.
	samImpls map[samKey]string
	// stabilities are the stabilities of types in the file's code, and
	// fileClasses its classes: see stability.go.
	stabilities map[types.Type]stability
	fileDecls   map[any]bool
	// stableInits are the values of the $stable fields of the file's
	// classes that their static initializers assign: see stableField.
	stableInits map[*syntax.ClassDecl]func()
	// singletons is the file's ComposableSingletons class, once a
	// composable lambda needs it: see singletons.go.
	singletons *composeSingletons
}

// privateMethod is a method that other classes call through a static
// accessor of class: a private method of the program, by its JVM name
// and descriptor and whether it is static, or one the accessor calls on
// an instance of class: a protected member of a superclass in another
// package (virtual), or super's method (super, an interface if iface).
type privateMethod struct {
	class, name, desc string
	static, virtual   bool
	super             string
	iface             bool
}

// Options are what code generation needs to know about the compilation.
type Options struct {
	// Stdlib is set when the Kotlin standard library is on the class
	// path: null checks call kotlin.jvm.internal.Intrinsics.
	Stdlib bool
	// Module is the module's name ("main" if empty).
	Module string
	// Spilling is set when the standard library has
	// SpillingKt.nullOutSpilledVariable, with which state machines store
	// dead references, and not in debug mode (kotlinc's -Xdebug), where
	// they are stored as they are.
	Spilling bool
	// Compose is set to compile as kotlinc does with the Compose compiler
	// plugin: see compose.go. ShouldExecute is set if the Compose runtime's
	// Composer has shouldExecute, which skippable composables ask whether
	// to run their bodies; older runtimes' have skipping only.
	Compose, ShouldExecute bool
	// JVMDefaultCompat is set to compile as kotlinc's -jvm-default=enable
	// does: classes get bridges to the default methods they inherit from
	// Kotlin interfaces. See defaultBridges.
	JVMDefaultCompat bool
	// Cache is shared by the files of a compilation, which compute what
	// it holds once; if nil, each file has its own.
	Cache *Cache
}

// Cache holds what code generation computes once for a compilation and
// shares between its files. It belongs to one compilation: kept longer,
// it would keep the compilation's symbols. The zero value is ready to
// use.
type Cache struct {
	methods sync.Map // *types.ClassSym -> map[string]bool: see jvmMethods
}

func (g *gen) cache() *Cache { return g.shared.opts.Cache }

// GenerateFile generates the facade class (if the file has top-level
// functions or properties) and one class per class declaration.
func GenerateFile(f *syntax.File, opts Options) (out []Output, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s: code generation failed: %v", f.Path, r)
			if os.Getenv("KC_PANIC") != "" {
				err = fmt.Errorf("%w\n%s", err, debug.Stack())
			}
		}
	}()
	if opts.Module == "" {
		opts.Module = "main"
	}
	if opts.Cache == nil {
		opts.Cache = &Cache{}
	}
	g := &gen{file: f, shared: &fileState{opts: opts, count: map[string]int{}, classes: map[string]*cf.Class{}, access: map[*types.VarSym]uint8{},
		annotated: map[*types.VarSym][2]string{}, markers: map[*types.FunSym]bool{}, impls: map[*types.ClassSym]*cf.Class{},
		begun: map[*types.ClassSym]*cf.Class{}, methods: map[privateMethod]bool{}, companions: map[*types.ClassSym]bool{}, stabilities: map[types.Type]stability{},
		stableInits: map[*syntax.ClassDecl]func(){}}}
	if fc := g.facade(); fc != nil {
		g.shared.classes[fc.Name] = fc
		o := Output{Name: fc.Name} // written below
		if g.file.Part != "" {
			o.Part = fc
		}
		out = append(out, o)
	}
	for _, d := range f.Decls {
		if cd, ok := d.(*syntax.ClassDecl); ok {
			out = g.classes(cd, out)
		}
	}
	for _, cd := range f.Anon {
		if reifiedOwner(cd) != nil {
			// Copies in other files rename its class: see copyObject.
			g.lockInline()
			defer g.unlockInline()
			break
		}
	}
	out = g.anonClasses(out)
	for _, name := range g.shared.later {
		out = append(out, Output{Name: name, Bytes: nil})
	}
	out = g.finishSingletons(out)
	g.privateAccessors()
	g.companionAccessors()
	g.markerConstructors()
	g.innerClasses()
	g.writeMetadata()
	for i, o := range out {
		if c := g.shared.classes[o.Name]; c != nil && o.Bytes == nil {
			out[i].Bytes = c.Bytes()
		}
	}
	return append(out, g.shared.extra...), nil
}

// privateAccessors adds the static accessors of the private properties
// that other classes use, as kotlinc does: access$getX$p(owner) and
// access$setX$p(owner, value).
func (g *gen) privateAccessors() {
	ps := make([]*types.VarSym, 0, len(g.shared.access))
	for p := range g.shared.access {
		ps = append(ps, p)
	}
	owner := func(p *types.VarSym) string {
		if p.Kind == types.Global {
			return p.Facade
		}
		return p.Owner.Name
	}
	sort.Slice(ps, func(i, j int) bool {
		if owner(ps[i]) != owner(ps[j]) {
			return owner(ps[i]) < owner(ps[j])
		}
		return ps[i].Name < ps[j].Name
	})
	for _, p := range ps {
		c := g.shared.classes[owner(p)]
		if c == nil {
			continue // another file's class, whose file writes them
		}
		// A top-level property's are static, and have no receiver.
		ft := types.FieldType(p)
		self, d := "", desc(ft)
		get, put := byte(cf.GETSTATIC), byte(cf.PUTSTATIC)
		if p.Kind != types.Global && !p.StaticField() {
			// An object's field is static too, as kotlinc writes it.
			self, get, put = "L"+p.Owner.Name+";", cf.GETFIELD, cf.PUTFIELD
		}
		const acc = cf.AccPublic | cf.AccStatic | cf.AccFinal | cf.AccSynthetic
		if g.shared.access[p]&1 != 0 {
			code := c.NewMethod(acc, privateGetter(p), "("+self+")"+d)
			if self != "" {
				code.Local(cf.ALOAD, code.AllocLocal(self))
			}
			code.FieldInsn(get, owner(p), p.Name, d)
			if p.Lateinit {
				lateinitCheck(code, p.Name, g.shared.opts.Stdlib)
			}
			code.Op(returnOp(ft))
			if err := code.Err(); err != nil {
				panic(err)
			}
		}
		if g.shared.access[p]&2 != 0 {
			code := c.NewMethod(acc, privateSetter(p), "("+self+d+")V")
			if self != "" {
				code.Local(cf.ALOAD, code.AllocLocal(self))
			}
			code.Local(loadOp(ft), code.AllocLocal(d))
			code.FieldInsn(put, owner(p), p.Name, d)
			code.Op(cf.RETURN)
			if err := code.Err(); err != nil {
				panic(err)
			}
		}
	}
	ms := make([]privateMethod, 0, len(g.shared.methods))
	for m := range g.shared.methods {
		ms = append(ms, m)
	}
	sort.Slice(ms, func(i, j int) bool {
		a, b := ms[i], ms[j]
		if a.class != b.class {
			return a.class < b.class
		}
		if a.accessor() != b.accessor() {
			return a.accessor() < b.accessor()
		}
		return a.desc < b.desc
	})
	for _, m := range ms {
		c := g.shared.classes[m.class]
		if c == nil {
			continue // another file's class, whose file writes it
		}
		d, op := m.desc, byte(cf.INVOKESTATIC)
		if !m.static {
			d, op = "(L"+m.class+";"+d[1:], cf.INVOKESPECIAL
		}
		acc := uint16(cf.AccPublic | cf.AccStatic | cf.AccFinal | cf.AccSynthetic)
		inIface := c.Access&cf.AccInterface != 0
		if inIface {
			acc &^= cf.AccFinal // an interface's methods can't be final
		}
		code := c.NewMethod(acc, m.accessor(), d)
		for _, pd := range descParams(d) {
			code.Local(loadOpOf(pd), code.AllocLocal(pd))
		}
		switch {
		case m.virtual:
			code.Invoke(cf.INVOKEVIRTUAL, m.class, m.name, m.desc)
		case m.iface && m.super == "":
			// A private method of the interface.
			code.InvokeSpecialInterface(m.class, m.name, m.desc)
		case m.iface:
			code.InvokeSpecialInterface(m.super, m.name, m.desc)
		case m.super != "":
			code.Invoke(cf.INVOKESPECIAL, m.super, m.name, m.desc)
		default:
			code.Invoke(op, m.class, m.name, m.desc)
		}
		code.Op(returnOpOf(d[strings.IndexByte(d, ')')+1:]))
		if err := code.Err(); err != nil {
			panic(err)
		}
	}
}

// innerClasses gives the file's nested classes and the classes they are
// declared in the entries of the InnerClasses attribute that reflection
// reads, as kotlinc writes them: a nested class's own entry, with its
// simple name and the class it is in, and those of the classes it is in,
// and in the class it is in, those of the classes nested in it.
func (g *gen) innerClasses() {
	var walk func(cd *syntax.ClassDecl, chain []cf.InnerClassEntry)
	walk = func(cd *syntax.ClassDecl, chain []cf.InnerClassEntry) {
		outer := g.shared.classes[cd.Sym.Name]
		for _, m := range cd.Members {
			nd, ok := m.(*syntax.ClassDecl)
			if !ok || nd.Sym == nil {
				continue
			}
			inner := g.shared.classes[nd.Sym.Name]
			if inner == nil || outer == nil {
				continue
			}
			access := inner.Access &^ (cf.AccSuper | cf.AccPublic)
			if !nd.Sym.Inner {
				access |= cf.AccStatic
			}
			switch {
			case nd.EnumEntry:
			case nd.Visibility == syntax.Private:
				access |= cf.AccPrivate
			case nd.Visibility == syntax.Protected:
				access |= cf.AccProtected
			default:
				access |= cf.AccPublic
			}
			e := cf.InnerClassEntry{Inner: nd.Sym.Name, Outer: cd.Sym.Name, Simple: nd.Name, Access: access}
			outer.InnerClasses = append(outer.InnerClasses, e)
			c := append(chain[:len(chain):len(chain)], e)
			inner.InnerClasses = append(inner.InnerClasses, c...)
			walk(nd, c)
		}
	}
	for _, d := range g.file.Decls {
		if cd, ok := d.(*syntax.ClassDecl); ok && cd.Sym != nil {
			walk(cd, nil)
		}
	}
	// An object expression's or a local class is in its own entries and
	// its enclosing class's, without an outer class, and has the method
	// it is in.
	for _, cd := range g.file.Anon {
		c := g.shared.classes[cd.Sym.Name]
		if c == nil {
			continue
		}
		e := cf.InnerClassEntry{Inner: cd.Sym.Name, Access: cf.AccPublic | cf.AccStatic | cf.AccFinal}
		if cd.Local {
			e.Simple = cd.Name
			e.Access = c.Access&^cf.AccSuper | cf.AccStatic
		}
		c.InnerClasses = append(c.InnerClasses, e)
		if oc := g.shared.classes[cd.EnclosingCls]; oc != nil {
			oc.InnerClasses = append(oc.InnerClasses, e)
		}
		c.Enclosing = &cf.EnclosingMethod{Class: cd.EnclosingCls}
		switch f := cd.EnclosingFun; {
		case f == nil:
			if od := cd.EnclosingInit; od != nil {
				c.Enclosing.Name, c.Enclosing.Desc = "<init>", ctorDesc(od)
			}
		case f.Decl == nil:
		default:
			switch d := f.Decl.(type) {
			case *syntax.LocalFun:
				c.Enclosing.Name, c.Enclosing.Desc = d.Method, d.Desc // as kotlinc's: f$g
			case *syntax.SecondaryCtor:
				c.Enclosing.Name, c.Enclosing.Desc = "<init>", ctorJVMDesc(f)
			default:
				if !strings.HasPrefix(f.Name, "<") {
					c.Enclosing.Name, c.Enclosing.Desc = f.JVM(), funDesc(f)
				}
			}
		}
	}
}

// annotationClass generates an annotation class: an annotation interface
// whose abstract methods are its parameters, with their defaults.
func (g *gen) annotationClass(cd *syntax.ClassDecl) *cf.Class {
	cls := cd.Sym
	c := cf.NewClass(cf.AccPublic|cf.AccInterface|cf.AccAbstract|cf.AccAnnotation, cls.Name, "java/lang/Object")
	c.Interfaces = []string{"java/lang/annotation/Annotation"}
	c.SourceFile = filepath.Base(g.file.Path)
	for i, p := range cls.AnnotationParams {
		c.AddAbstractMethod(cf.AccPublic|cf.AccAbstract, p.Name, "()"+annotationJVM(p.Type))
		if i < len(cd.CtorParams) && cd.CtorParams[i].Default != nil {
			if ev, ok := annotationDefault(p.Type, cd.CtorParams[i].Default); ok {
				c.LastMethod().SetDefault(ev)
			}
		}
	}
	return c
}

// annotationJVM returns the descriptor of the type of an annotation's
// parameter on the JVM: a KClass is a java.lang.Class in an annotation.
func annotationJVM(t types.Type) string {
	return strings.ReplaceAll(desc(t), "Lkotlin/reflect/KClass;", "Ljava/lang/Class;")
}

// classes generates a class and the classes nested in it.
func (g *gen) classes(cd *syntax.ClassDecl, out []Output) []Output {
	c := g.class(cd)
	c.Line = cd.P.Line
	writeAnnotations(c.Annotate, cd.Annotations, all)
	if cd.Sym.Annotation {
		annotationClassMeta(c, cd)
	}
	if cd.Visibility == syntax.Private || cd.EnumEntry {
		c.Access &^= cf.AccPublic // as kotlinc's: its package uses it
	}
	g.shared.classes[c.Name] = c
	out = append(out, Output{Name: c.Name, Bytes: nil}) // written by GenerateFile
	if ci := g.shared.classes[c.Name+"$ComposeDefaultImpls"]; ci != nil {
		out = append(out, Output{Name: ci.Name, Bytes: nil})
		e := cf.InnerClassEntry{Inner: ci.Name, Outer: c.Name, Simple: "ComposeDefaultImpls", Access: cf.AccPublic | cf.AccStatic | cf.AccFinal}
		c.InnerClasses = append(c.InnerClasses, e)
		ci.InnerClasses = append(ci.InnerClasses, e)
	}
	if di := g.defaultImpls(cd, c); di != nil {
		di.Line = cd.P.Line
		g.shared.classes[di.Name] = di
		out = append(out, Output{Name: di.Name, Bytes: nil})
		e := cf.InnerClassEntry{Inner: di.Name, Outer: c.Name, Simple: "DefaultImpls", Access: cf.AccPublic | cf.AccStatic | cf.AccFinal}
		c.InnerClasses = append(c.InnerClasses, e)
		di.InnerClasses = append(di.InnerClasses, e)
	}
	for _, m := range cd.Members {
		if nd, ok := m.(*syntax.ClassDecl); ok {
			out = g.classes(nd, out)
		}
	}
	return out
}

// Descriptors and type helpers.

func desc(t types.Type) string {
	d, ok := types.Desc(t)
	if !ok {
		panic("no descriptor for type " + t.String())
	}
	return d
}

// funDesc is the descriptor of a function or method: the one read from the
// class file for Java methods. A function with context parameters takes
// them first, then an extension function its receiver.
func funDesc(f *types.FunSym) string {
	if f.Desc != "" {
		return f.Desc
	}
	if f.Ext || len(f.Contexts) > 0 {
		return "(" + leadDesc(f) + sigDesc(f)[1:]
	}
	return sigDesc(f)
}

// leadDesc returns the descriptors of the parameters a function takes
// before its value parameters: its context parameters and its extension
// receiver.
func leadDesc(f *types.FunSym) string {
	var sb strings.Builder
	for _, p := range f.Contexts {
		sb.WriteString(paramDesc(p.Type))
	}
	if f.Ext {
		sb.WriteString(paramDesc(f.Recv))
	}
	return sb.String()
}

// sigDesc returns the descriptor of a function of the program's
// parameters and result: a suspend function's takes a Continuation last
// and returns an Object, and a composable function's takes a Composer and
// ints.
func sigDesc(f *types.FunSym) string {
	d := methodDesc(f.Params, types.JVMRet(f))
	if f.Suspend {
		return types.SuspendDesc(d)
	}
	if f.Composable {
		return types.ComposeDesc(d, f.Changed+f.Defaults)
	}
	return d
}

// paramDesc is the descriptor of a parameter of type t: a Unit one is a
// kotlin.Unit, and a Nothing? one a java.lang.Void.
func paramDesc(t types.Type) string {
	d, ok := types.ParamDesc(t)
	if !ok {
		panic("no descriptor for type " + t.String())
	}
	return d
}

func methodDesc(params []*types.VarSym, ret types.Type) string {
	var sb strings.Builder
	sb.WriteByte('(')
	for _, p := range params {
		sb.WriteString(paramDesc(p.Type))
	}
	sb.WriteByte(')')
	r, ok := types.RetDesc(ret)
	if !ok {
		panic("no descriptor for type " + ret.String())
	}
	sb.WriteString(r)
	return sb.String()
}

// size is the number of stack/local slots a value of type t takes.
func size(t types.Type) int {
	t = types.Repr(t)
	switch {
	case t.K == types.Unit || t.K == types.Nothing || t.K == types.Invalid:
		return 0
	case !t.N && (t.K == types.Long || t.K == types.Double):
		return 2
	}
	return 1
}

func loadOp(t types.Type) byte {
	if t = types.Repr(t); t.IsPrimitive() {
		return loadOpOf(desc(t))
	}
	return cf.ALOAD
}

func storeOp(t types.Type) byte {
	if t = types.Repr(t); t.IsPrimitive() {
		return storeOpOf(desc(t))
	}
	return cf.ASTORE
}

func returnOp(t types.Type) byte {
	if t.K == types.Unit || t.K == types.Nothing {
		return cf.RETURN
	}
	if t = types.Repr(t); t.IsPrimitive() {
		return returnOpOf(desc(t))
	}
	return cf.ARETURN
}

func (g *gen) facade() *cf.Class {
	if !g.file.HasFacade() {
		return nil // type aliases alone have one: its metadata holds them
	}
	var funs []*syntax.FunDecl
	var props []*syntax.PropDecl
	for _, d := range g.file.Decls {
		switch d := d.(type) {
		case *syntax.FunDecl:
			funs = append(funs, d)
		case *syntax.PropDecl:
			props = append(props, d)
		}
	}
	access := uint16(cf.AccPublic | cf.AccFinal | cf.AccSuper)
	if g.file.Part != "" {
		// A part of a multi-file class, which its facade calls.
		access = cf.AccFinal | cf.AccSuper | cf.AccSynthetic
	}
	c := cf.NewClass(access, g.file.Class(), "java/lang/Object")
	c.SourceFile = filepath.Base(g.file.Path)
	g.cls, g.owner = c, nil
	writeAnnotations(c.Annotate, g.file.Annotations, site("file"))
	for _, d := range g.file.Decls {
		if ad, ok := d.(*syntax.TypeAliasDecl); ok {
			// A type alias's annotations, on its synthetic method
			// name$annotations, as kotlinc writes them.
			g.propertyAnnotations(c, ad.Name, nil, ad.Annotations, all, ad.Visibility == syntax.Private)
		}
	}
	// The fields that the static initializer assigns: all but those
	// of the consts kc computes, which are their ConstantValue.
	var inits []*syntax.PropDecl
	for _, p := range props {
		var recv *types.Type
		if p.Getter != nil && p.Getter.Sym.Ext {
			recv = &p.Getter.Sym.Recv
		}
		g.propAnnotations(c, p, recv, p.Visibility == syntax.Private)
		// As with kotlinc, a property's field is private, with a getter
		// and a setter, but for a const's and a @JvmField property's,
		// which are public, and a private property's, which has no
		// accessors. A property whose accessors are custom and don't
		// use field has none.
		sym := p.Sym
		ft := types.FieldType(sym)
		d := desc(ft)
		if p.Recv == nil && sym.HasField() {
			acc := uint16(cf.AccPrivate | cf.AccStatic)
			switch {
			case (sym.Const || sym.JvmField) && p.Visibility != syntax.Private:
				acc = cf.AccPublic | cf.AccStatic
			case sym.Lateinit && !sym.Private:
				acc = lateinitAccess(p) | cf.AccStatic
			}
			if !p.Mutable {
				acc |= cf.AccFinal
			}
			if v := uint16(0); sym.Const {
				if v = constPool(c, sym); v != 0 {
					c.AddConstField(acc, p.Name, d, v)
					g.fieldAnnotations(c.LastField(), p.Annotations, sym.Type, placement{site: "field", dflt: onField})
					continue
				}
			}
			c.AddField(acc|fieldModifiers(p.Annotations), p.Name, d)
			g.signField(c.LastField(), ft, p.Type, d, p.Mutable, p.Annotations, nil)
			g.fieldAnnotations(c.LastField(), p.Annotations, sym.Type, placement{site: "field", dflt: onField})
			if p.Init != nil {
				inits = append(inits, p)
			}
		}
		if p.DelegateOf != nil || sym.Const || sym.JvmField || sym.Private {
			continue
		}
		if p.Getter != nil {
			// A custom getter is a static method.
			g.accessorAnnotations(g.function(p.Getter, cf.AccPublic|cf.AccStatic|cf.AccFinal|accessorSynthetic(p.Annotations, "get")), p, "get")
		} else {
			g.code = c.NewMethod(cf.AccPublic|cf.AccStatic|cf.AccFinal|accessorSynthetic(p.Annotations, "get"), types.GetterJVM(sym), "()"+d)
			g.signAccessor(c.LastMethod(), ft, p.Type, "()"+d, p.Annotations, nil)
			g.defaultAccessorAnnotations(c.LastMethod(), p.Annotations, "get", sym.Type)
			g.code.Line(p.P.Line)
			g.code.FieldInsn(cf.GETSTATIC, c.Name, p.Name, d)
			g.lateinit(sym)
			g.code.Op(returnOp(ft))
			g.check()
		}
		if p.Setter != nil {
			g.accessorAnnotations(g.function(p.Setter, cf.AccPublic|cf.AccStatic|cf.AccFinal|accessorSynthetic(p.Annotations, "set")), p, "set")
		} else if p.Mutable && !sym.SetterPrivate {
			g.code = c.NewMethod(cf.AccPublic|cf.AccStatic|cf.AccFinal|accessorSynthetic(p.Annotations, "set"), types.SetterJVM(sym), "("+d+")V")
			g.signAccessor(c.LastMethod(), ft, p.Type, "("+d+")V", p.Annotations, nil)
			g.defaultAccessorAnnotations(c.LastMethod(), p.Annotations, "set", sym.Type)
			g.code.Line(p.P.Line)
			slot := g.code.AllocLocal(d)
			g.code.NameLocal(slot, "<set-?>")
			g.checkParam(slot, ft, "<set-?>")
			g.code.Local(loadOp(ft), slot)
			g.code.FieldInsn(cf.PUTSTATIC, c.Name, p.Name, d)
			g.code.Op(cf.RETURN)
			g.check()
		}
	}
	if len(inits) > 0 || len(g.file.KProps) > 0 {
		g.code = c.NewMethod(cf.AccStatic, "<clinit>", "()V")
		g.prefix = "_clinit_$lambda"
		g.retType = types.UnitType
		g.kprops(g.file.KProps)
		for _, p := range inits {
			if defaultInit(p.Sym, p.Init) {
				continue // as kotlinc, which keeps the empty initializer
			}
			g.line(p.P)
			ft := types.FieldType(p.Sym)
			g.exprAs(p.Init, ft)
			g.code.FieldInsn(cf.PUTSTATIC, g.file.Class(), p.Name, desc(ft))
		}
		g.code.Op(cf.RETURN)
		g.check()
	}
	for _, fd := range funs {
		g.function(fd, cf.AccPublic|cf.AccStatic|cf.AccFinal)
		if fd.Inline && fd.Sym.Suspend && !fd.Sym.Private {
			g.forInline(fd, cf.AccPrivate|cf.AccStatic|cf.AccFinal)
		}
		if hasDefaults(fd.Params) && !fd.Sym.Composable {
			g.defaults(fd, nil)
			g.overloads(fd, nil, cf.AccPublic|cf.AccStatic|cf.AccFinal)
		}
		if fd.Entry != "" {
			g.suspendMain(fd)
		} else if fd.Name == "main" && fd.Recv == nil && len(fd.Params) == 0 && fd.Sym.Ret.K == types.Unit && !fd.Sym.Suspend {
			// The JVM entry point delegates to the Kotlin main, as with kotlinc.
			g.code = c.NewMethod(cf.AccPublic|cf.AccStatic|cf.AccSynthetic, "main", "([Ljava/lang/String;)V")
			g.code.NameLocal(g.code.AllocLocal("[Ljava/lang/String;"), "args")
			g.code.Invoke(cf.INVOKESTATIC, g.file.Class(), "main", "()V")
			g.code.Op(cf.RETURN)
			g.check()
		}
	}
	return c
}

// declPos returns the position of a symbol's declaration: a property, a
// constructor's property parameter, a function or a class.
func declPos(d any) syntax.Pos {
	switch d := d.(type) {
	case *syntax.PropDecl:
		return d.P
	case *syntax.Param:
		return d.P
	case *syntax.CtorParam:
		return d.P
	case *syntax.FunDecl:
		return d.P
	case *syntax.ClassDecl:
		return d.P
	}
	return syntax.Pos{}
}

// synthetic returns ACC_SYNTHETIC for a declaration that Java code
// shouldn't see, as kotlinc marks them: one that is
// @Deprecated(level = HIDDEN) or @JvmSynthetic.
// accessorSynthetic returns the synthetic flag of a property's default
// getter or setter, target get or set, if its annotations as have
// @get:JvmSynthetic or @set:JvmSynthetic.
func accessorSynthetic(as []syntax.Annotation, target string) uint16 {
	for _, a := range as {
		if a.Target == target && a.Is("JvmSynthetic") {
			return cf.AccSynthetic
		}
	}
	return 0
}

func synthetic(m *syntax.Modifiers) uint16 {
	if m.Hidden() || m.Find("", "JvmSynthetic") != nil {
		return cf.AccSynthetic
	}
	return 0
}

// reifiedInline reports whether f is an inline function with reified
// type parameters, which kotlinc makes synthetic: only Kotlin code can
// call it, by inlining it.
func reifiedInline(fd *syntax.FunDecl) bool {
	if !fd.Inline || fd.Sym == nil {
		return false
	}
	for _, tp := range fd.Sym.TypeParams {
		if tp.Reified {
			return true
		}
	}
	return false
}

// inlineAccessor reports whether fd is an inline get or inline set, which
// compiles as an ordinary accessor that kotlinc copies.
func inlineAccessor(fd *syntax.FunDecl) bool {
	if fd.AccessorOf == nil {
		return false
	}
	pd, ok := fd.AccessorOf.Decl.(*syntax.PropDecl)
	return ok && (fd == pd.Getter && pd.InlineGetter || fd == pd.Setter && pd.InlineSetter)
}

// hasVararg reports whether a function's last parameter is a vararg one:
// its method is then ACC_VARARGS, for Java callers, as with kotlinc. Java
// rejects the flag on a method whose last parameter isn't an array.
func hasVararg(ps []*types.VarSym) bool {
	return len(ps) > 0 && ps[len(ps)-1].Vararg
}

// varargs reports whether f's method is a variable arity method, as Java
// sees it: its last parameter is a vararg, but for a composable's, which
// the Composer and change flags follow.
func varargs(f *types.FunSym) bool {
	return hasVararg(f.Params) && !f.Composable
}

func (g *gen) check() {
	if err := g.code.Err(); err != nil {
		panic(fmt.Errorf("%s.%s: %w", g.cls.Name, g.code.Name(), err))
	}
}

func (g *gen) function(fd *syntax.FunDecl, access uint16) cf.MemberRef {
	sym := fd.Sym
	if varargs(sym) {
		access |= cf.AccVarargs
	}
	access |= synthetic(&fd.Modifiers) | methodModifiers(fd.Annotations)
	if reifiedInline(fd) || sym.Hidden {
		access |= cf.AccSynthetic
	}
	if fd.AccessorOf != nil {
		// A custom accessor of a class's property: @JvmSynthetic get() is
		// the property's @get:JvmSynthetic.
		if pd, ok := fd.AccessorOf.Decl.(*syntax.PropDecl); ok {
			access |= accessorSynthetic(pd.Annotations, fd.Name)
		}
	}
	switch {
	case sym.Private && g.inInterface():
		access = access&^(cf.AccPublic|cf.AccProtected) | cf.AccPrivate // an interface's methods can't be final
	case sym.Private:
		access = access&^(cf.AccPublic|cf.AccProtected) | cf.AccPrivate | cf.AccFinal
	case fd.Inline && fd.Find("", "InlineOnly") != nil && access&cf.AccStatic != 0:
		// As kotlinc writes them: callers inline its body, and Java
		// can't call it.
		access = access&^(cf.AccPublic|cf.AccProtected) | cf.AccPrivate
	case (fd.Visibility == syntax.Protected || sym.Inherited == types.InheritProtected && !sym.PublicOnJVM) && access&cf.AccStatic == 0:
		access = access&^cf.AccPublic | cf.AccProtected
	}
	if fd.Inline {
		// Its variables' locals are shared with its copies: see inlineMu.
		g.lockInline()
		defer g.unlockInline()
	}
	if (fd.Inline || inlineAccessor(fd)) && !sym.Private || objectForInline(g.owner, fd) {
		// Copied by other modules: an object expression's suspend member
		// too, with its lambdas (see memberForInline).
		saved := g.publicInline
		g.publicInline = true
		defer func() { g.publicInline = saved }()
	}
	d := funDesc(sym)
	if g.implsThis {
		d = "(L" + g.owner.Name + ";" + d[1:] // a DefaultImpls body takes this first
	}
	if sym.External {
		return g.external(fd, access, d)
	}
	g.code = g.cls.NewMethod(access, sym.JVM(), d)
	m := g.cls.LastMethod()
	if access&cf.AccSynthetic == 0 {
		this := types.InvalidType
		if g.implsThis {
			this = g.owner.This()
		}
		g.signFun(m, sym, fd, d, this)
	}
	g.cls.Throws(sym.Throws...)
	if g.suspendImpl(fd, access) {
		g.annotateFun(m, fd, false)
		return m
	}
	if access&cf.AccStatic == 0 {
		g.code.AllocLocal("L" + g.cls.Name + ";") // this
	} else if g.implsThis {
		name := "$this"
		if fd.SelfStatic {
			name = "self"
		}
		g.code.NameLocal(g.code.AllocLocal("L"+g.owner.Name+";"), name)
	} else if g.owner != nil && g.owner.Object {
		g.objThis = true // a @JvmStatic member's
		defer func() { g.objThis = false }()
	}
	if sym.Suspend {
		// The continuation calls the function to resume it.
		facade, cls, name, d := g.cls.Name, g.owner, sym.JVM(), funDesc(sym)
		g.resume = &resumeCall{this: access&cf.AccStatic == 0, public: objectForInline(g.owner, fd), call: func(g *gen) {
			if access&cf.AccStatic != 0 {
				g.callStatic(sym, facade, name, d)
			} else {
				g.callMethod(sym, cls, name, d)
			}
		}}
	}
	g.functionBody(fd)
	g.annotateFun(m, fd, access&(cf.AccPrivate|cf.AccSynthetic) != 0 || g.owner != nil && g.owner.Anonymous)
	return m
}

// external generates external function fd, of descriptor d: a native
// method, without code. A companion's @JvmStatic one calls the outer
// class's static method, which is the native one, as kotlinc writes it:
// see staticStub.
func (g *gen) external(fd *syntax.FunDecl, access uint16, d string) cf.MemberRef {
	sym := fd.Sym
	var outer *types.ClassSym
	if g.owner != nil && fd.Find("", "JvmStatic") != nil {
		outer = g.owner.CompanionOf
	}
	if outer == nil {
		g.cls.AddAbstractMethod(access|cf.AccNative, sym.JVM(), d)
	} else {
		g.code = g.cls.NewMethod(access, sym.JVM(), d)
	}
	m := g.cls.LastMethod()
	if access&cf.AccSynthetic == 0 {
		g.signFun(m, sym, fd, d, types.InvalidType)
	}
	g.cls.Throws(sym.Throws...)
	if outer != nil {
		g.code.NameLocal(g.code.AllocLocal("L"+g.cls.Name+";"), "this")
		for i, pd := range descParams(d) {
			slot := g.code.AllocLocal(pd)
			if !sym.Ext {
				g.code.NameLocal(slot, sym.Params[i].Name)
			}
			g.code.Local(loadOpOf(pd), slot)
		}
		g.code.Invoke(cf.INVOKESTATIC, outer.Name, sym.JVM(), d)
		g.code.Line(fd.P.Line)
		g.code.Op(returnOpOf(d[strings.IndexByte(d, ')')+1:]))
		g.check()
	}
	g.annotateFun(m, fd, access&(cf.AccPrivate|cf.AccSynthetic) != 0 || g.owner != nil && g.owner.Anonymous)
	return m
}

// functionBody generates a function's parameters and body, in the method
// g.code, after its this.
func (g *gen) functionBody(fd *syntax.FunDecl) {
	sym := fd.Sym
	g.prefix = sym.Name + "$lambda"
	g.retType = types.JVMRet(sym)
	g.loops, g.tries = nil, nil
	for _, p := range sym.Contexts {
		p.Slot = g.code.AllocLocal(paramDesc(p.Type))
		g.named(p)
	}
	if r := fd.RecvSym; r != nil {
		r.Slot = g.code.AllocLocal(paramDesc(r.Type))
		g.named(r)
	}
	for _, p := range sym.Params {
		p.Slot = g.code.AllocLocal(paramDesc(p.Type))
		g.named(p)
	}
	outer := g.compose // of a function this one's class is local to
	defer func() { g.compose = outer }()
	g.compose = nil
	if sym.Composable {
		g.compose = g.composeParams(sym, sym.Owner != nil)
	}
	if sym.Suspend {
		// As kotlinc does, the parameters aren't checked: a continuation
		// passes nulls when it resumes the function.
		k := g.code.AllocLocal(types.ContinuationDesc)
		g.code.NameLocal(k, "$completion")
		resume := g.resume
		g.resume = nil
		if resume == nil {
			panic("suspend function " + sym.Name + " is not generated yet")
		}
		g.suspendFun(fd, g.code.Name(), g.code.Desc(), k, resume)
	} else if fd.Visibility != syntax.Private && !sym.Private {
		params := sym.Params
		if sym.Composable && !sym.ComposeDefaults {
			// The omitted arguments are nulls until the default values.
			params = nil
			for i, p := range sym.Params {
				if fd.Params[i].Default == nil {
					params = append(params, p)
				}
			}
		}
		g.checkParams(sym.Contexts, fd.RecvSym, params)
	}
	for _, i := range sym.BoxedParams {
		p := sym.Params[i]
		g.code.Local(cf.ALOAD, p.Slot)
		g.castBox(types.AnyType.Nullable(), p.Type)
		g.unboxValue(p.Type)
		g.code.Local(cf.ASTORE, p.Slot)
	}
	if g.owner != nil && (g.owner.Anonymous || g.owner.Inner || g.owner.Local) && sym.Owner == g.owner {
		if cd, ok := g.owner.Decl.(*syntax.ClassDecl); ok {
			g.loadCaptures(cd, 0)
		}
	}
	saved := g.tail // of a function this one is local to
	defer func() { g.tail = saved }()
	g.tail = nil
	if fd.Tailrec && !sym.Suspend {
		g.tail = &tailrec{fd: fd, start: cf.NewLabel()}
		g.code.Place(g.tail.start)
	}
	if g.compose != nil {
		g.composeStart(fd)
	}
	if fd.Body != nil {
		g.block(fd.Body, types.UnitType, false)
		if g.code.Reachable() {
			g.line(fd.Body.End)
			if g.compose == nil || g.compose.end == nil {
				g.ret(types.UnitType)
			} // else the end of the restart group follows
		}
	} else {
		g.line(syntax.StartPos(fd.ExprBody))
		g.exprAs(fd.ExprBody, types.JVMRet(sym))
		g.ret(types.JVMRet(sym))
	}
	g.composeEnd(fd)
	g.endSuspend()
	g.check()
	if c := g.compose; c != nil && c.restart != nil {
		c.restart()
	}
}

func (g *gen) class(cd *syntax.ClassDecl) *cf.Class {
	cls := cd.Sym
	if cls.Annotation {
		return g.annotationClass(cd)
	}
	if cls.Value {
		return g.valueClass(cd)
	}
	super := "java/lang/Object"
	if cls.Super != nil {
		super = cls.Super.Name
	}
	access := uint16(cf.AccPublic | cf.AccSuper)
	switch {
	case cls.Interface:
		access = cf.AccPublic | cf.AccInterface | cf.AccAbstract
	case cls.Entries != nil:
		// An enum whose entries have bodies isn't final, and one with
		// abstract members is abstract.
		access |= cf.AccEnum
		if cls.Abstract {
			access |= cf.AccAbstract
		} else if cls.Final {
			access |= cf.AccFinal
		}
	case cls.Abstract:
		access |= cf.AccAbstract
	case cls.Final:
		access |= cf.AccFinal
	}
	if cd.Synthetic {
		access |= cf.AccSynthetic
	}
	c := cf.NewClass(access, cls.Name, super)
	// As with kotlinc, the Java interfaces of Kotlin's collection types
	// come first, then their marker, then the other interfaces.
	for _, i := range cls.Interfaces {
		if i.Java != nil {
			c.Interfaces = append(c.Interfaces, i.Name)
		}
	}
	c.Interfaces = append(c.Interfaces, collectionMarkers(cls)...)
	marker := false
	for _, i := range cls.Interfaces {
		if i.Java == nil {
			c.Interfaces = append(c.Interfaces, i.Name)
		}
		if i.Func && i.Suspend && strings.HasPrefix(i.Name, types.KotlinFunctionPackage) && !marker {
			// A suspend function type's, with kotlinc's marker.
			c.Interfaces = append(c.Interfaces, "kotlin/coroutines/jvm/internal/SuspendFunction")
			marker = true
		}
	}
	c.Signature = classSignature(cls, super, c.Interfaces)
	c.SourceFile = filepath.Base(g.file.Path)
	g.cls, g.owner = c, cls
	for _, v := range cd.Captures {
		c.AddField(cf.AccFinal|cf.AccSynthetic, captureField(cd, v), storageDesc(v))
	}
	for _, st := range cd.Supers {
		if st.Delegate != nil && strings.HasPrefix(st.Field, "$$delegate_") {
			c.AddField(cf.AccPrivate|cf.AccFinal|cf.AccSynthetic, st.Field, desc(st.T))
		}
	}
	inClinit := initsInOuter(cd)
	for _, p := range cls.Props {
		if !p.HasField() || p.Const || p.StaticField() && p.ConstOwner() != cls {
			continue // a companion's @JvmField, which its outer class holds
		}
		g.propField(c, p, inClinit)
	}
	for _, p := range companionFields(cd) {
		g.propField(c, p, false)
	}
	// The static fields of the consts of an object, or of the class's
	// companion object, as kotlinc writes them: with their values, if
	// kc computes them, or else the static initializer assigns them.
	for _, pd := range constDecls(cd) {
		acc := uint16(cf.AccPublic | cf.AccStatic | cf.AccFinal)
		if v := constPool(c, pd.Sym); v != 0 {
			if pd.Visibility == syntax.Private {
				acc = cf.AccPrivate | cf.AccStatic | cf.AccFinal
			}
			c.AddConstField(acc, pd.Name, desc(pd.Sym.Type), v)
		} else {
			c.AddField(acc, pd.Name, desc(pd.Sym.Type))
		}
		g.fieldAnnotations(c.LastField(), pd.Annotations, pd.Sym.Type, placement{"field", onField})
	}
	g.stableField(c, cd)
	self := "L" + cls.Name + ";"
	if cls.Entries != nil {
		for _, e := range cd.Entries {
			c.AddField(cf.AccPublic|cf.AccStatic|cf.AccFinal|cf.AccEnum, e.Name, self)
			writeAnnotations(c.LastField().Annotate, e.Annotations, all)
		}
		c.AddField(cf.AccPrivate|cf.AccStatic|cf.AccFinal|cf.AccSynthetic, "$VALUES", "["+self)
		if cd.EntriesList {
			c.AddField(cf.AccPrivate|cf.AccStatic|cf.AccFinal|cf.AccSynthetic, "$ENTRIES", "Lkotlin/enums/EnumEntries;")
		}
		g.enumMethods(cd)
	}
	if !cls.Interface {
		if cd.HasPrimary {
			g.constructor(cd)
			params := make([]*syntax.Param, len(cd.CtorParams))
			for i, cp := range cd.CtorParams {
				params[i] = &cp.Param
			}
			if hasDefaults(params) {
				g.defaultConstructor(cd.Ctor, params)
				g.ctorOverloads(&cd.CtorMods, cd.Ctor, params)
				g.emptyConstructor(cd, params)
			}
		}
		for _, sc := range cd.Secondaries() {
			if sc.Serial != nil {
				g.serialCtor(cd, sc)
				continue
			}
			g.secondary(cd, sc)
			if hasDefaults(sc.Params) {
				g.defaultConstructor(sc.Sym, sc.Params)
				g.ctorOverloads(&sc.Modifiers, sc.Sym, sc.Params)
			}
		}
	}
	g.staticInit(cd)
	var bodies []*syntax.FunDecl // an interface's, for its DefaultImpls
	for _, p := range cls.Props {
		if _, ok := p.Decl.(*syntax.Delegated); ok {
			continue // with the delegated members, below
		}
		as, _, propSite := classPropAnnotations(p)
		if name, md := g.propertyAnnotations(g.annotationsHolder(c, cls), types.GetterJVM(p), nil, as, propSite, p.Private); name != "" {
			g.shared.annotated[p] = [2]string{name, md}
		}
		if isDelegateField(p.Decl) || p.Const || p.JvmField {
			continue // only a field
		}
		ft := types.FieldType(p)
		d := desc(ft)
		acc := uint16(cf.AccPublic)
		pd, _ := p.Decl.(*syntax.PropDecl)
		if protectedProp(p, false) {
			acc = cf.AccProtected
		}
		if p.Final {
			acc |= cf.AccFinal
		}
		if p.JvmStatic && cls.CompanionOf == nil {
			acc |= cf.AccStatic // an object's: see staticStubs for a companion's
		}
		if cls.HasDefaultImpls() && !p.Abstract && !p.Private && p.Getter != nil {
			// Abstract accessors, and their bodies in DefaultImpls.
			gd := "()" + desc(types.GetterType(p))
			c.AddAbstractMethod(acc&(cf.AccPublic|cf.AccProtected)|cf.AccAbstract, types.GetterJVM(p), gd)
			g.signAccessor(c.LastMethod(), types.GetterType(p), propRef(p), gd, as, cls)
			g.defaultAccessorAnnotations(c.LastMethod(), as, "get", p.Type)
			bodies = append(bodies, p.Getter.Decl.(*syntax.FunDecl))
			if p.Mutable && p.Setter != nil {
				c.AddAbstractMethod(acc&(cf.AccPublic|cf.AccProtected)|cf.AccAbstract, types.SetterJVM(p), "("+d+")V")
				g.signAccessor(c.LastMethod(), ft, propRef(p), "("+d+")V", as, cls)
				g.defaultAccessorAnnotations(c.LastMethod(), as, "set", p.Type)
				bodies = append(bodies, p.Setter.Decl.(*syntax.FunDecl))
			}
			continue
		}
		switch {
		case p.Abstract:
			// Taking the context parameters first, if any.
			ctx := ""
			for _, q := range p.Contexts {
				ctx += paramDesc(q.Type)
			}
			c.AddAbstractMethod(acc&(cf.AccPublic|cf.AccProtected)|cf.AccAbstract, types.GetterJVM(p), "("+ctx+")"+desc(types.GetterType(p)))
			g.signAccessor(c.LastMethod(), types.GetterType(p), propRef(p), "("+ctx+")"+desc(types.GetterType(p)), as, cls, p.Contexts...)
			g.defaultAccessorAnnotations(c.LastMethod(), as, "get", p.Type)
			if p.Mutable {
				c.AddAbstractMethod(acc&(cf.AccPublic|cf.AccProtected)|cf.AccAbstract, types.SetterJVM(p), "("+ctx+d+")V")
				g.signAccessor(c.LastMethod(), ft, propRef(p), "("+ctx+d+")V", as, cls, p.Contexts...)
				g.defaultAccessorAnnotations(c.LastMethod(), as, "set", p.Type)
			}
		case p.Private:
			// Only the field: the static accessors that other classes
			// use are added at the end (privateAccessors).
		default:
			if p.Getter != nil {
				m := g.function(p.Getter.Decl.(*syntax.FunDecl), acc)
				if pd != nil {
					g.accessorAnnotations(m, pd, "get")
				}
			} else {
				g.code = c.NewMethod(acc|accessorSynthetic(as, "get"), types.GetterJVM(p), "()"+desc(types.GetterType(p)))
				g.signAccessor(c.LastMethod(), types.GetterType(p), propRef(p), "()"+desc(types.GetterType(p)), as, cls)
				g.defaultAccessorAnnotations(c.LastMethod(), as, "get", p.Type)
				g.code.Line(declPos(p.Decl).Line)
				if acc&cf.AccStatic == 0 {
					g.code.AllocLocal(self)
				}
				if p.StaticField() {
					g.code.FieldInsn(cf.GETSTATIC, cls.Name, p.Name, d)
				} else {
					g.code.Local(cf.ALOAD, 0)
					g.code.FieldInsn(cf.GETFIELD, cls.Name, p.Name, d)
				}
				g.lateinit(p)
				g.coerce(ft, types.GetterType(p))
				g.code.Op(returnOp(types.GetterType(p)))
				g.check()
			}
			switch {
			case !p.Mutable || p.SetterPrivate:
				// private set has no setter: its class writes the field.
			case p.Setter != nil:
				m := g.function(p.Setter.Decl.(*syntax.FunDecl), acc)
				if pd != nil {
					g.accessorAnnotations(m, pd, "set")
				}
			default:
				if pd != nil && pd.SetterVis == syntax.Protected {
					acc = acc&^cf.AccPublic | cf.AccProtected
				}
				g.code = c.NewMethod(acc|accessorSynthetic(as, "set"), types.SetterJVM(p), "("+d+")V")
				g.signAccessor(c.LastMethod(), ft, propRef(p), "("+d+")V", as, cls)
				g.defaultAccessorAnnotations(c.LastMethod(), as, "set", p.Type)
				g.code.Line(declPos(p.Decl).Line)
				if acc&cf.AccStatic == 0 {
					g.code.AllocLocal(self)
				}
				slot := g.code.AllocLocal(d)
				g.code.NameLocal(slot, "<set-?>")
				if !p.Private {
					g.checkParam(slot, ft, "<set-?>")
				}
				if p.StaticField() {
					g.code.Local(loadOp(ft), slot)
					g.code.FieldInsn(cf.PUTSTATIC, cls.Name, p.Name, d)
				} else {
					g.code.Local(cf.ALOAD, 0)
					g.code.Local(loadOp(ft), slot)
					g.code.FieldInsn(cf.PUTFIELD, cls.Name, p.Name, d)
				}
				g.code.Op(cf.RETURN)
				g.check()
			}
		}
		for _, b := range p.Bridges {
			if !g.cache().collectionBridge(cls, b) && !cls.HasDefaultImpls() {
				g.bridge(b, types.GetterJVM(p), nil, types.GetterType(p), false, nil, nil)
			}
		}
		for _, b := range p.SetterBridges {
			if !cls.HasDefaultImpls() {
				g.bridge(b, types.SetterJVM(p), []*types.VarSym{{Name: "value", Type: types.FieldType(p), Kind: types.Param}}, types.UnitType, false, nil, nil)
			}
		}
	}
	if cd.Data {
		g.dataMembers(cd)
	}
	for _, m := range cd.Members {
		if pd, ok := m.(*syntax.PropDecl); ok && pd.Recv != nil && pd.Getter != nil {
			// An extension property's accessors, which take the receiver.
			g.propAnnotations(g.annotationsHolder(c, cls), pd, &pd.Getter.Sym.Recv, pd.Visibility == syntax.Private)
			for _, a := range []*syntax.FunDecl{pd.Getter, pd.Setter} {
				if a == nil {
					continue
				}
				kind := a.Name // get or set
				f := a.Sym
				var ref cf.MemberRef
				switch {
				case cls.HasDefaultImpls() && !f.Abstract:
					// An abstract method, and the body in DefaultImpls.
					if pd.Visibility != syntax.Private {
						c.AddAbstractMethod(cf.AccPublic|cf.AccAbstract|accessorSynthetic(pd.Annotations, kind), f.JVM(), funDesc(f))
						g.signFun(c.LastMethod(), f, a, funDesc(f), types.InvalidType)
						g.accessorAnnotations(c.LastMethod(), pd, kind)
					}
					bodies = append(bodies, a)
					continue
				case f.Abstract:
					acc := uint16(cf.AccPublic)
					if pd.Visibility == syntax.Protected {
						acc = cf.AccProtected
					}
					c.AddAbstractMethod(acc|cf.AccAbstract|accessorSynthetic(pd.Annotations, kind), f.JVM(), funDesc(f))
					ref = c.LastMethod()
					g.signFun(ref, f, a, funDesc(f), types.InvalidType)
				case f.Final:
					ref = g.function(a, cf.AccPublic|cf.AccFinal|accessorSynthetic(pd.Annotations, kind))
				default:
					ref = g.function(a, cf.AccPublic|accessorSynthetic(pd.Annotations, kind))
				}
				g.accessorAnnotations(ref, pd, kind)
				params := append([]*types.VarSym{{Name: "<this>", Type: f.Recv, Kind: types.Param}}, f.Params...)
				for _, b := range f.Bridges {
					g.bridge(b, f.JVM(), params, types.JVMRet(f), slices.Contains(f.BoxingBridges, b), f.BridgeOf[b], f)
				}
			}
			continue
		}
		fd, ok := m.(*syntax.FunDecl)
		if !ok {
			continue
		}
		f := fd.Sym
		if fd.SelfStatic {
			// The serialization plugin's write$Self: a static method that
			// takes this first.
			g.implsThis = true
			g.function(fd, cf.AccPublic|cf.AccStatic|cf.AccFinal|cf.AccSynthetic)
			g.implsThis = false
			continue
		}
		if cls.HasDefaultImpls() && !f.Abstract {
			// An abstract method, and the body in DefaultImpls.
			if fd.Visibility != syntax.Private {
				c.AddAbstractMethod(cf.AccPublic|cf.AccAbstract|synthetic(&fd.Modifiers), f.JVM(), funDesc(f))
				g.signFun(c.LastMethod(), f, fd, funDesc(f), types.InvalidType)
				g.annotateFun(c.LastMethod(), fd, false)
				c.Throws(f.Throws...)
			}
			bodies = append(bodies, fd)
			continue
		}
		access := uint16(cf.AccPublic)
		owner := cls
		if f.Static {
			// An object's @JvmStatic function: only a static method.
			access |= cf.AccStatic
			owner = nil
		}
		switch {
		case f.Abstract:
			acc := uint16(cf.AccPublic)
			if fd.Visibility == syntax.Protected || f.Inherited == types.InheritProtected && !f.PublicOnJVM {
				acc = cf.AccProtected
			}
			c.AddAbstractMethod(acc|cf.AccAbstract|synthetic(&fd.Modifiers), f.JVM(), funDesc(f))
			g.signFun(c.LastMethod(), f, fd, funDesc(f), types.InvalidType)
			g.annotateFun(c.LastMethod(), fd, false)
			c.Throws(f.Throws...)
		case f.Final:
			access |= cf.AccFinal
			g.function(fd, access)
		default:
			g.function(fd, access)
		}
		if fd.Inline && f.Suspend && !f.Abstract && !f.Static && !f.Private {
			g.forInline(fd, cf.AccPrivate|access&cf.AccFinal)
		}
		if objectForInline(cls, fd) {
			g.memberForInline(fd, access)
		}
		if hasDefaults(fd.Params) && !f.Composable {
			// A composable function takes the masks itself.
			g.defaults(fd, owner)
			g.overloads(fd, owner, access)
		}
		if f.ComposeDefaults {
			g.composeDefaults(fd)
			if !f.Abstract {
				g.composeDefaultsStub(fd)
			}
		}
		params := bridgeParams(f)
		for _, b := range f.Bridges {
			if !g.cache().collectionBridge(cls, b) && !cls.HasDefaultImpls() {
				g.bridge(b, f.JVM(), params, types.JVMRet(f), slices.Contains(f.BoxingBridges, b), f.BridgeOf[b], f)
				bridgeAnnotations(g.cls.LastMethod(), fd.Annotations)
			}
		}
	}
	if cls.HasDefaultImpls() || len(cd.Forwarders) > 0 {
		g.implsClass(cd, bodies)
	}
	for _, dl := range cd.Delegated {
		g.delegated(dl)
	}
	for _, b := range cd.InheritedBridges {
		if i := strings.IndexByte(b.Bridge, '('); !g.cache().collectionBridge(cls, b.Bridge) && !c.HasMethod(b.Bridge[:i], b.Bridge[i:]) {
			g.bridge(b.Bridge, b.Target, b.Params, b.Ret, false, b.Over, b.Fun)
		}
	}
	g.collectionMembers(cls)
	g.staticStubs(cd)
	if g.shared.opts.JVMDefaultCompat && !cls.Interface {
		g.defaultBridges(cd, c)
	}
	return c
}

// delegated generates a member that class delegation implements: it
// calls the interface's member on the delegate, and has the bridges an
// override would.
func (g *gen) delegated(dl *syntax.Delegated) {
	f, target, st := dl.Sym, dl.Target, dl.Super
	d, td := funDesc(f), funDesc(target) // with a member extension's receiver
	acc := uint16(cf.AccPublic)
	if f.Hidden {
		acc |= cf.AccSynthetic // as kotlinc delegates a hidden member
	}
	if dl.Impls != "" {
		acc |= cf.AccBridge // as kotlinc marks a call of a DefaultImpls body
	}
	// A value class delegates to its value: the member is a static method
	// that takes the value first (see valueClass).
	value := st != nil && g.owner.Value
	self, md, first := "L"+g.owner.Name+";", d, 0
	if value {
		self = desc(g.owner.Underlying.Type)
		md, first = "("+self+d[1:], 1
		acc |= cf.AccStatic
	}
	g.code = g.cls.NewMethod(acc, f.JVM(), md)
	if m := g.cls.LastMethod(); true {
		if !value {
			g.signFun(m, f, nil, d, types.InvalidType)
		}
		nullability(methodAdder(m), f.Ret)
		for i, p := range f.Params {
			nullability(paramAdder(m, first+i), p.Type)
		}
	}
	g.code.AllocLocal(self)
	from, to := descParams(d), descParams(td)
	slots := make([]int, len(from))
	for i, pd := range from {
		slots[i] = g.code.AllocLocal(pd)
	}
	if len(f.Params) == len(from) {
		for i, p := range f.Params {
			g.code.NameLocal(slots[i], p.Name)
			g.checkParam(slots[i], p.Type, p.Name)
		}
	}
	switch u := g.owner.Underlying; {
	case value:
		g.code.Local(loadOp(u.Type), 0)
		if types.Unboxed(u.Type) {
			g.boxValue(u.Type) // a value class's, which implements the interface
		}
	case st != nil:
		g.code.Local(cf.ALOAD, 0)
		fd := desc(st.T)
		p := g.owner.Prop(st.Field)
		if p != nil && !strings.HasPrefix(st.Field, "$$") {
			fd = desc(p.Type)
		} else {
			p = nil
		}
		g.code.FieldInsn(cf.GETFIELD, g.owner.Name, st.Field, fd)
		switch {
		case p != nil && types.Unboxed(p.Type):
			g.boxValue(p.Type) // a val property of a value class
		case p != nil && p.Type.IsPrimitive():
			g.box(p.Type) // a val property of a primitive type, Comparable
		}
	default:
		g.code.Local(cf.ALOAD, 0)
	}
	params := bridgeParams(f)
	for i, pd := range from {
		g.code.Local(loadOpOf(pd), slots[i])
		if i < len(params) && types.Unboxed(params[i].Type) && pd != to[i] {
			g.boxValue(params[i].Type) // a value class, which the interface erases
			continue
		}
		g.convertDesc(pd, to[i])
	}
	if dl.Impls != "" {
		// The body of a Kotlin interface's member, which takes this first.
		g.code.Invoke(cf.INVOKESTATIC, dl.Impls, target.JVM(), "(L"+target.Owner.Name+";"+td[1:])
	} else {
		g.code.InvokeInterface(target.Owner.Name, target.JVM(), td)
	}
	ret, tret := d[strings.IndexByte(d, ')')+1:], td[strings.IndexByte(td, ')')+1:]
	switch {
	case ret == "V" && (tret == "J" || tret == "D"):
		g.code.Op(cf.POP2)
	case ret == "V" && tret != "V":
		g.code.Op(cf.POP)
	case tret == "V":
		g.unitValue() // Unit, as an Object
	case types.Unboxed(f.Ret) && ret != tret:
		g.castBox(types.AnyType.Nullable(), f.Ret)
		g.unboxValue(f.Ret)
	default:
		g.convertDesc(tret, ret)
	}
	g.code.Op(returnOpOf(ret))
	g.check()
	if value {
		g.valueOverride(f)
		return
	}
	for _, b := range f.Bridges {
		if !g.cache().collectionBridge(g.owner, b) { // a collection's own bridge, as contains(Object)
			g.bridge(b, f.JVM(), params, types.JVMRet(f), slices.Contains(f.BoxingBridges, b), f.BridgeOf[b], f)
		}
	}
}

// constructor generates a class's primary constructor: it calls the
// superclass's constructor, stores val/var parameters, then runs property
// initializers and init blocks in declaration order.
func (g *gen) constructor(cd *syntax.ClassDecl) {
	cls := cd.Sym
	c := g.cls
	// Only an object's own static initializer creates it, and only an
	// enum's creates its entries. A companion's constructor is private
	// too, and its outer class calls its marker constructor, as kotlinc's.
	access := uint16(cf.AccPublic)
	marker := false
	switch {
	case cls.Object && cls.CompanionOf == nil || cls.Entries != nil:
		access = cf.AccPrivate
		if cls.Entries != nil && !cls.Final && entryCallsCtor(cd, cls.Ctor) {
			// Its entries' classes call it.
			defer g.markerConstructor(cls.Ctor)
		}
	case interfaceCompanion(cls):
		access = cf.AccPrivate // its own static initializer creates it
	case cls.CompanionOf != nil:
		access = cf.AccPrivate
		defer g.markerConstructor(cls.Ctor)
	case cls.Anonymous && inInline(cd):
		access = cf.AccPublic // copies of the function create it, as with kotlinc
	case cls.Anonymous || cd.EnumEntry:
		access = 0 // as kotlinc's
	case hasValueParam(cls.Ctor) || cls.Ctor.Private:
		access, marker = cf.AccPrivate, true
	case cd.CtorMods.Visibility == syntax.Protected:
		access = cf.AccProtected
	}
	if hasVararg(cls.Ctor.Params) {
		access |= cf.AccVarargs
	}
	access |= synthetic(&cd.CtorMods)
	if marker && eagerMarker(cls.Ctor) {
		defer g.markerConstructor(cls.Ctor)
	}
	g.code = c.NewMethod(access, "<init>", ctorDesc(cd))
	if access&cf.AccSynthetic == 0 {
		ps := make([]*syntax.Param, len(cd.CtorParams))
		for i, cp := range cd.CtorParams {
			p := cp.Param
			p.Vararg = p.Vararg || cp.Modifiers.Vararg
			ps[i] = &p
		}
		g.signCtor(c.LastMethod(), cls, cls.Ctor, ps, ctorDesc(cd), cd.CtorMods.Annotations)
	}
	// An enum's name and ordinal, an inner class's outer instance or a
	// local class's captures come first.
	skip := len(descParams(ctorDesc(cd))) - len(cls.Ctor.Params)
	g.annotateCtor(c.LastMethod(), &cd.CtorMods, cls.Ctor.Params, nil, cd.CtorParams, skip, access&cf.AccPrivate != 0 || cls.Anonymous)
	g.prefix = "_init_$lambda"
	g.retType = types.UnitType
	g.loops = nil
	g.code.AllocLocal("L" + cls.Name + ";")
	g.outerParam(cd, true)
	if cls.Entries != nil || cd.EnumEntry {
		// An enum's constructor takes the entry's name and ordinal first,
		// and so does an entry's class, which passes them on.
		g.code.AllocLocal("Ljava/lang/String;")
		g.code.AllocLocal("I")
	}
	if !cls.Inner {
		// The captures come first, as kotlinc's local classes take them.
		for _, v := range cd.Captures {
			v.Slot = g.code.AllocLocal(storageDesc(v))
			g.named(v)
		}
	}
	for _, cp := range cd.CtorParams {
		cp.Sym.Slot = g.code.AllocLocal(paramDesc(cp.Sym.Type))
		g.named(cp.Sym)
	}
	if cls.Inner {
		g.outerCaptures(cd)
	}
	g.line(cd.P)
	if access&cf.AccPrivate == 0 && !cls.Anonymous {
		for _, cp := range cd.CtorParams {
			g.checkParam(cp.Sym.Slot, cp.Sym.Type, cp.Name)
		}
	}
	g.code.Local(cf.ALOAD, 0)
	if cls.Entries != nil {
		g.code.Local(cf.ALOAD, 1)
		g.code.Local(cf.ILOAD, 2)
		g.code.Invoke(cf.INVOKESPECIAL, "java/lang/Enum", "<init>", "(Ljava/lang/String;I)V")
	} else if x := cd.SuperCall; x != nil {
		g.ctorCall(x)
	} else {
		super := "java/lang/Object"
		if cls.Super != nil {
			super = cls.Super.Name
		}
		g.code.Invoke(cf.INVOKESPECIAL, super, "<init>", "()V")
	}
	if !cls.Inner {
		g.storeCaptures(cd)
	}
	g.initializers(cd)
	g.line(cd.P)
	g.code.Op(cf.RETURN)
	g.check()
}

// entryCallsCtor reports whether an entry of enum cd with a body calls
// constructor f with all its arguments, and so calls it through its
// marker constructor rather than the one that fills in default values.
func entryCallsCtor(cd *syntax.ClassDecl, f *types.FunSym) bool {
	for _, e := range cd.Entries {
		if e.Body != nil && e.Call.Sym == f && !omitsArgs(e.Call, f) {
			return true
		}
	}
	return false
}

// omitsArgs reports whether call x of f leaves out arguments that default
// values supply.
func omitsArgs(x *syntax.Call, f *types.FunSym) bool {
	if x.ArgMap == nil {
		return false
	}
	given := map[int]bool{}
	for _, j := range x.ArgMap {
		given[j] = true
	}
	for j, p := range f.Params {
		if !given[j] && (p.Default || !p.Vararg) {
			return true
		}
	}
	return false
}

// inInline reports whether object expression cd is in an inline
// function, whose copies in other packages create it.
func inInline(cd *syntax.ClassDecl) bool {
	if f := cd.EnclosingFun; f != nil {
		fd, ok := f.Decl.(*syntax.FunDecl)
		return ok && fd.Inline
	}
	return false
}

// ctorCall calls the constructor that x, a call of a superclass's or of
// the class's own constructor, resolved to, on this, below it.
func (g *gen) ctorCall(x *syntax.Call) {
	f := x.Sym
	d := ctorJVMDesc(f)
	if x.Ctor.Inner && x.Outer == nil {
		g.loadOuterThis() // this(...) of an inner class
	} else {
		g.outerArg(x) // and this(...) of a local class, its captures
	}
	enum := x.Ctor.Entries != nil
	if enum {
		// An entry's class passes on the entry's name and ordinal, to the
		// enum's private constructor through its marker constructor.
		g.code.Local(cf.ALOAD, 1)
		g.code.Local(cf.ILOAD, 2)
		d = "(Ljava/lang/String;I" + d[1:]
	}
	if mask := g.callArgs(x, f); mask != 0 {
		g.code.Iconst(int32(mask))
		g.code.Op(cf.ACONST_NULL)
		i := strings.IndexByte(d, ')')
		d = d[:i] + "ILkotlin/jvm/internal/DefaultConstructorMarker;" + d[i:]
	} else if enum && x.Ctor != g.owner {
		d = g.marker(d)
	} else if x.Ctor != g.owner && g.viaMarker(f) {
		g.useMarker(f)
		d = g.marker(d) // a superclass's constructor that is private
	}
	g.code.Invoke(cf.INVOKESPECIAL, x.Ctor.Name, "<init>", d)
}

// secondary generates a secondary constructor. One that calls the
// superclass's constructor runs the initializers after it, as the primary
// constructor does; one that calls this(...) leaves them to that
// constructor. The body comes last.
func (g *gen) secondary(cd *syntax.ClassDecl, sc *syntax.SecondaryCtor) {
	cls := cd.Sym
	f := sc.Sym
	access := uint16(cf.AccPublic)
	enum := cls.Entries != nil
	d := ctorJVMDesc(f)
	switch {
	case enum:
		// As the primary constructor, it takes the entry's name and
		// ordinal first.
		access = cf.AccPrivate
		d = "(Ljava/lang/String;I" + d[1:]
		if !cls.Final && entryCallsCtor(cd, f) {
			defer g.markerConstructor(f)
		}
	case hasValueParam(f) || f.Private:
		access = cf.AccPrivate
		if eagerMarker(f) {
			defer g.markerConstructor(f)
		}
	case sc.Visibility == syntax.Protected:
		access = cf.AccProtected
	}
	if hasVararg(f.Params) {
		access |= cf.AccVarargs
	}
	access |= synthetic(&sc.Modifiers)
	g.code = g.cls.NewMethod(access, "<init>", d)
	if access&cf.AccSynthetic == 0 {
		g.signCtor(g.cls.LastMethod(), cls, f, sc.Params, d, sc.Annotations)
	}
	skip := len(descParams(d)) - len(f.Params)
	g.annotateCtor(g.cls.LastMethod(), &sc.Modifiers, f.Params, sc.Params, nil, skip, access&cf.AccPrivate != 0)
	g.prefix = "_init_$lambda"
	g.retType = types.UnitType
	g.loops, g.tries = nil, nil
	g.code.AllocLocal("L" + cls.Name + ";")
	g.outerParam(cd, !sc.ToThis)
	if enum {
		g.code.AllocLocal("Ljava/lang/String;")
		g.code.AllocLocal("I")
	}
	if cls.Local {
		for _, v := range cd.Captures {
			v.Slot = g.code.AllocLocal(storageDesc(v))
			g.named(v)
		}
	}
	for _, p := range f.Params {
		p.Slot = g.code.AllocLocal(paramDesc(p.Type))
		g.named(p)
	}
	if cls.Inner {
		g.outerCaptures(cd)
	}
	g.line(sc.P)
	if access&cf.AccPrivate == 0 {
		g.checkParams(nil, nil, f.Params)
	}
	g.code.Local(cf.ALOAD, 0)
	switch {
	case sc.Delegation == nil && enum:
		g.code.Local(cf.ALOAD, 1)
		g.code.Local(cf.ILOAD, 2)
		g.code.Invoke(cf.INVOKESPECIAL, "java/lang/Enum", "<init>", "(Ljava/lang/String;I)V")
	case sc.Delegation == nil:
		g.code.Invoke(cf.INVOKESPECIAL, "java/lang/Object", "<init>", "()V") // without a JDK
	default:
		g.ctorCall(sc.Delegation)
	}
	if !sc.ToThis {
		if cls.Local {
			g.storeCaptures(cd)
		}
		g.initializers(cd)
		g.line(sc.P)
	}
	if sc.Body != nil {
		g.block(sc.Body, types.UnitType, false)
	}
	if g.code.Reachable() {
		if sc.Body != nil {
			g.line(sc.Body.End)
		}
		g.code.Op(cf.RETURN)
	}
	g.check()
}

// initializers generates what a constructor that calls the superclass's
// does after that call: it stores the delegates of delegated interfaces
// and val/var parameters, then runs property initializers and init blocks
// in declaration order.
func (g *gen) initializers(cd *syntax.ClassDecl) {
	cls := cd.Sym
	for _, st := range cd.Supers {
		if st.Delegate != nil && strings.HasPrefix(st.Field, "$$delegate_") {
			g.code.Local(cf.ALOAD, 0)
			g.exprAs(st.Delegate, st.T)
			g.code.FieldInsn(cf.PUTFIELD, cls.Name, st.Field, desc(st.T))
		}
	}
	for _, cp := range cd.CtorParams {
		if cp.IsProp {
			g.code.Local(cf.ALOAD, 0)
			g.code.Local(loadOp(cp.Sym.Type), cp.Sym.Slot) // a Unit one's, kotlin.Unit
			g.code.FieldInsn(cf.PUTFIELD, cls.Name, cp.Name, desc(types.FieldType(cp.Sym)))
		}
	}
	if cls.Object && cls.CompanionOf == nil && !cls.Anonymous || initsInOuter(cd) {
		return // in the static initializer: see staticInits
	}
	for _, m := range cd.Members {
		switch m := m.(type) {
		case *syntax.PropDecl:
			if m.Init == nil || m.Sym.Const || m.Sym.StaticField() {
				continue // a const's is its static field's, as a companion's @JvmField's
			}
			if defaultInit(m.Sym, m.Init) {
				continue // the field's value already, as kotlinc skips it
			}
			g.line(m.P)
			g.code.Local(cf.ALOAD, 0)
			g.prefix = m.Name + "$lambda" // as kotlinc names its lambdas
			g.exprAs(m.Init, types.FieldType(m.Sym))
			g.code.FieldInsn(cf.PUTFIELD, cls.Name, m.Name, desc(types.FieldType(m.Sym)))
		case *syntax.InitBlock:
			g.prefix = "_init_$lambda"
			g.block(m.Body, types.UnitType, false)
		}
	}
}

// initsInOuter reports whether cd is a companion object, whose property
// initializers and init blocks all run in its outer class's static
// initializer, in declaration order, once it has created the companion,
// as kotlinc runs them: they may read the companion through the outer
// class, as HttpStatusCode.Companion's allStatusCodes = listOf(
// HttpStatusCode.Continue, ...), and its @JvmField properties, which the
// outer class holds, as GrpcStatus.OK.
func initsInOuter(cd *syntax.ClassDecl) bool {
	return cd != nil && cd.Sym != nil && cd.Sym.CompanionOf != nil
}

// staticInits generates, in a static initializer after the object's
// instance is created, the initializers of object od's static fields: as
// kotlinc does, all of a named object's property initializers and init
// blocks, in declaration order, and a companion object's @JvmField
// initializers, whose fields its outer class holds.
func (g *gen) staticInits(od *syntax.ClassDecl) {
	saved := g.owner
	all := od.Sym.CompanionOf == nil || initsInOuter(od)
	g.owner, g.objThis, g.companionInits = od.Sym, true, od.Sym.CompanionOf != nil && all
	defer func() { g.owner, g.objThis, g.companionInits = saved, false, false }()
	for _, m := range od.Members {
		switch m := m.(type) {
		case *syntax.PropDecl:
			if m.Init == nil || m.Sym == nil || m.Sym.Const || defaultInit(m.Sym, m.Init) {
				continue
			}
			if !m.Sym.StaticField() {
				if !all || !m.Sym.HasField() {
					continue
				}
				// A companion's property, of its instance, set in order
				// with the @JvmField ones.
				g.line(m.P)
				g.loadThis()
				g.prefix = m.Name + "$lambda"
				g.exprAs(m.Init, types.FieldType(m.Sym))
				g.code.FieldInsn(cf.PUTFIELD, od.Sym.Name, m.Name, desc(types.FieldType(m.Sym)))
				continue
			}
			g.line(m.P)
			g.prefix = m.Name + "$lambda"
			g.exprAs(m.Init, types.FieldType(m.Sym))
			g.code.FieldInsn(cf.PUTSTATIC, m.Sym.ConstOwner().Name, m.Name, desc(types.FieldType(m.Sym)))
		case *syntax.InitBlock:
			if all {
				g.prefix = "_init_$lambda"
				g.block(m.Body, types.UnitType, false)
			}
		}
	}
}

// propField adds class property p's field to class c: private, but for a
// @JvmField's and a lateinit property's, and static for an object's, as
// kotlinc writes them.
func (g *gen) propField(c *cf.Class, p *types.VarSym, inClinit bool) {
	acc := uint16(cf.AccPrivate)
	cp, _ := p.Decl.(*syntax.CtorParam)
	switch pd, _ := p.Decl.(*syntax.PropDecl); {
	case p.JvmField && (pd != nil && pd.Visibility == syntax.Protected || cp != nil && cp.Visibility == syntax.Protected):
		acc = cf.AccProtected
	case p.JvmField:
		acc = cf.AccPublic
	case p.Lateinit && !p.Private:
		// As with kotlinc, a lateinit property's field has its
		// setter's visibility.
		acc = lateinitAccess(pd)
	}
	if inClinit && !p.StaticField() && acc == cf.AccPrivate {
		// The outer class's static initializer sets it: package-private
		// and not final.
		acc = 0
	} else if !p.Mutable {
		acc |= cf.AccFinal
	}
	if p.StaticField() {
		acc |= cf.AccStatic
	}
	as, field, _ := classPropAnnotations(p)
	ft := types.FieldType(p)
	c.AddField(acc|fieldModifiers(as), p.Name, desc(ft))
	g.signField(c.LastField(), ft, propRef(p), desc(ft), p.Mutable, as, p.Owner)
	g.fieldAnnotations(c.LastField(), as, p.Type, field)
}

// companionFields returns the properties of class cd's companion object
// whose static fields cd's class holds: its @JvmField ones.
func companionFields(cd *syntax.ClassDecl) []*types.VarSym {
	ccd := companionDecl(cd)
	if ccd == nil {
		return nil
	}
	var out []*types.VarSym
	for _, p := range ccd.Sym.Props {
		if p.StaticField() && p.HasField() {
			out = append(out, p)
		}
	}
	return out
}

// companionDecl returns the declaration of class cd's companion object, or
// nil.
func companionDecl(cd *syntax.ClassDecl) *syntax.ClassDecl {
	for _, m := range cd.Members {
		if ccd, ok := m.(*syntax.ClassDecl); ok && ccd.Sym != nil && ccd.Sym == cd.Sym.Companion {
			return ccd
		}
	}
	return nil
}

func hasDefaults(params []*syntax.Param) bool {
	for _, p := range params {
		if p.Default != nil {
			return true
		}
	}
	return false
}

// defaults generates the static method that supplies a function's omitted
// arguments, as kotlinc writes it: name$default takes the receiver of a
// method, the parameters, a mask with bit i set if parameter i was
// omitted, and an unused marker. It computes the omitted ones' default
// values, in order, and calls the function. A value class's member takes
// the value instead of the object, as its static method does.
func (g *gen) defaults(fd *syntax.FunDecl, owner *types.ClassSym) {
	f := fd.Sym
	d := sigDesc(f)
	i := strings.IndexByte(d, ')')
	self := "" // the method's object
	value := owner != nil && owner.Value
	switch {
	case value:
		self = desc(owner.Underlying.Type)
		g.valueThis = true
		defer func() { g.valueThis = false }()
	case owner != nil:
		self = "L" + owner.Name + ";"
	}
	recv := self // and the context parameters and an extension's receiver
	for _, p := range f.Contexts {
		recv += paramDesc(p.Type)
	}
	if r := fd.RecvSym; r != nil {
		recv += paramDesc(r.Type)
	}
	access := uint16(cf.AccPublic | cf.AccStatic | cf.AccSynthetic)
	if (f.Private || fd.Inline && fd.Find("", "InlineOnly") != nil) && !g.inInterface() {
		// As kotlinc's: its package calls it. An interface's methods
		// are public or private.
		access &^= cf.AccPublic
	}
	g.code = g.cls.NewMethod(access, f.JVM()+"$default", "("+recv+d[1:i]+"ILjava/lang/Object;"+d[i:])
	g.cls.Throws(f.Throws...) // as kotlinc declares them
	copied := (fd.Inline || inlineAccessor(fd)) && !f.Private
	if copied {
		// Copied by other modules too, with the lambdas of the default
		// values: see lambdaClass.
		saved := g.publicInline
		g.publicInline = true
		defer func() { g.publicInline = saved }()
		g.lockInline() // the body's variables' locals: see inlineMu
		defer g.unlockInline()
	}
	g.prefix = f.Name + "$default$lambda"
	g.retType = f.Ret
	g.loops, g.tries = nil, nil
	if owner != nil {
		g.code.AllocLocal(self)
	} else if g.owner != nil && g.owner.Object {
		g.objThis = true // a @JvmStatic member's
		defer func() { g.objThis = false }()
	}
	for _, p := range f.Contexts {
		p.Slot = g.code.AllocLocal(paramDesc(p.Type))
		g.named(p)
	}
	if r := fd.RecvSym; r != nil {
		r.Slot = g.code.AllocLocal(paramDesc(r.Type))
		g.named(r)
	}
	k := g.fillDefaultsCont(fd.Params, f.Params, len(f.Contexts), f.Suspend, nil)
	if copied && !f.Suspend && (fd.Body != nil || fd.ExprBody != nil) {
		// As kotlinc's, the body follows the default values, rather than
		// a call of the function: copiers copy the function's lambda
		// parameters into it.
		g.prefix = f.Name + "$lambda"
		g.retType = types.JVMRet(f)
		if fd.Body != nil {
			g.block(fd.Body, types.UnitType, false)
			if g.code.Reachable() {
				g.line(fd.Body.End)
				g.ret(types.UnitType)
			}
		} else {
			g.line(syntax.StartPos(fd.ExprBody))
			g.exprAs(fd.ExprBody, types.JVMRet(f))
			g.ret(types.JVMRet(f))
		}
		g.check()
		return
	}
	switch {
	case value:
		g.code.Local(loadOp(owner.Underlying.Type), 0)
	case owner != nil:
		g.code.Local(cf.ALOAD, 0)
	}
	for _, p := range f.Contexts {
		g.code.Local(loadOp(p.Type), p.Slot)
	}
	if r := fd.RecvSym; r != nil {
		g.code.Local(loadOp(r.Type), r.Slot)
	}
	for _, p := range f.Params {
		g.code.Local(loadOp(p.Type), p.Slot)
	}
	if k >= 0 {
		g.code.Local(cf.ALOAD, k)
	}
	switch {
	case owner == nil:
		g.code.Invoke(cf.INVOKESTATIC, g.cls.Name, f.JVM(), funDesc(f))
	case value:
		g.code.Invoke(cf.INVOKESTATIC, owner.Name, f.JVM(), "("+self+funDesc(f)[1:])
	default:
		g.callMethod(f, owner, f.JVM(), funDesc(f))
	}
	g.code.Op(returnOpOf(funDesc(f)[strings.IndexByte(funDesc(f), ')')+1:]))
	g.check()
}

// fillDefaults allocates the parameters of a $default method, followed by
// the mask and the marker, and stores the default value of each omitted
// parameter in its local.
func (g *gen) fillDefaults(params []*syntax.Param, syms []*types.VarSym) {
	g.fillDefaultsCont(params, syms, 0, false, nil)
}

// fillDefaultsCont is fillDefaults for a suspend function if suspend is
// set: its continuation comes before the mask. It returns its slot, or
// -1. prologue, if not nil, runs before the default values. shift is the
// mask's bit of the first parameter: as kotlinc's, a function's context
// parameters come before it.
func (g *gen) fillDefaultsCont(params []*syntax.Param, syms []*types.VarSym, shift int, suspend bool, prologue func()) int {
	for _, p := range syms {
		p.Slot = g.code.AllocLocal(paramDesc(p.Type))
		g.named(p)
	}
	k := -1
	if suspend {
		k = g.code.AllocLocal(types.ContinuationDesc)
		g.code.NameLocal(k, "$completion")
	}
	mask := g.code.AllocLocal("I")
	g.code.AllocLocal("Ljava/lang/Object;")
	if prologue != nil {
		prologue()
	}
	for j, p := range params {
		if p.Default == nil {
			continue
		}
		skip := cf.NewLabel()
		g.code.Local(cf.ILOAD, mask)
		g.code.Iconst(int32(1) << (shift + j))
		g.code.Op(cf.IAND)
		g.code.Jump(cf.IFEQ, skip)
		g.exprAs(p.Default, p.Sym.Type)
		g.code.Local(storeOp(p.Sym.Type), p.Sym.Slot)
		g.code.Place(skip)
	}
	return k
}

// defaultConstructor generates the constructor for calls that omit
// arguments: it takes a mask and an unused DefaultConstructorMarker after
// the parameters, as kotlinc writes it, and calls ctor, the primary or a
// secondary constructor. An enum's takes the entry's name and ordinal
// first, and passes them on.
func (g *gen) defaultConstructor(ctor *types.FunSym, params []*syntax.Param) {
	cls := ctor.Owner
	d := ctorJVMDesc(ctor)
	enum := cls.Entries != nil
	if enum {
		d = "(Ljava/lang/String;I" + d[1:]
	}
	i := strings.IndexByte(d, ')')
	access := uint16(cf.AccPublic | cf.AccSynthetic)
	if ctor.Private || enum {
		access = cf.AccSynthetic // as kotlinc's: its package calls it
	}
	g.code = g.cls.NewMethod(access, "<init>", d[:i]+"ILkotlin/jvm/internal/DefaultConstructorMarker;"+d[i:])
	g.prefix = "_init_$default$lambda"
	g.retType = types.UnitType
	g.loops, g.tries = nil, nil
	g.code.AllocLocal("L" + cls.Name + ";")
	cd, _ := cls.Decl.(*syntax.ClassDecl)
	if cls.Inner {
		g.outerParam(cd, false)
	}
	if enum {
		g.code.AllocLocal("Ljava/lang/String;")
		g.code.AllocLocal("I")
	}
	g.fillDefaultsCont(params, ctor.Params, 0, false, func() {
		if cls.Inner {
			g.outerCaptures(cd)
		}
	})
	g.code.Local(cf.ALOAD, 0)
	if cls.Inner {
		g.code.Local(storageLoadOp(cd.OuterThis), cd.OuterThis.Slot)
	}
	if enum {
		g.code.Local(cf.ALOAD, 1)
		g.code.Local(cf.ILOAD, 2)
	}
	for _, p := range ctor.Params {
		g.code.Local(loadOp(p.Type), p.Slot)
	}
	g.code.Invoke(cf.INVOKESPECIAL, cls.Name, "<init>", d)
	g.code.Op(cf.RETURN)
	g.check()
}

// hasValueParam reports whether a function of the program has a
// parameter of a value class type, other than Result: a constructor that
// does is private, as with kotlinc, and callers call one that takes an
// unused marker last.
func hasValueParam(f *types.FunSym) bool {
	for _, p := range f.Params {
		if types.Mangles(p.Type) {
			return true
		}
	}
	return false
}

// viaMarker reports whether code of the class being generated calls
// constructor f through the constructor that takes an unused marker last:
// f, of the program, is private, and of another class, or takes a value
// class, or f is one of the class path's that does (types.FunSym.Marker).
func (g *gen) viaMarker(f *types.FunSym) bool {
	return f.Marker || f.Desc == "" && (hasValueParam(f) || f.Private && !g.inClass(f.Owner))
}

// useMarker records a call of private constructor f through its marker
// constructor, which markerConstructors then adds to its class: kotlinc
// writes one only for a private constructor that another class calls.
func (g *gen) useMarker(f *types.FunSym) {
	if !eagerMarker(f) {
		g.shared.markers[f] = true
	}
}

// eagerMarker reports whether constructor f's class gets its marker
// constructor whether or not its file calls it: f takes a value class and
// isn't private, so callers anywhere call it so, or its class can be
// subclassed, maybe in other files of its package.
func eagerMarker(f *types.FunSym) bool { return !f.Private || !f.Owner.Final }

// markerConstructors adds the marker constructors that useMarker recorded.
func (g *gen) markerConstructors() {
	fs := make([]*types.FunSym, 0, len(g.shared.markers))
	for f := range g.shared.markers {
		fs = append(fs, f)
	}
	sort.Slice(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		return a.Owner.Name < b.Owner.Name || a.Owner.Name == b.Owner.Name && funDesc(a) < funDesc(b)
	})
	for _, f := range fs {
		if c := g.shared.classes[f.Owner.Name]; c != nil {
			g.cls, g.owner = c, f.Owner
			g.markerConstructor(f)
		}
	}
}

// marker pushes the unused marker that the constructor of descriptor d,
// which takes a value class, takes last, and returns its descriptor.
func (g *gen) marker(d string) string {
	g.code.Op(cf.ACONST_NULL)
	i := strings.IndexByte(d, ')')
	return d[:i] + "Lkotlin/jvm/internal/DefaultConstructorMarker;" + d[i:]
}

// markerConstructor generates the constructor that callers of a
// constructor that takes a value class call: it takes an unused marker
// last, and calls the private one.
func (g *gen) markerConstructor(f *types.FunSym) {
	d := ctorJVMDesc(f) // an inner class's takes the outer instance first
	if f.Owner.Entries != nil {
		d = "(Ljava/lang/String;I" + d[1:]
	}
	i := strings.IndexByte(d, ')')
	g.code = g.cls.NewMethod(cf.AccPublic|cf.AccSynthetic, "<init>", d[:i]+"Lkotlin/jvm/internal/DefaultConstructorMarker;"+d[i:])
	g.code.AllocLocal("L" + f.Owner.Name + ";")
	g.code.Local(cf.ALOAD, 0)
	for _, pd := range descParams(d) {
		g.code.Local(loadOpOf(pd), g.code.AllocLocal(pd))
	}
	g.code.AllocLocal("Lkotlin/jvm/internal/DefaultConstructorMarker;")
	g.code.Invoke(cf.INVOKESPECIAL, f.Owner.Name, "<init>", d)
	g.code.Op(cf.RETURN)
	g.check()
}

// ctorDesc returns the descriptor of a class's primary constructor. An
// enum's takes the entry's name and ordinal first.
func ctorDesc(cd *syntax.ClassDecl) string {
	if cd.Anonymous {
		return anonCtorDesc(cd)
	}
	d := ctorJVMDesc(cd.Ctor)
	if cd.Sym.Entries != nil || cd.EnumEntry {
		d = "(Ljava/lang/String;I" + d[1:]
	}
	return d
}

// ctorJVMDesc returns the descriptor of constructor f: one of an inner
// class takes the outer class's instance first.
func ctorJVMDesc(f *types.FunSym) string {
	if f.Desc != "" {
		return f.Desc
	}
	d := methodDesc(f.Params, types.UnitType)
	switch o := f.Owner; {
	case o == nil:
	case o.Inner:
		d = "(" + desc(o.Outer.This()) + d[1:] // unboxed, of a value class
	case o.Local:
		d = anonCtorDesc(o.Decl.(*syntax.ClassDecl))[:len(anonCtorDesc(o.Decl.(*syntax.ClassDecl)))-2] + d[1:]
	}
	return d
}

// outerArg pushes what a call of an inner or local class's constructor
// passes first: the outer instance, or the local class's captures.
func (g *gen) outerArg(x *syntax.Call) {
	switch {
	case x.Outer != nil:
		g.exprAs(x.Outer, x.Ctor.Outer.This())
	case x.Ctor.Local:
		cd := x.Ctor.Decl.(*syntax.ClassDecl)
		for _, v := range cd.Captures {
			if v == cd.OuterThis && !g.inLocal(x.Ctor) {
				g.loadThis()
			} else {
				g.loadStorage(v)
			}
		}
	}
}

// inLocal reports whether the code being generated is the code of local
// class cls, which has its captures in locals.
func (g *gen) inLocal(cls *types.ClassSym) bool {
	for k := g.owner; k != nil; k = k.Outer {
		if k == cls {
			return true
		}
	}
	return false
}

// outerParam allocates the parameter of an inner class's constructor that
// is the outer class's instance, and with store set, stores it in this$0,
// before the superclass's constructor runs, as kotlinc does.
func (g *gen) outerParam(cd *syntax.ClassDecl, store bool) {
	if !cd.Sym.Inner {
		return
	}
	v := cd.OuterThis
	v.Slot = g.code.AllocLocal(storageDesc(v))
	g.named(v)
	if store {
		g.code.Local(cf.ALOAD, 0)
		g.code.Local(storageLoadOp(v), v.Slot)
		g.code.FieldInsn(cf.PUTFIELD, cd.Sym.Name, "this$0", storageDesc(v))
	}
}

// loadOuterThis loads the outer class's instance in an inner class's
// constructor: its parameter.
func (g *gen) loadOuterThis() {
	cd := g.owner.Decl.(*syntax.ClassDecl)
	g.code.Local(storageLoadOp(cd.OuterThis), cd.OuterThis.Slot)
}

// staticInit generates a class's static initializer, if it needs one: it
// creates an enum's entries, and an object's instance or a class's
// companion object, which static fields hold.
func (g *gen) staticInit(cd *syntax.ClassDecl) {
	cls := cd.Sym
	obj := cls.Companion
	if cls.Object && cls.CompanionOf == nil || interfaceCompanion(cls) {
		obj = cls
	}
	var consts []*syntax.PropDecl // the consts whose values kc doesn't compute
	for _, pd := range constDecls(cd) {
		if _, ok := constValue(pd.Sym); !ok {
			consts = append(consts, pd)
		}
	}
	if obj == nil && cls.Entries == nil && len(cd.KProps) == 0 && len(consts) == 0 && !cd.Parcelize && g.shared.stableInits[cd] == nil {
		return
	}
	g.code = g.cls.NewMethod(cf.AccStatic, "<clinit>", "()V")
	g.prefix = "_clinit_$lambda"
	g.retType = types.UnitType
	g.loops, g.tries = nil, nil
	for _, pd := range consts {
		g.exprAs(pd.Init, pd.Sym.Type)
		g.code.FieldInsn(cf.PUTSTATIC, cls.Name, pd.Name, desc(pd.Sym.Type))
	}
	g.kprops(cd.KProps)
	if cls.Entries != nil {
		self := "L" + cls.Name + ";"
		for i, e := range cd.Entries {
			if e.Body != nil {
				// An instance of the entry's class, which passes the
				// arguments on.
				g.code.New(e.Body.Sym.Name)
				g.code.Op(cf.DUP)
				g.code.Sconst(e.Name)
				g.code.Iconst(int32(i))
				g.code.Invoke(cf.INVOKESPECIAL, e.Body.Sym.Name, "<init>", ctorDesc(e.Body))
				g.code.FieldInsn(cf.PUTSTATIC, cls.Name, e.Name, self)
				continue
			}
			g.code.New(cls.Name)
			g.code.Op(cf.DUP)
			g.code.Sconst(e.Name)
			g.code.Iconst(int32(i))
			f := e.Call.Sym
			d := "(Ljava/lang/String;I" + ctorJVMDesc(f)[1:]
			if mask := g.callArgs(e.Call, f); mask != 0 {
				g.code.Iconst(int32(mask))
				g.code.Op(cf.ACONST_NULL)
				i := strings.IndexByte(d, ')')
				d = d[:i] + "ILkotlin/jvm/internal/DefaultConstructorMarker;" + d[i:]
			}
			g.code.Invoke(cf.INVOKESPECIAL, cls.Name, "<init>", d)
			g.code.FieldInsn(cf.PUTSTATIC, cls.Name, e.Name, self)
		}
		g.code.Iconst(int32(len(cd.Entries)))
		g.code.ANewArray(cls.Name)
		for i, e := range cd.Entries {
			g.code.Op(cf.DUP)
			g.code.Iconst(int32(i))
			g.code.FieldInsn(cf.GETSTATIC, cls.Name, e.Name, self)
			g.code.Op(cf.AASTORE)
		}
		g.code.FieldInsn(cf.PUTSTATIC, cls.Name, "$VALUES", "["+self)
		if cd.EntriesList {
			g.code.FieldInsn(cf.GETSTATIC, cls.Name, "$VALUES", "["+self)
			g.code.Invoke(cf.INVOKESTATIC, "kotlin/enums/EnumEntriesKt", "enumEntries", "([Ljava/lang/Enum;)Lkotlin/enums/EnumEntries;")
			g.code.FieldInsn(cf.PUTSTATIC, cls.Name, "$ENTRIES", "Lkotlin/enums/EnumEntries;")
		}
	}
	switch {
	case obj == nil:
	case interfaceCompanion(cls):
		// An interface's companion creates itself, in a field the
		// interface's static initializer reads, as kotlinc's: the
		// interface can't call its private constructor.
		d := "L" + cls.Name + ";"
		g.cls.AddField(cf.AccStatic|cf.AccFinal, "$$INSTANCE", d)
		g.code.New(cls.Name)
		g.code.Op(cf.DUP)
		g.code.Invoke(cf.INVOKESPECIAL, cls.Name, "<init>", "()V")
		g.code.FieldInsn(cf.PUTSTATIC, cls.Name, "$$INSTANCE", d)
	case interfaceCompanion(obj):
		owner, name := obj.ObjectField()
		d := "L" + obj.Name + ";"
		g.cls.AddField(cf.AccPublic|cf.AccStatic|cf.AccFinal, name, d)
		nullability(g.cls.LastField().Annotate, types.ClassType(obj))
		g.code.FieldInsn(cf.GETSTATIC, obj.Name, "$$INSTANCE", d)
		g.code.FieldInsn(cf.PUTSTATIC, owner, name, d)
		if ccd := companionDecl(cd); ccd != nil {
			g.staticInits(ccd)
		}
	default:
		owner, name := obj.ObjectField()
		d := "L" + obj.Name + ";"
		access := uint16(cf.AccPublic)
		if privateCompanion(obj) {
			access = cf.AccPrivate
		}
		g.cls.AddField(access|cf.AccStatic|cf.AccFinal, name, d)
		nullability(g.cls.LastField().Annotate, types.ClassType(obj))
		g.code.New(obj.Name)
		g.code.Op(cf.DUP)
		if obj == cls {
			g.code.Invoke(cf.INVOKESPECIAL, obj.Name, "<init>", "()V")
		} else {
			g.code.Op(cf.ACONST_NULL) // the companion's marker constructor
			g.code.Invoke(cf.INVOKESPECIAL, obj.Name, "<init>", "(Lkotlin/jvm/internal/DefaultConstructorMarker;)V")
		}
		g.code.FieldInsn(cf.PUTSTATIC, owner, name, d)
		if obj == cls {
			g.staticInits(cd)
		} else if ccd := companionDecl(cd); ccd != nil {
			g.staticInits(ccd)
		}
	}
	if cd.Parcelize {
		// The parcelize plugin's Parcelable.Creator, last, as kotlinc's.
		const d = "Landroid/os/Parcelable$Creator;"
		creator := cls.Nested["Creator"].Name
		g.cls.AddField(cf.AccPublic|cf.AccStatic|cf.AccFinal, "CREATOR", d)
		g.cls.LastField().SetSignature("Landroid/os/Parcelable$Creator<L" + cls.Name + ";>;")
		g.code.New(creator)
		g.code.Op(cf.DUP)
		g.code.Invoke(cf.INVOKESPECIAL, creator, "<init>", "()V")
		g.code.FieldInsn(cf.PUTSTATIC, cls.Name, "CREATOR", d)
	}
	g.stableInit(cd)
	g.code.Op(cf.RETURN)
	g.check()
}

// interfaceCompanion reports whether cls is the companion object of an
// interface.
func interfaceCompanion(cls *types.ClassSym) bool {
	return cls != nil && cls.CompanionOf != nil && cls.CompanionOf.Interface
}

// enumMethods generates an enum class's static methods: values(), a copy
// of the entries, valueOf(name), and with the standard library,
// getEntries(), as kotlinc writes them.
func (g *gen) enumMethods(cd *syntax.ClassDecl) {
	cls := cd.Sym
	self := "L" + cls.Name + ";"
	g.code = g.cls.NewMethod(cf.AccPublic|cf.AccStatic, "values", "()["+self)
	g.code.FieldInsn(cf.GETSTATIC, cls.Name, "$VALUES", "["+self)
	g.code.Invoke(cf.INVOKEVIRTUAL, "["+self, "clone", "()Ljava/lang/Object;")
	g.code.Checkcast("[" + self)
	g.code.Op(cf.ARETURN)
	g.check()

	g.code = g.cls.NewMethod(cf.AccPublic|cf.AccStatic, "valueOf", "(Ljava/lang/String;)"+self)
	g.code.AllocLocal("Ljava/lang/String;")
	g.code.LdcClass(cls.Name)
	g.code.Local(cf.ALOAD, 0)
	g.code.Invoke(cf.INVOKESTATIC, "java/lang/Enum", "valueOf", "(Ljava/lang/Class;Ljava/lang/String;)Ljava/lang/Enum;")
	g.code.Checkcast(cls.Name)
	g.code.Op(cf.ARETURN)
	g.check()

	if cd.EntriesList {
		g.code = g.cls.NewMethod(cf.AccPublic|cf.AccStatic, "getEntries", "()Lkotlin/enums/EnumEntries;")
		g.cls.LastMethod().SetSignature("()Lkotlin/enums/EnumEntries<" + self + ">;")
		g.cls.LastMethod().Annotate(cf.Annotation{Type: notNull}, false)
		g.code.FieldInsn(cf.GETSTATIC, cls.Name, "$ENTRIES", "Lkotlin/enums/EnumEntries;")
		g.code.Op(cf.ARETURN)
		g.check()
	}
}

// objectRef pushes an object, or a companion object: its instance.
func (g *gen) objectRef(obj *types.ClassSym) {
	owner, name := obj.ObjectField()
	if privateCompanion(obj) && g.cls.Name != owner {
		// Its field is private, as kotlinc's: other classes call an
		// accessor.
		g.shared.companions[obj] = true
		g.code.Invoke(cf.INVOKESTATIC, owner, "access$get"+name+"$p", "()L"+obj.Name+";")
		return
	}
	g.code.FieldInsn(cf.GETSTATIC, owner, name, "L"+obj.Name+";")
}

// privateCompanion reports whether obj is a private companion object,
// whose field in its outer class is private.
func privateCompanion(obj *types.ClassSym) bool {
	cd, ok := obj.Decl.(*syntax.ClassDecl)
	return ok && obj.CompanionOf != nil && cd.Visibility == syntax.Private && !obj.CompanionOf.Interface // an interface's fields are public
}

// companionAccessors adds the accessors of the private companion objects
// that other classes use.
func (g *gen) companionAccessors() {
	objs := make([]*types.ClassSym, 0, len(g.shared.companions))
	for obj := range g.shared.companions {
		objs = append(objs, obj)
	}
	sort.Slice(objs, func(i, j int) bool { return objs[i].Name < objs[j].Name })
	for _, obj := range objs {
		owner, name := obj.ObjectField()
		c := g.shared.classes[owner]
		if c == nil {
			continue
		}
		code := c.NewMethod(cf.AccPublic|cf.AccStatic|cf.AccFinal|cf.AccSynthetic, "access$get"+name+"$p", "()L"+obj.Name+";")
		code.FieldInsn(cf.GETSTATIC, owner, name, "L"+obj.Name+";")
		code.Op(cf.ARETURN)
		if err := code.Err(); err != nil {
			panic(err)
		}
	}
}

// bridgeParams returns the parameters of method f on the JVM: its context
// parameters, then a member extension's receiver, then its parameters.
func bridgeParams(f *types.FunSym) []*types.VarSym {
	params := slices.Clone(f.Contexts)
	if f.Ext {
		params = append(params, &types.VarSym{Name: "<this>", Type: f.Recv, Kind: types.Param})
	}
	return append(params, f.Params...)
}

// bridge generates a bridge method: the name and erased descriptor d of
// an overridden method, which casts its arguments and calls the override,
// whose descriptor is target. It is what callers through the supertype
// call: compareTo(Object) for compareTo(Point). If boxing, the
// overridden method returns the value class that the override returns
// unboxed as a box. over and impl, if known, are the overridden function
// and the override: the value classes that impl takes and returns
// unboxed where over has other types, as type parameters, are unboxed
// from the boxes that over's erased types are, but for the Results that
// impl takes boxed.
func (g *gen) bridge(nameDesc, targetName string, params []*types.VarSym, result types.Type, boxing bool, over, impl *types.FunSym) {
	i := strings.IndexByte(nameDesc, '(')
	name, d := nameDesc[:i], nameDesc[i:]
	target := methodDesc(params, result)
	if ps := descParams(d); len(ps) > len(params)+1 && ps[len(params)] == types.ComposerDesc {
		// A composable function's: the Composer and the ints are passed
		// as they are.
		target = types.ComposeDesc(target, len(ps)-len(params)-1)
	} else if strings.HasSuffix(d, types.ContinuationDesc+")"+objectDesc) || len(descParams(d)) == len(params)+1 {
		// A suspend function's: the continuation and the result are
		// passed as they are. A suspend function type's invoke takes
		// the continuation as an Object.
		target = types.SuspendDesc(target)
	}
	access := uint16(cf.AccPublic | cf.AccSynthetic | cf.AccBridge)
	if (nameDesc == "length()I" || nameDesc == "charAt(I)C") && g.owner.IsSubclassOf(charSequenceClass(g.owner)) {
		// CharSequence's methods, which call length and get: as
		// kotlinc's, ordinary final ones.
		access = cf.AccPublic | cf.AccFinal
	}
	spread := d == types.VarArityDesc && len(params) > types.MaxArity
	if spread {
		access |= cf.AccFinal // as kotlinc's
	}
	g.code = g.cls.NewMethod(access, name, d)
	g.code.AllocLocal("L" + g.owner.Name + ";")
	from, to := descParams(d), descParams(target)
	var slots []int
	if spread {
		// FunctionN's invoke, which takes the arguments in an array.
		args := g.code.AllocLocal(from[0])
		from = slices.Repeat([]string{objectDesc}, len(params))
		slots = make([]int, len(params))
		for i := range slots {
			slots[i] = g.code.AllocLocal(objectDesc)
		}
		spreadArgs(g.code, args, slots)
	}
	g.code.Local(cf.ALOAD, 0)
	for i, pd := range from {
		var slot int
		if slots != nil {
			slot = slots[i]
		} else {
			slot = g.code.AllocLocal(pd)
		}
		g.code.Local(loadOpOf(pd), slot)
		if i < len(params) && types.Unboxed(params[i].Type) && (pd != to[i] || boxedIn(over, impl, len(params), i)) {
			t := params[i].Type
			g.castBox(types.AnyType.Nullable(), t) // an erased value class
			g.unboxValue(t)
			continue
		}
		g.convertDesc(pd, to[i])
	}
	g.invokeVirtual(g.owner, targetName, target)
	ret, tret := d[strings.IndexByte(d, ')')+1:], target[strings.IndexByte(target, ')')+1:]
	switch {
	case tret == "V" && ret != "V":
		g.unitValue() // Unit, as an Object
	case types.Unboxed(result) && (ret != tret || boxing):
		g.boxValue(result)
	case over != nil && types.Unboxed(types.JVMRet(over)) && !types.Unboxed(result) && ret != tret:
		g.castBox(types.AnyType.Nullable(), types.JVMRet(over))
		g.unboxValue(types.JVMRet(over))
	default:
		g.convertDesc(tret, ret)
	}
	g.code.Op(returnOpOf(ret))
	g.check()
}

// boxedIn reports whether overridden function over takes parameter i of
// n, counting an extension's receiver, as other than a value class, as a
// type parameter, whose erasure is the value class's box, where impl
// takes it unboxed.
func boxedIn(over, impl *types.FunSym, n, i int) bool {
	if over == nil {
		return false
	}
	t := types.InvalidType
	switch j := i - (n - len(over.Params)); {
	case j >= 0 && j < len(over.Params):
		if impl != nil && slices.Contains(impl.BoxedParams, j) {
			return false
		}
		t = over.Params[j].Type
	case j == -1 && over.Ext:
		t = over.Recv
	default:
		return false
	}
	return !types.Unboxed(t)
}

// unitValue pushes Unit as a value: kotlin.Unit, or null without the
// standard library.
func (g *gen) unitValue() {
	if g.shared.opts.Stdlib {
		g.code.FieldInsn(cf.GETSTATIC, "kotlin/Unit", "INSTANCE", "Lkotlin/Unit;")
	} else {
		g.code.Op(cf.ACONST_NULL)
	}
}

// charSequenceClass returns java.lang.CharSequence if cls implements it,
// or nil.
func charSequenceClass(cls *types.ClassSym) *types.ClassSym {
	var out *types.ClassSym
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s.Name == "java/lang/CharSequence" {
			out = s
		}
		return out == nil
	})
	return out
}

// convertDesc converts the value on the stack from one erased type to
// another: unboxing, boxing or a cast.
func (g *gen) convertDesc(from, to string) {
	switch {
	case from == to || to == "V":
	case len(to) == 1: // a primitive, from its box or a supertype
		box := descBox[to]
		if from != "L"+box+";" {
			g.code.Checkcast(box)
		}
		g.unbox(descType[to])
	case len(from) == 1:
		g.box(descType[from])
	case to != "Ljava/lang/Object;":
		g.code.Checkcast(className(to)) // an array's class name is its descriptor
	}
}

// descType and descBox are the primitive types by descriptor, and their
// box classes.
var (
	descType = map[string]types.Type{
		"I": types.IntType, "J": types.LongType, "D": types.DoubleType, "Z": types.BoolType,
		"C": types.CharType, "F": types.FloatType, "B": types.ByteType, "S": types.ShortType,
	}
	descBox = func() map[string]string {
		m := map[string]string{}
		for d, t := range descType {
			m[d] = types.Boxes[t.K]
		}
		return m
	}()
)

func loadOpOf(d string) byte {
	switch d {
	case "I", "Z", "C", "B", "S":
		return cf.ILOAD
	case "J":
		return cf.LLOAD
	case "F":
		return cf.FLOAD
	case "D":
		return cf.DLOAD
	}
	return cf.ALOAD
}

func storeOpOf(d string) byte {
	switch d {
	case "I", "Z", "C", "B", "S":
		return cf.ISTORE
	case "J":
		return cf.LSTORE
	case "F":
		return cf.FSTORE
	case "D":
		return cf.DSTORE
	}
	return cf.ASTORE
}

func returnOpOf(d string) byte {
	switch d {
	case "V":
		return cf.RETURN
	case "I", "Z", "C", "B", "S":
		return cf.IRETURN
	case "J":
		return cf.LRETURN
	case "F":
		return cf.FRETURN
	case "D":
		return cf.DRETURN
	}
	return cf.ARETURN
}

// callMethod calls method f of the program, by its JVM name and
// descriptor d, on a receiver of class cls below the arguments. As with
// kotlinc, a private one is called directly in its class, and through
// its static accessor access$name in others.
func (g *gen) callMethod(f *types.FunSym, cls *types.ClassSym, name, d string) {
	switch {
	case implsPrivate(f):
		// A private member of an interface compiled without JVM default
		// methods: a static method of its DefaultImpls class.
		g.code.Invoke(cf.INVOKESTATIC, f.Owner.Name+"$DefaultImpls", name, "(L"+f.Owner.Name+";"+d[1:])
	case selfStatic(f):
		g.code.Invoke(cf.INVOKESTATIC, f.Owner.Name, name, "(L"+f.Owner.Name+";"+d[1:])
	case f.Ext && f.Owner != nil && f.Owner.Value:
		// A value class's member extension: a static method that takes
		// the value first, which a class path one's descriptor includes.
		if f.Desc == "" {
			d = "(" + desc(f.Owner.Underlying.Type) + d[1:]
		}
		g.code.Invoke(cf.INVOKESTATIC, f.Owner.Name, name, d)
	case protectedFun(f) && g.protectedAccess(f.Owner, name, d):
	case !f.Private || f.Owner == nil:
		g.invokeVirtual(cls, name, d)
	case g.inClass(f.Owner):
		if g.publicInline {
			g.shared.methods[privateMethod{class: f.Owner.Name, name: name, desc: d, iface: f.Owner.Interface}] = true // for copies
		}
		if f.Owner.Interface {
			g.code.InvokeSpecialInterface(f.Owner.Name, name, d)
		} else {
			g.code.Invoke(cf.INVOKESPECIAL, f.Owner.Name, name, d)
		}
	default:
		g.shared.methods[privateMethod{class: f.Owner.Name, name: name, desc: d, iface: f.Owner.Interface}] = true
		if f.Owner.Interface {
			g.code.InvokeStaticInterface(f.Owner.Name, "access$"+name, "(L"+f.Owner.Name+";"+d[1:])
		} else {
			g.code.Invoke(cf.INVOKESTATIC, f.Owner.Name, "access$"+name, "(L"+f.Owner.Name+";"+d[1:])
		}
	}
}

// accessor returns the name of the accessor of m: access$name, and for a
// supertype's method access$name$s and the hash of its simple name, as
// kotlinc names them.
func (m privateMethod) accessor() string {
	if m.super == "" {
		return "access$" + m.name
	}
	n := m.super[strings.LastIndexAny(m.super, "/$")+1:]
	var h int32
	for _, r := range utf16.Encode([]rune(n)) {
		h = 31*h + int32(r)
	}
	return "access$" + m.name + "$s" + strconv.Itoa(int(h))
}

// protectedFun reports whether method f is protected on the JVM.
func protectedFun(f *types.FunSym) bool {
	if f.Protected || f.Inherited == types.InheritProtected && !f.PublicOnJVM {
		return true
	}
	fd, ok := f.Decl.(*syntax.FunDecl)
	return ok && fd.Visibility == syntax.Protected
}

// protectedAccess calls protected method name of class owner, with
// descriptor d, on the receiver below the arguments, through a static
// accessor of the class that may call it, if the code is in another
// class: a lambda's, an object's or a nested class's in a subclass of
// owner in another package, which the JVM doesn't let call it, or a
// class's whose companion is the subclass, on the companion. As with
// kotlinc, the body of an inline function that isn't private, which
// classes of other packages copy, and its copies call it through an
// accessor of the function's class too. It reports whether it did.
func (g *gen) protectedAccess(owner *types.ClassSym, name, d string) bool {
	var host *types.ClassSym
	switch {
	case owner == nil:
	case g.publicCopy() != nil:
		host = protectedHost(g.publicCopy().Owner, owner)
	case g.publicInline:
		host = protectedHost(g.owner, owner)
	case pkgPath(owner.Name) != pkgPath(g.cls.Name):
		if k := protectedHost(g.owner, owner); k != nil && k.Name != g.cls.Name {
			host = k
		}
	}
	if host == nil {
		return false
	}
	m := privateMethod{class: host.Name, name: name, desc: d, virtual: true}
	g.shared.methods[m] = true
	g.code.Invoke(cf.INVOKESTATIC, host.Name, m.accessor(), "(L"+host.Name+";"+d[1:])
	return true
}

// superAccess calls method name of supertype sup, with descriptor d, on
// this below the arguments, as super.name() does, through a static
// accessor of the subclass whose super it is, if the code is in another
// class: invokespecial only calls the current class's supertypes. It
// reports whether it did.
func (g *gen) superAccess(sup *types.ClassSym, name, d string) bool {
	host := g.subclassHost(sup)
	if host == nil {
		return false
	}
	m := privateMethod{class: host.Name, name: name, desc: d, super: sup.Name, iface: sup.Interface}
	g.shared.methods[m] = true
	g.code.Invoke(cf.INVOKESTATIC, host.Name, m.accessor(), "(L"+host.Name+";"+d[1:])
	return true
}

// subclassHost returns the class the code is in, or encloses it, that is
// a subclass of cls, if it isn't the class being generated: the code is
// then in a lambda's class, an object's or a nested class's, and the
// subclass, of the same file, has the accessors of cls's members it uses.
func (g *gen) subclassHost(cls *types.ClassSym) *types.ClassSym {
	if k := subclassOf(g.owner, cls); k != nil && k.Name != g.cls.Name {
		return k
	}
	return nil
}

// protectedHost returns class k, or the class enclosing it, that is a
// subclass of cls, or else the companion of one of them that is, if it
// is of the program: B's code calls the protected members of A on B's
// companion object : A().
func protectedHost(k, cls *types.ClassSym) *types.ClassSym {
	if s := subclassOf(k, cls); s != nil || k == nil {
		return s
	}
	for ; k != nil; k = k.Outer {
		if c := k.Companion; c != nil && !c.External && c.IsSubclassOf(cls) {
			return c
		}
	}
	return nil
}

// subclassOf returns class k, or the class enclosing it, that is a
// subclass of cls, if it is of the program.
func subclassOf(k, cls *types.ClassSym) *types.ClassSym {
	for ; k != nil; k = k.Outer {
		if k.IsSubclassOf(cls) {
			if k.External {
				return nil
			}
			return k
		}
	}
	return nil
}

// publicCopy returns the inline member that isn't private whose copy, or
// a copy in whose body, the code is.
func (g *gen) publicCopy() *types.FunSym {
	for i := len(g.srcInl) - 1; i >= 0; i-- {
		if f := g.srcInl[i].d.Sym; !f.Private && f.Owner != nil {
			return f
		}
	}
	return nil
}

// pkgPath returns the package of a JVM class name: a/b of a/b/C.
func pkgPath(class string) string {
	if i := strings.LastIndexByte(class, '/'); i >= 0 {
		return class[:i]
	}
	return ""
}

// selfStatic reports whether f is the serialization plugin's write$Self,
// a static method that takes the instance first.
func selfStatic(f *types.FunSym) bool {
	fd, ok := f.Decl.(*syntax.FunDecl)
	return ok && fd.SelfStatic
}

// implsPrivate reports whether f is a private member of an interface
// compiled without JVM default methods: a private static method of its
// DefaultImpls class, which takes the instance first.
func implsPrivate(f *types.FunSym) bool {
	fd, ok := f.Decl.(*syntax.FunDecl)
	return ok && fd.Visibility == syntax.Private && f.Owner != nil && f.Owner.HasDefaultImpls()
}

// callStatic calls a top-level function of the program, as callMethod a
// method: a private one through its accessor from another class.
func (g *gen) callStatic(f *types.FunSym, facade, name, d string) {
	switch {
	case !f.Private || f.Desc != "":
	case facade != g.cls.Name:
		g.shared.methods[privateMethod{class: facade, name: name, desc: d, static: true}] = true
		name = "access$" + name
	case g.publicInline:
		g.shared.methods[privateMethod{class: facade, name: name, desc: d, static: true}] = true // for copies
	}
	g.code.Invoke(cf.INVOKESTATIC, facade, name, d)
}

// invokeVirtual calls a method of the program on a receiver of class cls,
// below the arguments: invokeinterface for an interface.
func (g *gen) invokeVirtual(cls *types.ClassSym, name, d string) {
	if cls.Interface {
		g.code.InvokeInterface(cls.Name, name, d)
	} else {
		g.code.Invoke(cf.INVOKEVIRTUAL, cls.Name, name, d)
	}
}

// directGet and directSet report whether code reads or writes a
// property's field directly rather than through its getter or setter: as
// kotlinc does, in the class that declares it, if the accessor is a
// default one that can't be overridden. The body of a public inline
// function or accessor, which other classes copy, calls the accessors of
// the properties that aren't private.
func (g *gen) directGet(p *types.VarSym) bool {
	if p.Lateinit && !p.Private {
		return false // its getter checks it, as kotlinc calls it
	}
	direct := p.JvmField || g.inClass(p.Owner) && (!g.publicInline || p.Private) && (p.Final || p.Private || p.Deferred && !p.Mutable) && p.HasField() && p.Getter == nil && !p.Const
	if direct && !p.JvmField {
		g.forCopies(p, 1)
	}
	return direct
}

// lateinitAccess returns the access flags of a lateinit property's field,
// which has its setter's visibility: internal ones are public.
func lateinitAccess(pd *syntax.PropDecl) uint16 {
	vis := syntax.Public
	if pd != nil {
		vis = pd.Visibility
		if pd.SetterVis != syntax.Public {
			vis = pd.SetterVis
		}
	}
	switch vis {
	case syntax.Private:
		return cf.AccPrivate
	case syntax.Protected:
		return cf.AccProtected
	}
	return cf.AccPublic
}

func (g *gen) directSet(p *types.VarSym) bool {
	in := g.inClass(p.Owner) || g.companionInits && p.Owner == g.owner
	direct := p.JvmField || in && (!g.publicInline || p.Private) && (p.Final || p.Private || p.SetterPrivate || p.Deferred && !p.Mutable) && p.HasField() && p.Setter == nil && !p.Const
	if direct && !p.JvmField {
		g.forCopies(p, 2)
	}
	return direct
}

// backingField loads or, with load false, stores the field of property p
// in its accessors, field: a static one of the file facade at the top
// level, or else this's. A store's value is on the stack, and for a class
// property this under it.
func (g *gen) backingField(p *types.VarSym, load bool) {
	ft := types.FieldType(p)
	d := desc(ft)
	switch {
	case p.Kind == types.Global && load:
		g.code.FieldInsn(cf.GETSTATIC, p.FieldClass(), p.Name, d)
	case p.Kind == types.Global:
		g.coerce(p.Type, ft)
		g.code.FieldInsn(cf.PUTSTATIC, p.FieldClass(), p.Name, d)
		return
	case load && p.StaticField():
		g.code.FieldInsn(cf.GETSTATIC, p.ConstOwner().Name, p.Name, d)
	case load:
		g.loadThis()
		g.code.FieldInsn(cf.GETFIELD, p.Owner.Name, p.Name, d)
	default:
		g.putField(p)
		return
	}
	g.coerce(ft, p.Type)
}

// getField reads class property p's field, of the object on the stack,
// as putField writes it, the value on the stack above the object: the
// object's own, or a static one, without it.
func (g *gen) getField(p *types.VarSym) {
	ft := types.FieldType(p)
	d := desc(ft)
	if p.StaticField() {
		g.code.Op(cf.POP)
		g.code.FieldInsn(cf.GETSTATIC, p.ConstOwner().Name, p.Name, d)
	} else {
		g.code.FieldInsn(cf.GETFIELD, p.Owner.Name, p.Name, d)
	}
	g.coerce(ft, p.Type)
}

func (g *gen) putField(p *types.VarSym) {
	ft := types.FieldType(p)
	d := desc(ft)
	g.coerce(p.Type, ft)
	if p.StaticField() {
		g.dropUnder(ft)
		g.code.FieldInsn(cf.PUTSTATIC, p.ConstOwner().Name, p.Name, d)
		return
	}
	g.code.FieldInsn(cf.PUTFIELD, p.Owner.Name, p.Name, d)
}

// dropUnder drops the one-word value under a value of type t on the stack:
// the receiver of a static field's or accessor's store.
func (g *gen) dropUnder(t types.Type) {
	if size(t) == 2 {
		g.code.Op(cf.DUP2_X1)
		g.code.Op(cf.POP2)
	} else {
		g.code.Op(cf.SWAP)
	}
	g.code.Op(cf.POP)
}

// swapUnder moves the one-word value on top of the stack below the value
// of type t under it.
func (g *gen) swapUnder(t types.Type) {
	if size(t) == 2 {
		g.code.Op(cf.DUP_X2)
		g.code.Op(cf.POP)
	} else {
		g.code.Op(cf.SWAP)
	}
}

// inClass reports whether the code being generated is in class c's own
// class file: not in a suspend lambda's class, though its owner is c.
func (g *gen) inClass(c *types.ClassSym) bool {
	return c != nil && c == g.owner && g.cls.Name == c.Name
}

// privateGetter and privateSetter are the names of the static accessors
// of private property p, which other classes, such as its class's
// companion, call.
func privateGetter(p *types.VarSym) string { return "access$" + types.GetterName(p.Name) + "$p" }
func privateSetter(p *types.VarSym) string { return "access$" + types.SetterName(p.Name) + "$p" }

// getProp calls the getter of property p on a receiver of class cls on the
// stack, or its static accessor if p is private.
func (g *gen) getProp(cls *types.ClassSym, p *types.VarSym) {
	ft := types.FieldType(p)
	d := desc(ft)
	switch {
	case p.Const:
		g.code.Op(cf.POP)
		g.constProp(p)
		return
	case p.JvmField:
		g.getField(p)
		return
	case p.Owner != nil && p.Owner.Value && p == p.Owner.Underlying && !underlyingGetter(p.Owner):
		g.code.Invoke(cf.INVOKEVIRTUAL, p.Owner.Name, "unbox-impl", "()"+d) // the box's value
		return
	case p.JvmStatic && p.Owner.CompanionOf == nil && !p.Private:
		// An object's @JvmStatic property: its getter is static.
		g.code.Op(cf.POP)
		g.code.Invoke(cf.INVOKESTATIC, p.Owner.Name, types.GetterJVM(p), "()"+desc(types.GetterType(p)))
		g.coerce(types.GetterType(p), p.Type)
		return
	}
	if p.Private {
		g.shared.access[p] |= 1
		if p.StaticField() {
			g.code.Op(cf.POP)
			g.code.Invoke(cf.INVOKESTATIC, p.Owner.Name, privateGetter(p), "()"+d)
		} else {
			g.code.Invoke(cf.INVOKESTATIC, p.Owner.Name, privateGetter(p), "(L"+p.Owner.Name+";)"+d)
		}
		g.coerce(ft, p.Type)
		return
	}
	if p.Owner != nil && p.Owner.Annotation {
		if jd := annotationJVM(p.Type); jd != d {
			// A KClass, or an array of them, is a java.lang.Class in an
			// annotation, which the standard library's Reflection converts.
			g.invokeVirtual(cls, p.JVMGetter, "()"+jd)
			conv, cd := "getOrCreateKotlinClass", "(Ljava/lang/Class;)Lkotlin/reflect/KClass;"
			if jd[0] == '[' {
				conv, cd = "getOrCreateKotlinClasses", "([Ljava/lang/Class;)[Lkotlin/reflect/KClass;"
			}
			g.code.Invoke(cf.INVOKESTATIC, "kotlin/jvm/internal/Reflection", conv, cd)
			return
		}
	}
	gd := composeGetterDesc(p, "()"+desc(types.GetterType(p)))
	if p.Getter != nil {
		g.composeArgs(p.Getter, 0, nil)
	}
	switch {
	case protectedProp(p, false) && g.protectedAccess(p.Owner, types.GetterJVM(p), gd):
	case p.Getter != nil:
		g.callMethod(p.Getter, cls, types.GetterJVM(p), gd)
	default:
		g.invokeVirtual(cls, types.GetterJVM(p), gd)
	}
	g.coerce(types.GetterType(p), p.Type)
}

// protectedProp reports whether the getter of property p of the program,
// or with set, its setter, is protected on the JVM.
func protectedProp(p *types.VarSym, set bool) bool {
	pd, _ := p.Decl.(*syntax.PropDecl)
	if cp, ok := p.Decl.(*syntax.CtorParam); pd != nil && pd.Visibility == syntax.Protected || ok && cp.Visibility == syntax.Protected || p.InheritsProtected {
		return true
	}
	return set && pd != nil && pd.SetterVis == syntax.Protected
}

// setProp calls the setter of property p, as getProp the getter.
func (g *gen) setProp(cls *types.ClassSym, p *types.VarSym) {
	if p.JvmField {
		g.putField(p)
		return
	}
	ft := types.FieldType(p)
	d := desc(ft)
	g.coerce(p.Type, ft)
	switch {
	case p.JvmStatic && p.Owner.CompanionOf == nil && !p.Private && !p.SetterPrivate:
		g.dropUnder(ft)
		g.code.Invoke(cf.INVOKESTATIC, p.Owner.Name, types.SetterJVM(p), "("+d+")V")
		return
	}
	if p.Private || p.SetterPrivate {
		g.shared.access[p] |= 2
		if p.StaticField() {
			g.dropUnder(ft)
			g.code.Invoke(cf.INVOKESTATIC, p.Owner.Name, privateSetter(p), "("+d+")V")
			return
		}
		g.code.Invoke(cf.INVOKESTATIC, p.Owner.Name, privateSetter(p), "(L"+p.Owner.Name+";"+d+")V")
		return
	}
	switch sd := "(" + d + ")V"; {
	case protectedProp(p, true) && g.protectedAccess(p.Owner, types.SetterJVM(p), sd):
	case p.Setter != nil:
		g.callMethod(p.Setter, cls, types.SetterJVM(p), sd)
	default:
		g.invokeVirtual(cls, types.SetterJVM(p), sd)
	}
}

// recvClass returns the class of a receiver of type t, by whose name its
// methods are called.
func recvClass(t types.Type) *types.ClassSym {
	t = t.NonNull()
	if t.IsParam() {
		t = types.Erase(t).NonNull()
	}
	return t.C
}
