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
			if t := n.X.Type(); definitelyNonNull(t) {
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

// definitelyNonNull reports whether a value of type t is certainly not
// null: t isn't nullable, a platform type, a type parameter with a
// nullable bound, an inference variable, Nothing or a type kc didn't
// find.
func definitelyNonNull(t types.Type) bool {
	switch t.K {
	case types.Invalid, types.TypeVar, types.Null, types.Nothing, types.Range:
		return false
	case types.Class:
		if t.C == nil || t.C.Missing {
			return false
		}
	}
	return !t.P && !types.MayBeNull(t)
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
		if t := x.X.Type(); definitelyNonNull(t) {
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
