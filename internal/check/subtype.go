package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// subtype reports whether a value of type t can be used where u is
// expected, without a conversion other than boxing. Platform types are
// flexible: String! is both String and String?, and a Java List is both a
// MutableList and a List.
func (c *Checker) subtype(t, u types.Type) bool {
	if c.subtype1(t, u) {
		return true
	}
	if ro, ok := readOnlyView(u); ok {
		return c.subtype1(t, ro)
	}
	return false
}

// readOnlyView returns a platform mutable collection type as the read-only
// type: Kotlin sees java.util.List as (Mutable)List, which also takes
// read-only lists.
func readOnlyView(u types.Type) (types.Type, bool) {
	if !u.P || u.K != types.Class || u.C.ReadOnly == nil {
		return u, false
	}
	u.C = u.C.ReadOnly
	return u, true
}

func (c *Checker) subtype1(t, u types.Type) bool {
	if r := c.stubConstraint(t, u); r != nil {
		return r.add(t, u)
	}
	switch {
	case t == u, t.K == types.Invalid, u.K == types.Invalid, t.K == types.Nothing:
		return true
	case t.K == types.Null:
		return types.AcceptsNull(u)
	case t.K == types.Unit && u.K == types.Any:
		return true // kotlin.Unit, an object
	case t.K == types.Unit && u.K == types.Class && u.C.Name == "kotlin/Unit":
		return true // Unit as a Unit?
	case t.K == types.Unit, u.K == types.Unit && !t.IsParam(), t.K == types.Range, u.K == types.Range, u.K == types.Nothing, u.K == types.Null:
		return false
	}
	if t.P {
		t = t.Flex()
	}
	// Nullability first; then t and u are compared as non-null types.
	switch {
	case !u.IsParam():
		if types.MayBeNull(t) && !types.AcceptsNull(u) {
			return false
		}
	case u.N: // T? takes anything that T takes, and null
	case u.D: // T & Any
		if types.MayBeNull(t) {
			return false
		}
	default: // T, which may or may not be nullable
		if t.N {
			return false
		}
	}
	if u.IsParam() {
		return c.reachesParam(t, u.Param())
	}
	t, u = t.NonNull(), u.NonNull()
	if t.IsParam() {
		if len(t.Param().Bounds) == 0 {
			return u.K == types.Any
		}
		for _, b := range t.Param().Bounds {
			if c.subtype(b.NonNull(), u) {
				return true
			}
		}
		return false
	}
	switch u.K {
	case types.Any:
		return true
	case types.Class:
		if u.A == nil {
			// A non-generic class: no arguments to compare.
			var tc *types.ClassSym
			switch t.K {
			case types.Class:
				if t.C.Array && (u.C.Name == "java/lang/Cloneable" || u.C.Name == "java/io/Serializable") {
					return true // as JVM arrays are
				}
				tc = t.C
			case types.Int, types.Long, types.Double, types.Bool, types.Char, types.Float, types.Byte, types.Short, types.String:
				tc = c.jvmClass(t)
			}
			return tc != nil && (tc == u.C || tc.IsSubclassOf(u.C))
		}
		sup, ok := c.asSuper(t, u.C)
		return ok && c.argsFit(sup, u)
	}
	return t.K == u.K && t.K != types.Class && t.K != types.Any
}

// reachesParam reports whether t is the type parameter p, or a parameter
// bounded (directly or not) by p, as U is in <T, U : T>.
func (c *Checker) reachesParam(t types.Type, p *types.TypeParamSym) bool {
	if !t.IsParam() {
		return false
	}
	if t.Param() == p {
		return true
	}
	for _, b := range t.Param().Bounds {
		if c.reachesParam(b, p) {
			return true
		}
	}
	return false
}

// argsFit compares the type arguments of sup, which has u's class, with
// u's, by each argument's variance.
func (c *Checker) argsFit(sup, u types.Type) bool {
	if u.A == nil || sup.A == nil || sup.A.Len() != u.A.Len() {
		return true // non-generic, or a raw Java type
	}
	u.C.Ensure() // its type parameters' variance
	for i, ua := range u.A.List {
		declared, bound := types.Inv, types.AnyType.Nullable()
		if i < len(u.C.TypeParams) {
			tp := u.C.TypeParams[i]
			declared = tp.Var
			if len(tp.Bounds) == 1 && !tp.Bounds[0].HasParams() {
				bound = tp.Bounds[0]
			}
		}
		if !c.argFits(sup.A.List[i], ua, declared, bound) {
			return false
		}
	}
	return true
}

// argFits reports whether type argument ta fits ua, of a type parameter
// of variance declared: a star, as out, reads as the parameter's bound.
func (c *Checker) argFits(ta, ua types.Arg, declared types.Variance, bound types.Type) bool {
	if ua.V == types.Star {
		return true
	}
	v := ua.V
	if v == types.Inv {
		v = declared
	}
	switch v {
	case types.Out:
		switch ta.V {
		case types.Star:
			return c.subtype(bound, ua.T)
		case types.In:
			return c.subtype(types.AnyType.Nullable(), ua.T)
		}
		return c.subtype(ta.T, ua.T)
	case types.In:
		if ta.V == types.Star || ta.V == types.Out {
			return ua.T.K == types.Nothing
		}
		return c.subtype(ua.T, ta.T)
	}
	return ta.V == types.Inv && c.sameType(ta.T, ua.T)
}

// sameType reports whether t and u are equivalent: subtypes of each
// other, as String! and String are.
func (c *Checker) sameType(t, u types.Type) bool {
	return t == u || c.subtype(t, u) && c.subtype(u, t)
}

// asSuper returns t seen as an instance of class target, with target's
// type arguments: ArrayList<String> as List<String>, or Int as
// Comparable<Int!>. It reports false if t is not a subtype of target.
func (c *Checker) asSuper(t types.Type, target *types.ClassSym) (types.Type, bool) {
	t = t.NonNull()
	if t.HasVars() {
		return c.asSuper1(t, target, 0) // inference variables are not reused
	}
	k := superKey{t, target}
	if r, ok := c.u.ext.supers.Load(k); ok {
		r := r.(superResult)
		return r.t, r.ok
	}
	st, ok := c.asSuper1(t, target, 0)
	c.u.ext.supers.Store(k, superResult{st, ok})
	return st, ok
}

func (c *Checker) asSuper1(t types.Type, target *types.ClassSym, depth int) (types.Type, bool) {
	if depth > 50 {
		return types.InvalidType, false
	}
	switch t.K {
	case types.Class:
		if t.C == target {
			return t, true
		}
		if t.C.Func && t.A != nil && target.Name == "kotlin/Function" && !t.C.Suspend && !t.C.Composable {
			// A function type is a kotlin.Function of its result:
			// callsInPlace(block) takes a Function<R>.
			return types.GenericType(target, []types.Arg{{T: t.A.List[t.A.Len()-1].T}}), true
		}
		for _, s := range c.directSupers(t) {
			if r, ok := c.asSuper1(s, target, depth+1); ok {
				return r, true
			}
		}
	case types.TypeParam, types.TypeVar:
		for _, b := range t.Param().Bounds {
			if r, ok := c.asSuper1(b.NonNull(), target, depth+1); ok {
				return r, true
			}
		}
	case types.Int, types.Long, types.Double, types.Bool, types.Char, types.Float, types.Byte, types.Short, types.String:
		if jc := c.jvmClass(t); jc != nil {
			return c.asSuper1(types.ClassType(jc), target, depth+1)
		}
	}
	return types.InvalidType, false
}

// directSupers returns the direct supertypes of a Java class type, with
// its type arguments substituted. A class read without a signature has
// raw supertypes, seen as star-projected.
func (c *Checker) directSupers(t types.Type) []types.Type {
	cls := t.C
	cls.Ensure()
	var supers []types.Type
	if cls.Supers != nil {
		supers = cls.Supers
	} else {
		if cls.Super != nil {
			supers = append(supers, rawType(cls.Super))
		}
		for _, i := range cls.Interfaces {
			supers = append(supers, rawType(i))
		}
	}
	s := classSubst(t)
	if s == nil {
		return supers
	}
	out := make([]types.Type, len(supers))
	for i, st := range supers {
		out[i] = s.Apply(st)
	}
	return out
}

// rawType returns a class used without type arguments, as Java code may
// do: star projections for each of its type parameters.
func rawType(cls *types.ClassSym) types.Type {
	cls.Ensure()
	if len(cls.TypeParams) == 0 {
		return types.ClassType(cls)
	}
	args := make([]types.Arg, len(cls.TypeParams))
	for i := range args {
		args[i] = types.Arg{V: types.Star}
	}
	return types.GenericType(cls, args)
}

// classSubst maps the type parameters of t's class to t's arguments. A raw
// type maps them to star projections.
func classSubst(t types.Type) types.Subst {
	ps := t.C.TypeParams
	if len(ps) == 0 {
		return nil
	}
	s := make(types.Subst, len(ps))
	for i, p := range ps {
		if t.A != nil && i < t.A.Len() {
			s[p] = t.A.Get(i)
		} else {
			s[p] = types.Arg{V: types.Star}
		}
	}
	return s
}

// memberSubst returns the substitution for members that class owner
// declares, seen through a receiver of type rt: for ArrayList<String>
// and List's get, E := String.
func (c *Checker) memberSubst(rt types.Type, owner *types.ClassSym) types.Subst {
	if owner == nil || len(owner.TypeParams) == 0 {
		return nil
	}
	sup, ok := c.asSuper(rt, owner)
	if !ok {
		return nil
	}
	return classSubst(sup)
}

// stubJoin returns the join of a and b, if one has a variable that
// stubInference is inferring: as Kotlin's call that joins branches, a
// variable above both, in a system that joins the inference (see joinRun).
// if (c) replaceTypeVariable(x) else DifferentType() gives PTV no upper
// bound DifferentType.
func (c *Checker) stubJoin(a, b types.Type) (types.Type, bool) {
	isStub := func(u types.Type) bool { return u.K == types.TypeVar && c.stubs[u.Param()] != nil }
	if len(c.stubs) == 0 || !a.Has(isStub) && !b.Has(isStub) {
		return types.InvalidType, false
	}
	s := newSystem(c)
	p := &types.TypeParamSym{Name: "K"}
	k := s.fresh([]*types.TypeParamSym{p}, nil, syntax.Pos{})[p].T
	s.sub(a, k)
	s.sub(b, k)
	r := c.joinRun(s)
	if !s.ok || r == nil {
		return types.InvalidType, false
	}
	r.join(s)
	return k, true
}

// join returns the common supertype of two branches, or false if they
// have none in the subset. Different builtin types, such as Int and
// String, have none.
func (c *Checker) join(a, b types.Type) (types.Type, bool) {
	switch {
	case a.K == types.Invalid || b.K == types.Invalid:
		return types.InvalidType, true
	case a.K == types.Nothing:
		return b, true
	case b.K == types.Nothing:
		return a, true
	case a.K == types.Null:
		return b.Nullable(), b.K != types.Unit && b.K != types.Range
	case b.K == types.Null:
		return a.Nullable(), a.K != types.Unit && a.K != types.Range
	}
	if a.NonNull() == b.NonNull() {
		switch {
		case b.N && !b.P:
			return b, true
		case a.N && !a.P:
			return a, true // T? and T, for a T that may be null, give T?
		case types.MayBeNull(b) && !b.P:
			return b, true
		}
		return a, true
	}
	if t, ok := c.stubJoin(a, b); ok {
		return t, true
	}
	if c.subtype(a, b) {
		return b, true
	}
	if c.subtype(b, a) {
		return a, true
	}
	if isBuiltin(a) && isBuiltin(b) {
		// Two different value types, as Char and String, have only Any
		// in common to kc: Kotlin's Comparable<*> & Serializable is an
		// intersection type.
		if a.K == types.Range || b.K == types.Range {
			return types.InvalidType, false
		}
		t := types.AnyType
		if a.Flex().NonNull().IsNumeric() && b.Flex().NonNull().IsNumeric() {
			// Two numbers: Number, which they also are.
			if n := c.u.ext.class("java/lang/Number"); n != nil {
				t = types.ClassType(n)
			}
		}
		if types.MayBeNull(a) || types.MayBeNull(b) {
			return t.Nullable(), true
		}
		return t, true
	}
	t := c.commonSuper(a.NonNull(), b.NonNull(), 0)
	if types.MayBeNull(a) || types.MayBeNull(b) {
		t = t.Nullable()
	}
	return t, true
}

// joinBranch joins the type of a branch of an if, when or try, bt, with
// the result of the branches before it. Where they are different value
// types, as Int and String, and the expected type is one of them, the
// branch is reported, as kotlinc checks each branch against it.
func (c *Checker) joinBranch(result, bt, want types.Type, p syntax.Pos) (types.Type, bool) {
	if want.K != types.Invalid && !want.HasVars() && isBuiltin(result) && isBuiltin(bt) && result.NonNull() != bt.NonNull() &&
		!c.subtype(types.AnyType, want.NonNull()) {
		if !c.subtype(bt, want) {
			c.errorf(p, "type mismatch: inferred type is %s but %s was expected", bt, want)
		} else {
			c.errorf(p, "type mismatch: inferred type is %s but %s was expected", result, want)
		}
		return want, true
	}
	t, ok := c.join(result, bt)
	if want.K != types.Invalid && !want.HasVars() && (!ok || !c.subtype(t, want)) && c.subtype(result, want) && c.subtype(bt, want) {
		// Both fit the expected type, which their first common supertype
		// doesn't: MutableList for an ArrayList and another RandomAccess
		// list.
		return want, true
	}
	return t, ok
}

func isBuiltin(t types.Type) bool {
	switch t.K {
	case types.Unit, types.Range, types.Int, types.Long, types.Double, types.Bool, types.Char, types.Float, types.Byte, types.Short, types.String:
		return true
	}
	return false
}

// commonSuper returns the first supertype of a, in breadth-first order,
// that b also has, or Any. Marker interfaces such as Serializable, and
// Java interfaces, are only used if nothing else is shared.
func (c *Checker) commonSuper(a, b types.Type, depth int) types.Type {
	if a.IsParam() {
		a = a.Param().Bound().NonNull()
	}
	// Kotlin's is the intersection of the common supertypes, of which
	// kc takes one: not Parcelable for objects of a sealed class some of
	// which are Parcelable too.
	var weak types.Type
	for _, s := range c.supertypes(a) {
		sb, ok := c.asSuper(b, s.C)
		if !ok {
			continue
		}
		r := c.joinArgs(s, sb, depth)
		javaIface := s.C.External && s.C.Interface && s.C.KProps == nil && s.C.KotlinName == ""
		if !markerInterfaces[s.C.Name] && !javaIface {
			return r
		}
		if weak.K == types.Invalid {
			weak = r
		}
	}
	if weak.K != types.Invalid {
		return weak
	}
	return types.AnyType
}

var markerInterfaces = map[string]bool{
	"java/io/Serializable":            true,
	"java/lang/Cloneable":             true,
	"java/lang/constant/Constable":    true,
	"java/lang/constant/ConstantDesc": true,
}

// joinArgs combines the arguments of two instances of the same class:
// equal arguments stay, others become out-projected common supertypes,
// or star projections when nested too deeply.
func (c *Checker) joinArgs(a, b types.Type, depth int) types.Type {
	if a.A == nil || b.A == nil || a.A.Len() != b.A.Len() {
		return a
	}
	args := make([]types.Arg, a.A.Len())
	for i, x := range a.A.List {
		y := b.A.List[i]
		// The argument of a covariant parameter needs no projection, and
		// is joined deeper: List<Pair<String, Nothing?>> and
		// List<Pair<String, String>> give List<Pair<String, String?>>.
		covariant := i < len(a.C.TypeParams) && a.C.TypeParams[i].Var == types.Out
		contravariant := i < len(a.C.TypeParams) && a.C.TypeParams[i].Var == types.In
		switch {
		case x.V == types.Inv && y.V == types.Inv && c.sameType(x.T, y.T):
			args[i] = x
		case contravariant:
			// That of a contravariant parameter is the smaller one:
			// Comparable<Int> and Comparable<UShort> give Comparable<*>.
			switch {
			case x.V == types.Inv && y.V == types.Inv && c.subtype(x.T, y.T):
				args[i] = x
			case x.V == types.Inv && y.V == types.Inv && c.subtype(y.T, x.T):
				args[i] = y
			default:
				args[i] = types.Arg{V: types.Star}
			}
		case x.V != types.Star && y.V != types.Star && x.V != types.In && y.V != types.In && (depth < 1 || covariant && depth < 3):
			t := c.commonSuper(x.T.NonNull(), y.T.NonNull(), depth+1)
			switch {
			case x.T.NonNull().K == types.Nothing:
				// Pair<String, Nothing?> and Pair<String, String> give
				// Pair<String, String?>.
				t = y.T
				if types.MayBeNull(x.T) {
					t = t.Nullable()
				}
			case y.T.NonNull().K == types.Nothing:
				t = x.T
				if types.MayBeNull(y.T) {
					t = t.Nullable()
				}
			case c.subtype(x.T, y.T):
				t = y.T
			case c.subtype(y.T, x.T):
				t = x.T
			case types.MayBeNull(x.T) || types.MayBeNull(y.T):
				t = t.Nullable()
			}
			args[i] = types.Arg{V: types.Out, T: t}
			if covariant {
				args[i].V = types.Inv
			}
		default:
			args[i] = types.Arg{V: types.Star}
		}
	}
	return types.GenericType(a.C, args)
}

// supertypes lists t and its supertypes as class types, breadth first,
// without java.lang.Object.
func (c *Checker) supertypes(t types.Type) []types.Type {
	switch t.K {
	case types.Class:
	case types.Int, types.Long, types.Double, types.Bool, types.Char, types.Float, types.Byte, types.Short, types.String:
		jc := c.jvmClass(t)
		if jc == nil {
			return nil
		}
		t = types.ClassType(jc)
	default:
		return nil
	}
	var out []types.Type
	seen := map[*types.ClassSym]bool{}
	queue := []types.Type{t}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		if seen[s.C] || s.C.Name == "java/lang/Object" {
			continue
		}
		seen[s.C] = true
		out = append(out, s)
		queue = append(queue, c.directSupers(s)...)
	}
	return out
}

// stubConstraint returns the run to record t <: u in, if t or u is a
// variable that stubInference is inferring.
func (c *Checker) stubConstraint(t, u types.Type) *stubRun {
	if len(c.stubs) == 0 {
		return nil
	}
	if r := c.stub(t.Flex()); r != nil {
		return r
	}
	return c.stub(u.Flex())
}
