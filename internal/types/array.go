package types

// Arrays. Kotlin's Array<T> and its primitive arrays, such as IntArray,
// have no class files: kotlinc knows them, and so does kc. They are
// classes whose members are builtins, generated as JVM array
// instructions. Array<T> is invariant, as in Kotlin; its descriptor is
// that of a JVM array of the erasure of T, with primitives boxed:
// Array<Int> is [Ljava/lang/Integer;, IntArray is [I.

// ArrayClass is kotlin.Array<T>.
var ArrayClass = newArrayClass()

// primArrays are the primitive arrays, by element kind.
var primArrays = map[Kind]*ClassSym{}

// arraysByID are the array classes by class id: kotlin/IntArray.
var arraysByID = map[string]*ClassSym{"kotlin/Array": ArrayClass}

func init() {
	for _, a := range []struct {
		name string
		elem Type
		desc string
	}{
		{"IntArray", IntType, "[I"},
		{"LongArray", LongType, "[J"},
		{"DoubleArray", DoubleType, "[D"},
		{"BooleanArray", BoolType, "[Z"},
		{"CharArray", CharType, "[C"},
		{"FloatArray", FloatType, "[F"},
		{"ByteArray", ByteType, "[B"},
		{"ShortArray", ShortType, "[S"},
	} {
		c := &ClassSym{Name: a.desc, KotlinName: "kotlin/" + a.name, External: true, Final: true, Array: true, Elem: a.elem}
		arrayMembers(c, a.elem)
		primArrays[a.elem.K] = c
		arraysByID[c.KotlinName] = c
	}
}

func newArrayClass() *ClassSym {
	c := &ClassSym{Name: "[Ljava/lang/Object;", KotlinName: "kotlin/Array", External: true, Final: true, Array: true}
	t := &TypeParamSym{Name: "T", Owner: c}
	c.TypeParams = []*TypeParamSym{t}
	arrayMembers(c, ParamType(t))
	return c
}

// arrayMembers declares an array class's members, whose elements have
// type elem: size, get, set and clone.
func arrayMembers(c *ClassSym, elem Type) {
	index := func() *VarSym { return &VarSym{Name: "index", Type: IntType, Kind: Param} }
	c.Methods = map[string][]*FunSym{
		"get": {{Name: "get", Owner: c, Params: []*VarSym{index()}, Ret: elem, Builtin: ArrayGet,
			Operator: true, Final: true, Resolved: true}},
		"set": {{Name: "set", Owner: c, Params: []*VarSym{index(), {Name: "value", Type: elem, Kind: Param}}, Ret: UnitType,
			Builtin: ArraySet, Operator: true, Final: true, Resolved: true}},
		"clone": {{Name: "clone", Owner: c, Ret: c.This(), Builtin: ArrayClone, Final: true, Resolved: true}},
	}
	c.KProps = map[string]*KProp{
		"size": {Getter: &FunSym{Name: "size", Owner: c, Ret: IntType, Builtin: ArraySize, Final: true, Resolved: true}},
	}
	c.Statics = map[string][]*FunSym{}
	c.Fields = map[string]*VarSym{}
}

// ArrayByID returns the array class with a Kotlin class id, such as
// kotlin/IntArray, or nil.
func ArrayByID(id string) *ClassSym { return arraysByID[id] }

// ArrayByName returns the array class with a simple name, such as
// IntArray, or nil.
func ArrayByName(name string) *ClassSym { return arraysByID["kotlin/"+name] }

// IsArray reports whether t is an array type: Array<T> or a primitive
// array, maybe nullable.
func (t Type) IsArray() bool { return t.K == Class && t.C.Array }

// ArrayOf returns the type of arrays of elem: a primitive array for a
// non-null primitive, otherwise Array<elem>. It returns Invalid for an
// invalid element type.
func ArrayOf(elem Type) Type {
	if elem.K == Invalid {
		return InvalidType
	}
	if c, ok := primArrays[elem.K]; ok && !elem.N {
		return ClassType(c)
	}
	return GenericType(ArrayClass, []Arg{{T: elem}})
}

// VarargArray returns the type a vararg parameter whose elements have type
// elem has in the function's body: IntArray for Int, Array<out T> for T.
func VarargArray(elem Type) Type {
	if elem.K == Invalid {
		return InvalidType
	}
	if c, ok := primArrays[elem.K]; ok && !elem.N {
		return ClassType(c)
	}
	return GenericType(ArrayClass, []Arg{{V: Out, T: elem}})
}

// VarargElem returns the type of the elements of a vararg parameter of
// type t: an array's, or for an unsigned array, which a vararg of
// unsigned integers is (UIntArray), its elements' (UInt).
func VarargElem(t Type) Type {
	if t.K == Class && IsUnsignedArray(t.C) {
		t.C.Ensure()
		for _, f := range t.C.Methods["get"] {
			return f.Ret
		}
		return InvalidType
	}
	return ArrayElem(t)
}

// IsUnsignedArray reports whether cls is one of the standard library's
// arrays of unsigned integers, such as UIntArray.
func IsUnsignedArray(cls *ClassSym) bool {
	switch cls.Name {
	case "kotlin/UIntArray", "kotlin/ULongArray", "kotlin/UByteArray", "kotlin/UShortArray":
		return true
	}
	return false
}

// ArrayElem returns the type of the elements read from an array of type
// t: T for Array<T> and Array<out T>, Any? through in and star
// projections. It returns Invalid if t is not an array type.
func ArrayElem(t Type) Type {
	if !t.IsArray() {
		return InvalidType
	}
	if t.C != ArrayClass {
		return t.C.Elem
	}
	if t.A == nil {
		return AnyType.Nullable()
	}
	switch a := t.A.Get(0); a.V {
	case Inv, Out:
		return a.T
	}
	return AnyType.Nullable()
}

// StoredElem returns the type of the elements of an array of type t as
// the JVM array holds them: the element type, boxed in an Array<T>.
func StoredElem(t Type) Type {
	e := ArrayElem(t)
	switch {
	case t.C == ArrayClass && e.IsPrimitive():
		return e.Nullable()
	case t.C == ArrayClass && e.K == Unit:
		return AnyType.Nullable() // the kotlin.Unit object
	case IsValue(e):
		return Boxed(e)
	}
	return e
}

// arrayDesc returns the descriptor of an array type.
func arrayDesc(t Type) (string, bool) {
	if t.C != ArrayClass {
		return t.C.Name, true
	}
	elem := AnyType.Nullable()
	if t.A != nil {
		if a := t.A.Get(0); a.V == Inv || a.V == Out {
			elem = a.T
		}
	}
	switch {
	case IsValue(elem):
		return "[" + BoxedDesc(elem), true // an array of boxes
	case elem.K == Unit:
		return "[Lkotlin/Unit;", true
	case elem.K == Nothing:
		return "[Ljava/lang/Void;", true
	}
	d, ok := Desc(elem.Nullable())
	return "[" + d, ok
}

// ClassName returns the JVM name of the class of values of type t, as
// instructions such as checkcast and instanceof take it: an internal
// name, or the descriptor of an array type.
func ClassName(t Type) string {
	if IsValue(t) {
		return t.C.Name
	}
	if t.K == Unit {
		return "kotlin/Unit" // Unit as a type argument, which Nullable leaves
	}
	d, _ := Desc(t.Nullable())
	if d[0] == 'L' {
		return d[1 : len(d)-1]
	}
	return d
}
