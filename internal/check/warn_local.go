package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// localWarnings reports the warnings about the file's checked code that
// need no more than the types the checker gave it: unnecessary !! and ?.,
// comparisons with null that are always true or false, useless elvis
// operators and useless casts.
//
// Each is reported only when the type makes it certain: a platform type,
// a type parameter that may stand for a nullable type, or a type the
// checker didn't find are never not null here. A wrong warning breaks a
// build with -Werror.
func (c *Checker) localWarnings() {
	walkCode(c.file.Decls, func(n any) bool {
		switch n := n.(type) {
		case *syntax.NotNull:
			if t := n.X.Type(); definitelyNonNull(t) && !c.maybeNull[n] && !lambdaLike(n.X) {
				c.warnf(n.P, "UNNECESSARY_NOT_NULL_ASSERTION", "unnecessary non-null assertion (!!) on a non-null receiver of type '%s'", t)
			}
		case *syntax.Member:
			if n.Safe && n.X != nil {
				if t := n.X.Type(); definitelyNonNull(t) {
					c.warnf(n.P, "UNNECESSARY_SAFE_CALL", "unnecessary safe call on a non-null receiver of type '%s'", t)
				}
			}
		case *syntax.Binary:
			c.binaryWarnings(n)
		case *syntax.As:
			c.castWarnings(n)
		}
		return true
	})
}

// noteMaybeNull records that a check of x!! or x ?: y found x of type t,
// if x may be null.
func (c *Checker) noteMaybeNull(n syntax.Expr, t types.Type) {
	if definitelyNonNull(t) || t.K == types.Nothing && !t.N {
		return
	}
	if c.maybeNull == nil {
		c.maybeNull = map[syntax.Expr]bool{}
	}
	c.maybeNull[n] = true
}

// lambdaLike reports whether x is a lambda or a callable reference,
// which kotlinc resolves as an argument of !! or ?: and doesn't warn of.
func lambdaLike(x syntax.Expr) bool {
	_, ok := x.(*syntax.Lambda)
	return ok
}

// definitelyNonNull reports whether a value of type t is certainly not
// null: t isn't nullable, a platform type, a type parameter with a
// nullable bound, an inference variable, Nothing or a type kc didn't
// find.
func definitelyNonNull(t types.Type) bool {
	switch t.K {
	case types.Invalid, types.TypeVar, types.Null, types.Nothing, types.Range:
		return false
	}
	return !t.P && !types.MayBeNull(t) && !hasInvalid(t, 0)
}

// hasInvalid reports whether type t, or a type it is made of, is one kc
// didn't find.
func hasInvalid(t types.Type, depth int) bool {
	if t.K == types.Invalid || depth > 16 || t.K == types.Class && (t.C == nil || t.C.Missing) {
		return true
	}
	if t.A != nil && !t.IsParam() {
		for i := range t.A.Len() {
			if a := t.A.Get(i); a.V != types.Star && hasInvalid(a.T, depth+1) {
				return true
			}
		}
	}
	return false
}

func (c *Checker) binaryWarnings(x *syntax.Binary) {
	switch x.Op {
	case syntax.EQ, syntax.NE, syntax.EQ_EQ_EQ, syntax.NOT_EQ_EQ:
		other := x.X
		if _, ok := x.X.(*syntax.NullLit); ok {
			other = x.Y
		} else if _, ok := x.Y.(*syntax.NullLit); !ok {
			return
		}
		if _, ok := other.(*syntax.NullLit); ok || other == nil {
			return
		}
		if definitelyNonNull(other.Type()) {
			result := "false"
			if x.Op == syntax.NE || x.Op == syntax.NOT_EQ_EQ {
				result = "true"
			}
			c.warnf(x.X.Pos(), "SENSELESS_COMPARISON", "condition is always '%s'", result)
		}
	case syntax.ELVIS:
		if x.X == nil {
			return
		}
		// A left operand of type Nothing never gives a value: kotlinc
		// warns of the elvis too.
		if t := x.X.Type(); (definitelyNonNull(t) || t.K == types.Nothing && !t.N) && !c.maybeNull[x] && !lambdaLike(x.X) {
			c.warnf(x.P, "USELESS_ELVIS", "elvis operator (?:) always returns the left operand of non-nullable type '%s'", t)
		}
	}
}

// castWarnings reports a cast x as T of a value that is already of type T.
func (c *Checker) castWarnings(x *syntax.As) {
	if x.Safe || x.X == nil {
		return
	}
	switch x.X.(type) {
	case *syntax.Lambda, *syntax.NullLit, *syntax.IntLit, *syntax.DoubleLit, *syntax.FloatLit:
		// A cast that gives a lambda, a reference or a literal its
		// type changes what it is.
		return
	case *syntax.Call:
		// A generic call without type arguments may take them from the
		// type the cast expects: foo() as String.
		if call := x.X.(*syntax.Call); len(call.TypeArgs) == 0 &&
			(call.Sym != nil && len(call.Sym.TypeParams) > 0 || call.Ctor != nil && len(call.Ctor.TypeParams) > 0) {
			return
		}
	}
	t, target := x.X.Type(), x.Target
	if t.K == types.Invalid || target.K == types.Invalid || t.P || t.IsParam() || target.IsParam() {
		return
	}
	if t.N != target.N || t != target && !c.sameType(t, target) {
		return
	}
	c.warnf(x.P, "USELESS_CAST", "cast is redundant")
}
