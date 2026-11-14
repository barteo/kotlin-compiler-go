package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file checks type checks and casts, x is T and x as T, and the smart
// casts that follow from them.

func (c *Checker) isExpr(x *syntax.Is) types.Type {
	st := c.value(x.X, noWant)
	declared := types.InvalidType
	var known []types.Type
	if v := c.narrowSym(x.X); v != nil {
		declared = v.Type
		known = append(c.earlierCasts(x.X, v), declared)
	}
	x.Target = c.checkTarget(x.Ref, st, known, true)
	orig := st
	if declared.K != types.Invalid {
		orig = declared // kotlinc checks the type before smart casts
	}
	c.impossibleIs(x.P, orig, st, x.Target, x.Not)
	return types.BoolType
}

// whenIsCheck checks the is or !is condition cond of when x, whose
// subject has type st, as isExpr checks x is T.
func (c *Checker) whenIsCheck(x *syntax.When, cond *syntax.WhenCond, st types.Type) {
	orig := st
	if x.SubjVar == "" {
		if v := c.narrowSym(x.Subject); v != nil {
			orig = v.Type
		}
	}
	c.impossibleIs(cond.P, orig, st, cond.Target, cond.NotIs)
}

// impossibleIs reports x is T, or x !is T, that can never be true, or
// never false, as kotlinc's IMPOSSIBLE_IS_CHECK does (its RULES1): the
// type orig of x before smart casts, and T, with their type arguments
// erased, are unrelated, and one of them is a final class, or both are
// classes rather than interfaces. smart is the smart-cast type of x: if
// both it and T may be null, the check holds for null alone.
func (c *Checker) impossibleIs(p syntax.Pos, orig, smart, t types.Type, not bool) {
	if orig.K == types.Invalid || t.K == types.Invalid || c.subtype(orig, t) {
		return // a useless check, if anything
	}
	l, ok1 := c.castInfo(orig, 0)
	r, ok2 := c.castInfo(t, 0)
	if !ok1 || !ok2 {
		return
	}
	switch {
	case l.final || r.final:
	case l.concrete && r.concrete && l.class && r.class:
	default:
		return
	}
	if c.erasedSub(l.parts, r.parts) || c.erasedSub(r.parts, l.parts) {
		return
	}
	null := types.MayBeNull(smart) && types.MayBeNull(t)
	diag := "IMPOSSIBLE_IS_CHECK_ERROR"
	if null {
		diag = "IMPOSSIBLE_IS_CHECK_RELYING_ON_NULL_ERROR"
	}
	if c.file != nil && suppresses(c.file.Annotations, diag) {
		return
	}
	if null {
		c.errorf(p, "check for instance is always '%v': %s and %s have no common instances but null", !not, orig, t)
		return
	}
	c.errorf(p, "check for instance is always '%v': %s and %s are incompatible", not, orig, t)
}

// castTypeInfo describes a type, as impossibleIs compares it: the
// intersection of parts, non-null types with their type arguments
// star-projected, and whether one of them is a final class, one is a
// class rather than an interface, and there is only one.
type castTypeInfo struct {
	parts                  []types.Type
	final, class, concrete bool
}

// mappedClasses are Java classes that kotlinc maps to Kotlin ones, whose
// relation to them kc's types may not show.
var mappedClasses = map[string]bool{
	"java/lang/Object": true, "java/lang/String": true, "java/lang/Integer": true, "java/lang/Long": true,
	"java/lang/Short": true, "java/lang/Byte": true, "java/lang/Character": true, "java/lang/Boolean": true,
	"java/lang/Float": true, "java/lang/Double": true, "java/lang/Void": true, "java/lang/Number": true,
	"kotlin/Unit": true, "kotlin/Nothing": true,
}

// castInfo returns t's castTypeInfo. It reports false for a type that
// impossibleIs doesn't judge.
func (c *Checker) castInfo(t types.Type, depth int) (castTypeInfo, bool) {
	t = t.NonNull()
	t.P = false
	switch t.K {
	case types.Bool, types.Int, types.Long, types.Double, types.Char, types.Float, types.Byte, types.Short, types.String:
		return castTypeInfo{parts: []types.Type{t}, final: true, class: true, concrete: true}, true
	case types.Any:
		return castTypeInfo{parts: []types.Type{t}, class: true, concrete: true}, true
	case types.Class:
		cls := t.C
		cls.Ensure()
		if cls.Missing || mappedClasses[cls.Name] || cls.Anonymous {
			return castTypeInfo{}, false
		}
		if t.A != nil {
			args := make([]types.Arg, t.A.Len())
			for i := range args {
				args[i] = types.Arg{V: types.Star}
			}
			t = types.GenericType(cls, args)
		}
		return castTypeInfo{parts: []types.Type{t}, final: cls.Final && !cls.Interface, class: !cls.Interface, concrete: true}, true
	case types.TypeParam:
		p := t.Param()
		if depth > 10 {
			return castTypeInfo{}, false
		}
		if len(p.Bounds) == 0 {
			return c.castInfo(types.AnyType, depth+1)
		}
		var info castTypeInfo
		for _, b := range p.Bounds {
			bi, ok := c.castInfo(b, depth+1)
			if !ok {
				return castTypeInfo{}, false
			}
			info.parts = append(info.parts, bi.parts...)
			info.final = info.final || bi.final
			info.class = info.class || bi.class
		}
		info.concrete = len(info.parts) == 1
		return info, true
	}
	return castTypeInfo{}, false
}

// erasedSub reports whether the intersection of types ts is a subtype of
// the intersection of us.
func (c *Checker) erasedSub(ts, us []types.Type) bool {
	for _, u := range us {
		if !slices.ContainsFunc(ts, func(t types.Type) bool { return c.subtype(t, u) }) {
			return false
		}
	}
	return true
}

func (c *Checker) asExpr(x *syntax.As) types.Type {
	st := c.castOperand(x) // see expected.go
	t := c.checkTarget(x.Ref, st, nil, false)
	x.Target = t
	if t.K == types.Invalid || st.K == types.Invalid {
		return types.InvalidType
	}
	if x.Safe {
		return t.Nullable()
	}
	if k, ok := c.castNarrow(x); ok {
		// x is a T from here on: in (x as T).f(x), (x as T) + x.
		c.scope.narrows = append(c.scope.narrows, k)
	}
	if t.K == types.Class && t.C.Name == "kotlin/Unit" && !t.N {
		return types.UnitType // x as Unit, a cast to kotlin.Unit
	}
	return t
}

// earlierCasts returns the smart casts of a val x, named v, that a later
// one replaced: all of them still hold, but kc, without intersection
// types, keeps only the last in x's type. After x as Function2<A, B, C>
// and assertTrue(x is SuspendFunction1<A, C>), x is Function2<A, B, C>
// is still known.
func (c *Checker) earlierCasts(x syntax.Expr, v *types.VarSym) []types.Type {
	if _, ok := x.(*syntax.Ident); !ok || v.Mutable {
		return nil
	}
	var ts []types.Type
	for s := c.scope; s != nil; s = s.parent {
		for _, n := range s.narrows {
			if n.sym == v && n.recv == nil && !n.onlyNot {
				ts = append(ts, n.t)
			}
		}
	}
	return ts
}

// checkTarget resolves the type T of x is T or x as T, where x has type
// subject. On the JVM, type arguments are erased: is can only check them
// when the subject's type determines them, or one of the types known
// that x has, if a smart cast has changed it, which kc, without
// intersection types, no longer sees in subject: the type x is declared
// with and its earlier smart casts. as doesn't check them.
func (c *Checker) checkTarget(ref *syntax.TypeRef, subject types.Type, known []types.Type, is bool) types.Type {
	c.bareRef, c.bareSubject = ref, subject
	t := c.resolveType(ref)
	c.bareRef = nil
	switch {
	case t.K == types.Invalid:
		return t
	case t.K == types.Unit:
		// The kotlin.Unit object's class.
		if u := c.u.ext.class("kotlin/Unit"); u != nil {
			return types.ClassType(u)
		}
		c.errorf(ref.P, "cannot check for an instance of %s", t)
		return types.InvalidType
	case t.K == types.Range:
		c.errorf(ref.P, "cannot check for an instance of %s", t)
		return types.InvalidType
	case t.IsParam():
		if is && !t.Param().Reified && !c.paramKnown(t, subject) && !slices.ContainsFunc(known, func(k types.Type) bool { return c.paramKnown(t, k) }) {
			c.errorf(ref.P, "cannot check for instance of erased type: %s", t)
			return types.InvalidType
		}
		return t
	case t.K != types.Class:
		return t
	case t.C.Func && t.C.Composable:
		c.unsupported(ref.P, "compose", "is and as with composable function types are not supported")
		return types.InvalidType
	}
	if is && t.A != nil && !allStar(t) && !c.argsKnown(t, subject, ref) && !slices.ContainsFunc(known, func(k types.Type) bool { return c.argsKnown(t, k, ref) }) {
		c.errorf(ref.P, "cannot check for instance of erased type: %s", t)
		return types.InvalidType
	}
	if is && t.A != nil && allStar(t) {
		// x is C<*> narrows x to C<*> and its own type, whose arguments
		// it has where they follow from it: a Flow<T> that is a
		// CancellableFlow<*> is a CancellableFlow<T>. Not if they are
		// outside C's bounds, as a DeserializationStrategy<T> that is an
		// AbstractPolymorphicSerializer<*>, whose T : Any.
		if d, _, _ := c.bareArgs(t.C, ref, subject); c.argsInBounds(d) {
			return d
		}
	}
	return t
}

// paramKnown reports whether x is T, for a type parameter T that isn't
// reified, can be checked where x has type subject: as in kotlinc, if it
// is an upcast, or checks that a T? is a T that can't be null.
func (c *Checker) paramKnown(t, subject types.Type) bool {
	if subject.K == types.Invalid {
		return false
	}
	return c.subtype(subject, t) || !types.MayBeNull(t) && subject.NonNull() == t.NonNull()
}

// argsInBounds reports whether the type arguments of class type t are
// within the bounds of its class's type parameters.
func (c *Checker) argsInBounds(t types.Type) bool {
	if t.K != types.Class || t.A == nil {
		return true
	}
	ps := t.C.TypeParams
	s := classSubst(t)
	for i, a := range t.A.List {
		if a.V == types.Star || i >= len(ps) {
			continue
		}
		for _, b := range ps[i].Bounds {
			if !c.subtype(a.T, s.Apply(b)) {
				return false
			}
		}
	}
	return true
}

// bareType returns the type of class cls, written without its type
// arguments in x is C or x as C, where x has type subject: as in Kotlin,
// with the arguments that follow from the subject's type, x is List for
// an x: Collection<String> giving List<String>, and * for the others.
func (c *Checker) bareType(cls *types.ClassSym, ref *syntax.TypeRef, subject types.Type) types.Type {
	t, known, _ := c.bareArgs(cls, ref, subject)
	if !known && ref == c.bareRef && ref != nil && subject.K != types.Invalid && !subject.IsParam() {
		c.errorf(ref.P, "%s expected for %s: its type arguments don't follow from %s; use %s",
			countArgs(len(cls.TypeParams)), types.ClassType(cls), subject, t.NonNull())
	}
	return t
}

// bareArgs returns bareType's type, and whether the subject's type gives
// each of its arguments, and whether it gives one two different ones, as
// in Derived<S> : Base<S, S> for a Base<A, B>.
func (c *Checker) bareArgs(cls *types.ClassSym, ref *syntax.TypeRef, subject types.Type) (t types.Type, known, conflict bool) {
	args := make([]types.Arg, len(cls.TypeParams))
	set := make([]bool, len(args))
	for i := range args {
		args[i] = types.Arg{V: types.Star}
	}
	if s := subject.NonNull(); s.K == types.Class {
		if sup, ok := c.asSuper(s, cls); ok {
			return c.nullable(sup, ref), true, false // a supertype of the subject's
		}
		if sup, ok := c.asSuper(cls.This(), s.C); ok {
			conflict = c.unify(cls, sup, s, args, set, 0)
		}
	}
	known = !slices.Contains(set, false)
	return c.nullable(types.GenericType(cls, args), ref), known, conflict
}

// unify matches type sup, written with the type parameters of cls, with
// type s, as kotlinc does to find what x is C tells of C's arguments:
// Base<D, String> and Base<T, U> give D = T. It sets args and set, and
// reports whether a parameter matches two different arguments. Other
// mismatches are not conflicts.
func (c *Checker) unify(cls *types.ClassSym, sup, s types.Type, args []types.Arg, set []bool, depth int) bool {
	if depth > 10 || sup.K != types.Class || s.K != types.Class || sup.C != s.C || sup.A == nil || s.A == nil {
		return false
	}
	conflict := false
	for i, a := range sup.A.List {
		if i >= s.A.Len() || a.V == types.Star {
			continue
		}
		v := s.A.Get(i)
		if a.T.IsParam() && a.V == types.Inv {
			j := slices.Index(cls.TypeParams, a.T.Param())
			if j < 0 {
				continue
			}
			if a.T.N && !a.T.P && v.V != types.Star {
				// T? and String? give T = String; a Java supertype's
				// E! and String? give E = String?.
				v.T = v.T.NonNull()
			}
			if set[j] && (args[j].V != v.V || v.V != types.Star && !c.sameType(args[j].T, v.T)) {
				conflict = true
			}
			args[j], set[j] = v, true
			continue
		}
		if v.V != types.Star && c.unify(cls, a.T.NonNull(), v.T.NonNull(), args, set, depth+1) {
			conflict = true
		}
	}
	return conflict
}

func allStar(t types.Type) bool {
	for _, a := range t.A.List {
		if a.V != types.Star {
			return false
		}
	}
	return true
}

// argsKnown reports whether the type arguments of t, written as ref,
// follow from the subject's type: a Collection<String> that is a List is
// a List<String>. As with kotlinc, they do if the type with the arguments
// that follow from it is a subtype of t, and of the subject's type: a
// KFunction1<Int, Int> is a KFunction1<Int, Any?>, as a suspend (Int) ->
// Int that is a KSuspendFunction1 is a KSuspendFunction1<Int, Any?>.
func (c *Checker) argsKnown(t, subject types.Type, ref *syntax.TypeRef) bool {
	s := subject.NonNull()
	if c.subtype(s, t) {
		return true
	}
	if s.K != types.Class {
		return false
	}
	if sup, ok := c.asSuper(t, s.C); ok && c.sameType(sup, s) {
		return true
	}
	// Not if the subject's arguments conflict, as Base<D, B> for a
	// Derived<S> : Base<S, S>.
	d, _, conflict := c.bareArgs(t.C, ref, subject)
	return !conflict && !hasStar(d) && c.subtype(d, t)
}

func hasStar(t types.Type) bool {
	return t.A != nil && slices.ContainsFunc(t.A.List, func(a types.Arg) bool { return a.V == types.Star })
}

// stable reports whether a variable's value can't change between a check
// and a use, so smart casts apply: a val local or parameter, or a val
// property of the program without a getter, read through this, or a
// top-level val without a getter.
func (c *Checker) stable(v *types.VarSym) bool {
	if v.Mutable {
		return isVar(v) && c.varStable(v, types.Type{})
	}
	switch v.Kind {
	case types.Param:
		if cp, ok := v.Decl.(*syntax.CtorParam); ok && cp.IsProp && cp.PropSym != nil {
			// As kotlinc has it, x of class C(var x: Int?) is the
			// property in the initializers, which may change.
			return c.stableOn(cp.PropSym, c.thisType())
		}
		return true
	case types.Local, types.BackingField:
		return true
	case types.Prop:
		return c.stableOn(v, c.thisType())
	case types.Global:
		return v.Getter == nil
	}
	return false
}

// narrowSym returns the variable that a smart cast of x narrows: a stable
// variable, or this. It returns nil if x can't be smart cast.
func (c *Checker) narrowSym(x syntax.Expr) *types.VarSym {
	switch x := x.(type) {
	case *syntax.Ident:
		if x.Sym != nil && x.Member == nil && c.stable(x.Sym) {
			return x.Sym
		}
		if x.Member != nil {
			return c.narrowSym(x.Member) // p of an implicit receiver, this.p
		}
	case *syntax.This:
		if x.Recv != nil {
			return x.Recv // a lambda's receiver, a parameter
		}
		return c.this
	case *syntax.NotNull:
		return c.narrowSym(x.X) // a!!.b is a.b
	case *syntax.Checked:
		return c.narrowSym(x.X) // a temporary's value: val t = a in a?.b = v
	case *syntax.Member:
		// a.b.c is stable if a.b is: a stable property of a val. So is
		// a?.b.c, which is a.b.c where a isn't null.
		prop := c.memberSym(x)
		if prop == nil {
			return nil
		}
		if this, ok := x.X.(*syntax.This); ok && this.Recv == nil {
			return prop // this.p is p, as in memberKey
		}
		if r := c.narrowSym(x.X); r != nil && !r.Mutable {
			k := [2]*types.VarSym{r, prop}
			if c.paths == nil {
				c.paths = map[[2]*types.VarSym]*types.VarSym{}
			}
			if p := c.paths[k]; p != nil {
				return p
			}
			p := &types.VarSym{Name: r.Name + "." + prop.Name, Type: prop.Type, Kind: types.Prop}
			c.paths[k] = p
			return p
		}
	}
	return nil
}

// isNarrowed returns the type that a value of type cur has once it is
// known to be a T. If cur isn't a subtype of T, the value is seen as a T,
// non-null if cur is: ktor's first !is T for a first: Node and a reified
// T leaves a T & Any (see declaredIfNull). An interface unrelated to a
// class type gives their intersection, as in Kotlin: an a: A? that is a B
// is an A & B, whatever expression it is passed through.
func (c *Checker) isNarrowed(cur, t types.Type) types.Type {
	if c.subtype(cur.NonNull(), t) {
		if types.MayBeNull(t) {
			if t.IsParam() && !c.subtype(cur, t) && (c.subtype(t, cur) || cur.K == types.Null || cur.K == types.Nothing) {
				// t: T? is T & T?, a T, after t is T, as a t known
				// to be null is a Nothing? & T.
				return t
			}
			return cur
		}
		return cur.NonNull()
	}
	if t.IsParam() && !types.MayBeNull(cur) {
		return t.NonNull()
	}
	if i, ok := c.narrowedIntersection(cur, t); ok {
		return i
	}
	if i, ok := c.paramIntersection(cur, t); ok {
		return i
	}
	return t
}

// paramIntersection returns t & cur for a smart cast of a value of a type
// parameter's type cur to a class type t, as K2 has it: o of type T is
// a String & T after o is String, which is still a T, as listOf(o) for
// an expected Collection<T> needs. Not to a primitive type or a value
// class, whose values are represented otherwise.
func (c *Checker) paramIntersection(cur, t types.Type) (types.Type, bool) {
	a, b := cur.Flex(), t.NonNull()
	if a.K != types.TypeParam || a.IsIntersection() || types.IsValue(b) {
		return types.InvalidType, false
	}
	switch b.K {
	case types.String:
	case types.Class:
		b.C.Ensure()
		if b.C.Func || b.HasVars() {
			return types.InvalidType, false
		}
	default:
		return types.InvalidType, false
	}
	i := c.intersect([]types.Type{b, a.NonNull()})
	if types.MayBeNull(t) && types.MayBeNull(cur) {
		i = i.Nullable()
	}
	return i, true
}

// narrowedIntersection returns cur & t for a smart cast of a value of
// type cur to an unrelated t, where one of them is an interface, both are
// Kotlin or Java class types without type parameters, and t is no
// subclass of cur's class: t & cur.
func (c *Checker) narrowedIntersection(cur, t types.Type) (types.Type, bool) {
	a, b := cur.Flex().NonNull(), t.NonNull()
	if a.IsIntersection() && b.K == types.Class && intersectable(b) && !b.C.Func && !c.subtype(b, a) {
		// A further smart cast of an intersection adds to it: b of a
		// class B that is an A and a C is a C & A & B. Each component
		// must be intersectable with the target.
		b.C.Ensure()
		for _, u := range a.Param().Bounds {
			if u.K != types.Class || !intersectable(u) {
				return types.InvalidType, false
			}
			if c.subtype(u, b) {
				continue
			}
			u.C.Ensure()
			if !u.C.Interface && !b.C.Interface && !c.subtype(b, u) {
				return types.InvalidType, false
			}
		}
		i := c.intersect(append([]types.Type{b}, a.Param().Bounds...))
		if types.MayBeNull(t) && types.MayBeNull(cur) {
			i = i.Nullable()
		}
		return i, true
	}
	if a.K != types.Class || b.K != types.Class || !intersectable(a) || !intersectable(b) ||
		a.C.Func || b.C.Func || c.subtype(b, a) {
		return types.InvalidType, false
	}
	a.C.Ensure()
	b.C.Ensure()
	if !a.C.Interface && !b.C.Interface {
		return types.InvalidType, false
	}
	// The target first, whose members win, as they did when the value
	// was seen as a T alone; the class erases it either way.
	i := c.intersect([]types.Type{b, a})
	if types.MayBeNull(t) && types.MayBeNull(cur) {
		i = i.Nullable()
	}
	return i, true
}

// needsCast reports whether a value read as type from needs a cast to be
// used as type to: whether a smart cast changes its class, beyond making
// it non-null.
func needsCast(from, to types.Type) bool {
	d1, ok1 := types.Desc(from.Nullable())
	d2, ok2 := types.Desc(to.Nullable())
	return ok1 && ok2 && d1 != d2 && to.K != types.Nothing && to.K != types.Null
}

// newThis returns the symbol that smart casts of this in a class's code
// narrow, or nil outside classes.
func newThis(cls *types.ClassSym) *types.VarSym {
	if cls == nil {
		return nil
	}
	return &types.VarSym{Name: "this", Type: cls.This(), Kind: types.Param}
}

// narrowedThis returns the type of this if a smart cast has changed it.
func (c *Checker) narrowedThis() (types.Type, bool) {
	if c.this == nil {
		return types.InvalidType, false
	}
	t := c.scope.narrowed(c.this)
	return t, t != c.this.Type
}

// thisMember checks a name as a member of this once a smart cast has
// changed this's type: in when (this) { is Leaf -> value }, value is
// this.value. It reports false if the smart-cast type has no such member.
func (c *Checker) thisMember(x *syntax.Ident) (types.Type, bool) {
	if _, ok := c.narrowedThis(); !ok {
		return types.InvalidType, false
	}
	this := &syntax.This{Class: true} // not a lambda's receiver
	this.P = x.P
	m := &syntax.Member{X: this, Name: x.Name}
	m.P = x.P
	n := len(c.errs)
	t := c.expr(m, noWant)
	if len(c.errs) > n {
		c.errs = c.errs[:n]
		return types.InvalidType, false
	}
	x.Member = m
	return t, true
}

// thisMethods returns the methods named name of this's smart-cast type.
func (c *Checker) thisMethods(name string) bool {
	t, ok := c.narrowedThis()
	if !ok {
		return false
	}
	cls := c.classOf(t)
	return cls != nil && len(c.javaMethods(cls, name)) > 0
}

// coverage is what the branches of a when with a subject cover, for its
// exhaustiveness.
type coverage struct {
	null, isTrue, isFalse bool
	entries               map[string]bool   // enum entries
	classes               []*types.ClassSym // is checks, and objects compared with
	// owned are the enum entries by their enum, for an enum below a
	// sealed type: C.A and C.B of an enum C : Base, for a Base subject.
	owned map[*types.ClassSym]map[string]bool
	// types are the types of is checks, which cover a subject of a
	// subtype: is Int, with null, an Int?, and is List<*> a List<T>.
	types []types.Type
	// not are the classes of !is checks, which cover the subclasses of
	// a sealed type that aren't theirs, as K2 has it: is First and
	// !is First cover a Sealed.
	not []*types.ClassSym
}

// add records a condition that compares the subject with x.
func (cv *coverage) add(x syntax.Expr) {
	switch x := x.(type) {
	case *syntax.NullLit:
		cv.null = true
	case *syntax.BoolLit:
		cv.isTrue = cv.isTrue || x.Val
		cv.isFalse = cv.isFalse || !x.Val
	case *syntax.Ident:
		if x.Member != nil {
			cv.add(x.Member) // an imported entry
			return
		}
		if x.Object != nil {
			cv.classes = append(cv.classes, x.Object)
		}
		if x.Sym != nil && x.Sym.Kind == types.Static {
			cv.entry(x.Sym.Name)
			cv.ownedEntry(x.Sym.Owner, x.Sym.Name)
		}
	case *syntax.Member:
		if x.Object != nil {
			cv.classes = append(cv.classes, x.Object)
		}
		if x.Field != nil {
			cv.entry(x.Field.Name)
			cv.ownedEntry(x.Field.Owner, x.Field.Name)
		}
	}
}

func (cv *coverage) ownedEntry(cls *types.ClassSym, name string) {
	if cls == nil {
		return
	}
	if cv.owned == nil {
		cv.owned = map[*types.ClassSym]map[string]bool{}
	}
	if cv.owned[cls] == nil {
		cv.owned[cls] = map[string]bool{}
	}
	cv.owned[cls][name] = true
}

func (cv *coverage) entry(name string) {
	if cv.entries == nil {
		cv.entries = map[string]bool{}
	}
	cv.entries[name] = true
}

// merge adds what o covers.
func (cv *coverage) merge(o *coverage) {
	cv.null = cv.null || o.null
	cv.isTrue = cv.isTrue || o.isTrue
	cv.isFalse = cv.isFalse || o.isFalse
	for e := range o.entries {
		cv.entry(e)
	}
	for cls, es := range o.owned {
		for e := range es {
			cv.ownedEntry(cls, e)
		}
	}
	cv.classes = append(cv.classes[:len(cv.classes):len(cv.classes)], o.classes...)
	cv.types = append(cv.types[:len(cv.types):len(cv.types)], o.types...)
	cv.not = append(cv.not[:len(cv.not):len(cv.not)], o.not...)
}

// empty reports whether cv covers nothing.
func (cv *coverage) empty() bool {
	return !cv.null && !cv.isTrue && !cv.isFalse && len(cv.entries) == 0 && len(cv.classes) == 0 && len(cv.types) == 0 && len(cv.not) == 0
}

// covers reports whether the branches cover every value of a subject of
// type t: true and false, every enum entry, or every subclass of a sealed
// class, and null if t is nullable.
func (c *Checker) covers(t types.Type, cv *coverage) bool {
	if t.K == types.Nothing {
		return true // no value: when (n) {} for an n: Nothing
	}
	if types.MayBeNull(t) && !t.P && !cv.null && !slices.ContainsFunc(cv.types, types.AcceptsNull) {
		return false // is String? covers null
	}
	if t.K == types.Null {
		return true // a subject known to be null, which null covers
	}
	t = t.NonNull()
	for _, it := range cv.types {
		// An is check of a supertype: is Int of an Int?, with null.
		if c.subtype(t, it) {
			return true
		}
	}
	switch {
	case t.K == types.Bool:
		return cv.isTrue && cv.isFalse
	case t.IsIntersection():
		// A subject smart cast to A & B is covered as an A or a B: as a
		// sealed I1 after require(x is I2).
		for _, b := range t.Param().Bounds {
			if c.covers(b, cv) {
				return true
			}
		}
		return false
	case t.K != types.Class:
		return false
	}
	cls := t.C
	cls.Ensure()
	if cls.Entries != nil && !slices.ContainsFunc(cv.classes, cls.IsSubclassOf) {
		for _, e := range cls.Entries {
			if !cv.entries[e] {
				return false
			}
		}
		return true
	}
	return c.sealedCovered(cls, cv, nil)
}

// sealedCovered reports whether the branches cover every instance of cls:
// cls or one of its supertypes is among the classes of is checks, cls is
// an enum whose entries are all compared with, or cls is sealed and each
// of its subclasses is covered. A subclass of a sealed type, among those
// of root, is covered by a !is check of a class it isn't related to.
func (c *Checker) sealedCovered(cls *types.ClassSym, cv *coverage, root *types.ClassSym) bool {
	for _, k := range cv.classes {
		if cls.IsSubclassOf(k) {
			return true
		}
	}
	if root != nil {
		for _, n := range cv.not {
			if !cls.IsSubclassOf(n) && !n.IsSubclassOf(cls) {
				return true
			}
		}
	}
	cls.Ensure()
	if root != nil && cls.Entries != nil {
		for _, e := range cls.Entries {
			if !cv.owned[cls][e] {
				return false
			}
		}
		return true
	}
	if !cls.Sealed {
		return false
	}
	if root == nil {
		root = cls
	}
	for _, s := range cls.Subclasses {
		if !c.sealedCovered(s, cv, root) {
			return false
		}
	}
	return true
}

// enumerable reports whether a when over a subject of type t must be
// exhaustive even as a statement: t is Boolean, an enum or a sealed class.
func enumerable(t types.Type) bool {
	t = t.NonNull()
	switch {
	case t.K == types.Bool:
		return true
	case t.K == types.Class:
		t.C.Ensure()
		return t.C.Entries != nil || t.C.Sealed
	}
	return false
}
