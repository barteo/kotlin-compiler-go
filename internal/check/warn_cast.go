package check

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports the warnings of kotlinc's FirCastOperatorsChecker
// about type checks and casts whose outcome the types decide: x is T that
// is always true (USELESS_IS_CHECK) or, by a smart cast, never
// (IMPOSSIBLE_IS_CHECK_WARNING), and x as T that never succeeds
// (CAST_NEVER_SUCCEEDS and its kin). As kotlinc, it judges x by its type
// before smart casts first, and then, if that decides nothing, by the
// intersection of the types its smart casts give it. Those are the
// typeFacts its check recorded. Types kc may not know as kotlinc does,
// platform types and those it didn't find among them, are never judged.

// whenIsWarnings reports the is and !is conditions of when x as
// isWarnings does.
func (c *Checker) whenIsWarnings(x *syntax.When) {
	for i, br := range x.Branches {
		for j := range br.Conds {
			cond := &br.Conds[j]
			if !cond.Is && !cond.NotIs {
				continue
			}
			// kotlinc doesn't warn of the only condition of the last of
			// several branches of an exhaustive when. An is condition
			// that always holds makes it exhaustive; with a !is one, the
			// branches before it must.
			last := i == len(x.Branches)-1 && len(x.Branches) > 1 && len(br.Conds) == 1
			last = last && (cond.NotIs && c.coveredWhens[x] || cond.Is && br.Guard == nil)
			c.isWarnings(cond, x.Subject, cond.P, cond.Target, cond.NotIs, last)
		}
	}
}

// isWarnings reports the check n, x is T or x !is T at p, whose outcome
// x's type decides: USELESS_IS_CHECK if x is always a T, unless last is
// set, and IMPOSSIBLE_IS_CHECK_WARNING if its smart casts make it never
// one. If its type before smart casts does, the check is the error
// IMPOSSIBLE_IS_CHECK (see impossibleIs). x is the value checked.
func (c *Checker) isWarnings(n any, x syntax.Expr, p syntax.Pos, t types.Type, not, last bool) {
	f, ok := c.typeFacts[n]
	useless := func() {
		switch {
		case last:
		case isLiteral(x):
			c.robustWarnf(p, "USELESS_IS_CHECK", "check for instance is always '%v'", !not)
		default:
			c.warnf(p, "USELESS_IS_CHECK", "check for instance is always '%v'", !not)
		}
	}
	if ok && f.orig.P && certainCastType(t) {
		// A platform type is judged by its upper bound, T?, and only as
		// useless: T! is T? always holds.
		if up := platformUpper(f.orig); certainCastType(up) && c.subtype(up, t) {
			useless()
		}
		return
	}
	if !ok || !certainCastType(t) || !f.certain() || f.orig.K == types.Nothing {
		return
	}
	if f.orig.K == types.Null {
		// null is T? always holds; that null !is Nothing does isn't told.
		if c.subtype(f.orig, t) {
			useless()
		}
		return
	}
	if c.subtype(f.orig, t) {
		useless()
		return
	}
	r, ok := c.castInfo(t, 0)
	if !ok {
		return
	}
	if l, ok := c.castInfo(f.orig, 0); ok && c.rules1(l, r) {
		return // the error
	}
	if len(f.casts) == 0 {
		return
	}
	if slices.ContainsFunc(f.casts, func(s types.Type) bool { return c.subtype(s, t) }) {
		useless()
		return
	}
	l, ok := c.intersectionInfo(f.all())
	if !ok || !c.rules1(l, r) {
		return
	}
	if f.mayBeNull() && types.MayBeNull(t) {
		c.warnf(p, "IMPOSSIBLE_IS_CHECK_RELYING_ON_NULL_WARNING", "this check only evaluates to '%v' when the left-hand side is null", !not)
		return
	}
	c.warnf(p, "IMPOSSIBLE_IS_CHECK_WARNING", "check for instance is always '%v'", not)
}

// castNeverSucceeds reports x as T, or x as? T, that can't give a T: as
// CAST_NEVER_SUCCEEDS, or, between primitive numbers, as
// NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL, or, if
// both may be null, so that only null passes, as UNSAFE_CAST_RELYING_ON_NULL
// or SAFE_CAST_RELYING_ON_NULL.
func (c *Checker) castNeverSucceeds(x *syntax.As) {
	f, ok := c.typeFacts[x]
	t := x.Target
	if !ok || !certainCastType(t) || !f.certain() {
		return
	}
	if !c.castImpossible(f.orig, []types.Type{f.orig}, t, x.Safe) {
		if len(f.casts) == 0 || !c.castImpossible(f.casts[len(f.casts)-1], f.all(), t, x.Safe) {
			return
		}
	}
	switch {
	case f.mayBeNull() && types.MayBeNull(t) && x.Safe:
		c.warnf(x.P, "SAFE_CAST_RELYING_ON_NULL", "this cast always returns null, even when it succeeds")
	case f.mayBeNull() && types.MayBeNull(t):
		c.warnf(x.P, "UNSAFE_CAST_RELYING_ON_NULL", "this cast only succeeds when the expression is null; consider rewriting it explicitly")
	case c.numericCast(f, t):
		conv := "to" + numberName(t)
		if t.N {
			conv += "?"
		}
		c.warnf(x.P, "NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL", "this cast can never succeed; use '%s' to perform numeric conversion", conv)
	default:
		c.warnf(x.P, "CAST_NEVER_SUCCEEDS", "this cast can never succeed")
	}
}

// castImpossible reports whether a value of the intersection of ls, whose
// type as kotlinc names it is l, can never be cast to t, as kotlinc's
// checkAsApplicability decides: Nothing can be cast to anything, nothing
// can be cast to Nothing, null can't be cast to a type that excludes it,
// and otherwise RULES1 (see rules1).
func (c *Checker) castImpossible(l types.Type, ls []types.Type, t types.Type, safe bool) bool {
	nullNothing := func(t types.Type) bool { return t.K == types.Null || t.K == types.Nothing && t.N }
	mayBeNull := !slices.ContainsFunc(ls, func(t types.Type) bool { return !types.MayBeNull(t) })
	switch {
	case l.K == types.Nothing && !l.N:
		return false
	case t.K == types.Nothing && !t.N:
		return true
	case !mayBeNull && nullNothing(t) || nullNothing(l) && !types.MayBeNull(t):
		return !safe // null as? T is null, and a useless cast
	}
	li, ok1 := c.intersectionInfo(ls)
	r, ok2 := c.castInfo(t, 0)
	return ok1 && ok2 && c.rules1(li, r)
}

// numericCast reports whether a cast to t of a value with facts f is
// between primitive numbers, or unsigned ones, nullable or not: kotlinc
// suggests a conversion function instead.
func (c *Checker) numericCast(f typeFacts, t types.Type) bool {
	if numberName(t) == "" {
		return false
	}
	l, ok := c.intersectionInfo(f.all())
	return ok && len(l.parts) == 1 && numberName(l.parts[0]) != ""
}

// numberName returns the simple name of t if it is a primitive number
// type or an unsigned one, nullable or not, or else "".
func numberName(t types.Type) string {
	switch t.K {
	case types.Byte:
		return "Byte"
	case types.Short:
		return "Short"
	case types.Int:
		return "Int"
	case types.Long:
		return "Long"
	case types.Float:
		return "Float"
	case types.Double:
		return "Double"
	case types.Class:
		if t.C != nil {
			switch t.C.Name {
			case "kotlin/UByte", "kotlin/UShort", "kotlin/UInt", "kotlin/ULong":
				return strings.TrimPrefix(t.C.Name, "kotlin/")
			}
		}
	}
	return ""
}

// all returns the types f knows of its value: its declared type and its
// smart casts.
func (f typeFacts) all() []types.Type {
	return append([]types.Type{f.orig}, f.casts...)
}

// mayBeNull reports whether the value f knows of may be null: none of
// its types excludes null.
func (f typeFacts) mayBeNull() bool {
	return !slices.ContainsFunc(f.all(), func(t types.Type) bool { return !types.MayBeNull(t) })
}

// certain reports whether kc knows all of f's types as kotlinc does.
func (f typeFacts) certain() bool {
	return !slices.ContainsFunc(f.all(), func(t types.Type) bool { return !certainCastType(t) })
}

// platformUpper returns the upper bound of the platform type t, T? of
// T!, which kotlinc judges a platform type's casts and checks by.
func platformUpper(t types.Type) types.Type {
	t.P, t.N = false, true
	return t
}

// certainCastType reports whether kc knows type t as kotlinc does, for
// judging casts: t isn't a platform type, an inference variable, or a
// type kc didn't find, nor made of one.
func certainCastType(t types.Type) bool {
	switch t.K {
	case types.Invalid, types.TypeVar, types.Range:
		return false
	}
	return !t.P && !hasInvalid(t, 0)
}
