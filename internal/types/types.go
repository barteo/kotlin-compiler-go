// Package types defines Kotlin types and the symbols the checker resolves
// names to. It does not depend on the AST, so AST nodes can carry types.
package types

import (
	"strings"
	"sync"

	"github.com/barteo/kotlin-compiler-go/internal/inline"
)

type Kind uint8

const (
	Invalid Kind = iota
	Unit
	Nothing
	Null // the type of the literal `null`, i.e. Nothing?
	Bool
	Int
	Long
	Double
	Char
	Float
	Byte
	Short
	String
	Any // kotlin.Any, java/lang/Object
	Class
	Range     // an Int, Long or Char range or progression; only valid in `for` and `in`
	TypeParam // a type parameter T of a class or function
	TypeVar   // an inference variable, only seen during type checking
)

// Type is a small value type so types can be compared with ==. Every
// expression node holds one, so it is kept to three words.
type Type struct {
	K Kind
	N bool // nullable
	P bool // platform type from Java (T!): nullable, but usable as T
	D bool // TypeParam and TypeVar: definitely non-null (T & Any)
	// E marks a function type with a receiver, A.(B) -> R, which is the
	// function type (A, B) -> R. Each is assignable to the other.
	E bool
	// X is the number of context parameters of a context function type,
	// context(A) B.(C) -> R, which is the function type (A, B, C) -> R
	// with its first X parameters, before the receiver, marked as
	// contexts. Each is assignable to the other too.
	X uint8
	// B marks the boxed form of a value class type, its instances, which
	// only codegen uses: the elements of an array of a value class.
	B bool
	C *ClassSym // Class
	// A is a Class's interned type arguments (nil if it has none). For
	// TypeParam and TypeVar it holds the parameter: see Param.
	A *Args
}

var (
	InvalidType = Type{K: Invalid}
	UnitType    = Type{K: Unit}
	NothingType = Type{K: Nothing}
	NullType    = Type{K: Null, N: true}
	BoolType    = Type{K: Bool}
	IntType     = Type{K: Int}
	LongType    = Type{K: Long}
	DoubleType  = Type{K: Double}
	CharType    = Type{K: Char}
	FloatType   = Type{K: Float}
	ByteType    = Type{K: Byte}
	ShortType   = Type{K: Short}
	StringType  = Type{K: String}
	AnyType     = Type{K: Any}
	RangeType   = Type{K: Range} // an Int range; see RangeOf
)

// RangeOf returns the type of ranges of elem, Int, Long or Char.
func RangeOf(elem Type) Type {
	if elem.K == Char || elem.K == Long {
		return Type{K: Range, A: MakeArgs([]Arg{{T: elem}})}
	}
	return RangeType
}

// RangeElem returns the type of a range's elements.
func (t Type) RangeElem() Type {
	if t.K == Range && t.A != nil {
		return t.A.Get(0).T
	}
	return IntType
}

func ClassType(c *ClassSym) Type { return Type{K: Class, C: c} }

func (t Type) Nullable() Type {
	if t.K == Unit || t.K == Invalid || t.K == Range {
		return t
	}
	if t.K == Nothing {
		return NullType
	}
	t.N, t.P, t.D = true, false, false
	return t
}

func (t Type) NonNull() Type {
	if t.K == Null {
		return NothingType
	}
	t.N, t.P = false, false
	if t.IsParam() {
		t.D = t.Param().NullableBound()
	}
	return t
}

// Platform marks t as a platform type, as Java values are.
func (t Type) Platform() Type {
	if t.K == Unit || t.IsPrimitive() {
		return t
	}
	t.N, t.P, t.D = true, true, false
	return t
}

// Flex returns the non-null form of a platform type, which is how
// operators and member accesses see it, and t otherwise.
func (t Type) Flex() Type {
	if t.P {
		return t.NonNull()
	}
	return t
}

// IsNumeric reports whether t is a non-null number: Int, Long, Double,
// Float, Byte or Short. Char is no number in Kotlin.
func (t Type) IsNumeric() bool {
	if t.N {
		return false
	}
	switch t.K {
	case Int, Long, Double, Float, Byte, Short:
		return true
	}
	return false
}

// IsPrimitive reports whether t is stored as a JVM primitive.
func (t Type) IsPrimitive() bool { return !t.N && IsPrimitiveKind(t.K) }

// IsPrimitiveKind reports whether values of kind k are JVM primitives
// when they are not nullable.
func IsPrimitiveKind(k Kind) bool {
	switch k {
	case Int, Long, Double, Bool, Char, Float, Byte, Short:
		return true
	}
	return false
}

// IsIntLike reports whether a primitive of type t is an int on the JVM's
// stack: Int, Char, Byte, Short and Boolean.
func (t Type) IsIntLike() bool {
	switch t.K {
	case Int, Char, Byte, Short, Bool:
		return !t.N
	}
	return false
}

func (t Type) String() string {
	var s string
	switch t.K {
	case Invalid:
		return "<error>"
	case Unit:
		s = "Unit"
	case Nothing:
		s = "Nothing"
	case Null:
		return "Nothing?"
	case Bool:
		s = "Boolean"
	case Int:
		s = "Int"
	case Long:
		s = "Long"
	case Double:
		s = "Double"
	case Char:
		s = "Char"
	case Float:
		s = "Float"
	case Byte:
		s = "Byte"
	case Short:
		s = "Short"
	case String:
		s = "String"
	case Any:
		s = "Any"
	case Class:
		if t.IsFunc() {
			if fs, ok := funcString(t); ok {
				s = fs
				break
			}
		}
		s = t.C.DisplayName()
		if t.A != nil {
			args := make([]string, len(t.A.List))
			for i, a := range t.A.List {
				args[i] = a.String()
			}
			s += "<" + strings.Join(args, ", ") + ">"
		}
	case Range:
		s = t.RangeElem().String() + "Progression"
	case TypeParam, TypeVar:
		s = t.Param().Name
		if t.D {
			s += " & Any"
		}
	}
	switch {
	case t.P:
		s += "!"
	case t.N:
		s += "?"
	}
	return s
}

// Wider returns the result type of arithmetic on two numeric types: the
// wider of Int, Long, Float and Double. Byte and Short are promoted to Int.
func Wider(a, b Type) Type {
	switch {
	case a.K == Double || b.K == Double:
		return DoubleType
	case a.K == Float || b.K == Float:
		return FloatType
	case a.K == Long || b.K == Long:
		return LongType
	}
	return IntType
}

type VarKind uint8

const (
	Local VarKind = iota
	Param
	Global // top-level property, a static field on the file facade
	Prop   // class property
	Static // static field of a Java class
	// BackingField is field in a property's accessors: Prop's field.
	BackingField
)

// VarSym is a local, parameter, top-level property or class property.
type VarSym struct {
	Name    string
	Type    Type
	Kind    VarKind
	Mutable bool
	// Captured is set when a lambda uses a Local or Param. A captured var
	// is kept in a one-element array that the lambda shares.
	Captured bool
	Owner    *ClassSym // Prop, Static
	Facade   string    // Global: internal name of the file facade class
	// Part is set on a Global of a multi-file class's part that isn't
	// private: the part, whose field and accessors the facade's call.
	Part string
	Decl any

	// Inference of an omitted type, run before the parallel phase.
	Infer      func()
	Resolved   bool
	InProgress bool

	Slot int // JVM local slot, assigned by codegen (Local and Param)
	// Deferred is set on a local or a class property declared without a
	// value, which code assigns later: val x: Int, then x = 1. The checker
	// checks that it is assigned before it is read, and a val only once.
	Deferred bool
	// Delegate is set on a local delegated property: the hidden local
	// that holds its delegate, which reading it calls getValue on.
	Delegate *VarSym

	// NoInfer is set on a context parameter whose type, or one of its
	// type arguments, is annotated @NoInfer: its argument is chosen by the
	// call's type arguments, not used to infer them.
	NoInfer bool

	// Parameters of Kotlin and Java functions read from class files.
	Vararg  bool // takes any number of arguments of Type, passed in an array
	Default bool // declares a default value, so the argument can be omitted
	// UnsafeVariance is set on a parameter of a function in the program
	// whose type is annotated @UnsafeVariance: through an out projection
	// of its class, it takes the projection's type, and through a star,
	// the type parameter's bound, not Nothing.
	UnsafeVariance bool

	// Class properties (Prop).
	Abstract bool    // declared without a value: in an interface, or abstract
	Final    bool    // can't be overridden: not open, abstract or override
	Getter   *FunSym // a custom getter
	Setter   *FunSym // a custom setter, or a delegated property's
	// Contexts are the context parameters of a property, which its
	// accessors take first, as their Contexts.
	Contexts []*VarSym
	// Alt links the properties of the same name and owner, which Kotlin
	// allows if their context parameters differ, in a ring: the next one,
	// or nil if there is no other. Lookups find the first declared.
	Alt *VarSym
	// FieldUsed is set on a property whose accessors use field, which
	// then has a backing field though both accessors are custom.
	FieldUsed bool
	// SetterPrivate is set on a property with a default private setter,
	// private set: as with kotlinc, there is no setter method, and its
	// class writes the field. SetterModule is set on one with an
	// internal setter of a property that isn't internal: the setter's
	// JVM name ends with it, as Module ends both accessors' names.
	SetterPrivate bool
	SetterModule  string
	// Prop is the property whose field a BackingField is.
	Prop *VarSym
	// OuterOf is set on the outer instance of an outer instance, which an
	// inner class of an inner class reads: OuterOf's field this$0, of
	// class OuterCls.
	OuterOf  *VarSym
	OuterCls *ClassSym
	// BoxedGetter is set on a property whose getter returns the box of
	// its primitive type, as an override's: see FunSym.BoxedRet.
	BoxedGetter bool
	// Lateinit is set on a lateinit property or local var: reading it
	// before it is assigned throws an UninitializedPropertyAccessException.
	Lateinit bool
	// Private is set on a private class or top-level property with
	// default accessors. As with kotlinc, it has no getter or setter: its
	// class or file facade uses its field, and other classes static
	// access$getX$p methods.
	Private bool
	// Protected is set on a protected field of a Java class, which only
	// code in its subclasses sees.
	Protected bool
	// InheritsProtected is set on an override declared without a
	// visibility of a protected property: its accessors are protected.
	InheritsProtected bool
	// PrivateToThis is set on a private property of a class with in or
	// out type parameters whose type breaks their variance: only the
	// class's own this may use it.
	PrivateToThis bool
	// Const is set on a const val, and JvmField on a class or top-level
	// property annotated @JvmField: on the JVM, either is only its field,
	// without accessors. A const of an object is a static field of its
	// class, and one of a companion object of the class it is in: see
	// ConstOwner.
	Const, JvmField bool
	// Constant is the value of a static field of a class from the class
	// path that has one (its ConstantValue), or of a const val of a
	// compiled Kotlin package: an int64 for an integral, Char or Boolean
	// value, a float32, a float64 or a string. Constant expressions can
	// use it, as kotlinc's do.
	Constant any
	// JvmStatic is set on a property of an object annotated @JvmStatic:
	// its accessors are static methods of the object's class, or for a
	// companion object, static methods of the outer class call them.
	JvmStatic bool
	// Bridges are the JVM names and descriptors, getX()I, of the getters
	// of overridden properties whose erasure or name differs from this
	// property's getter's.
	Bridges []string
	// SetterBridges are those of the setters of overridden properties
	// whose erasure or name differs from this property's setter's.
	SetterBridges []string
	// Suspend is set on kotlin.coroutines.coroutineContext, which only
	// suspend code can read: the context of its continuation.
	Suspend bool
	// Composable is set on a property whose getter is @Composable: it
	// takes a Composer and an int of change flags, as FunSym.Composable.
	Composable bool
	// JVMGetter and JVMSetter are the names that @get:JvmName and
	// @set:JvmName give the accessors of a property of the program.
	JVMGetter, JVMSetter string
	// Module is set on an internal class property: the end of its
	// accessors' JVM names, $ and the module's name, as kotlinc mangles them.
	Module string
}

// Visibilities an override declared without one inherits.
const (
	InheritProtected = 1 + iota
	InheritInternal
)

// HasDefaultImpls reports whether Kotlin interface c keeps its members'
// bodies in static methods of a class c$DefaultImpls, which take the
// instance first, and has abstract methods for them: if it was compiled
// without JVM default methods (kotlinc's -jvm-default=disable). A class
// that implements it gets methods that call those it doesn't implement.
func (c *ClassSym) HasDefaultImpls() bool {
	return c.Interface && !c.JVMDefault && !c.Annotation && c.Java == nil && (!c.External || c.KotlinName != "")
}

// StaticField reports whether class property v's field is static, as
// kotlinc writes it: a property of a named object, whose class holds it,
// or a @JvmField property of a companion object, whose outer class does
// (ConstOwner).
func (v *VarSym) StaticField() bool {
	o := v.Owner
	if v.Kind != Prop || o == nil || !o.Object || o.Anonymous || o.External || v.Const {
		return false
	}
	return o.CompanionOf == nil || v.JvmField
}

// ConstOwner returns the class whose static field holds const property p
// of an object: the object's class, or a companion's outer class.
func (v *VarSym) ConstOwner() *ClassSym {
	if c := v.Owner.CompanionOf; c != nil {
		return c
	}
	return v.Owner
}

// HasField reports whether a class or top-level property has a backing
// field: as in Kotlin, if one of its accessors is a default one, or uses
// field. A delegated property's accessors are custom, and only its
// delegate has a field.
func (v *VarSym) HasField() bool {
	return !v.Abstract && (v.Getter == nil || v.Mutable && v.Setter == nil || v.FieldUsed)
}

// FieldClass returns the internal name of the class whose static field
// holds top-level property v: its file facade, or its multi-file class's
// part.
func (v *VarSym) FieldClass() string {
	if v.Part != "" {
		return v.Part
	}
	return v.Facade
}

// Boxed reports whether v is a var captured by a lambda, which is kept in a
// one-element array.
func (v *VarSym) Boxed() bool { return v.Captured && v.Mutable && v.Kind == Local }

// UnitLocal reports whether v is a local of type Unit, which has no
// storage: its value is the Unit object, and an assignment to it only
// evaluates the value.
func (v *VarSym) UnitLocal() bool { return v.Kind == Local && v.Type.K == Unit && v.Delegate == nil }

// FunSym is a top-level function, method, constructor or builtin.
// Effect is a contract's returns(value) implies (p != null) or (p is
// T): if the function returns true (Returns 1), false (Returns 0) or at
// all (Returns -1), its parameter at Param (-1: the extension receiver)
// isn't null, or is an Is when that isn't nil.
type Effect struct {
	Returns, Param int
	Is             *Type
	Holds          bool // the parameter, a Boolean, is true: its argument's smart casts
}

type FunSym struct {
	Name       string
	TypeParams []*TypeParamSym
	Params     []*VarSym
	Ret        Type
	Owner      *ClassSym // methods
	Facade     string    // top-level functions
	Builtin    Builtin
	Decl       any

	// Java methods and Kotlin declarations read from class files.
	Static   bool
	Abstract bool
	Desc     string // JVM descriptor
	JVMName  string // JVM method name, if it differs from Name
	Operator bool   // can be called by an operator, as get by x[i]: Java methods and Kotlin operator funs
	Infix    bool   // can be called as an infix operator: a to b
	Ext      bool   // an extension function, called on a receiver of type Recv
	Recv     Type   // Invalid if kc doesn't support the receiver's type
	// Contexts are the context parameters of a function of the program,
	// context(a: A) fun f(), which calls pass implicitly. On the JVM,
	// it takes them first, before the extension receiver.
	Contexts []*VarSym
	// BoxedRet is set on an override whose result is a primitive where
	// a member it overrides has a type parameter or a nullable type: as
	// with kotlinc, its JVM method returns the box, Integer for Int. On a
	// suspend override whose result is a value class where a member it
	// overrides has another type, it returns the value class's box.
	BoxedRet bool
	// ExtSetter is set on the getter of a var extension property of the
	// program: its setter, which takes the receiver and the value.
	ExtSetter   *FunSym
	Unsupported bool   // kc can't call it: see Why
	Hidden      bool   // @Deprecated(level = HIDDEN): compiled, but not a candidate for calls
	Why         string // why it is unsupported, if not for its types
	// Feature is the feature that Why reports unsupported, if it isn't
	// an inline-only call.
	Feature string
	// Impl is set on a member of a value class of the class path: a
	// static method of Facade, the class, that takes the value first.
	Impl bool
	// Suspend is set on a suspend function, which on the JVM takes a
	// Continuation last and returns its result as an Object, or
	// COROUTINE_SUSPENDED.
	Suspend bool
	// External is set on an external function or accessor of the
	// program: a native method, without code.
	External bool
	// Composable is set on a @Composable function, as Compose's compiler
	// plugin compiles it: on the JVM it takes a Composer after its
	// parameters, then Changed ints of change flags and Defaults ints of
	// masks of the omitted arguments. See ComposeDesc.
	Composable bool
	Changed    int
	Defaults   int
	// ComposeDefaults is set on an open composable member with default
	// values: its method takes no masks, and callers call the static
	// name$default method of its owner's ComposeDefaultImpls class, a
	// composable function that takes the parameters, then the object and
	// the extension receiver, computes the omitted ones and calls it.
	ComposeDefaults bool

	Infer      func()
	Resolved   bool
	InProgress bool

	// Inheritance.
	Final     bool // can't be overridden
	Protected bool // a protected Java or Kotlin member, for subclasses only
	// Private is set on the program's private functions, accessors and
	// constructors, which are private on the JVM. As kotlinc does, other
	// classes call a function through a static accessor, access$name, and
	// a constructor through one that takes an unused marker last.
	Private bool
	// Marker is set on a constructor of the class path that takes a value
	// class: kotlinc makes it private, and callers call the public
	// synthetic one that takes an unused marker last. Desc leaves the
	// marker out, as the constructor for omitted arguments adds its own.
	Marker bool
	// PrivateToThis is set on a private function of a class with in or
	// out type parameters whose signature breaks their variance, as on a
	// VarSym.
	PrivateToThis bool
	// Throws are the classes of the checked exceptions that a function of
	// the program declares with @Throws, by internal name.
	Throws []string
	// Module is set on an internal member of a class, or an override of
	// one: the end of its JVM name, $ and the module's name, as kotlinc
	// mangles them so that other modules don't call or override it. On a
	// private function of a multi-file class's part, it is $ and the
	// part's name.
	Module string
	// Bridges are the JVM names and descriptors, f(Ljava/lang/Object;)V,
	// of overridden methods whose erasure or name differs from this one's.
	Bridges []string
	// BoxingBridges are the Bridges of an override that returns a value
	// class, unboxed, whose overridden methods return it boxed: their
	// result's type is Any, an interface or a type parameter. A suspend
	// function's result is boxed where it resumes its caller.
	BoxingBridges []string
	// BridgeOf are the overridden functions of the Bridges, by bridge.
	BridgeOf map[string]*FunSym
	// BoxedParams are the indexes of the parameters of type Result, whose
	// functions keep their names, that an overridden function takes as
	// another type, as T: as with kotlinc, they are passed boxed and the
	// function unboxes them.
	BoxedParams []int
	// DefaultsOf is set on an override that inherits default values: the
	// overridden method, whose name$default method supplies them.
	DefaultsOf *FunSym
	// KotlinOperator is set on an override of a Kotlin operator function,
	// which is one too (Operator is also set for Java methods), and
	// Inherited on an override declared without a visibility: the
	// overridden function's, InheritProtected or InheritInternal.
	KotlinOperator bool
	Inherited      uint8
	// PublicOnJVM is set on an override that inherits protected from a
	// member of the class path that is public on the JVM: Kotlin's
	// AbstractCollection.toArray(), which is Collection.toArray().
	PublicOnJVM bool

	// Once marks the parameters whose lambdas the function calls exactly
	// once, as its contract says, callsInPlace(f, EXACTLY_ONCE): definite
	// assignment follows the lambda's body as the call's code.
	Once []bool

	// Implies are the effects of the function's contract that say what a
	// parameter is if it returns: returns(false) implies (this != null)
	// of String?.isNullOrEmpty().
	Implies []Effect

	// LowPriority is set on a function annotated
	// @LowPriorityInOverloadResolution: a call chooses another that fits
	// as well, as the JVM runBlocking loses to the common one.
	LowPriority bool

	// Inline is set on an inline function of the class path whose body
	// kc can copy into its callers.
	Inline *InlineInfo
	// Platform is set on a member of a Kotlin collection type that only
	// its Java interface declares, such as forEach(Consumer). As in
	// kotlinc, an extension that fits the call comes first.
	Platform bool
}

// InlineInfo describes an inline function read from a class file. Its
// body is read and analyzed when a call first resolves to it, from any
// of the goroutines that check bodies.
type InlineInfo struct {
	Class   string // the class file with the body: a multifile part or a file facade
	Name    string // the method's name and descriptor
	Desc    string
	Private bool // inline-only: there is no method to call instead
	// Crossinline and Noinline are the flags of each value parameter.
	Crossinline, Noinline []bool
	// Suspend is set on a suspend function, whose body for inlining is
	// name$$forInline if its class has it: name's is a state machine.
	Suspend bool
	// Load reads and analyzes a method of Class.
	Load func(name, desc string) (*inline.Body, error)
	// Owner is set on a member: the class whose instance its static
	// name$default method takes first.
	Owner string

	body, defaults lazyBody
}

type lazyBody struct {
	once sync.Once
	b    *inline.Body
	err  error
}

func (l *lazyBody) get(load func(name, desc string) (*inline.Body, error), name, desc string) (*inline.Body, error) {
	l.once.Do(func() { l.b, l.err = load(name, desc) })
	return l.b, l.err
}

// Body returns the function's analyzed body.
func (i *InlineInfo) Body() (*inline.Body, error) {
	if i.Suspend {
		load := func(name, desc string) (*inline.Body, error) {
			b, err := i.Load(name+"$$forInline", desc)
			if err != nil && i.Private {
				// An inline-only function's body is for inlining.
				return i.Load(name, desc)
			}
			return b, err
		}
		return i.body.get(load, i.Name, i.Desc)
	}
	return i.body.get(i.Load, i.Name, i.Desc)
}

// Defaults returns the body of the function's name$default method, which
// takes the masks of the omitted arguments of its n parameters (see
// MaskInts) and a marker after the parameters, and computes their default
// values first.
func (i *InlineInfo) Defaults(n int) (*inline.Body, error) {
	k := strings.IndexByte(i.Desc, ')')
	desc := i.Desc[:k] + strings.Repeat("I", MaskInts(n)) + "Ljava/lang/Object;" + i.Desc[k:]
	if i.Owner == "" {
		return i.defaults.get(i.Load, i.Name+"$default", desc)
	}
	// A member's takes the receiver first, as an instance method's body
	// does.
	load := func(name, desc string) (*inline.Body, error) {
		b, err := i.Load(name, desc)
		if b != nil {
			b.This = true
		}
		return b, err
	}
	return i.defaults.get(load, i.Name+"$default", "(L"+i.Owner+";"+desc[1:])
}

// JVM returns the name of the method on the JVM. The program's functions
// that take or, as members, return values of value classes have mangled
// names, as kotlinc gives them: see Mangle.
func (f *FunSym) JVM() string {
	if f.JVMName != "" {
		return f.JVMName
	}
	if f.Desc == "" && f.Builtin == NotBuiltin && f.Name != "<init>" {
		return mangledName(f) + f.Module
	}
	return f.Name
}

// KProp is a property of a Kotlin class read from metadata, accessed
// through its getter and setter methods.
type KProp struct {
	Getter *FunSym // Ret is the property's type
	Setter *FunSym // nil for a val
	Alt    *KProp  // as VarSym.Alt
}

// ClassSym is a user class or a Java class from the class path. Name is the
// JVM internal name: the simple name for user classes, which are in the
// default package, or a name like java/lang/StringBuilder.
type ClassSym struct {
	Name       string
	TypeParams []*TypeParamSym
	CtorParams []*VarSym // all primary constructor parameters
	Props      []*VarSym
	Ctor       *FunSym
	Methods    map[string][]*FunSym
	// ExtMethods are the extension functions declared in the class, which
	// only code in its scope can call, with an instance of it as an
	// implicit receiver.
	ExtMethods map[string][]*FunSym
	// ExtProps are the getters of the extension properties declared in
	// the class, by the properties' names, which ExtMethods' rules apply to.
	ExtProps  map[string][]*FunSym
	Decl      any
	Func      bool // a function type's interface: see FunctionClass
	Anonymous bool // an object expression's class
	// Array is set on Array<T> and the primitive arrays, whose element
	// type is Elem: see ArrayClass.
	Array bool
	Elem  Type
	// Suspend is set on the interface of a suspend function type, and
	// RestrictsSuspension on a class annotated @RestrictsSuspension, in
	// whose suspend lambdas only its members and extensions can suspend.
	Suspend, RestrictsSuspension bool
	// Composable is set on the interface of a composable function type:
	// see Functions.
	Composable bool

	// Java classes. They are created when first referenced and filled in by
	// load on first use; call Ensure before reading the fields below or
	// Methods.
	External  bool
	Missing   bool // not found or unreadable
	Interface bool
	Abstract  bool // an abstract class or an interface: it can't be instantiated
	Final     bool // can't be subclassed
	Private   bool // not public
	// Internal is set on a Kotlin class that is internal to its module,
	// with Private: code under @Suppress("INVISIBLE_REFERENCE") may use it.
	Internal   bool
	Super      *ClassSym
	Interfaces []*ClassSym
	Supers     []Type // generic supertypes from the class's signature, if it has one
	Ctors      []*FunSym
	Statics    map[string][]*FunSym
	Fields     map[string]*VarSym // static fields

	// Objects and nested classes. Object is set on object declarations
	// and companion objects, whose one instance is in a static field: see
	// ObjectField. Outer is the class that a nested class or a companion
	// object is declared in.
	Object      bool
	Companion   *ClassSym // the class's companion object, or nil
	CompanionOf *ClassSym // set on a companion object
	Outer       *ClassSym
	Nested      map[string]*ClassSym // the program's nested classes, by name
	Aliases     map[string]*AliasSym // the program's nested typealiases, by name
	// Entries are an enum class's entries, in order; nil for other classes.
	Entries []string
	// Sealed is set on sealed classes and interfaces of the program, whose
	// Subclasses are their direct subclasses.
	Sealed     bool
	Subclasses []*ClassSym
	// Value is set on the program's value classes, whose instances are
	// the value of their one property, Underlying, wherever they aren't
	// boxed: see Unboxed.
	Value      bool
	Underlying *VarSym
	// Fun is set on fun interfaces, of the program or the class path, to
	// whose single abstract method lambdas passed for them convert.
	Fun bool
	// Annotation is set on the program's annotation classes: interfaces
	// whose AnnotationParams are their abstract methods.
	Annotation       bool
	AnnotationParams []*VarSym
	// Retention and Targets are set on annotation classes, the program's
	// and the class path's.
	Retention Retention
	Targets   Targets
	// Container is set on a repeatable annotation class of the class
	// path: the annotation class that holds several of its annotations.
	Container *ClassSym

	// Kotlin classes read from metadata.
	KotlinName string // class id, e.g. kotlin/collections/Map.Entry; "" for Java classes
	// JVMDefault is set on a Kotlin interface, the class path's or the
	// program's, whose members' bodies are default methods, with their
	// name$default methods static methods of the interface (kotlinc's
	// -jvm-default=all). See HasDefaultImpls for the others.
	JVMDefault bool
	// JVMCompat is set on such an interface whose bodies are also static
	// methods of its DefaultImpls class, for code compiled to call them
	// there: kotlinc's -jvm-default=enable, or @JvmDefaultWithCompatibility.
	JVMCompat bool
	KProps    map[string]*KProp // properties
	// Java is set on Kotlin's built-in collection types, such as List and
	// MutableList: the Java class they are on the JVM, java.util.List.
	Java *ClassSym
	// ReadOnly is set on the mutable collection types: List for
	// MutableList. A Java List is a platform MutableList that also takes
	// read-only lists. Mutable is the other way: MutableList for List.
	ReadOnly, Mutable *ClassSym
	// Local is set on a class declared in a block, whose instances hold
	// what its code captures, as an object expression's do.
	Local bool
	// File is set on the program's classes: the class of the file that
	// declares them (see syntax.File.Class), in which a private
	// top-level class is visible.
	File string
	// Inner is set on an inner class, whose instances hold their outer
	// class's in this$0. Its TypeParams are its own and then its outer
	// class's, the last OuterParams of them.
	Inner       bool
	OuterParams int
	// JavaFields are the public and protected instance fields of a Java
	// class, which Kotlin sees as properties: JvmField ones, read and
	// written as fields.
	JavaFields map[string]*VarSym
	// JVMMethods are the methods of a class on the class path, by name
	// and descriptor, with their access flags: also those its Kotlin
	// metadata doesn't declare, such as kotlinc's collection stubs.
	JVMMethods map[string]uint16
	// Annotations are the classes of the annotations of a class on the
	// class path that its class file keeps, such as Compose's @Stable, or
	// of the program's that are classes, and StabilityInferred is the parameters of its
	// @StabilityInferred, which Compose's compiler plugin writes, or -1:
	// see codegen/stability.go.
	Annotations       []*ClassSym
	StabilityInferred int

	// Global is set on a class that a package-level variable declares for
	// all compilations in the process, such as kotlin.Array: see MakeArgs.
	Global bool

	load    func(*ClassSym)
	once    sync.Once
	samOnce sync.Once
	sam     *FunSym
	lists   argsTable // the interned lists whose table is the class's: see MakeArgs
}

// SAM returns the single abstract method of a Java interface, which
// lambdas can implement, computing it with find on first use.
func (c *ClassSym) SAM(find func(*ClassSym) *FunSym) *FunSym {
	c.samOnce.Do(func() { c.sam = find(c) })
	return c.sam
}

// NewExternal returns a Java class that load fills in on first use.
func NewExternal(name string, load func(*ClassSym)) *ClassSym {
	return &ClassSym{Name: name, External: true, load: load}
}

// Ensure loads a Java class's members. It is safe for concurrent use.
func (c *ClassSym) Ensure() {
	if c.load != nil {
		c.once.Do(func() { c.load(c) })
	}
}

// This returns the type of this inside the class: the class applied to its
// own type parameters.
func (c *ClassSym) This() Type {
	if len(c.TypeParams) == 0 {
		return ClassType(c)
	}
	args := make([]Arg, len(c.TypeParams))
	for i, p := range c.TypeParams {
		args[i] = Arg{T: ParamType(p)}
	}
	return GenericType(c, args)
}

// DisplayName is the name used in messages: the simple name, with the
// classes a nested class is in: Map.Entry.
func (c *ClassSym) DisplayName() string {
	if c.Anonymous {
		return "<anonymous object>"
	}
	if c.KotlinName != "" {
		return c.KotlinName[strings.LastIndexByte(c.KotlinName, '/')+1:]
	}
	return strings.ReplaceAll(c.Name[strings.LastIndexByte(c.Name, '/')+1:], "$", ".")
}

// ObjectField returns the static field that holds an object's instance:
// INSTANCE in the object's class, or for a companion object, a field of
// the class it belongs to named after it.
func (c *ClassSym) ObjectField() (owner, name string) {
	if c.CompanionOf != nil {
		return c.CompanionOf.Name, c.Name[strings.LastIndexByte(c.Name, '$')+1:]
	}
	return c.Name, "INSTANCE"
}

// Supertypes calls fn on c and then on each of its supertypes, superclasses
// before interfaces, until fn returns false.
func (c *ClassSym) Supertypes(fn func(*ClassSym) bool) {
	seen := map[*ClassSym]bool{}
	queue := []*ClassSym{c}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		if seen[s] {
			continue
		}
		seen[s] = true
		s.Ensure()
		if !fn(s) {
			return
		}
		if s.Super != nil {
			queue = append(queue, s.Super)
		}
		queue = append(queue, s.Interfaces...)
	}
}

// IsSubclassOf reports whether c is d or inherits from it.
func (c *ClassSym) IsSubclassOf(d *ClassSym) bool {
	found := false
	c.Supertypes(func(s *ClassSym) bool {
		found = s == d
		return !found
	})
	return found
}

func (c *ClassSym) Prop(name string) *VarSym {
	for _, p := range c.Props {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// GetterName and SetterName give the JVM accessor names of a property,
// following Kotlin's rule for properties named isSomething. Kotlin uses the
// same rule to see Java getters and setters as properties.
func GetterName(p string) string {
	if isPrefixed(p) {
		return p
	}
	return "get" + capitalize(p)
}

func SetterName(p string) string {
	if isPrefixed(p) {
		return "set" + p[2:]
	}
	return "set" + capitalize(p)
}

func capitalize(s string) string { return strings.ToUpper(s[:1]) + s[1:] }

func isPrefixed(p string) bool {
	return len(p) > 2 && strings.HasPrefix(p, "is") && 'A' <= p[2] && p[2] <= 'Z'
}

// Builtin identifies a function or member that maps directly to JDK code.
type Builtin uint8

const (
	NotBuiltin Builtin = iota
	Println
	Print
	StrLength
	StrIsEmpty
	StrUppercase
	StrLowercase
	StrSubstring
	ToInt
	ToLong
	ToDouble
	ToChar
	ToFloat
	ToByte
	ToShort
	ToString
	StrGet         // String.get(index), s[i]: charAt
	StrToCharArray // String.toCharArray()
	StrPlus        // String.plus(other): concatenation, as with +
	NewString      // String(chars)
	CharCode       // Char.code: the char's int value
	CharMethod     // a Char function that is a static method of java.lang.Character: see CharMethods
	NewInstance    // an inline-only factory such as mutableListOf(): a constructor of the class in Facade
	MapSet         // MutableMap.set(key, value), map[key] = value: put
	EntryKey       // Map.Entry.component1(): getKey
	EntryValue     // Map.Entry.component2(): getValue
	Bitwise        // Int's and Long's and, or, xor, shl, shr, ushr and inv: see BitwiseOps
	OperatorFun    // a number's or Char's operator function, called by name: 1.plus(2), see OperatorFunType
	IsInitialized  // this::x.isInitialized, of a lateinit property: its field isn't null
	RefName        // ::f.name, a constant: the Member's X is the name's string literal, or the reference if its receiver has effects

	// Inline-only preconditions and errors of the standard library.
	Require
	Check
	ErrorCall // error(message)
	Todo      // TODO(), TODO(reason)
	RequireNotNull
	CheckNotNull

	// Arrays: the members of the array classes, their constructors and
	// the standard library's factories, which kotlinc knows.
	ArrayGet
	ArraySet
	ArraySize
	ArrayClone
	ArrayNew     // IntArray(size), arrayOfNulls<T>(size)
	ArrayInit    // Array(size) { init }, IntArray(size) { init }
	ArrayLiteral // arrayOf(...), intArrayOf(...), emptyArray(): the vararg array
	ToTypedArray // Collection<T>.toTypedArray()
	EnumValues   // enumValues<T>(): T.values()
	EnumValueOf  // enumValueOf<T>(name): T.valueOf(name)
	TypeOf       // typeOf<T>(): T's KType, built with kotlin.jvm.internal.Reflection

	// ThenToString is an inline-only function that calls another facade
	// method (its JVMName and Desc) and converts the result with
	// toString(): String.trim().
	ThenToString

	// SuspendIntrinsic is suspendCoroutineUninterceptedOrReturn { c ->
	// ... }: the lambda is inlined, with c the caller's continuation, and
	// its result is the call's, COROUTINE_SUSPENDED if it suspends.
	SuspendIntrinsic

	// ComposeKey is Compose's key(keys...) { ... }: the lambda is inlined
	// in a movable group, whose key the keys are (see codegen/compose.go).
	ComposeKey
)

// BitwiseOps are the bitwise functions of Int and Long, which are JVM
// instructions: the shifts take an Int count, the others another operand
// of the receiver's type, and inv none.
var BitwiseOps = map[string]bool{"and": true, "or": true, "xor": true, "shl": true, "shr": true, "ushr": true, "inv": true}

// OperatorFuns are the operator functions of the numbers and Char that
// are JVM instructions, by their number of arguments.
var OperatorFuns = map[string]int{
	"plus": 1, "minus": 1, "times": 1, "div": 1, "rem": 1, "compareTo": 1,
	"inc": 0, "dec": 0, "unaryMinus": 0, "unaryPlus": 0,
}

// OperatorFunType returns the type operator function name of a receiver
// of type recv computes in, given an argument of type arg (ignored for
// the unary ones), and its result's type. As with the operators, numbers
// are computed in the wider of their types, with Byte and Short promoted
// to Int, and Chars as Ints: a Char plus or minus an Int is a Char, and
// the difference of two Chars an Int. inc and dec keep the receiver's
// type.
func OperatorFunType(name string, recv, arg Type) (op, result Type, ok bool) {
	switch {
	case name == "inc" || name == "dec":
		return Wider(recv, IntType), recv, recv.IsNumeric() || recv == CharType
	case name == "unaryMinus" || name == "unaryPlus":
		w := Wider(recv, IntType)
		return w, w, recv.IsNumeric()
	case recv.IsNumeric() && arg.IsNumeric():
		w := Wider(recv, arg)
		if name == "compareTo" {
			return w, IntType, true
		}
		return w, w, true
	case recv != CharType:
	case (name == "plus" || name == "minus") && arg == IntType:
		return IntType, CharType, true
	case (name == "minus" || name == "compareTo") && arg == CharType:
		return IntType, IntType, true
	}
	return InvalidType, InvalidType, false
}

// Boxes are the classes that hold primitives as objects, by kind.
var Boxes = map[Kind]string{
	Int:    "java/lang/Integer",
	Long:   "java/lang/Long",
	Double: "java/lang/Double",
	Bool:   "java/lang/Boolean",
	Char:   "java/lang/Character",
	Float:  "java/lang/Float",
	Byte:   "java/lang/Byte",
	Short:  "java/lang/Short",
}

// CharMethods are Char functions that the standard library declares
// inline-only and that are static methods of java.lang.Character, by
// Kotlin name. The is functions return Boolean, the others Char.
var CharMethods = map[string]string{
	"isDigit": "isDigit", "isLetter": "isLetter", "isLetterOrDigit": "isLetterOrDigit",
	"isUpperCase": "isUpperCase", "isLowerCase": "isLowerCase", "isTitleCase": "isTitleCase",
	"isISOControl": "isISOControl", "isDefined": "isDefined",
	"isJavaIdentifierStart": "isJavaIdentifierStart", "isJavaIdentifierPart": "isJavaIdentifierPart",
	"uppercaseChar": "toUpperCase", "lowercaseChar": "toLowerCase", "titlecaseChar": "toTitleCase",
}

// Desc returns the JVM descriptor of a type: its erasure, with nullable
// primitives boxed. It reports false for types that have none, such as
// an unresolved type.
func Desc(t Type) (string, bool) {
	if Unboxed(t) {
		return Desc(Repr(t))
	}
	if t.IsParam() {
		return Desc(Erase(t))
	}
	if t.N {
		if box, ok := Boxes[t.K]; ok {
			return "L" + box + ";", true
		}
	}
	switch t.K {
	case Unit:
		return "V", true
	case Nothing, Null:
		// Never a value, or only null: as with kotlinc, a java.lang.Void,
		// of a parameter, field or result that only ever holds null.
		return "Ljava/lang/Void;", true
	case Any:
		return "Ljava/lang/Object;", true
	case Int:
		return "I", true
	case Long:
		return "J", true
	case Double:
		return "D", true
	case Bool:
		return "Z", true
	case Char:
		return "C", true
	case Float:
		return "F", true
	case Byte:
		return "B", true
	case Short:
		return "S", true
	case String:
		return "Ljava/lang/String;", true
	case Class:
		if t.C.Array {
			return arrayDesc(t)
		}
		return "L" + t.C.Name + ";", true
	}
	return "", false
}

// ParamsDesc returns the parameter part of a method descriptor, (I...),
// or false if a parameter's type has no descriptor.
func ParamsDesc(params []*VarSym) (string, bool) {
	var sb strings.Builder
	sb.WriteByte('(')
	for _, p := range params {
		d, ok := ParamDesc(p.Type)
		if !ok {
			return "", false
		}
		sb.WriteString(d)
	}
	sb.WriteByte(')')
	return sb.String(), true
}

// ParamDesc returns the descriptor of a parameter of type t: as with
// kotlinc, one of type Nothing?, which only ever holds null, is a
// java.lang.Void, and one of type Unit a kotlin.Unit.
func ParamDesc(t Type) (string, bool) {
	switch t.K {
	case Null:
		return "Ljava/lang/Void;", true
	case Unit:
		return "Lkotlin/Unit;", true
	}
	return Desc(t)
}

// MethodDesc returns the descriptor of a method with these parameters and
// result, or false if a type has no descriptor.
func MethodDesc(params []*VarSym, ret Type) (string, bool) {
	ps, ok := ParamsDesc(params)
	r, rok := RetDesc(ret)
	return ps + r, ok && rok
}

// RetDesc returns the descriptor of a method's result of type t. As with
// kotlinc, a function that returns Nothing, which it never does, or
// Nothing?, which is only ever null, returns a java.lang.Void.
func RetDesc(t Type) (string, bool) {
	if t.K == Nothing && !t.N || t.K == Null {
		return "Ljava/lang/Void;", true
	}
	return Desc(t)
}

// FunDesc returns a function's JVM descriptor: the one read from its class
// file, or the one computed from its signature. It is "" if a type in the
// signature has no descriptor, such as an unresolved type.
func FunDesc(f *FunSym) string {
	if f.Desc != "" {
		return f.Desc
	}
	d, ok := MethodDesc(f.Params, JVMRet(f))
	if !ok {
		return ""
	}
	if f.Suspend {
		d = SuspendDesc(d)
	}
	if f.Composable {
		d = ComposeDesc(d, f.Changed+f.Defaults)
	}
	return d
}

// GetterType returns the type that property p's getter returns on the
// JVM: its field's, boxed if BoxedGetter is set.
func GetterType(p *VarSym) Type {
	if p.BoxedGetter && p.Type.IsPrimitive() {
		return p.Type.Nullable()
	}
	return FieldType(p)
}

// unitObject is kotlin.Unit's class, the type of Unit as an object.
var unitObject = &ClassSym{Name: "kotlin/Unit", KotlinName: "kotlin/Unit", External: true, Final: true, Global: true}

// FieldType returns the type of property p's field, and of its setter's
// parameter, on the JVM: its type, but as with kotlinc a Unit property
// holds the kotlin.Unit object.
func FieldType(p *VarSym) Type {
	if p.Type.K == Unit {
		return ClassType(unitObject)
	}
	return p.Type
}

// JVMRet returns the type of f's result on the JVM: its result type,
// boxed if BoxedRet is set, as it is on a Unit property's getter, which
// returns the kotlin.Unit object.
func JVMRet(f *FunSym) Type {
	if f.Ret.K == Unit && f.BoxedRet && !f.Suspend {
		return ClassType(unitObject)
	}
	if f.Suspend && f.BoxedRet && Unboxed(f.Ret) {
		return Boxed(f.Ret)
	}
	if !f.Ret.IsPrimitive() || f.Suspend {
		return f.Ret // a suspend function's continuation unboxes it
	}
	boxed := f.BoxedRet
	if i := strings.IndexByte(f.Desc, ')'); i >= 0 && strings.HasPrefix(f.Desc[i+1:], "L") {
		boxed = true // a class file's method that returns the box
	}
	if boxed {
		return f.Ret.Nullable()
	}
	return f.Ret
}

// ContinuationDesc is the descriptor of the parameter that suspend
// functions take last.
const ContinuationDesc = "Lkotlin/coroutines/Continuation;"

// SuspendDesc returns the descriptor of a suspend function whose
// descriptor without its continuation is d.
func SuspendDesc(d string) string {
	return d[:strings.IndexByte(d, ')')] + ContinuationDesc + ")Ljava/lang/Object;"
}

// ComposerDesc is the descriptor of the parameter that composable
// functions take after their own.
const ComposerDesc = "Landroidx/compose/runtime/Composer;"

// ComposeDesc returns the descriptor of a composable function whose
// descriptor without its Composer and ints is d: it takes ints more
// ints after the Composer.
func ComposeDesc(d string, ints int) string {
	i := strings.IndexByte(d, ')')
	return d[:i] + ComposerDesc + strings.Repeat("I", ints) + d[i:]
}

// ChangedInts returns how many ints of change flags a composable function
// takes for slots parameters and receivers: each holds the flags of ten,
// and there is one at least.
func ChangedInts(slots int) int { return max(1, (slots+9)/10) }

// DefaultInts returns how many ints of masks of the omitted arguments a
// composable function with n parameters takes, if any has a default
// value: each holds the bits of 31.
func DefaultInts(n int) int { return (n + 30) / 31 }

// MaskInts returns how many ints of masks of the omitted arguments a
// name$default method or a constructor for omitted arguments takes for a
// function with n parameters, as kotlinc writes them: one for every 32.
func MaskInts(n int) int { return max(1, (n+31)/32) }

// DefaultBits returns the number of bits of f's masks of omitted
// arguments: as kotlinc's, its context parameters come before its
// parameters, but not its receivers.
func DefaultBits(f *FunSym) int { return len(f.Contexts) + len(f.Params) }

// KotlinParamsDesc returns the parameters part of descriptor d, "(...)",
// without a composable function's Composer and ints, if f is one.
func KotlinParamsDesc(d string, f *FunSym) string {
	d = d[:strings.IndexByte(d, ')')+1]
	if i := strings.LastIndex(d, ComposerDesc); f.Composable && i >= 0 {
		d = d[:i] + ")"
	}
	return d
}
