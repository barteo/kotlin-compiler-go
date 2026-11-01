package syntax

import "github.com/barteo/kotlin-compiler-go/internal/types"

// File is a parsed source file.
type File struct {
	Path    string
	Package string // dotted package name; "" for the default package
	JvmName string // the facade's simple name from @file:JvmName, if any
	// Annotations are the file's annotations, @file:X, for its facade.
	Annotations []Annotation
	Facade      string // JVM internal name of the class holding top-level declarations
	// Part is set on a file annotated @file:JvmMultifileClass: the class
	// that holds its top-level declarations, Facade__FileKt, whose
	// non-private ones Facade, the multi-file class, calls. Others call
	// them through Facade, and the private ones in the file directly.
	Part    string
	Imports []*Import
	Decls   []Decl
	// Expects are the file's expect declarations. kc compiles for the
	// JVM only, where the actual declarations are the ones that count, so
	// these are only checked against them.
	Expects []Decl
	// Errors are the syntax errors parsing went on after, as kotlinc
	// does: context parameters where Kotlin doesn't allow them, which the
	// checker reports.
	Errors []*Error
	// SuppressInvisible is set on a file with an annotation, on any
	// declaration or expression, that suppresses INVISIBLE_MEMBER or
	// INVISIBLE_REFERENCE: its code may call the class path's internal
	// top-level functions, as ktor's calls kotlinx.coroutines's
	// recoverStackTrace.
	SuppressInvisible bool
	// SuppressShadowed is set on a file with an annotation that
	// suppresses RECEIVER_SHADOWED_BY_CONTEXT_PARAMETER, an error kotlinc
	// lets code suppress: the checker doesn't report it in the file.
	SuppressShadowed bool
	// SuppressDSL is set on a file with an annotation that suppresses
	// DSL_SCOPE_VIOLATION, which the checker then doesn't report.
	SuppressDSL bool
	// Annotated are the annotations of statements and expressions, by
	// node: of local variables and destructuring declarations, loops,
	// assignments, and expressions, @A x, which the checker checks but
	// class files don't keep.
	Annotated map[any][]Annotation
	// Arities is set by the checker: bit n is set if the file uses
	// function types with n parameters.
	Arities uint32
	// Anon are the classes of the file's object expressions, which the
	// checker adds.
	Anon []*ClassDecl
	// Named are the names the checker gives the file's anonymous
	// classes: those of object expressions, suspend lambdas and
	// continuations, which code generation's names for other lambdas'
	// classes skip.
	Named map[string]bool
	// KProps are the KProperty objects of the delegated properties, top
	// level and local, whose delegates use them, in the facade's
	// $$delegatedProperties array.
	KProps []*PropRef
}

// Class returns the class that holds the file's top-level declarations:
// its facade, or its part of a multi-file class.
func (f *File) Class() string {
	if f.Part != "" {
		return f.Part
	}
	return f.Facade
}

// HasFacade reports whether file f has a facade class: whether it declares
// top-level functions, properties or type aliases, but for expect ones.
func (f *File) HasFacade() bool {
	for _, d := range f.Decls {
		switch d.(type) {
		case *FunDecl, *PropDecl, *TypeAliasDecl:
			return true
		}
	}
	return false
}

// Import is `import a.b.C`, `import a.b.C as D` or `import a.b.*`.
type Import struct {
	P     Pos
	Path  string // dotted, without the .*
	Star  bool
	Alias string
}

type Decl interface{ declNode() }

type TypeRef struct {
	P        Pos
	Name     string     // may be qualified: java.util.Random
	Args     []*TypeArg // as written; nil if there are none
	Nullable bool
	// Known is set on a type of a declaration the checker writes: the
	// type, which needs no resolving.
	Known *types.Type

	// A function type (Params) -> Ret has a Ret and no Name, and a
	// function type with a receiver, Recv.(Params) -> Ret, a Recv.
	Params []*TypeRef
	Ret    *TypeRef
	Recv   *TypeRef
	// Suspend is set on a suspend function type: suspend (A) -> R.
	Suspend bool
	// Contexts are the context parameters' types of a context function
	// type, context(A, B) R.(C) -> T, which on the JVM takes them first.
	Contexts []*TypeRef
	// Composable is set on a type annotated @Composable, as Compose's
	// composable function types are.
	Composable bool
	// Outer is set on a type of an inner class written with the outer
	// class's type arguments, Outer<A>.Inner: Outer<A>. Name is then the
	// qualified name, Outer.Inner, and Args the inner class's own.
	Outer *TypeRef
	// UnsafeVariance is set on a type annotated @UnsafeVariance, whose
	// type parameters' variance isn't checked.
	UnsafeVariance bool
	// NoInfer is set on a type annotated @kotlin.internal.NoInfer, which
	// doesn't infer a call's type arguments.
	NoInfer bool
	// Wildcards is what the type's annotations say of the wildcards of
	// generic signatures: SuppressWildcards for @JvmSuppressWildcards,
	// ForceWildcards for @JvmWildcard or @JvmSuppressWildcards(false).
	Wildcards Wildcards
	// And is the right part of an intersection type, T & Any, a
	// definitely non-nullable type: the type written before & is this
	// TypeRef without it.
	And *TypeRef
}

// Wildcards says whether the generic signatures of a declaration or a
// type have wildcards for declaration-site variance.
type Wildcards int8

const (
	DefaultWildcards Wildcards = iota
	SuppressWildcards
	ForceWildcards
)

// WildcardsOf returns what annotations as say of wildcards: those of a
// type, or of a declaration, which @JvmSuppressWildcards can annotate.
func WildcardsOf(as []Annotation) Wildcards {
	for i := range as {
		a := &as[i]
		switch {
		case a.Target != "":
		case a.Is("JvmSuppressWildcards"):
			if arg := a.Arg(0, "suppress"); arg != nil && len(arg.Toks) == 1 && arg.Toks[0].Text == "false" {
				return ForceWildcards
			}
			return SuppressWildcards
		case a.Is("JvmWildcard"):
			return ForceWildcards
		}
	}
	return DefaultWildcards
}

// TypeArg is a type argument: T, out T, in T or *.
type TypeArg struct {
	P        Pos
	Variance types.Variance // Inv, Out, In or Star
	Type     *TypeRef       // nil for *
}

// TypeParam is a declared type parameter: T, out T : Bound.
type TypeParam struct {
	P        Pos
	Name     string
	Variance types.Variance // Inv, Out or In
	// Bounds are its upper bounds, none for Any?: the one declared with
	// it, then those of the declaration's where clause, in their order.
	Bounds  []*TypeRef
	Sym     *types.TypeParamSym
	Reified bool
}

type Param struct {
	P       Pos
	Name    string
	Type    *TypeRef // the element type of a vararg parameter
	Vararg  bool
	Default Expr // the default value, or nil
	Sym     *types.VarSym
	// Noinline and Crossinline are set on parameters of inline functions:
	// a lambda passed for a noinline one isn't inlined, and one passed
	// for a crossinline one can't return from the caller.
	Noinline, Crossinline bool
	Annotations           []Annotation
	// External is set on a parameter marked external, which is an error.
	External  bool
	ExternalP Pos
}

// Modifiers are the modifiers of a declaration that kc uses.
type Modifiers struct {
	Open, Abstract, Override, Final bool
	Companion, Data, Enum, Sealed   bool
	Operator, Infix                 bool // only on functions
	Vararg                          bool // only on parameters
	VarargP                         Pos
	Expect, Actual                  bool
	ExpectP                         Pos
	Annotation                      bool // only on classes
	Value                           bool // only on classes
	Inline                          bool // on functions, and the deprecated inline classes
	Noinline, Crossinline           bool // only on parameters
	Suspend                         bool // only on functions
	Tailrec                         bool // only on functions
	Lateinit                        bool // only on properties
	Inner                           bool // only on classes
	// External is set on an external function or accessor, a native
	// method without a body, and on the declarations it is an error on.
	External  bool
	ExternalP Pos
	// JvmInline is set if the declaration is annotated @JvmInline, which a
	// value class needs on the JVM.
	JvmInline  bool
	Const      bool
	Visibility Visibility
	// Explicit is set if the declaration has a visibility modifier: an
	// override without one has the overridden member's visibility.
	Explicit bool
	// Annotations are the declaration's annotations, with their
	// use-site targets (@get:JvmName applies to a property's getter).
	Annotations []Annotation
	// Contexts are the context parameters of a function or a property,
	// context(a: A, _: B) fun f(), whose names may be _. ContextP is the position of
	// context, if it is written.
	Contexts []*Param
	ContextP Pos
}

// Visibility is a declaration's visibility modifier, which the checker
// checks uses of (see check/visibility.go), and the JVM flags and names
// kc writes depend on.
type Visibility uint8

const (
	Public Visibility = iota // also the default
	Private
	Protected
	Internal
)

// An Annotation is kept as written: its name, its use-site target and the
// tokens of its arguments.
type Annotation struct {
	P      Pos
	Target string // use-site target: file, get, set, field, param, ...; or empty
	Name   string // as written: JvmName or kotlin.jvm.JvmName
	Args   []AnnotationArg
	// TypeArgs are the explicit type arguments of a generic annotation
	// class: @A<String>(..).
	TypeArgs []*TypeArg
	// Implicit is set on an annotation that isn't written but that
	// kotlinc adds, as @JvmInline to an inline class: class files keep it,
	// but the declaration's metadata doesn't count it.
	Implicit bool
	// Accessor is set on an annotation written on an accessor, @A get(),
	// which the property keeps with Target get or set.
	Accessor bool

	// Set by the checker on an annotation that class files keep (its
	// class's retention isn't SOURCE): its class, and its arguments by
	// parameter, in the parameters' order.
	Class  *types.ClassSym
	Values []AnnotationValue
	// Source is set instead of Class on an annotation of a class whose
	// retention is SOURCE: class files don't keep it, but a property's
	// still gets a $annotations method, as with kotlinc.
	Source *types.ClassSym
}

// AnnotationValue is an argument of an annotation that class files keep:
// the value of the annotation class's parameter Name, of type Type. An
// array parameter's value has its elements; another has one.
type AnnotationValue struct {
	Name  string
	Type  types.Type
	Array bool
	Elems []AnnotationElem
}

// AnnotationElem is a value, or an element of an array value: a checked
// expression, or an annotation (ReplaceWith("x") in @Deprecated).
type AnnotationElem struct {
	X      Expr
	Nested *Annotation
}

type AnnotationArg struct {
	Name string  // the argument's name, if it is named
	Toks []Token // the argument's tokens, without newlines
}

// Is reports whether a names the annotation name, by its simple name or
// qualified in kotlin or kotlin.jvm: Is("JvmName") matches @JvmName and
// @kotlin.jvm.JvmName.
func (a *Annotation) Is(name string) bool {
	return a.Name == name || a.Name == "kotlin."+name || a.Name == "kotlin.jvm."+name
}

// Arg returns the argument named name, or else the i-th positional
// argument, or nil.
func (a *Annotation) Arg(i int, name string) *AnnotationArg {
	for j := range a.Args {
		if a.Args[j].Name == name && name != "" {
			return &a.Args[j]
		}
	}
	n := 0
	for j := range a.Args {
		if a.Args[j].Name != "" {
			break // positional arguments come first
		}
		if n == i {
			return &a.Args[j]
		}
		n++
	}
	return nil
}

// Str returns the first argument, or the argument named name, if it is
// a string literal without templates.
func (a *Annotation) Str(name string) (string, bool) {
	arg := a.Arg(0, name)
	if arg == nil || len(arg.Toks) != 1 || arg.Toks[0].Kind != STRING {
		return "", false
	}
	s := ""
	for _, part := range arg.Toks[0].Parts {
		if part.IsExpr {
			return "", false
		}
		s += part.Lit
	}
	return s, true
}

// Find returns the annotation named name with the use-site target target,
// or nil.
func (m *Modifiers) Find(target, name string) *Annotation {
	for i := range m.Annotations {
		if a := &m.Annotations[i]; a.Target == target && a.Is(name) {
			return a
		}
	}
	return nil
}

// Hidden reports whether the declaration is @Deprecated(level = HIDDEN):
// it is compiled, but invisible to callers.
func (m *Modifiers) Hidden() bool {
	a := m.Find("", "Deprecated")
	if a == nil {
		return false
	}
	arg := a.Arg(2, "level")
	return arg != nil && len(arg.Toks) > 0 && arg.Toks[len(arg.Toks)-1].Text == "HIDDEN"
}

type FunDecl struct {
	Modifiers
	P          Pos
	Name       string
	TypeParams []*TypeParam
	Recv       *TypeRef // an extension function's receiver type, or nil
	// RecvAnnotations are the annotations on the receiver type with the
	// use-site target receiver: fun @receiver:A T.f().
	RecvAnnotations []Annotation
	// RecvSym is set by the checker on an extension function: its
	// receiver, this in its body, which the JVM method takes first.
	RecvSym  *types.VarSym
	Params   []*Param
	Ret      *TypeRef // nil: Unit for block bodies, inferred for expression bodies
	Body     *Block   // block body, or
	ExprBody Expr     // expression body; neither for an abstract function
	Sym      *types.FunSym
	// AccessorOf is set by the checker on a property's getter or setter:
	// the property, whose backing field is field in the body.
	AccessorOf *types.VarSym
	// Property is the name of the property whose getter or setter it is.
	Property string
	// Inlinable is set by the checker on an inline function that calls
	// inline: whether the lambda passed for each parameter is inlined,
	// which the function only invokes.
	Inlinable []bool
	// Continuation is set by the checker on a suspend function: the name
	// of the class of its continuations, numbered as object expressions
	// are, before those in its body. Entry is set on suspend fun main():
	// the class of the lambda that the JVM entry point runs it with.
	Continuation, Entry string
	// Once names the parameters that the function's contract, which the
	// checker drops, says it calls exactly once.
	Once map[string]bool
	// Implies are the contract's returns(value) implies (p != null) and
	// (p is T) effects.
	Implies []Implies
	// Plugin is set on a function that kc declares as a compiler plugin
	// of kotlinc's would: serialization's serializer().
	Plugin bool
	// SelfStatic is set on the serialization plugin's write$Self: a
	// member whose JVM method is static, and takes this first, as self.
	SelfStatic bool
	// Group is set by the checker on a composable function with a
	// return before its end: its body, if it has no restart group, runs
	// in a replace group.
	Group bool
}

// SourceInline returns the declaration of f if a call of f copies its
// body: a top-level or extension inline function of the program.
func SourceInline(f *types.FunSym) *FunDecl {
	if f == nil || f.Owner != nil && (!f.Final || f.Owner.Interface || f.Owner.Value) {
		return nil
	}
	d, ok := f.Decl.(*FunDecl)
	if !ok || !d.Inline || d.Body == nil && d.ExprBody == nil || d.Inlinable == nil {
		return nil
	}
	return d
}

type PropDecl struct {
	Modifiers
	P          Pos
	Name       string
	Mutable    bool
	TypeParams []*TypeParam // only on an extension property: val <T> List<T>.x
	Recv       *TypeRef     // an extension property's receiver type, or nil
	Type       *TypeRef
	Init       Expr       // nil for an abstract property, or one with a getter
	Getter     *FunDecl   // get() = ..., or nil
	SetterVis  Visibility // private set, ...
	// InlineGetter and InlineSetter are set on inline get and inline set,
	// which compile as ordinary accessors.
	InlineGetter, InlineSetter bool
	// Plugin is set by the checker on a property it declares as a
	// compiler plugin's that metadata doesn't list: serialization's
	// $cachedSerializer$delegate.
	Plugin bool
	Sym    *types.VarSym

	// A delegated property, val x by d, is parsed as a private property
	// x$delegate = d, its Field, declared before it, and x with a getter
	// that calls x$delegate.getValue(this, ref) and, for a var, a setter
	// that calls x$delegate.setValue(this, ref, value), with null for this
	// at top level.
	Field      *PropDecl
	Setter     *FunDecl
	Ref        *PropRef
	DelegateOf *PropDecl // set on the field, x$delegate

	// RecvAnnotations are the annotations on the receiver type with the
	// use-site target receiver: val @receiver:A T.x.
	RecvAnnotations []Annotation
}

// TypeAliasDecl is a typealias, typealias Name<T> = Type, at the top
// level or nested in a class.
type TypeAliasDecl struct {
	Modifiers
	P          Pos
	Name       string
	TypeParams []*TypeParam
	Type       *TypeRef
	Sym        *types.AliasSym
}

// CtorParam is a primary constructor parameter; Prop is set for val/var.
type CtorParam struct {
	Modifiers
	Param
	IsProp  bool
	Mutable bool
	PropSym *types.VarSym
}

type ClassDecl struct {
	Modifiers
	P          Pos
	Name       string
	Interface  bool
	Fun        bool // a fun interface
	Object     bool // an object declaration, or a companion object
	TypeParams []*TypeParam
	CtorParams []*CtorParam
	Supers     []*SuperType
	// CtorMods are the primary constructor's modifiers and annotations:
	// class A private constructor(...).
	CtorMods Modifiers
	Members  []Decl     // *PropDecl, *FunDecl, *InitBlock, *ClassDecl
	KProps   []*PropRef // the class's $$delegatedProperties, as File.KProps
	// Anonymous is set on an object expression's class. Captures, set by
	// the checker, are the locals and receivers of the enclosing code that
	// its members use, which its constructor takes and keeps in fields;
	// OuterThis, if among them, stands for the enclosing class's this.
	Anonymous bool
	Captures  []*types.VarSym
	OuterThis *types.VarSym
	// Local is set on a class declared in a block, which, like an object
	// expression's, has Captures.
	Local bool
	// EnumEntry is set on the class of an enum entry with a body, a
	// subclass of the enum among its Members, which its name, the entry's,
	// doesn't refer to.
	EnumEntry bool
	// EnclosingCls and EnclosingFun are set by the checker on an object
	// expression's or a local class: the class or facade, and the
	// function, it is declared in (nil outside a function).
	EnclosingCls string
	EnclosingFun *types.FunSym
	// EnclosingInit is set by the checker on one declared in the
	// constructor code of a class that isn't an object, outside a
	// function: a property initializer or an init block. Its enclosing
	// method is the class's primary constructor.
	EnclosingInit *ClassDecl
	Entries       []*EnumEntry // an enum class's entries
	// EntriesList is set on an enum class when the standard library is
	// there: the class gets E.entries, an EnumEntries<E>.
	EntriesList bool
	Sym         *types.ClassSym
	Ctor        *types.FunSym
	// SuperCall is the superclass's constructor call, from Supers.
	SuperCall *Call
	// Delegated are the members that class delegation implements.
	Delegated []*Delegated
	// Forwarders are set on an interface compiled without JVM default
	// methods: the bodies it inherits, which its DefaultImpls class has
	// static methods that call, as kotlinc writes them.
	Forwarders []ImplsForwarder
	// InheritedBridges are set on a class whose superclass implements a
	// member of one of its interfaces with another JVM descriptor, as
	// val channel: Channel<E> does ProducerScope's val channel:
	// SendChannel<E>: the class has a bridge that calls it, as kotlinc
	// writes it.
	InheritedBridges []InheritedBridge
	// HasPrimary is set if the class has a primary constructor: declared
	// in its header, or the implicit one of a class without secondary
	// constructors.
	HasPrimary bool
	// Parcelize is set by the checker on a class annotated @Parcelize, to
	// which it adds the parcelize plugin's members: its static field
	// CREATOR holds an instance of its class Creator.
	Parcelize bool
	// Serial is set by the checker on a class annotated @Serializable, to
	// which it adds the kotlinx.serialization plugin's members.
	Serial *Serial
	// Synthetic is set by the checker on a class it declares as a
	// compiler plugin's, which is synthetic on the JVM: the
	// kotlinx.serialization plugin's $serializer.
	Synthetic bool
}

// Serial is what the kotlinx.serialization plugin adds to a class: its
// serializer, which writes and reads Props, and a synthetic constructor,
// Ctor, which takes them and sets their fields rather than call the
// primary constructor.
type Serial struct {
	// Name is the class's serial name: its qualified name, or its
	// @SerialName.
	Name string
	// Props are the class's own properties that its serializer writes, in
	// order: the primary constructor's, then the body's. Those of its
	// superclass, if it is @Serializable too (Super), come before them.
	Props []*SerialProp
	Super *ClassDecl
	Ctor  *SecondaryCtor
}

// All returns the properties that the class's serializer writes: its
// superclass's, then its own.
func (s *Serial) All() []*SerialProp {
	var out []*SerialProp
	if s.Super != nil {
		out = s.Super.Serial.All()
	}
	return append(out, s.Props...)
}

// SerialProp is a property of a class annotated @Serializable that its
// serializer writes: a primary constructor's, Param, or one declared in
// the class's body, Prop.
type SerialProp struct {
	Name     string // its serial name: its name, or its @SerialName
	Param    *CtorParam
	Prop     *PropDecl
	Optional bool // it has a default value, and isn't @Required
}

// Sym returns the property.
func (p *SerialProp) Sym() *types.VarSym {
	if p.Param != nil {
		return p.Param.PropSym
	}
	return p.Prop.Sym
}

// Default returns the property's default value, or its initializer.
func (p *SerialProp) Default() Expr {
	if p.Param != nil {
		return p.Param.Default
	}
	return p.Prop.Init
}

// Secondaries returns the class's secondary constructors.
func (cd *ClassDecl) Secondaries() []*SecondaryCtor {
	var out []*SecondaryCtor
	for _, m := range cd.Members {
		if sc, ok := m.(*SecondaryCtor); ok {
			out = append(out, sc)
		}
	}
	return out
}

// EnumEntry is an entry of an enum class: RED, or GREEN(0x00ff00).
type EnumEntry struct {
	P           Pos
	Name        string
	Call        *Call // the constructor call that creates it, with its arguments
	Annotations []Annotation
	// Body is the class of an entry with a body, whose superclass's
	// constructor call is Call; or nil.
	Body *ClassDecl
}

// SuperType is a supertype in a class header: B(x), or I.
type SuperType struct {
	Type     *TypeRef
	Call     *Call      // the superclass's constructor call: B(x); nil for an interface
	Delegate Expr       // the object an interface is delegated to: I by x; or nil
	T        types.Type // set by the checker
	// Field is set by the checker on a delegated interface: the field
	// that holds the delegate, $$delegate_N, or the constructor property
	// the delegate is.
	Field string
}

// ImplsForwarder is a member with a body, Fun, that an interface
// inherits from interface Of, whose DefaultImpls class Impls has it.
// InheritedBridge is a bridge with the JVM name and descriptor Bridge,
// f(Ljava/lang/Object;)V, to the method Target that a class inherits,
// which takes Params and returns Ret.
type InheritedBridge struct {
	Bridge, Target string
	Params         []*types.VarSym
	Ret            types.Type
	// Over is the interface's member, whose result may be a value class
	// that the bridge returns unboxed, as I's fun foo(): IC that A<IC>'s
	// fun foo(): T implements, and Fun the superclass's member.
	Over, Fun *types.FunSym
}

type ImplsForwarder struct {
	Fun   *types.FunSym
	Of    *types.ClassSym
	Impls string
}

// Delegated is a member that class delegation implements: it calls
// Target on the object in the field of the delegated supertype Super.
type Delegated struct {
	Sym    *types.FunSym // the method, or a delegated property's getter or setter
	Target *types.FunSym // the member of the interface it calls
	Super  *SuperType
	// Impls is set instead of Super on a method that calls the body of a
	// class-path Kotlin interface's member: the interface's DefaultImpls
	// class, whose static method takes the instance first.
	Impls string
	// ExtProp is the name of the extension property whose getter or
	// setter Sym is.
	ExtProp string
}

type InitBlock struct {
	P           Pos
	Body        *Block
	Annotations []Annotation // which are errors
}

// SecondaryCtor is a secondary constructor: constructor(x: Int) : this(x, 0)
// { ... }.
type SecondaryCtor struct {
	Modifiers
	P      Pos
	Params []*Param
	// Delegation is the call of this(...) or super(...), nil if omitted:
	// then super() is called. Its Fun is an Ident, this or super.
	Delegation *Call
	ToThis     bool
	Body       *Block // nil without a body
	Sym        *types.FunSym
	// Serial is set on the synthetic constructor that the serialization
	// plugin adds to a class: it takes the bits of the properties read,
	// the properties and a marker, and sets the properties' fields, the
	// default values of those not read.
	Serial *Serial
}

func (*FunDecl) declNode()       {}
func (*TypeAliasDecl) declNode() {}
func (*PropDecl) declNode()      {}
func (*ClassDecl) declNode()     {}
func (*InitBlock) declNode()     {}
func (*SecondaryCtor) declNode() {}

// Statements.

type Stmt interface{ stmtNode() }

// StmtPos returns the position of a statement, or the zero Pos.
func StmtPos(s Stmt) Pos {
	switch s := s.(type) {
	case *ExprStmt:
		return StartPos(s.X)
	case *LocalFun:
		return s.Decl.P
	case *VarStmt:
		return s.P
	case *Assign:
		if p := StartPos(s.Target); p.Line > 0 {
			return p
		}
		return s.P
	case *While:
		return s.P
	case *For:
		return s.P
	case *Destructure:
		return s.P
	case *Block:
		return s.P
	}
	return Pos{}
}

type Block struct {
	P     Pos
	End   Pos // the closing brace, if the block has braces
	Stmts []Stmt
	T     types.Type // value type: the last expression's type, Unit, or Nothing
}

type ExprStmt struct{ X Expr }

// LocalFun is a function declared in a block: fun g(x: Int) = x + 1.
// LocalClass is a class declared in a block.
type LocalClass struct {
	Decl *ClassDecl
	// Annotations are those written before it, which only the
	// serialization plugin reads: the class doesn't have them.
	Annotations []Annotation
}

type LocalFun struct {
	Decl *FunDecl
	// Set by the checker. Frame records what the function captures, as a
	// lambda's Captures and This do: the locals and parameters of the
	// enclosing functions it uses, also through the local functions it
	// calls, and whether it uses the enclosing class's this.
	Frame *Lambda
	// Method is set by codegen: the name of the static method the function
	// is, such as main$g, of class Owner, and Desc its descriptor.
	Method, Owner, Desc string
}

type VarStmt struct {
	P       Pos
	Name    string
	Mutable bool
	// Lateinit is set on a local lateinit var, written at LateinitP.
	Lateinit  bool
	LateinitP Pos
	Type      *TypeRef
	Init      Expr // nil for a variable assigned later
	Sym       *types.VarSym

	// A local delegated property, val x by d, keeps d in Delegate, which
	// the checker puts in a hidden local, DelegateSym: reading x is Get,
	// x$delegate.getValue(null, ref). Assigning a var is Set,
	// x$delegate.setValue(null, ref, v), with v in the hidden local
	// ValueSym.
	Delegate    Expr
	DelegateSym *types.VarSym
	Get, Set    *Call
	ValueSym    *types.VarSym
}

type Assign struct {
	P      Pos
	Op     Kind // ASSIGN, PLUS_EQ, ...
	Target Expr // *Ident, *Member or *Index
	Value  Expr
	OpType types.Type // operand type for compound assignment
	// Stmts replace a compound assignment that calls operators, as the
	// checker rewrites it: x.plusAssign(v), or x = x.plus(v) with the
	// receiver and indices of x in temporaries.
	Stmts []Stmt
}

// While is a while loop, or with Do, a do-while loop, whose condition is
// in the body's scope.
type While struct {
	P        Pos
	Label    string // loop@ while
	Do       bool
	Cond     Expr
	Body     *Block
	Infinite bool // while (true) with no break: the loop never completes
	// Group is set by the checker on a loop in composable code that calls
	// composables, and that composable code follows: it runs in a
	// replace group. BodyGroup is set if it is continued too: each run of
	// its body runs in one.
	Group, BodyGroup bool
}

type For struct {
	P     Pos
	Label string // loop@ for
	Var   string
	Names []string   // for ((a, b) in xs): the names of a destructuring declaration
	Types []*TypeRef // and their types, nil where omitted
	Type  *TypeRef   // for (x: T in xs): the variable's type, nil if omitted
	Sym   *types.VarSym
	Range Expr // *Binary with RANGE, RANGE_LT, UNTIL, DOWN_TO or STEP, or a collection
	Body  *Block

	// A loop over a collection calls Range's iterator(), and the
	// iterator's hasNext() and next(). Iter is nil for a range, and for
	// an iterator, which is its own. HasNext is nil for a range.
	Iter, HasNext, Next *types.FunSym
	IterT               types.Type // the iterator's type
	Group, BodyGroup    bool       // see While.Group
}

// Destructure is a destructuring declaration, val (a, b) = e. The checker
// turns it into Stmts: a temporary holding e, then val a =
// temp.component1(), and so on. _ declares nothing.
type Destructure struct {
	P       Pos
	Names   []string
	Types   []*TypeRef // val (a: A, b) = e: the names' types, nil where omitted
	Mutable bool
	Init    Expr
	Stmts   []*VarStmt
}

func (*Destructure) stmtNode() {}
func (*Block) stmtNode()       {}
func (*ExprStmt) stmtNode()    {}
func (*VarStmt) stmtNode()     {}
func (*Assign) stmtNode()      {}
func (*While) stmtNode()       {}
func (*For) stmtNode()         {}
func (*LocalFun) stmtNode()    {}
func (*LocalClass) stmtNode()  {}

// Expressions.

// StartPos returns the position of an expression's first token, as far as
// the tree tells: the receiver of a call chain, or the left operand of a
// binary expression. It is x.Pos() for most expressions.
func StartPos(x Expr) Pos {
	p := x.Pos()
	var first Expr
	switch x := x.(type) {
	case *Call:
		first = x.Fun
	case *Member:
		first = x.X
	case *Binary:
		first = x.X
	case *Index:
		first = x.X
	case *NotNull:
		first = x.X
	case *As:
		first = x.X
	case *Is:
		first = x.X
	case *Unary:
		if !x.Prefix {
			first = x.X
		}
	}
	if first != nil {
		if q := StartPos(first); q.Line > 0 && (p.Line == 0 || q.Line < p.Line || q.Line == p.Line && q.Col < p.Col) {
			return q
		}
	}
	return p
}

type Expr interface {
	Pos() Pos
	Type() types.Type
	SetType(types.Type)
}

type exprBase struct {
	P Pos
	T types.Type
}

func (e *exprBase) Pos() Pos             { return e.P }
func (e *exprBase) Type() types.Type     { return e.T }
func (e *exprBase) SetType(t types.Type) { e.T = t }

type Ident struct {
	exprBase
	Name string
	Sym  *types.VarSym
	// Getter is set for a property inherited from a Java or Kotlin class,
	// read through this: message in a subclass of Exception.
	Getter *types.FunSym
	// Cast is set when a smart cast changes the variable's class, not only
	// its nullability: the value read needs a cast to T.
	Cast bool
	// Member is set for a member of this that is only visible through a
	// smart cast of this, or of an object in scope (a companion): the
	// identifier is that member access.
	Member *Member
	// Object is set when the identifier names an object, or a class's
	// companion object, used as a value.
	Object *types.ClassSym
	// Unit is set when the identifier is the Unit object, or a local of
	// type Unit, which has no storage.
	Unit bool
	// Call is set on the read or the assignment of a property with
	// context parameters: the call of its getter or setter, which the
	// checker writes and which is evaluated instead.
	Call *Call
}

type This struct {
	exprBase
	Label string // A in this@A: a class, or a lambda with a receiver
	Cast  bool   // a smart cast changes the class of this: see Ident.Cast
	// Recv is set by the checker on this of a lambda with a receiver: the
	// local that holds the receiver.
	Recv *types.VarSym
	// Class is set on a this the checker writes for the instance of the
	// class the code is in, which lambdas' receivers don't hide.
	Class bool
	// Implicit is set on a this the checker writes for an implicit
	// receiver or context argument.
	Implicit bool
}

// Super is super, or super<I>, as the receiver of a member access or call,
// and with a label, super@Outer, an outer class's.
type Super struct {
	exprBase
	Iface *TypeRef        // I in super<I>, or nil
	Label string          // Outer in super@Outer, or ""
	Cls   *types.ClassSym // the supertype the member is looked up in
	// Outer is set by the checker on a super of an outer class: this@Outer,
	// on which the outer class's accessor of the supertype's member is
	// called.
	Outer *This
}

type IntLit struct {
	exprBase
	Val  int64 // an unsigned literal's bits
	Long bool  // L suffix, or too large for Int (UInt for an unsigned one)
	// Unsigned is set on an unsigned literal, 1u or 1uL.
	Unsigned bool
}

type DoubleLit struct {
	exprBase
	Val float64
}

// FloatLit is a Float literal: 1f, 2.5F.
type FloatLit struct {
	exprBase
	Val float32
}

// CharLit is a Char literal: 'a', '\n', '\u0041'.
type CharLit struct {
	exprBase
	Val rune // a UTF-16 code unit
}

// Implies is a contract's returns(value) implies (p != null) or (p is T)
// effect: Returns is 1 for returns(true), 0 for returns(false) and -1 for
// returns(); Param names the parameter, "this" the extension receiver;
// Is is T, or nil for p != null.
type Implies struct {
	Returns int
	Param   string
	Is      *TypeRef
	Holds   bool // implies (p): p, a Boolean, is true
}

type BoolLit struct {
	exprBase
	Val bool
}

type NullLit struct {
	exprBase
}

// ObjectExpr is an object expression, object : I { ... }: an instance of
// an anonymous class, Decl.
type ObjectExpr struct {
	exprBase
	Decl *ClassDecl
}

// PropRef is the KProperty of a delegated property, which its delegate's
// getValue and setValue take. The checker fills in where it is: kotlinc
// keeps a class's in a static array, $$delegatedProperties.
type PropRef struct {
	exprBase
	Name    string
	Mutable bool
	Recv    bool // a class's property, whose KProperty takes a receiver
	Ext     bool // an extension property, whose KProperty takes its receiver too
	// Sig is the getter's JVM name and descriptor, getX()I, or <v#N> for
	// the Nth local delegated property of the class or file.
	Sig string
	// Owner is the class with the array, and Index the KProperty's index
	// in it, or -1 when the delegate doesn't use it, so null is passed.
	Owner string
	Index int
}

type TmplPart struct {
	Lit string
	X   Expr // nil for literal parts
}

type StringLit struct {
	exprBase
	Parts []TmplPart
}

type Unary struct {
	exprBase
	Op Kind // MINUS, PLUS, NOT, INC, DEC
	X  Expr
	// Prefix is set on ++x and --x, whose value is the new one; x++ and
	// x-- have the old one.
	Prefix bool
	// Call is the operator call that -x, +x or !x is on other types than
	// numbers and Boolean: x.unaryMinus(), x.unaryPlus(), x.not().
	Call *Call
	// Stmts replace x++ or x-- on other types than numbers and Char, as
	// the checker rewrites it: x = x.inc(). As a value, they end with an
	// ExprStmt of the value.
	Stmts []Stmt
}

type Binary struct {
	exprBase
	Op     Kind
	X, Y   Expr
	OpType types.Type // operand type for arithmetic and comparisons
	// Call is the operator call of an arithmetic operator or a comparison
	// on other types than the built-in ones: x.plus(y), x.compareTo(y),
	// whose result is compared with 0; or of in: y.contains(x).
	Call *Call
	// Groups is set by the checker on &&, || and ?: in composable code
	// whose right side calls composables: it runs in a replace group, and
	// the code that skips it in an empty one.
	Groups bool
}

// ClassLit is a class literal: X::class, or x::class for the class of a
// value. Generic class names come with type arguments in Type
// (List<*>::class, in Ref); otherwise X holds the left side as parsed, a name or
// a value, and the checker decides which.
type ClassLit struct {
	exprBase
	X   Expr
	Ref *TypeRef

	// Set by the checker.
	Class types.Type // the class named, or the static type of the value
	Bound bool       // x::class: the value's run-time class
	Java  bool       // the java.lang.Class itself, for X::class.java
}

// Checked is an expression that the checker already checked, used again
// by a rewrite, as the receiver of an operator call: its type is X's, and
// it is generated as X.
type Checked struct {
	exprBase
	X Expr
	// SAM is set on a function value converted to a fun interface, T:
	// collect(handler) for a FlowCollector. It is the interface's method.
	SAM *types.FunSym
}

type Member struct {
	exprBase
	X       Expr
	Name    string
	Safe    bool
	Prop    *types.VarSym
	Builtin types.Builtin
	// Groups is set by the checker on a safe call in composable code that
	// calls composables: the call runs in a replace group, and a null
	// receiver's path in an empty one.
	Groups bool
	// Nothing is set by the checker on a safe call that, on a non-null
	// receiver, is of type Nothing: a generic one, which may return, as
	// x?.f() for a T := Nothing.
	Nothing bool

	// Object is set when the member access names an object, or a class's
	// companion object, used as a value: Outer.Companion, a.b.Obj.
	Object *types.ClassSym

	// Java members.
	Static *types.ClassSym // X names this class (Math in Math.max) and is not evaluated
	// EvalX is set on a static field read on an object's value with
	// effects, (if (c) A else B).CONST: X is evaluated and dropped.
	EvalX  bool
	Field  *types.VarSym // static field
	Getter *types.FunSym // getX() seen as property x
	Setter *types.FunSym // setX(v), if the property is assigned
	// Entries is set on E.entries of an enum E without getEntries(), a
	// Java enum's: Getter is E.values(), whose array enumEntries makes
	// the list.
	Entries bool

	// Dispatch is set on a call of an extension declared in an object,
	// or the read of such an extension property: the object is the
	// extension's dispatch receiver. DispatchThis is set where it is an
	// implicit receiver, a lambda's. Otherwise this is.
	Dispatch     *types.ClassSym
	DispatchThis *This
	// Cast is set when a smart cast of the property changes its class, as
	// Ident.Cast.
	Cast bool
	// Reified are the type arguments of a read of an extension property
	// with reified type parameters, whose getter it copies, as
	// Call.Reified.
	Reified []types.Type
	// Call is set on the read or the assignment of a property with
	// context parameters, as Ident.Call.
	Call *Call
}

// Index is x[i]. It calls the operator get, or set when it is assigned.
type Index struct {
	exprBase
	X    Expr
	Args []Expr
	Call *Call // x.get(args), or x.set(args, value) for an assignment
}

type Call struct {
	exprBase
	Fun      Expr       // *Ident or *Member, or any expression if Invoke
	TypeArgs []*TypeArg // explicit type arguments: f<Int>(x); a trailing lambda is the last of Args
	Args     []Expr
	Sym      *types.FunSym
	Ctor     *types.ClassSym // constructor call
	Builtin  types.Builtin
	Invoke   bool // Fun is a value of function type, and Sym its invoke
	SAM      bool // Runnable { ... }: the value is the lambda argument
	Infix    bool // a f b, which is a.f(b)
	Literal  bool // [a, b], a collection literal, which is arrayOf(a, b)
	Recv     bool // r.(f)(b), which is (f)(r, b): Fun is a value of a function type with a receiver
	// Outer is set by the checker on a call of an inner class's
	// constructor: the instance of the outer class it is in, which the
	// constructor takes first.
	Outer Expr
	// Tail is set by the checker on a tailrec function's call of itself
	// in tail position, which jumps back to the start of its body.
	Tail     bool
	Operator bool // a call that an operator makes: Sym must be operator
	// Group is set by the checker on a call in composable code of an
	// inline function that isn't composable, whose inlined lambda calls
	// composables, as on a loop (see While.Group).
	Group bool
	// ArgMap gives the parameter of Sym each argument is passed to, if
	// that is not the one at its position: with varargs, omitted default
	// arguments or a trailing lambda.
	ArgMap []int
	// Reified are the type arguments of a call of the program's inline
	// function with reified type parameters, set by the checker.
	Reified []types.Type
	// Vararg is the type of the array a vararg parameter's arguments are
	// passed in, with the call's type arguments: Array<out Int> for
	// listOf(1, 2), whose parameter is vararg elements: T.
	Vararg types.Type
	// Names are the names of named arguments, name = value, by position:
	// "" for a positional argument. It is nil without named arguments, and
	// may be shorter than Args.
	Names []string
	// Ctx is set by the checker on a call of a function with context
	// parameters: the context arguments, implicit values the call passes
	// first, before the receiver.
	Ctx []Expr
	// RefArgs is set by the checker on the call a callable reference's
	// lambda makes: the types of its arguments, the reference's
	// parameters, which go to the callee's as Kotlin adapts a reference
	// (see refNames).
	RefArgs  []types.Type
	trailing bool // has a trailing lambda, or passes the last argument last
}

// Trailing reports whether the last argument is passed to the last
// parameter: a lambda after the parentheses, or the value of an indexed
// assignment's call of set (see PassLast).
func (c *Call) Trailing() bool { return c.trailing }

// PassLast makes the last argument go to the last parameter, as an
// indexed assignment's value goes to set's: a[i] = v passes v after the
// indexes, with the default values or a vararg between, as a trailing
// lambda.
func (c *Call) PassLast() { c.trailing = true }

// AssertArgs splits the arguments of require, check, requireNotNull or
// checkNotNull into the checked ones and the lazy message, a () -> Any
// passed second or as a trailing lambda, or nil.
func AssertArgs(c *Call) (args []Expr, msg Expr) {
	args = c.Args
	if c.trailing || len(args) == 2 {
		return args[:len(args)-1], args[len(args)-1]
	}
	return args, nil
}

// Name returns the name of argument i if it is a named argument, or "".
func (c *Call) Name(i int) string {
	if i < len(c.Names) {
		return c.Names[i]
	}
	return ""
}

// LambdaParam is a lambda's parameter: x, x: Int, or _, or a
// destructuring declaration, (a, b). The checker names a destructured
// parameter and declares its components at the start of the body.
type LambdaParam struct {
	P     Pos
	Name  string
	Names []string   // the names of a destructured parameter's components
	Types []*TypeRef // and their types, nil where omitted
	Type  *TypeRef   // nil: from the expected type
	Sym   *types.VarSym
}

// Lambda is a lambda literal: { x, y -> body } or { body }.
type Lambda struct {
	exprBase
	Params []*LambdaParam
	Arrow  bool // -> was written, so Params are all there are (no `it`)
	Body   *Block
	Label  string // its label, lit@{ }, or the name of the function it is passed to, for return@label
	// SuspendMod is set on a lambda written with the suspend modifier,
	// suspend { x: Int -> x }, a suspend lambda whatever its expected type.
	SuspendMod bool

	// Set by the checker.
	// Ctx are the context parameters of a lambda whose expected type is
	// a context function type: its first parameters, before a receiver.
	Ctx      []*types.VarSym
	Recv     *types.VarSym   // the receiver of a lambda with a receiver, its first parameter
	It       *types.VarSym   // the implicit parameter
	Ret      types.Type      // the body's result type, as the function type sees it
	Captures []*types.VarSym // locals and parameters of enclosing functions it uses
	This     bool            // it uses the enclosing class's this
	SAM      *types.FunSym   // the Java interface method it implements, if converted; T is then the interface
	Suspend  bool            // a suspend lambda: its expected type is a suspend function type
	Class    string          // the class of a suspend lambda that isn't inlined
	// Inline is set on a lambda passed to an inline function that
	// invokes it: its body is generated where the function's inlined body
	// invokes it, and it has no object, method or captures.
	Inline bool
	// Crossinline is set on an inlined lambda passed for a crossinline
	// parameter, whose body isn't the code around the call's: Compose's
	// plugin doesn't remember lambdas in it.
	Crossinline bool
	// Once is set by the checker on a lambda passed to a function that
	// calls it exactly once, as its contract says: the vars it assigns
	// keep their narrowing at its start.
	Once bool
	// Ref is set on a callable reference, which is parsed as a lambda: the
	// checker gives it parameters and a body that calls the function, or
	// reads the property, it names.
	Ref *CallableRef
	// KProp is set on a reference to a property whose expected type is
	// one of kotlin.reflect's, or isn't a function type: its object is a
	// KProperty, whose get is the lambda.
	KProp *KProp
	// KFun is set on a reference to a function whose expected type is
	// kotlin.reflect's KFunctionN: its object is a FunctionReference,
	// which implements the function type and has the function's name.
	// It is the function's call, the body of the lambda. On a reference
	// converted to a fun interface, its object implements the interface
	// and FunctionAdapter, whose function delegate is a FunctionReference;
	// one whose method is suspend keeps its Class, without a KFunClass.
	KFun *Call
	// KFunClass is set by the checker with KFun: the name of the
	// reference's class, after the declarations it is in, as kotlinc
	// names it.
	KFunClass string
	// KFunAdapted is set with KFun on a reference that doesn't call the
	// function as its function type says, an AdaptedFunctionReference:
	// it omits default arguments, passes a vararg's elements, is of a
	// suspend function type or ignores the result. KFunFlags are its
	// flags, as kotlinc's: 2 for the vararg's elements, 4 for the suspend
	// function type, whose continuation invoke ignores, 8 for the ignored
	// result.
	KFunAdapted bool
	KFunFlags   int32
	// Anon is set on an anonymous function, fun (x: Int): Int { ... },
	// with its receiver and return types, if written; AnonUnit on one
	// with a block body and no return type, which returns Unit.
	Anon     bool
	AnonRecv *TypeRef
	AnonRet  *TypeRef
	AnonUnit bool
	// AnonCtx are the context parameters of an anonymous function,
	// context(a: A) fun () = a, whose symbols are Ctx's. The checker
	// names a lambda's it makes too, without types: see suspendConverted.
	AnonCtx []*Param
	// Annotated is set on a lambda annotated @Composable, which is
	// composable whatever its expected type, with -compose.
	Annotated bool
	// DontMemoize is set on a lambda annotated @DontMemoize, which
	// composable code doesn't remember.
	DontMemoize bool
	// Composable is set by the checker on a composable lambda: its
	// expected type is a composable function type, or it is Annotated.
	Composable bool
}

// CallableRef is a callable reference: ::f, C::m, x::m, ::C, or the same
// with a generic class name, Box<*>::get.
type CallableRef struct {
	X    Expr     // the left side as parsed, a class name or a value; nil for ::f
	Type *TypeRef // the left side when it has type arguments
	Name string

	// Set by the checker.
	Bound     *types.VarSym // a local holding a bound receiver, x in x::m
	BoundExpr Expr          // the value it is initialized with, evaluated once
}

// KProp is the property a property reference names, and how its
// KProperty object sets it.
type KProp struct {
	Prop   *types.VarSym // a class's or top-level property
	Getter *types.FunSym // or the getter of an extension property, or of a class path class's property
	// Bound is set on a reference with a receiver, x::p, which is the
	// lambda's capture Ref.Bound.
	Bound bool
	// Set is set on a var's: the lambda { p0, v -> p0.p = v } for C::p,
	// { v -> x.p = v } for x::p and ::p.
	Set *Lambda
}

// ParamSyms returns the lambda's parameters, including its context
// parameters and a receiver, which come first, and an implicit it.
func (l *Lambda) ParamSyms() []*types.VarSym {
	syms := l.Ctx[:len(l.Ctx):len(l.Ctx)]
	if l.Recv != nil {
		syms = append(syms, l.Recv)
	}
	if l.It != nil {
		return append(syms, l.It)
	}
	for _, p := range l.Params {
		syms = append(syms, p.Sym)
	}
	return syms
}

// RefName returns the name of the callable that reference l, checked,
// names: <init> for a constructor's, ::Foo.
func (l *Lambda) RefName() string {
	if n := len(l.Body.Stmts); n > 0 {
		if es, ok := l.Body.Stmts[n-1].(*ExprStmt); ok {
			if call, ok := es.X.(*Call); ok && call.Ctor != nil {
				return "<init>"
			}
		}
	}
	return l.Ref.Name
}

// Singleton reports whether the composable lambda l is one that Compose
// creates once, in its file's ComposableSingletons class, as kotlinc's
// plugin does: one that returns Unit, isn't inlined and captures nothing,
// as the checker found, nor calls a local function, or creates an
// instance of a local class, declared outside it, which the plugin
// counts as captures.
func (l *Lambda) Singleton() bool {
	if !l.Composable || l.Ret.K != types.Unit || l.Ret.N || l.Inline || l.SAM != nil || l.KProp != nil || l.This || len(l.Captures) > 0 {
		return false
	}
	inner := map[any]bool{}
	Inspect(l.Body, func(n any) bool {
		switch n := n.(type) {
		case *LocalFun:
			inner[n] = true
		case *LocalClass:
			inner[n.Decl] = true
		}
		return true
	})
	outer := false
	Inspect(l.Body, func(n any) bool {
		if c, ok := n.(*Call); ok {
			if c.Sym != nil {
				if lf, ok := c.Sym.Decl.(*LocalFun); ok && !inner[lf] {
					outer = true
				}
			}
			if cls := c.Ctor; cls != nil && cls.Local && !inner[cls.Decl] {
				outer = true
			}
		}
		return !outer
	})
	return !outer
}

type NotNull struct {
	exprBase
	X Expr
}

// Spread is *xs, an array passed to a vararg parameter. It is only parsed
// as a call argument.
type Spread struct {
	exprBase
	X Expr
}

type If struct {
	exprBase
	Cond       Expr
	Then, Else *Block // Else may be nil
	// Groups is set by the checker on an if in composable code whose
	// branches call composables: each runs in a replace group of its own,
	// a missing else in an empty one.
	Groups bool
}

type WhenCond struct {
	X      Expr       // nil for is and !is
	In     bool       // `in X`
	NotIn  bool       // `!in X`
	Call   *Call      // X.contains(subject), for in X when X isn't a range
	OpType types.Type // comparison type for subject == X
	// SubjType is the type subject == X takes the subject as, if not the
	// subject's own: a number it was smart cast to by the branches before.
	SubjType types.Type

	// is T, !is T.
	Is, NotIs bool
	P         Pos
	Type      *TypeRef
	Target    types.Type // T, resolved
}

type WhenBranch struct {
	P     Pos
	Conds []WhenCond
	Else  bool
	// Guard is a branch's guard condition, cond if guard -> body, which
	// must hold too, with the smart casts of the conditions.
	Guard Expr
	Body  *Block
}

type When struct {
	exprBase
	Subject  Expr     // may be nil
	SubjVar  string   // x in when (val x = e)
	SubjType *TypeRef // its declared type, if any
	Branches []*WhenBranch
	SubjSym  *types.VarSym // the local holding the subject: x, or a temp
	// NoMatch is the exception class that a when used as a value throws if
	// no branch matches, when it is exhaustive without an else branch.
	NoMatch string
	// Soft is set by the checker on a when without else that ends a
	// lambda: it is the lambda's value if it is exhaustive, and else a
	// statement.
	Soft bool
	// Groups is set as on an If: each branch, and no branch matching,
	// runs in a replace group of its own.
	Groups bool
}

// Is is x is T, or x !is T.
type Is struct {
	exprBase
	X      Expr
	Ref    *TypeRef // T
	Not    bool
	Target types.Type // T, resolved
}

// As is x as T, or x as? T.
type As struct {
	exprBase
	X      Expr
	Ref    *TypeRef // T
	Safe   bool
	Target types.Type // T, resolved
}

// Try is try { } catch (e: E) { } finally { }.
type Try struct {
	exprBase
	Body    *Block
	Catches []*Catch
	Finally *Block // or nil
}

// Catch is a catch block: catch (Name: Type) { Body }.
type Catch struct {
	P           Pos
	Name        string
	Type        *TypeRef
	Sym         *types.VarSym
	Body        *Block
	Annotations []Annotation // the parameter's
}

// Jump is return, break, continue or throw; its type is Nothing.
type Jump struct {
	exprBase
	Kind  Kind
	X     Expr   // return value, or the exception thrown
	Label string // return@label, break@label, continue@label
	// Lambda is set by the checker on a return from a lambda,
	// return@label: the lambda it leaves.
	Lambda *Lambda
	// Loop is set by the checker on a break or continue: the *While or
	// *For it leaves or continues.
	Loop Stmt
}
