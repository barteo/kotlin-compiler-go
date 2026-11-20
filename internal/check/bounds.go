package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file checks the declarations of type parameters, as kotlinc's
// FirTypeParameterBoundsChecker, FirFiniteBoundRestrictionChecker and
// FirNonExpansiveInheritanceRestrictionChecker do: what a parameter's
// upper bounds may be together, and what a class's bounds and
// supertypes may do with its parameters.

// checkBoundSets checks each of decls's sets of upper bounds, once they
// are resolved: one declared with the parameter beside the where clause's
// (MISPLACED_TYPE_PARAMETER_CONSTRAINTS), a bound's class repeated (REPEATED_BOUND), a bound no
// type can be a subtype of with the others (CONFLICTING_UPPER_BOUNDS),
// and bounds that reach a generic class with different type arguments
// (INCONSISTENT_TYPE_PARAMETER_BOUNDS).
func (c *Checker) checkBoundSets(decls []*syntax.TypeParam) {
	for _, d := range decls {
		if d.DeclBound && d.WhereBound {
			at := d.P
			if len(d.Annotations) > 0 {
				at = d.Annotations[0].P
			}
			c.warnf(at, "MISPLACED_TYPE_PARAMETER_CONSTRAINTS", "If a type parameter has multiple constraints, they all need to be placed in the 'where' clause.")
		}
		if d.Sym != nil && slices.ContainsFunc(d.Sym.Bounds, types.Type.IsArray) {
			// A subtype of an array type is the array type: the type
			// parameter would be it.
			c.errorf(d.P, "upper bound of a type parameter cannot be an array")
		}
		if d.Sym != nil && len(d.Bounds) == len(d.Sym.Bounds) && !c.overrideBounds && !overrideOwner(d.Sym.Owner) {
			for i, b := range d.Sym.Bounds {
				if b.K != types.Invalid && !c.hasSubtypesK1(b) {
					c.warnf(d.Bounds[i].P, "FINAL_UPPER_BOUND", "type '%s' is final, so the value of the type parameter is predetermined", b)
				}
			}
		}
		if d.Sym == nil || len(d.Sym.Bounds) < 2 || len(d.Bounds) != len(d.Sym.Bounds) {
			continue
		}
		repeated := false
		seen := map[any]bool{}
		for i, b := range d.Sym.Bounds {
			k := boundKey(b)
			if k == nil {
				continue
			}
			if seen[k] {
				c.errorf(d.Bounds[i].P, "repeated upper bound")
				repeated = true
			}
			seen[k] = true
		}
		if c.conflictingBounds(d.Sym.Bounds) {
			c.errorf(d.P, "upper bounds of '%s' have empty intersection", d.Name)
		}
		if !repeated {
			if s := c.inconsistentBounds(d.Sym.Bounds); s != "" {
				c.errorf(d.P, "type parameter '%s' has inconsistent bounds: %s", d.Name, s)
			}
		}
	}
}

// overrideOwner reports whether a type parameter's owner is an override,
// whose bounds kotlinc doesn't warn about: those of the member it
// overrides decide them.
func overrideOwner(owner any) bool {
	f, ok := owner.(*types.FunSym)
	if !ok {
		return false
	}
	d, ok := f.Decl.(*syntax.FunDecl)
	if !ok {
		return false
	}
	if d.Override {
		return true
	}
	if d.AccessorOf != nil {
		if p, ok := d.AccessorOf.Decl.(*syntax.PropDecl); ok {
			return p.Override
		}
	}
	return false
}

// hasSubtypesK1 reports whether type t has subtypes other than Nothing,
// as kotlinc's canHaveSubtypesAccordingToK1 has it for FINAL_UPPER_BOUND:
// a nullable type does, and a class that isn't final or is an enum
// class; a final generic class does where one of its arguments may vary:
// a star, an argument that has subtypes where the parameter or the
// projection is out, or one below a bound of the parameter where it is
// in, and either where it's invariant.
func (c *Checker) hasSubtypesK1(t types.Type) bool {
	if t.N || t.P {
		return true
	}
	switch t.K {
	case types.Unit, types.Nothing, types.String:
		return false
	case types.Class:
	default:
		return !types.IsPrimitiveKind(t.K)
	}
	cls := t.C
	cls.Ensure()
	if !cls.Final || cls.Interface || cls.Abstract || cls.Entries != nil || cls.Func || cls.Array || cls.Sealed {
		return true
	}
	if t.A == nil {
		return false
	}
	for i, a := range t.A.List {
		if i >= len(cls.TypeParams) {
			break
		}
		p := cls.TypeParams[i]
		var has bool
		switch {
		case a.V == types.Star:
			has = true
		case a.V == types.Out:
			has = c.hasSubtypesK1(a.T)
		case a.V == types.In:
			has = c.belowBound(a.T, p)
		case p.Var == types.Out:
			has = c.hasSubtypesK1(a.T)
		case p.Var == types.In:
			has = c.belowBound(a.T, p)
		default:
			has = c.hasSubtypesK1(a.T) || c.belowBound(a.T, p)
		}
		if has {
			return true
		}
	}
	return false
}

// belowBound reports whether t is a subtype of one of p's bounds other
// than the bound itself, as K1's lowerThanBound.
func (c *Checker) belowBound(t types.Type, p *types.TypeParamSym) bool {
	bounds := p.Bounds
	if len(bounds) == 0 {
		bounds = []types.Type{types.AnyType.Nullable()}
	}
	for _, b := range bounds {
		if t != b && c.subtype(t, b) {
			return true
		}
	}
	return false
}

// boundKey is what makes two bounds the same bound: their classifier.
func boundKey(b types.Type) any {
	switch {
	case b.K == types.Class:
		return b.C
	case b.IsParam():
		return b.Param()
	case b.K == types.Invalid:
		return nil
	}
	return b.K
}

// classUpperBound reports whether bound b is a class rather than an interface
// or a type parameter, of which a type parameter may have only one.
func classUpperBound(b types.Type) bool {
	switch b.K {
	case types.Class:
		return !b.C.Interface
	case types.String, types.Any, types.Unit:
		return true
	}
	return types.IsPrimitiveKind(b.K)
}

// conflictingBounds reports whether one of bounds can't have subtypes and
// isn't related to another: no type is a subtype of both, as of Int and
// String.
func (c *Checker) conflictingBounds(bounds []types.Type) bool {
	for _, b := range bounds {
		if !finalBound(b) {
			continue
		}
		for _, o := range bounds {
			if o.IsParam() || o.K == types.Invalid || boundKey(o) == boundKey(b) {
				continue
			}
			if !c.subtype(b.NonNull(), o.Nullable()) && !c.subtype(o.NonNull(), b.Nullable()) {
				return true
			}
		}
	}
	return false
}

// finalBound reports whether bound b is a type with no subtypes but
// Nothing: a final class without type parameters, or a primitive or
// String.
func finalBound(b types.Type) bool {
	switch b.K {
	case types.Class:
		return b.C.Final && !b.C.Interface && !b.C.Abstract && len(b.C.TypeParams) == 0 && !b.C.Func && !b.C.Array
	case types.String:
		return true
	}
	return types.IsPrimitiveKind(b.K)
}

// inconsistentBounds returns a generic class that bounds reach with
// different type arguments, the two types joined, or "".
func (c *Checker) inconsistentBounds(bounds []types.Type) string {
	seen := map[*types.ClassSym]types.Type{}
	var queue []types.Type
	for _, b := range bounds {
		switch {
		case b.K == types.Class:
			queue = append(queue, b.NonNull())
		case b.K == types.String || types.IsPrimitiveKind(b.K):
			if cls := c.jvmClass(b); cls != nil {
				queue = append(queue, cls.This())
			}
		}
	}
	for steps := 0; len(queue) > 0 && steps < 1000; steps++ {
		s := queue[0]
		queue = queue[1:]
		if s.K != types.Class {
			continue
		}
		prev, ok := seen[s.C]
		if !ok {
			seen[s.C] = s
			queue = append(queue, c.directSupers(s)...)
			continue
		}
		if s.A != nil && prev.A != nil && !c.sameBoundArgs(prev, s) {
			return prev.String() + ", " + s.String()
		}
	}
	return ""
}

// sameBoundArgs is sameArgs, but for the kinds of projections, which
// don't make bounds inconsistent: T : A<out T>, T : B<in T>, where A and
// B both extend R<T>. A star projection is consistent only with another,
// as in K2.
func (c *Checker) sameBoundArgs(t, u types.Type) bool {
	for i, a := range t.A.List {
		if i >= u.A.Len() {
			break
		}
		b := u.A.Get(i)
		if a.V == types.Star && b.V == types.Star {
			continue
		}
		if a.V == types.Star || b.V == types.Star {
			return false
		}
		if !c.subtype(a.T, b.T) || !c.subtype(b.T, a.T) {
			return false
		}
	}
	return true
}

// checkClassTypeParams checks the type parameters of class d, once its
// signature is resolved: their bounds' type arguments against the
// bounds' classes' parameters, their sets of bounds (checkBoundSets),
// and the Finite Bound and Non-Expansive Inheritance Restrictions.
func (c *Checker) checkClassTypeParams(d *syntax.ClassDecl) {
	if len(d.TypeParams) == 0 {
		return
	}
	if d.Enum {
		c.errorf(d.TypeParams[0].P, "enum class cannot have type parameters")
	}
	saved := c.tparams
	c.tparams = d.Sym.TypeParams
	for _, tp := range d.TypeParams {
		if tp.Sym == nil || len(tp.Bounds) != len(tp.Sym.Bounds) {
			continue
		}
		for i, b := range tp.Sym.Bounds {
			if ref := tp.Bounds[i]; b.K == types.Class && b.A != nil {
				if len(ref.Args) != len(b.A.List) {
					if c.lookupAlias(ref.Name) != nil {
						c.refBounds(b.NonNull(), ref, nil, nil) // T : One<Any>
					}
					continue
				}
				c.checkBounds(b.NonNull(), ref)
			}
		}
	}
	c.tparams = saved
	c.headerVariance(d)
	c.checkBoundSets(d.TypeParams)
	if p := finiteBoundsViolation(d.Sym); p != nil {
		c.errorf(d.TypeParams[p.Index].P, "this type parameter violates the Finite Bound Restriction")
	}
	for _, p := range expansiveParams(d.Sym) {
		c.errorf(d.TypeParams[p.Index].P, "this type parameter violates the Non-Expansive Inheritance Restriction")
	}
}

// paramGraph is a graph of type parameters, of any classes, whose edges
// may be marked.
type paramGraph map[*types.TypeParamSym]map[*types.TypeParamSym]bool

func (g paramGraph) add(from, to *types.TypeParamSym, mark bool) {
	m := g[from]
	if m == nil {
		m = map[*types.TypeParamSym]bool{}
		g[from] = m
	}
	m[to] = m[to] || mark
}

// reaches reports whether there is a path from a to b.
func (g paramGraph) reaches(a, b *types.TypeParamSym) bool {
	seen := map[*types.TypeParamSym]bool{}
	stack := []*types.TypeParamSym{a}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for q := range g[p] {
			if q == b {
				return true
			}
			if !seen[q] {
				seen[q] = true
				stack = append(stack, q)
			}
		}
	}
	return false
}

// generics calls f with each generic class type among t and its type
// arguments.
func generics(t types.Type, f func(types.Type)) {
	if t.K != types.Class || t.A == nil {
		return
	}
	f(t)
	for _, a := range t.A.List {
		if a.V != types.Star {
			generics(a.T, f)
		}
	}
}

// finiteBoundsViolation returns the first type parameter of cls in a
// cycle of the graph of the Finite Bound Restriction, or nil: for each
// projected type argument of a generic type among the bounds of a type
// parameter T (and those of the type parameters that bound T), an edge
// from T to the corresponding type parameter of the type's class.
func finiteBoundsViolation(cls *types.ClassSym) *types.TypeParamSym {
	g := paramGraph{}
	done := map[*types.TypeParamSym]bool{}
	var build func(p *types.TypeParamSym)
	build = func(p *types.TypeParamSym) {
		if done[p] {
			return
		}
		done[p] = true
		// The bounds' closure: T : U takes U's bounds too.
		var bounds []types.Type
		seen := map[*types.TypeParamSym]bool{p: true}
		queue := append([]types.Type(nil), p.Bounds...)
		for len(queue) > 0 {
			b := queue[0]
			queue = queue[1:]
			if b.IsParam() {
				if q := b.Param(); !seen[q] {
					seen[q] = true
					queue = append(queue, q.Bounds...)
				}
				continue
			}
			bounds = append(bounds, b)
		}
		for _, b := range bounds {
			generics(b, func(t types.Type) {
				ps := t.C.TypeParams
				for i, a := range t.A.List {
					if a.V != types.Inv && i < len(ps) {
						g.add(p, ps[i], false)
						build(ps[i])
					}
				}
			})
		}
	}
	for _, p := range cls.TypeParams {
		if p.Owner == cls {
			build(p)
		}
	}
	for _, p := range cls.TypeParams {
		if p.Owner == cls && g.reaches(p, p) {
			return p
		}
	}
	return nil
}

// expansiveParams returns the type parameters of cls that violate the
// Non-Expansive Inheritance Restriction: in the graph of the type
// parameters of cls and the classes its supertypes reach, with an edge
// from a class's parameter X to the parameter of a generic type among
// its supertypes whose argument mentions X, marked expansive if the
// argument isn't X itself, those at an end of an expansive edge in a
// cycle.
func expansiveParams(cls *types.ClassSym) []*types.TypeParamSym {
	g := paramGraph{}
	done := map[*types.ClassSym]bool{}
	var build func(k *types.ClassSym)
	build = func(k *types.ClassSym) {
		if done[k] || len(done) > 200 {
			return
		}
		done[k] = true
		k.Ensure()
		for _, st := range k.Supers {
			generics(st, func(t types.Type) {
				ps := t.C.TypeParams
				for i, a := range t.A.List {
					if a.V == types.Star || i >= len(ps) {
						continue
					}
					for _, x := range k.TypeParams {
						if x.Owner != k {
							continue
						}
						if a.T.IsParam() && a.T.Param() == x {
							g.add(x, ps[i], false)
						} else if a.T.Has(func(u types.Type) bool { return u.IsParam() && u.Param() == x }) {
							g.add(x, ps[i], true)
						}
					}
				}
				build(t.C)
			})
		}
	}
	build(cls)
	bad := map[*types.TypeParamSym]bool{}
	for from, m := range g {
		for to, expansive := range m {
			if expansive && (to == from || g.reaches(to, from)) {
				bad[from], bad[to] = true, true
			}
		}
	}
	var out []*types.TypeParamSym
	for _, p := range cls.TypeParams {
		if p.Owner == cls && bad[p] {
			out = append(out, p)
		}
	}
	return out
}

// headerVariance checks that the in and out type parameters of class d
// occur in its type parameters' bounds and its supertypes only where
// their variance allows: both are out positions.
func (c *Checker) headerVariance(d *syntax.ClassDecl) {
	if !variantClass(d.Sym) {
		return
	}
	for _, tp := range d.TypeParams {
		if tp.Sym == nil || len(tp.Bounds) != len(tp.Sym.Bounds) {
			continue
		}
		for i, b := range tp.Sym.Bounds {
			c.checkVariance(b, tp.Bounds[i], types.Out, true)
		}
	}
	for _, st := range d.Supers {
		if st.T.K == types.Class {
			c.checkVariance(st.T, st.Type, types.Out, true)
		}
	}
}
