package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports kotlinc's INCOMPATIBLE_ENUM_COMPARISON: a comparison
// of a value of an enum with one of a class or interface the enum isn't
// related to, which K2 finds inapplicable but, where K1 found nothing
// wrong or a smart cast is involved, only warns about. Where it is an
// error (INCOMPATIBLE_ENUM_COMPARISON_ERROR), kc's equality check has
// reported it, and nothing is warned about. As kotlinc does, it judges
// the operands by their types after smart casts, a type parameter by the
// classes of its bounds, and two classes by themselves, not their type
// arguments: an E : Inv<Int> may be compared with an Inv<String>, and
// any enum with an Enum<E2>. x === y of two nullable types isn't judged,
// as kotlinc doesn't: only of non-null ones.

// enumComparisonWarning reports the comparison x of incompatible enums.
func (c *Checker) enumComparisonWarning(x *syntax.Binary) {
	if x.X == nil || x.Y == nil || x.OpType.K == types.Invalid {
		return // an error
	}
	ts, ok := c.identTypes[x]
	if !ok {
		return
	}
	xt, yt := ts[2], ts[3]
	switch x.Op {
	case syntax.EQ, syntax.NE:
	case syntax.EQ_EQ_EQ, syntax.NOT_EQ_EQ:
		if types.MayBeNull(xt) || types.MayBeNull(yt) {
			return
		}
	default:
		return
	}
	if c.incompatibleEnums(xt, yt) {
		c.warnf(syntax.StartPos(x), "INCOMPATIBLE_ENUM_COMPARISON", "comparison of incompatible enums '%s' and '%s' is always unsuccessful", xt, yt)
	}
}

// whenEnumWarnings reports the conditions of when x that compare its
// subject with a value incompatible with it, as enumComparisonWarning.
func (c *Checker) whenEnumWarnings(x *syntax.When) {
	if x.Subject == nil {
		return
	}
	st := x.Subject.Type()
	for _, br := range x.Branches {
		for _, cond := range br.Conds {
			if cond.X == nil || cond.In || cond.NotIn || cond.Is || cond.NotIs || cond.SubjType.K != types.Invalid || cond.OpType.K == types.Invalid {
				continue
			}
			if ct := cond.X.Type(); c.incompatibleEnums(st, ct) {
				c.warnf(syntax.StartPos(cond.X), "INCOMPATIBLE_ENUM_COMPARISON", "comparison of incompatible enums '%s' and '%s' is always unsuccessful", st, ct)
			}
		}
	}
}

// incompatibleEnums reports whether neither of xt and yt is a subtype of
// the other and one of them, or one of its bounds, is an enum class that a class or interface the other is, or is
// bounded by, is neither a superclass nor a subclass of. Platform types
// and types kc couldn't resolve aren't judged.
func (c *Checker) incompatibleEnums(xt, yt types.Type) bool {
	xs, okx := c.enumBoundClasses(xt, nil, 0)
	ys, oky := c.enumBoundClasses(yt, nil, 0)
	if !okx || !oky {
		return false
	}
	if x, y := xt.NonNull(), yt.NonNull(); c.subtype(x, y) || c.subtype(y, x) {
		// One a subtype of the other, as En & En2 of En for a value
		// smart cast to both enums.
		return false
	}
	unrelated := func(es, os []*types.ClassSym) bool {
		for _, e := range es {
			if !c.enumType(rawType(e)) {
				continue
			}
			for _, o := range os {
				if o == e {
					continue
				}
				if _, ok := c.asSuper(rawType(e), o); ok {
					continue
				}
				if _, ok := c.asSuper(rawType(o), e); ok {
					continue
				}
				return true
			}
		}
		return false
	}
	return unrelated(xs, ys) || unrelated(ys, xs)
}

// enumBoundClasses appends to out the classes of t: its own, or those of
// a type parameter's bounds, and reports whether t can be judged: it is
// no platform type and no type kc couldn't resolve. Any and other built-in
// types add nothing.
func (c *Checker) enumBoundClasses(t types.Type, out []*types.ClassSym, depth int) ([]*types.ClassSym, bool) {
	if depth > 20 || t.P || t.K == types.Invalid {
		return out, false
	}
	switch {
	case t.IsParam():
		for _, b := range t.Param().Bounds {
			var ok bool
			if out, ok = c.enumBoundClasses(b, out, depth+1); !ok {
				return out, false
			}
		}
	case t.K == types.Class:
		out = append(out, t.C)
	}
	return out, true
}
