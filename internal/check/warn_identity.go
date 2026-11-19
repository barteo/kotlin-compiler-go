package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports kotlinc's warnings about a === b and a !== b where a
// primitive is involved, which has no identity: DEPRECATED_IDENTITY_EQUALS
// where both operands are primitives, IMPLICIT_BOXING_IN_IDENTITY_EQUALS
// where only one is, and the other is boxed to compare it. As kotlinc,
// it judges an operand by its type before smart casts: x === 1 for an x
// of type Any smart cast to Int boxes 1, and so does x === y for a y of
// type Any.

// noteIdentTypes records the types of the operands of the identity
// comparison x before smart casts, given those after them, xt and yt.
func (c *Checker) noteIdentTypes(x *syntax.Binary, xt, yt types.Type) {
	orig := func(e syntax.Expr, t types.Type) types.Type {
		m, _ := e.(*syntax.Member)
		if id, ok := e.(*syntax.Ident); ok && id.Member != nil {
			m = id.Member
		}
		if m == nil {
			if v := c.narrowSym(e); v != nil && v.Type.K != types.Invalid {
				return v.Type
			}
			return t
		}
		// A property's declared type may name its class's type
		// parameters, which only its receiver's type gives: b.t is an
		// Int for a b: Box<Int>. It is t unless smart cast.
		k, ok := c.narrowKeyOf(e)
		if !ok {
			return t
		}
		if c.scope.narrowedMember(k.recv, k.sym, types.InvalidType).K == types.Invalid {
			return t
		}
		if k.sym.Type.HasParams() {
			return types.InvalidType // not judged
		}
		return k.sym.Type
	}
	if c.identTypes == nil {
		c.identTypes = map[*syntax.Binary][4]types.Type{}
	}
	c.identTypes[x] = [4]types.Type{orig(x.X, xt), orig(x.Y, yt), xt, yt}
}

// identityWarnings reports the identity comparison x of a primitive. As
// kotlinc, it judges the operands' types before smart casts, and, only if
// those decide nothing and a smart cast is involved, the smart-cast types.
func (c *Checker) identityWarnings(x *syntax.Binary) {
	ts, ok := c.identTypes[x]
	if !ok || x.X == nil || x.Y == nil || x.OpType.K == types.Invalid {
		return // not checked, or an error
	}
	diag := c.identityDiag(ts[0], ts[1])
	if diag == "" && (ts[0] != ts[2] || ts[1] != ts[3]) {
		diag = c.identityDiag(ts[2], ts[3])
	}
	xt, yt := ts[2], ts[3]
	switch diag {
	case "DEPRECATED_IDENTITY_EQUALS":
		c.warnf(x.X.Pos(), diag, "identity equality for arguments of types '%s' and '%s' is deprecated", xt, yt)
	case "IMPLICIT_BOXING_IN_IDENTITY_EQUALS":
		c.warnf(x.X.Pos(), diag, "identity equality for arguments of types '%s' and '%s' can be unstable because of implicit boxing", xt, yt)
	}
}

// identityDiag returns the warning about a === b for operands of types l
// and r: DEPRECATED_IDENTITY_EQUALS for two primitives of a type, and
// IMPLICIT_BOXING_IN_IDENTITY_EQUALS for one and a type it is a subtype
// of. It returns "" if neither is a primitive, and "skip" where kotlinc
// reports FORBIDDEN_IDENTITY_EQUALS instead (two primitives of different
// types, or one and an unrelated type), or kc can't judge.
func (c *Checker) identityDiag(l, r types.Type) string {
	lp, lok := identityPrimitive(l)
	rp, rok := identityPrimitive(r)
	switch {
	case !lok || !rok:
		return "skip"
	case lp.K == types.Invalid && rp.K == types.Invalid:
		return ""
	case lp.K != types.Invalid && rp.K != types.Invalid:
		if lp.K != rp.K {
			return "skip"
		}
		return "DEPRECATED_IDENTITY_EQUALS"
	}
	p, other := lp, r
	if p.K == types.Invalid {
		p, other = rp, l
	}
	if !c.boxesTo(p, other) {
		return "skip"
	}
	return "IMPLICIT_BOXING_IN_IDENTITY_EQUALS"
}

// boxesTo reports whether a value of the primitive type p, boxed, may be
// a value of type t, its type arguments erased, or of each of its bounds
// if it is a type parameter.
func (c *Checker) boxesTo(p, t types.Type) bool {
	if t.K == types.TypeParam {
		for _, b := range t.Param().Bounds {
			if !c.boxesTo(p, b) {
				return false
			}
		}
		return true
	}
	t.P = false
	if c.subtype(p, t.Nullable()) {
		return true
	}
	// As kotlinc, with type arguments erased: Int boxes to a
	// Comparable<String>.
	r, ok := c.castInfo(t, 0)
	return ok && c.erasedSub([]types.Type{p}, r.parts)
}

// identityPrimitive returns the primitive type of a value of type t, an
// operand of === or !==, or InvalidType if it is boxed, and whether kc
// knows: it doesn't judge types it didn't find, null or Nothing. A
// platform type (Int!) is boxed, as a nullable one, and a type parameter
// is the primitive type that bounds it, if any.
func identityPrimitive(t types.Type) (types.Type, bool) {
	switch t.K {
	case types.Invalid, types.TypeVar, types.Null, types.Nothing, types.Range:
		return types.InvalidType, false
	case types.TypeParam:
		if !t.N {
			for _, b := range t.Param().Bounds {
				if b.IsPrimitive() && !b.P {
					return b, true
				}
			}
		}
		return types.InvalidType, true
	}
	if hasInvalid(t, 0) {
		return types.InvalidType, false
	}
	if t.IsPrimitive() && !t.P {
		return t, true
	}
	return types.InvalidType, true
}
