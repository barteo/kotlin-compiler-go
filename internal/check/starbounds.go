package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// starBoundsProp returns the type of property p, t as read on a receiver
// of type rt, where p is of a type parameter of its class with several
// bounds that rt's argument for it is a star: as in K2, the intersection
// of the bounds, A & B for the value of an Inv<*> of Inv<T> where T : A,
// T : B.
func (c *Checker) starBoundsProp(rt types.Type, p *types.VarSym, t types.Type) types.Type {
	pt := p.Type
	if p.Owner == nil || !pt.IsParam() || pt.K != types.TypeParam || pt.N {
		return t
	}
	tp := pt.Param()
	i := slices.Index(p.Owner.TypeParams, tp)
	if i < 0 || len(tp.Bounds) < 2 || slices.ContainsFunc(tp.Bounds, func(b types.Type) bool { return !intersectable(b) }) {
		return t
	}
	sup, ok := c.asSuper(rt.NonNull(), p.Owner)
	if !ok || sup.A == nil || sup.A.Get(i).V != types.Star {
		return t
	}
	if r := c.intersect(tp.Bounds); r.K != types.Invalid {
		return r
	}
	return t
}
