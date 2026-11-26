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
		tp := &types.TypeParamSym{Name: "T", Owner: f, Global: true}
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
		c.reified(x, t, sink)
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
	if len(x.Args) == 2 {
		// The init lambda is inlined, as kotlinc's constructors are
		// inline functions: it may return from the code around it.
		init := x.Args[1]
		if len(x.Names) == 2 && x.Names[0] == "init" {
			init = x.Args[0] // Array(init = { ... }, size = n)
		}
		if l, ok := init.(*syntax.Lambda); ok && l.Ref == nil {
			l.Inline = true
			j := c.jumps[l]
			j.callee = name
			c.jumps[l] = j
		}
	}
	t := c.resolve(x, cands, want, sink)
	if x.Sym != nil {
		x.Builtin = x.Sym.Builtin
	}
	c.reified(x, t, sink)
	return t
}

// typedDeadLast reports whether a lambda argument of call x ends in an
// expression that isn't a Nothing, after a jump: kotlinc infers the
// element type of Array(5) { i -> if (i == 0) break; return "Fail"; i * 2 }
// from i * 2, an Int, where kc infers a Nothing.
func typedDeadLast(x *syntax.Call) bool {
	for _, a := range x.Args {
		l, ok := a.(*syntax.Lambda)
		if !ok || l.Body == nil || len(l.Body.Stmts) == 0 {
			continue
		}
		if es, ok := l.Body.Stmts[len(l.Body.Stmts)-1].(*syntax.ExprStmt); ok {
			if t := es.X.Type(); t.K != types.Nothing && t.K != types.Invalid {
				return true
			}
		}
	}
	return false
}

// reified checks the element type of an array a call creates, which must
// be known when the program runs: not a type parameter, which is erased.
//
// A call nested in another's arguments may have its type only from the
// outer call's constraint system, sink: it is checked once that is fixed,
// as with Pair(arrayOf(), arrayOf()) for a Pair<Array<T>, Array<T>>.
func (c *Checker) reified(x *syntax.Call, t types.Type, sink *system) {
	if t.K != types.Class || t.C != types.ArrayClass {
		return
	}
	if t.HasVars() {
		if sink != nil {
			sink.fixups = append(sink.fixups, func(s *system) {
				if f := s.final(t); !f.HasVars() {
					c.reified(x, f, nil)
				}
			})
		}
		return
	}
	switch e := types.ArrayElem(t); {
	case e.K == types.TypeParam && !e.Param().Reified:
		c.errorf(x.P, "cannot use '%s' as reified type parameter: use a class instead", e.Param().Name)
	case erasedArrayElem(e) != nil:
		c.errorf(x.P, "cannot use an array of type parameter '%s' as a reified type argument", erasedArrayElem(e).Name)
	case e.K == types.Nothing && !e.N && (len(x.TypeArgs) == 1 || !typedDeadLast(x)):
		p := x.P
		if len(x.TypeArgs) == 1 {
			p = x.TypeArgs[0].P
		}
		c.errorf(p, "cannot use 'Nothing' as reified type parameter")
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

// arrayOfNothing returns the first array type of elements Nothing or
// Nothing? in t, its arguments included, which the JVM has no class for.
func arrayOfNothing(t types.Type) (types.Type, bool) {
	if t.K != types.Class || t.A == nil {
		return types.Type{}, false
	}
	if t.C == types.ArrayClass {
		if a := t.A.List[0]; a.V != types.Star && nothingElem(a.T) {
			return t, true
		}
	}
	for _, a := range t.A.List {
		if a.V == types.Star {
			continue
		}
		if at, ok := arrayOfNothing(a.T); ok {
			return at, true
		}
	}
	return types.Type{}, false
}

// nothingElem reports whether t is Nothing or Nothing?, of which the JVM
// has no arrays.
func nothingElem(t types.Type) bool {
	return t.K == types.Nothing || t.K == types.Null
}

// arrayNothingMsg is kotlinc's error for array type t, of elements Nothing
// or Nothing?.
func arrayNothingMsg(t types.Type) string {
	if t.A.List[0].T.N {
		return "'Array<Nothing?>' isn't supported on the JVM"
	}
	return "'Array<Nothing>' is an invalid array type"
}

// arrayNothingRef reports an array of Nothing written as ref, of type t:
// its own type, not its arguments', which are references of their own.
type arrayNothingRef struct{ *syntax.TypeRef }

func (c *Checker) arrayNothingRef(ref *syntax.TypeRef, t types.Type) {
	if t.K != types.Class || t.C != types.ArrayClass || t.A == nil || c.done[arrayNothingRef{ref}] {
		return
	}
	if a := t.A.List[0]; a.V != types.Star && nothingElem(a.T) {
		c.done[arrayNothingRef{ref}] = true
		c.errorf(ref.P, "%s", arrayNothingMsg(t))
	}
}

// arrayNothingExpr reports call x, of type t, whose type has an array of
// Nothing in it.
func (c *Checker) arrayNothingExpr(x *syntax.Call, t types.Type) {
	if t.HasVars() {
		return
	}
	at, ok := arrayOfNothing(t)
	if !ok || len(x.TypeArgs) == 0 && typedDeadLast(x) {
		return
	}
	if at.A.List[0].T.N {
		c.errorf(x.P, "%s", arrayNothingMsg(at))
		return
	}
	c.errorf(x.P, "expression cannot have a type of 'Array<Nothing>'")
}

// aliasArrayNothing reports a typealias that expands to a type with an
// array of Nothing in it, and the arrays of Nothing? written in its right
// side.
func (c *Checker) aliasArrayNothing(d *syntax.TypeAliasDecl) {
	a := d.Sym
	if a == nil || !a.Resolved || a.Type.K == types.Invalid {
		return
	}
	if at, ok := arrayOfNothing(a.Type); ok && !at.A.List[0].T.N {
		c.errorf(d.Type.P, "type alias expanded to malformed type '%s'", a.Type)
		return
	}
	saved, savedCls, savedChecks, n := c.tparams, c.lookupCls, c.noBoundChecks, len(c.errs)
	c.tparams, c.lookupCls, c.noBoundChecks = a.TypeParams, a.Outer, true
	var refs []*syntax.TypeRef
	var walk func(ref *syntax.TypeRef)
	walk = func(ref *syntax.TypeRef) {
		if ref == nil {
			return
		}
		t := c.resolveType(ref)
		if t.K == types.Class && t.C == types.ArrayClass && t.A != nil {
			if e := t.A.List[0]; e.V != types.Star && nothingElem(e.T) && e.T.N {
				refs = append(refs, ref)
			}
		}
		var used func(i int) bool = func(int) bool { return true }
		if al := c.lookupAlias(ref.Name); al != nil && c.lookupTypeParam(ref.Name) == nil && len(al.TypeParams) == len(ref.Args) {
			used = func(i int) bool { return mentionsParam(al.Type, al.TypeParams[i]) }
		}
		for i, arg := range ref.Args {
			if used(i) {
				walk(arg.Type)
			}
		}
	}
	walk(d.Type)
	c.errs = c.errs[:n]
	c.tparams, c.lookupCls, c.noBoundChecks = saved, savedCls, savedChecks
	for _, ref := range refs {
		c.errorf(ref.P, "'Array<Nothing?>' isn't supported on the JVM")
	}
}

// erasedArrayElem returns the type parameter, not reified, that t is an
// array of, directly or through arrays of arrays (Array<Array<T>>), or
// nil. Such an array's class isn't known when the program runs, as
// kotlinc's TYPE_PARAMETER_AS_REIFIED_ARRAY_ERROR has it.
func erasedArrayElem(t types.Type) *types.TypeParamSym {
	for t.K == types.Class && t.C == types.ArrayClass && !t.HasVars() {
		t = types.ArrayElem(t)
		if t.K == types.TypeParam {
			if t.Param().Reified {
				return nil
			}
			return t.Param()
		}
	}
	return nil
}
