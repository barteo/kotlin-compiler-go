package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Arrays. The array classes and their members are in types (see
// types.ArrayClass). Their factories, arrayOf and the like, are compiler
// intrinsics or inline-only functions with reified type parameters in the
// standard library; kc has them built in, with and without it.

// arrayFuns are the standard library's array factories, and its Char(code)
// and String(chars), which are inline-only.
var arrayFuns = map[string]*types.FunSym{}

func init() {
	generic := func(name string, b types.Builtin, params func(t types.Type) []*types.VarSym, ret func(t types.Type) types.Type) {
		f := &types.FunSym{Name: name, Builtin: b, Resolved: true}
		tp := &types.TypeParamSym{Name: "T", Owner: f}
		t := types.ParamType(tp)
		f.TypeParams, f.Params, f.Ret = []*types.TypeParamSym{tp}, params(t), ret(t)
		arrayFuns[name] = f
	}
	elements := func(t types.Type) []*types.VarSym {
		return []*types.VarSym{{Name: "elements", Type: types.VarargArray(t), Kind: types.Param, Vararg: true}}
	}
	generic("arrayOf", types.ArrayLiteral, elements, types.ArrayOf)
	generic("emptyArray", types.ArrayLiteral, func(types.Type) []*types.VarSym { return nil }, types.ArrayOf)
	generic("arrayOfNulls", types.ArrayNew,
		func(types.Type) []*types.VarSym {
			return []*types.VarSym{{Name: "size", Type: types.IntType, Kind: types.Param}}
		},
		func(t types.Type) types.Type { return types.ArrayOf(t.Nullable()) })
	for name, elem := range map[string]types.Type{
		"intArrayOf": types.IntType, "longArrayOf": types.LongType, "doubleArrayOf": types.DoubleType, "booleanArrayOf": types.BoolType,
		"charArrayOf": types.CharType, "floatArrayOf": types.FloatType, "byteArrayOf": types.ByteType, "shortArrayOf": types.ShortType,
	} {
		arrayFuns[name] = &types.FunSym{Name: name, Builtin: types.ArrayLiteral, Resolved: true, Params: elements(elem), Ret: types.ArrayOf(elem)}
	}
	// enumValues and enumValueOf are intrinsics with a reified type
	// parameter, an enum class.
	generic("enumValues", types.EnumValues, func(types.Type) []*types.VarSym { return nil }, types.ArrayOf)
	generic("enumValueOf", types.EnumValueOf,
		func(types.Type) []*types.VarSym {
			return []*types.VarSym{{Name: "name", Type: types.StringType, Kind: types.Param}}
		},
		func(t types.Type) types.Type { return t })
	arrayFuns["Char"] = &types.FunSym{Name: "Char", Builtin: types.ToChar, Resolved: true,
		Params: []*types.VarSym{{Name: "code", Type: types.IntType, Kind: types.Param}}, Ret: types.CharType}
	arrayFuns["String"] = &types.FunSym{Name: "String", Builtin: types.NewString, Resolved: true,
		Params: []*types.VarSym{{Name: "chars", Type: types.ArrayOf(types.CharType), Kind: types.Param}}, Ret: types.StringType}
}

// arrayCall checks a call of an array factory.
func (c *Checker) arrayCall(x *syntax.Call, f *types.FunSym, want types.Type, sink *system) types.Type {
	t := c.resolve(x, []callee{{f: f, tparams: f.TypeParams}}, want, sink)
	x.Builtin = f.Builtin
	switch f.Builtin {
	case types.EnumValues, types.EnumValueOf:
		c.enumIntrinsic(x, t)
	default:
		c.reified(x, t)
	}
	return t
}

// enumIntrinsic checks the type argument of enumValues<T>() or
// enumValueOf<T>(name), whose result is t: an enum class, or a reified
// type parameter.
func (c *Checker) enumIntrinsic(x *syntax.Call, t types.Type) {
	e := t
	if x.Builtin == types.EnumValues {
		if t.K != types.Class || t.C != types.ArrayClass {
			return
		}
		e = types.ArrayElem(t)
	}
	switch {
	case e.K == types.Invalid || e.HasVars():
	case e.K == types.TypeParam:
		if !e.Param().Reified {
			c.errorf(x.P, "cannot use '%s' as reified type parameter: use a class instead", e.Param().Name)
		}
	case e.K != types.Class || !c.isSubclassOf(e.C, "java/lang/Enum"):
		c.errorf(x.P, "type argument is not within its bounds: '%s' is not an enum class", e)
	}
}

// arrayConstruct checks a call of an array class's constructor:
// IntArray(size), IntArray(size) { init } or Array(size) { init }.
func (c *Checker) arrayConstruct(x *syntax.Call, cls *types.ClassSym, want types.Type, sink *system) types.Type {
	size := &types.VarSym{Name: "size", Type: types.IntType, Kind: types.Param}
	init := func(elem types.Type) *types.VarSym {
		c.useArity(1)
		return &types.VarSym{Name: "init", Type: c.u.ext.funcs.Type([]types.Type{types.IntType}, elem), Kind: types.Param}
	}
	name := cls.DisplayName()
	var cands []callee
	if cls == types.ArrayClass {
		f := &types.FunSym{Name: name, Builtin: types.ArrayInit, Resolved: true}
		tp := &types.TypeParamSym{Name: "T", Owner: f}
		t := types.ParamType(tp)
		f.TypeParams, f.Params, f.Ret = []*types.TypeParamSym{tp}, []*types.VarSym{size, init(t)}, types.ArrayOf(t)
		cands = []callee{{f: f, tparams: f.TypeParams}}
	} else {
		t := types.ClassType(cls)
		cands = []callee{
			{f: &types.FunSym{Name: name, Builtin: types.ArrayNew, Resolved: true, Params: []*types.VarSym{size}, Ret: t}},
			{f: &types.FunSym{Name: name, Builtin: types.ArrayInit, Resolved: true, Params: []*types.VarSym{size, init(cls.Elem)}, Ret: t}},
		}
	}
	t := c.resolve(x, cands, want, sink)
	if x.Sym != nil {
		x.Builtin = x.Sym.Builtin
	}
	c.reified(x, t)
	return t
}

// reified checks the element type of an array a call creates, which must
// be known when the program runs: not a type parameter, which is erased.
func (c *Checker) reified(x *syntax.Call, t types.Type) {
	if t.K != types.Class || t.C != types.ArrayClass || t.HasVars() {
		return
	}
	if e := types.ArrayElem(t); e.K == types.TypeParam && !e.Param().Reified {
		c.errorf(x.P, "cannot use '%s' as reified type parameter: use a class instead", e.Param().Name)
	}
}

// isArrayFactory reports whether a builtin creates an array whose
// element class must be known.
func isArrayFactory(b types.Builtin) bool {
	switch b {
	case types.ArrayNew, types.ArrayInit, types.ArrayLiteral, types.ToTypedArray:
		return true
	}
	return false
}
