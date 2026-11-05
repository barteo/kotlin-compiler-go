package kmeta

// The declarations in Kotlin metadata, decoded from the messages of
// core/metadata/src/metadata.proto and the JVM extensions of
// core/metadata.jvm/src/jvm_metadata.proto in the Kotlin repository.

// Type is a use of a type in a signature.
type Type struct {
	Class     string // class id, e.g. kotlin/collections/Map.Entry; "" for a type parameter
	Param     int    // type parameter id, if Class is ""
	ParamName string // type parameter name, if the type refers to one by name
	Args      []Arg
	Nullable  bool
	Flags     int   // TypeSuspend, TypeDefinitelyNonNull
	Upper     *Type // the upper bound of a flexible type, or nil
	Outer     *Type // the outer class's type, for an inner class
	// ExtFunc is set on a function type with a receiver, A.(B) -> C:
	// kotlin.Function2<A, B, C> annotated @ExtensionFunctionType.
	ExtFunc bool
	// Contexts is the number of context parameters of a context function
	// type, context(A) (B) -> C: kotlin.Function2<A, B, C> annotated
	// @ContextFunctionTypeParams(1).
	Contexts int
	// Composable is set on a composable function type, @Composable () ->
	// Unit: kotlin.Function0<Unit> annotated
	// @androidx.compose.runtime.Composable, as Compose's compiler plugin
	// writes it.
	Composable bool
	// NoInfer is set on a type annotated @kotlin.internal.NoInfer, as
	// contextOf's context parameter's, which doesn't infer a call's type
	// arguments.
	NoInfer bool
}

// Type flags.
const (
	TypeSuspend           = 1 << 0
	TypeDefinitelyNonNull = 1 << 1
)

// Arg is a type argument. Type is nil for Star.
type Arg struct {
	Proj Projection
	Type *Type
}

type Projection int

const (
	In Projection = iota
	Out
	Inv
	Star
)

// TypeParam is a type parameter of a class, function or property.
type TypeParam struct {
	ID       int // what Type.Param refers to
	Name     string
	Reified  bool
	Variance Projection // In, Out or Inv
	Bounds   []*Type
}

// Param is a value parameter.
type Param struct {
	Flags  int
	Name   string
	Type   *Type
	Vararg *Type // the element type of a vararg parameter, or nil
}

// Value parameter flags.
const (
	ParamDeclaresDefault = 1 << 1
	ParamCrossinline     = 1 << 2
	ParamNoinline        = 1 << 3
)

// JVMSig is the JVM name and descriptor of a method or field.
type JVMSig struct{ Name, Desc string }

// Visibility is a declaration's visibility (Flags.VISIBILITY).
type Visibility int

const (
	Internal Visibility = iota
	Private
	Protected
	Public
	PrivateToThis
	Local
)

// Modality is a declaration's modality (Flags.MODALITY).
type Modality int

const (
	Final Modality = iota
	Open
	Abstract
	Sealed
)

// Flags common to classes and members (Flags.java): hasAnnotations, then
// visibility in 3 bits, then modality in 2 bits.
type Flags int

func (f Flags) Visibility() Visibility { return Visibility(f >> 1 & 7) }
func (f Flags) Modality() Modality     { return Modality(f >> 4 & 3) }

// MemberKind distinguishes declarations from fake overrides and the like.
func (f Flags) MemberKind() int { return int(f >> 6 & 3) }

// Function is a function: top-level, member or extension.
type Function struct {
	Flags      Flags
	Name       string
	TypeParams []*TypeParam
	Recv       *Type // the extension receiver, or nil
	Params     []*Param
	Ret        *Type
	Contexts   int // number of context receivers and parameters
	// ContextParams are the context parameters of a function that kc
	// writes, whose types it writes as context receivers' too, as
	// kotlinc does.
	ContextParams []*Param
	JVM           JVMSig
	// InPlace are the callsInPlace effects of the function's contract.
	InPlace []InPlace
	// Implies are the conditional effects of the function's contract
	// that say a parameter isn't null, or is of a type.
	Implies []Implies
	// Annotations are the classes of annotations, without arguments,
	// that the metadata keeps: @JvmStatic on the serialization plugin's
	// write$Self, a static method of a class.
	Annotations []string
	// PluginData are what compiler plugins keep of the function: see
	// ComposeMetadata.
	PluginData []PluginData
}

// PluginData is what compiler plugin Plugin keeps of a declaration.
type PluginData struct {
	Plugin string
	Data   []byte
}

// ComposeMetadata is what Compose's compiler plugin keeps of an open
// composable function with default values: the language version, 2.4,
// whose callers call its owner's ComposeDefaultImpls class's
// name$default method, rather than the function with masks.
var ComposeMetadata = PluginData{Plugin: "androidx.compose.compiler.plugins.kotlin", Data: []byte{2, 4}}

// Implies is a contract's returns(value) implies (p != null) or (p is
// T) effect: if the function returns Returns, the parameter at Param (-1:
// the extension receiver) isn't null, or is an Is if that isn't nil.
// Returns is ReturnsTrue, ReturnsFalse or ReturnsAny, for returns().
type Implies struct {
	Returns, Param int
	Is             *Type
}

// The values of Implies.Returns.
const (
	ReturnsTrue  = 0
	ReturnsFalse = 1
	ReturnsAny   = -1
)

// InPlace is a contract's callsInPlace(lambda, kind) effect: the lambda
// is the parameter at Param (-1: the extension receiver), and Kind is
// AtMostOnce, ExactlyOnce, AtLeastOnce or Unknown.
type InPlace struct {
	Param, Kind int
}

// The kinds of invocation of callsInPlace, as metadata numbers them, and
// Unknown, for none.
const (
	AtMostOnce  = 0
	ExactlyOnce = 1
	AtLeastOnce = 2
	Unknown     = -1
)

// Function flags, after MemberKind.
func (f *Function) IsOperator() bool { return f.Flags>>8&1 != 0 }
func (f *Function) IsInfix() bool    { return f.Flags>>9&1 != 0 }
func (f *Function) IsInline() bool   { return f.Flags>>10&1 != 0 }
func (f *Function) IsSuspend() bool  { return f.Flags>>13&1 != 0 }
func (f *Function) IsExpect() bool   { return f.Flags>>14&1 != 0 }

// Property is a property: top-level, member or extension.
type Property struct {
	Flags       Flags
	GetterFlags Flags
	SetterFlags Flags
	Name        string
	TypeParams  []*TypeParam
	Recv        *Type
	Type        *Type
	Contexts    int // number of context receivers and parameters
	// ContextParams are the context parameters of a property that kc
	// supports, as Function's.
	ContextParams []*Param
	Getter        *JVMSig // nil if the property has no getter method
	Setter        *JVMSig
	// SetterParam is the setter's parameter, which kotlinc writes, and
	// reads, when the setter isn't a default one.
	SetterParam *Param
	Field       *JVMSig // the backing field, if it is part of the API
	// Synthetic is the method that holds the property's annotations,
	// getX$annotations, if it has any.
	Synthetic *JVMSig
}

// Property flags, after MemberKind.
func (p *Property) IsVar() bool     { return p.Flags>>8&1 != 0 }
func (p *Property) HasGetter() bool { return p.Flags>>9&1 != 0 }
func (p *Property) HasSetter() bool { return p.Flags>>10&1 != 0 }
func (p *Property) IsConst() bool   { return p.Flags>>11&1 != 0 }

// Property flags beyond IsConst.
const (
	PropVar       = 1 << 8
	PropGetter    = 1 << 9
	PropSetter    = 1 << 10
	PropConst     = 1 << 11
	PropLateinit  = 1 << 12
	PropConstant  = 1 << 13 // hasConstant: the field has a ConstantValue
	PropDelegated = 1 << 15
)

// Flags common to all declarations, and the member kinds.
const FlagAnnotations = 1 << 0

const (
	KindDeclaration = iota
	KindFakeOverride
	KindDelegation
	KindSynthesized
)

// MakeFlags returns flags with hasAnnotations, a visibility, a modality
// and a member kind (0 for classes and constructors).
func MakeFlags(annotations bool, v Visibility, m Modality, kind int) Flags {
	f := Flags(v)<<1 | Flags(m)<<4 | Flags(kind)<<6
	if annotations {
		f |= FlagAnnotations
	}
	return f
}

// Function flags, after the member kind.
const (
	FunOperator = 1 << 8
	FunInfix    = 1 << 9
	FunInline   = 1 << 10
	FunTailrec  = 1 << 11
	FunExternal = 1 << 12
	FunSuspend  = 1 << 13
	FunExpect   = 1 << 14
)

// Accessor flags, after the modality.
const (
	AccessorNotDefault = 1 << 6
	AccessorExternal   = 1 << 7
	AccessorInline     = 1 << 8
)

// Constructor flags, after the visibility.
const CtorSecondary = 1 << 4

// Class flags, after the modality: the kind in 3 bits, then these.
const (
	ClassInner       = 1 << 9
	ClassData        = 1 << 10
	ClassExternal    = 1 << 11
	ClassExpect      = 1 << 12
	ClassValue       = 1 << 13
	ClassFun         = 1 << 14
	ClassEnumEntries = 1 << 15
)

// MakeClassFlags returns a class's flags.
func MakeClassFlags(annotations bool, v Visibility, m Modality, kind ClassKind) Flags {
	f := Flags(v)<<1 | Flags(m)<<4 | Flags(kind)<<6
	if annotations {
		f |= FlagAnnotations
	}
	return f
}

// Constructor is a class constructor.
type Constructor struct {
	Flags  Flags
	Params []*Param
	JVM    JVMSig
}

// TypeAlias is a typealias declaration.
type TypeAlias struct {
	Flags      Flags
	Name       string
	TypeParams []*TypeParam
	Underlying *Type
	Expanded   *Type
	// Annotations are its annotations, which metadata keeps, as a type
	// alias has nothing on the JVM to annotate. Decoding leaves them out.
	Annotations []*Annotation
}

// Annotation is an annotation that metadata keeps with its arguments.
type Annotation struct {
	Class string // class id
	Args  []AnnotationArg
}

// AnnotationArg is a named argument of an Annotation.
type AnnotationArg struct {
	Name  string
	Value AnnotationValue
}

// AnnotationValue is an argument's value: Kind tells which of the other
// fields holds it.
type AnnotationValue struct {
	Kind  ValueKind
	Int   int64   // the integral kinds, ValueChar and ValueBoolean
	Float float64 // ValueFloat and ValueDouble
	Str   string  // ValueString, and the entry's name of a ValueEnum
	Class string  // ValueClass, and the class id of a ValueEnum
	Ann   *Annotation
	Elems []AnnotationValue // ValueArray
}

// ValueKind is the type of an AnnotationValue (Annotation.Argument.Value.Type).
type ValueKind int

const (
	ValueByte ValueKind = iota
	ValueChar
	ValueShort
	ValueInt
	ValueLong
	ValueFloat
	ValueDouble
	ValueBoolean
	ValueString
	ValueClass
	ValueEnum
	ValueAnnotation
	ValueArray
)

// ClassKind is the kind of a class (Flags.CLASS_KIND).
type ClassKind int

const (
	KindClass ClassKind = iota
	KindInterface
	KindEnum
	KindEnumEntry
	KindAnnotation
	KindObject
	KindCompanion
)

// Class is a class, interface or object.
type Class struct {
	Flags      Flags
	Name       string // class id, e.g. kotlin/Pair
	Companion  string // simple name of the companion object, or ""
	TypeParams []*TypeParam
	Supers     []*Type
	Nested     []string
	Ctors      []*Constructor
	Funcs      []*Function
	Props      []*Property
	Aliases    []*TypeAlias
	Contexts   int
	// A value class's underlying property and its type.
	UnderlyingProp string
	Underlying     *Type
	// EnumEntries are an enum class's entries, and SealedSubclasses a
	// sealed class's direct subclasses, by class id.
	EnumEntries      []string
	SealedSubclasses []string
	// ModuleName is the module the class is in, if it isn't "main".
	ModuleName string
	// JVMFlags are an interface's jvm_class_flags: JVMDefaultMode if its
	// members' bodies are default methods, with JVMCompatibilityMode if
	// it also has a DefaultImpls class.
	JVMFlags int
	// SerialProps are the properties, by name, that the serializer of an
	// open or abstract class annotated @Serializable writes, in order:
	// the kotlinx.serialization plugin's extension, which it reads in
	// other modules to call the class's synthetic constructor from its
	// subclasses'.
	SerialProps []string
}

// Bits of Class.JVMFlags.
const (
	JVMDefaultMode       = 1 << 0
	JVMCompatibilityMode = 1 << 1
)

func (c *Class) Kind() ClassKind { return ClassKind(c.Flags >> 6 & 7) }
func (c *Class) IsInner() bool   { return c.Flags>>9&1 != 0 }
func (c *Class) IsData() bool    { return c.Flags>>10&1 != 0 }
func (c *Class) IsValue() bool   { return c.Flags>>13&1 != 0 }

// scope is what decoding a message needs besides its bytes: the name
// resolver and the type table that type ids refer to.
type scope struct {
	names names
	types *typeTable
}

// typeTable holds the encoded types of a TypeTable message. They are
// decoded each time they are used, so decoding needs no locks.
type typeTable struct {
	raw           [][]byte
	firstNullable int
}

func readTypeTable(msg []byte) *typeTable {
	t := &typeTable{firstNullable: -1}
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		switch {
		case f == 1 && w == wireBytes:
			t.raw = append(t.raw, d.bytes())
		case f == 2:
			t.firstNullable = d.int()
		default:
			d.skip(w)
		}
	}
	return t
}

// withTable returns the scope for a message that may have its own type
// table in the given field.
func (s scope) withTable(msg []byte, field int) scope {
	if t := find(msg, field); t != nil {
		s.types = readTypeTable(t)
	}
	return s
}

func (s scope) typeID(id int) *Type {
	if s.types == nil || id < 0 || id >= len(s.types.raw) {
		fail("type id %d out of range", id)
	}
	t := s.typ(s.types.raw[id])
	if s.types.firstNullable >= 0 && id >= s.types.firstNullable {
		t.Nullable = true
	}
	return t
}

func (s scope) typ(msg []byte) *Type {
	t := &Type{Param: -1}
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		switch f {
		case 1:
			t.Flags = d.int()
		case 2:
			t.Args = append(t.Args, s.arg(d.bytes()))
		case 3:
			t.Nullable = d.bool()
		case 5:
			t.Upper = s.typ(d.bytes())
		case 8:
			t.Upper = s.typeID(d.int())
		case 6:
			t.Class = s.names.class(d.int())
		case 7:
			t.Param = d.int()
		case 9:
			t.ParamName = s.names.str(d.int())
		case 10:
			t.Outer = s.typ(d.bytes())
		case 11:
			t.Outer = s.typeID(d.int())
		case 100: // JvmProtoBuf.typeAnnotation
			a := d.bytes()
			switch s.annotationClass(a) {
			case "kotlin/ExtensionFunctionType":
				t.ExtFunc = true
			case "androidx/compose/runtime/Composable":
				t.Composable = true
			case "kotlin/ContextFunctionTypeParams":
				t.Contexts = intArg(a)
			case "kotlin/internal/NoInfer":
				t.NoInfer = true
			}
		default: // abbreviations, other extensions
			d.skip(w)
		}
	}
	return t
}

// intArg returns the value of the one argument of an Annotation message,
// an integer: count of @ContextFunctionTypeParams(count).
func intArg(msg []byte) int {
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		if f != 2 || w != wireBytes {
			d.skip(w)
			continue
		}
		arg := decoder{d.bytes()}
		for arg.more() {
			f, w := arg.key()
			if f != 2 || w != wireBytes {
				arg.skip(w)
				continue
			}
			v := decoder{arg.bytes()}
			for v.more() {
				f, w := v.key()
				if f == 2 && w == wireVarint {
					n := v.varint()
					return int(n>>1) ^ -int(n&1) // zigzag
				}
				v.skip(w)
			}
		}
	}
	return 0
}

// annotationClass returns the class of an Annotation message.
func (s scope) annotationClass(msg []byte) string {
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		if f == 1 && w == wireVarint {
			return s.names.class(d.int())
		}
		d.skip(w)
	}
	return ""
}

func (s scope) arg(msg []byte) Arg {
	a := Arg{Proj: Inv}
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		switch f {
		case 1:
			a.Proj = Projection(d.int())
		case 2:
			a.Type = s.typ(d.bytes())
		case 3:
			a.Type = s.typeID(d.int())
		default:
			d.skip(w)
		}
	}
	return a
}

func (s scope) typeParam(msg []byte) *TypeParam {
	p := &TypeParam{Variance: Inv}
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		switch f {
		case 1:
			p.ID = d.int()
		case 2:
			p.Name = s.names.str(d.int())
		case 3:
			p.Reified = d.bool()
		case 4:
			p.Variance = Projection(d.int())
		case 5:
			p.Bounds = append(p.Bounds, s.typ(d.bytes()))
		case 6:
			for _, id := range d.ints(w, nil) {
				p.Bounds = append(p.Bounds, s.typeID(id))
			}
		default:
			d.skip(w)
		}
	}
	return p
}

func (s scope) param(msg []byte) *Param {
	p := &Param{}
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		switch f {
		case 1:
			p.Flags = d.int()
		case 2:
			p.Name = s.names.str(d.int())
		case 3:
			p.Type = s.typ(d.bytes())
		case 5:
			p.Type = s.typeID(d.int())
		case 4:
			p.Vararg = s.typ(d.bytes())
		case 6:
			p.Vararg = s.typeID(d.int())
		default:
			d.skip(w)
		}
	}
	return p
}

// methodSig reads a JvmMethodSignature or JvmFieldSignature; either field
// may be absent.
func (s scope) methodSig(msg []byte) (name, desc string) {
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		switch f {
		case 1:
			name = s.names.str(d.int())
		case 2:
			desc = s.names.str(d.int())
		default:
			d.skip(w)
		}
	}
	return name, desc
}

// bindNames sets Param for types that refer to a type parameter by name.
// Metadata refers by name to the type parameters of the declaration being
// described, and by id to those of enclosing declarations.
func bindNames(params []*TypeParam, ts ...*Type) {
	if len(params) == 0 {
		return
	}
	var bind func(t *Type)
	bind = func(t *Type) {
		if t == nil {
			return
		}
		if t.ParamName != "" && t.Class == "" {
			for _, p := range params {
				if p.Name == t.ParamName {
					t.Param = p.ID
				}
			}
		}
		for _, a := range t.Args {
			bind(a.Type)
		}
		bind(t.Upper)
		bind(t.Outer)
	}
	for _, t := range ts {
		bind(t)
	}
	for _, p := range params {
		for _, b := range p.Bounds {
			bind(b)
		}
	}
}

func paramTypes(ps []*Param) []*Type {
	var ts []*Type
	for _, p := range ps {
		ts = append(ts, p.Type, p.Vararg)
	}
	return ts
}

// contract reads the effects of a Contract message. The callsInPlace
// ones are Effects of type CALLS (1) whose argument is a parameter
// reference, 0 for the receiver and i+1 for parameter i. The implies ones
// are of type RETURNS_CONSTANT (0), whose argument, if any, is the
// constant TRUE (0) or FALSE (1), and whose conclusion is a negated
// (flag 1) null check (flag 2) or an instance check of a parameter
// reference.
func (s scope) contract(msg []byte) ([]InPlace, []Implies) {
	var out []InPlace
	var implies []Implies
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		if f != 1 {
			d.skip(w)
			continue
		}
		e := decoder{d.bytes()}
		typ, kind, param, returns, args := 0, Unknown, -2, ReturnsAny, 0
		var conclusion expression
		hasConclusion := false
		for e.more() {
			f, w := e.key()
			switch f {
			case 1:
				typ = e.int()
			case 4:
				kind = e.int()
			case 2:
				x := s.expression(e.bytes())
				param = x.param
				if x.constant == ReturnsTrue || x.constant == ReturnsFalse {
					returns = x.constant
				} else {
					returns = -2
				}
				args++
			case 3:
				conclusion, hasConclusion = s.expression(e.bytes()), true
			default:
				e.skip(w)
			}
		}
		switch {
		case typ == 1 && param >= -1:
			out = append(out, InPlace{Param: param, Kind: kind})
		case typ == 0 && args <= 1 && returns != -2 && hasConclusion && conclusion.param >= -1 &&
			(conclusion.flags == 3 && conclusion.is == nil || conclusion.flags == 0 && conclusion.is != nil):
			implies = append(implies, Implies{Returns: returns, Param: conclusion.param, Is: conclusion.is})
		}
	}
	return out, implies
}

// expression is the part of a contract's Expression message that kc
// reads: its flags, the parameter it refers to (-1: the receiver, -2:
// none), its constant (-1: none) and the type of its instance check.
type expression struct {
	flags, param, constant int
	is                     *Type
}

func (s scope) expression(msg []byte) expression {
	x := expression{param: -2, constant: -1}
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		switch f {
		case 1:
			x.flags = d.int()
		case 2:
			x.param = d.int() - 1
		case 3:
			x.constant = d.int()
		case 4:
			x.is = s.typ(d.bytes())
		case 5:
			x.is = s.typeID(d.int())
		default:
			d.skip(w)
		}
	}
	return x
}

// oldFlags converts the flags of metadata older than Kotlin 1.1.
func oldFlags(f int) Flags { return Flags(f&0x3f + (f>>8)<<6) }

func (s scope) function(msg []byte) *Function {
	s = s.withTable(msg, 30)
	fn := &Function{Flags: 6}
	var sig []byte
	hasFlags := false
	recvs := 0 // context receivers' types, which kotlinc writes for context parameters too
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		switch f {
		case 9:
			fn.Flags, hasFlags = Flags(d.int()), true
		case 1:
			if o := d.int(); !hasFlags {
				fn.Flags = oldFlags(o)
			}
		case 2:
			fn.Name = s.names.str(d.int())
		case 3:
			fn.Ret = s.typ(d.bytes())
		case 7:
			fn.Ret = s.typeID(d.int())
		case 4:
			fn.TypeParams = append(fn.TypeParams, s.typeParam(d.bytes()))
		case 5:
			fn.Recv = s.typ(d.bytes())
		case 8:
			fn.Recv = s.typeID(d.int())
		case 10:
			d.skip(w)
			recvs++
		case 11:
			recvs += len(d.ints(w, nil))
		case 13:
			fn.ContextParams = append(fn.ContextParams, s.param(d.bytes()))
		case 6:
			fn.Params = append(fn.Params, s.param(d.bytes()))
		case 32:
			fn.InPlace, fn.Implies = s.contract(d.bytes())
		case 100:
			sig = d.bytes()
		default:
			d.skip(w)
		}
	}
	if fn.Ret == nil {
		fail("function %s has no return type", fn.Name)
	}
	fn.Contexts = max(recvs, len(fn.ContextParams))
	bindNames(fn.TypeParams, append(paramTypes(append(fn.ContextParams[:len(fn.ContextParams):len(fn.ContextParams)], fn.Params...)), fn.Recv, fn.Ret)...)
	// JvmProtoBufUtil.getJvmMethodSignature: absent parts are the defaults.
	fn.JVM.Name = fn.Name
	if sig != nil {
		name, desc := s.methodSig(sig)
		if name != "" {
			fn.JVM.Name = name
		}
		fn.JVM.Desc = desc
	}
	if fn.JVM.Desc == "" {
		types := make([]*Type, 0, len(fn.Params)+1)
		if fn.Recv != nil {
			types = append(types, fn.Recv)
		}
		for _, p := range fn.Params {
			types = append(types, p.Type)
		}
		fn.JVM.Desc = defaultDesc(types, fn.Ret)
	}
	return fn
}

func (s scope) property(msg []byte) *Property {
	p := &Property{Flags: 518}
	var sig []byte
	recvs := 0
	hasFlags, hasGetter, hasSetter := false, false, false
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		switch f {
		case 11:
			p.Flags, hasFlags = Flags(d.int()), true
		case 1:
			if o := d.int(); !hasFlags {
				p.Flags = oldFlags(o)
			}
		case 2:
			p.Name = s.names.str(d.int())
		case 3:
			p.Type = s.typ(d.bytes())
		case 9:
			p.Type = s.typeID(d.int())
		case 4:
			p.TypeParams = append(p.TypeParams, s.typeParam(d.bytes()))
		case 5:
			p.Recv = s.typ(d.bytes())
		case 10:
			p.Recv = s.typeID(d.int())
		case 6:
			p.SetterParam = s.param(d.bytes())
		case 12:
			d.skip(w)
			recvs++
		case 13:
			recvs += len(d.ints(w, nil))
		case 17:
			p.ContextParams = append(p.ContextParams, s.param(d.bytes()))
		case 7:
			p.GetterFlags, hasGetter = Flags(d.int()), true
		case 8:
			p.SetterFlags, hasSetter = Flags(d.int()), true
		case 100:
			sig = d.bytes()
		default:
			d.skip(w)
		}
	}
	if p.Type == nil {
		fail("property %s has no type", p.Name)
	}
	p.Contexts = max(recvs, len(p.ContextParams))
	bindNames(p.TypeParams, append(paramTypes(p.ContextParams), p.Recv, p.Type)...)
	// Absent accessor flags share the property's annotations, visibility
	// and modality.
	if !hasGetter {
		p.GetterFlags = p.Flags & 0x3f
	}
	if !hasSetter {
		p.SetterFlags = p.Flags & 0x3f
	}
	if sig != nil {
		sd := decoder{sig}
		for sd.more() {
			f, w := sd.key()
			switch f {
			case 1:
				name, desc := s.methodSig(sd.bytes())
				if name == "" {
					name = p.Name
				}
				if desc == "" {
					desc = mapTypeDefault(p.Type)
				}
				p.Field = &JVMSig{name, desc}
			case 3:
				name, desc := s.methodSig(sd.bytes())
				p.Getter = &JVMSig{name, desc}
			case 4:
				name, desc := s.methodSig(sd.bytes())
				p.Setter = &JVMSig{name, desc}
			case 2:
				name, desc := s.methodSig(sd.bytes())
				p.Synthetic = &JVMSig{name, desc}
			default:
				sd.skip(w)
			}
		}
	}
	return p
}

func (s scope) constructor(msg []byte) *Constructor {
	c := &Constructor{Flags: 6}
	var sig []byte
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		switch f {
		case 1:
			c.Flags = Flags(d.int())
		case 2:
			c.Params = append(c.Params, s.param(d.bytes()))
		case 100:
			sig = d.bytes()
		default:
			d.skip(w)
		}
	}
	c.JVM.Name = "<init>"
	if sig != nil {
		name, desc := s.methodSig(sig)
		if name != "" {
			c.JVM.Name = name
		}
		c.JVM.Desc = desc
	}
	if c.JVM.Desc == "" {
		types := make([]*Type, len(c.Params))
		for i, p := range c.Params {
			types[i] = p.Type
		}
		c.JVM.Desc = defaultDesc(types, &Type{Class: "kotlin/Unit"})
	}
	return c
}

func (s scope) typeAlias(msg []byte) *TypeAlias {
	a := &TypeAlias{Flags: 6}
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		switch f {
		case 1:
			a.Flags = Flags(d.int())
		case 2:
			a.Name = s.names.str(d.int())
		case 3:
			a.TypeParams = append(a.TypeParams, s.typeParam(d.bytes()))
		case 4:
			a.Underlying = s.typ(d.bytes())
		case 5:
			a.Underlying = s.typeID(d.int())
		case 6:
			a.Expanded = s.typ(d.bytes())
		case 7:
			a.Expanded = s.typeID(d.int())
		default:
			d.skip(w)
		}
	}
	bindNames(a.TypeParams, a.Underlying, a.Expanded)
	return a
}

func (s scope) class(msg []byte) *Class {
	s = s.withTable(msg, 30)
	c := &Class{Flags: 6}
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		switch f {
		case 1:
			c.Flags = Flags(d.int())
		case 3:
			c.Name = s.names.class(d.int())
		case 4:
			c.Companion = s.names.str(d.int())
		case 5:
			c.TypeParams = append(c.TypeParams, s.typeParam(d.bytes()))
		case 6:
			c.Supers = append(c.Supers, s.typ(d.bytes()))
		case 2:
			for _, id := range d.ints(w, nil) {
				c.Supers = append(c.Supers, s.typeID(id))
			}
		case 7:
			for _, n := range d.ints(w, nil) {
				c.Nested = append(c.Nested, s.names.str(n))
			}
		case 20:
			d.skip(w)
			c.Contexts++
		case 21:
			c.Contexts += len(d.ints(w, nil))
		case 8:
			c.Ctors = append(c.Ctors, s.constructor(d.bytes()))
		case 9:
			c.Funcs = append(c.Funcs, s.function(d.bytes()))
		case 10:
			c.Props = append(c.Props, s.property(d.bytes()))
		case 11:
			c.Aliases = append(c.Aliases, s.typeAlias(d.bytes()))
		case 13: // EnumEntry: its name in field 1
			ed := decoder{d.bytes()}
			for ed.more() {
				if f, w := ed.key(); f == 1 && w == wireVarint {
					c.EnumEntries = append(c.EnumEntries, s.names.str(ed.int()))
				} else {
					ed.skip(w)
				}
			}
		case 16:
			for _, n := range d.ints(w, nil) {
				c.SealedSubclasses = append(c.SealedSubclasses, s.names.class(n))
			}
		case 101:
			c.ModuleName = s.names.str(d.int())
		case 104:
			c.JVMFlags = d.int()
		case 17:
			c.UnderlyingProp = s.names.str(d.int())
		case 18:
			c.Underlying = s.typ(d.bytes())
		case 19:
			c.Underlying = s.typeID(d.int())
		default:
			d.skip(w)
		}
	}
	if c.Underlying != nil {
		bindNames(c.TypeParams, c.Underlying)
	}
	bindNames(c.TypeParams, c.Supers...)
	for _, k := range c.Ctors {
		bindNames(c.TypeParams, paramTypes(k.Params)...)
	}
	return c
}
