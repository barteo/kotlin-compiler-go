package check

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// interKey identifies an intersection type by its first component and
// the intersection of the others (or the other), as Args are interned.
type interKey struct{ head, tail types.Type }

// intersect returns the intersection type of ts, A & B: a value that is
// each of them. It is a synthetic type parameter bounded by them, which
// subtyping, member lookup and erasure see as they see any parameter
// with several bounds. Components that are supertypes of others are
// dropped, as nested intersections are flattened: one left is no
// intersection. It is nullable only if every component is.
func (c *Checker) intersect(ts []types.Type) types.Type {
	nullable := true
	var flat []types.Type
	for _, t := range ts {
		if !types.MayBeNull(t) {
			nullable = false
		}
		t = t.NonNull()
		if t.IsIntersection() {
			flat = append(flat, t.Param().Bounds...)
		} else {
			flat = append(flat, t)
		}
	}
	var comps []types.Type
	for i, t := range flat {
		keep := true
		for j, u := range flat {
			if i == j {
				continue
			}
			// A supertype of another is implied by it; of equal ones
			// the first stays.
			if c.subtype(u, t) && (!c.subtype(t, u) || j < i) {
				keep = false
				break
			}
		}
		if keep {
			comps = append(comps, t)
		}
	}
	var r types.Type
	switch len(comps) {
	case 0:
		return types.InvalidType
	case 1:
		r = comps[0]
	default:
		r = c.internIntersection(comps)
	}
	if nullable {
		r = r.Nullable()
	}
	return r
}

// internIntersection returns the one intersection type of comps, two
// or more non-null types none of which is a supertype of another.
func (c *Checker) internIntersection(comps []types.Type) types.Type {
	tail := comps[1]
	if len(comps) > 2 {
		tail = c.internIntersection(comps[1:])
	}
	k := interKey{comps[0], tail}
	cache := c.u.cache(comps...)
	if v, ok := cache.inters.Load(k); ok {
		return v.(types.Type)
	}
	names := make([]string, len(comps))
	for i, t := range comps {
		names[i] = t.String()
	}
	p := &types.TypeParamSym{Name: strings.Join(names, " & "), Bounds: comps, Intersection: true}
	v, _ := cache.inters.LoadOrStore(k, types.ParamType(p))
	return v.(types.Type)
}

// intersectable reports whether t can be a component of an intersection
// type that kc infers: a class or builtin type that mentions no type
// parameter or variable.
func intersectable(t types.Type) bool {
	t = t.Flex().NonNull()
	switch {
	case t.K == types.Class:
		return !t.HasParams() && !t.HasVars()
	case t.IsIntersection():
		return true
	}
	return isBuiltin(t) && t.K != types.Unit && t.K != types.Range
}

// approxIntersections replaces the intersection types in t, a type that
// a declaration visible outside infers, by the common supertype of their
// components, as kotlinc approximates them: A & B of two interfaces is
// Any.
func (c *Checker) approxIntersections(t types.Type) types.Type {
	if t.IsIntersection() {
		bs := t.Param().Bounds
		r := bs[0]
		for _, b := range bs[1:] {
			r = c.commonSuper(r, b, 0)
		}
		if r.IsIntersection() {
			r = types.AnyType
		}
		if types.MayBeNull(t) {
			r = r.Nullable()
		}
		return r
	}
	if t.K != types.Class || t.A == nil || !t.HasParams() {
		return t
	}
	var args []types.Arg
	for i, a := range t.A.List {
		if a.V == types.Star {
			continue
		}
		if r := c.approxIntersections(a.T); r != a.T {
			if args == nil {
				args = slices.Clone(t.A.List)
			}
			args[i].T = r
		}
	}
	if args != nil {
		t.A = types.MakeArgs(args)
	}
	return t
}
