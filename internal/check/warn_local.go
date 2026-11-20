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
	u := &unusedWarnings{usedBlocks: map[*syntax.Block]bool{}, unused: map[syntax.Expr]bool{}}
	c.shadowWarnings()
	var deadClasses map[*syntax.ClassDecl]bool
	if c.u.extraWarnings {
		c.unusedVarWarnings()
		// Before the walk, so that unreachableWarnings skips the blocks
		// reported here.
		deadClasses = map[*syntax.ClassDecl]bool{}
		for _, d := range c.classes {
			c.classDeadCode(d, deadClasses)
		}
	}
	callees := map[*syntax.Member]bool{} // the members called, which reflectionWarning judges with the call
	// refCalls are the calls that callable references are lambdas of, as
	// s.toString() of s::toString, which aren't written.
	refCalls := map[*syntax.Call]bool{}
	walkCode(c.file.Decls, func(n any) bool {
		c.unusedNode(u, n)
		c.divisionWarning(n)
		if lf, ok := n.(*syntax.LocalFun); ok {
			c.extShadowed(lf.Decl)
		}
		switch n := n.(type) {
		case *syntax.NotNull:
			if l, ok := unchecked(n.X).(*syntax.Lambda); ok {
				switch {
				case l.Ref != nil:
					c.warnf(n.P, "NOT_NULL_ASSERTION_ON_CALLABLE_REFERENCE", "Non-null assertion (!!) called on a callable reference expression.")
				case !l.Anon:
					c.warnf(n.P, "NOT_NULL_ASSERTION_ON_LAMBDA_EXPRESSION", "Non-null assertion (!!) called on a lambda expression.")
				}
			}
			if t := n.X.Type(); definitelyNonNull(t) && !c.maybeNull[n] && !lambdaLike(n.X) {
				c.warnf(n.P, "UNNECESSARY_NOT_NULL_ASSERTION", "unnecessary non-null assertion (!!) on a non-null receiver of type '%s'", t)
			}
		case *syntax.Member:
			if !callees[n] {
				c.reflectionWarning(n, nil)
			}
			if n.Safe && n.X != nil {
				// As for ?:, a receiver of type Nothing too.
				if t := n.X.Type(); definitelyNonNull(t) || t.K == types.Nothing && !t.N {
					c.warnf(n.P, "UNNECESSARY_SAFE_CALL", "unnecessary safe call on a non-null receiver of type '%s'", t)
				}
			}
		case *syntax.Binary:
			c.arrayEqualityWarning(n)
			if c.u.extraWarnings || c.u.experimentalWarnings {
				c.emptyRangeWarning(n)
			}
			c.binaryWarnings(n)
			c.enumComparisonWarning(n)
			if n.Op == syntax.EQ_EQ_EQ || n.Op == syntax.NOT_EQ_EQ {
				c.identityWarnings(n)
			}
		case *syntax.Lambda:
			if n.Anon {
				uselessVarargs(c, n)
			}
			if n.Ref != nil {
				syntax.Inspect(n.Body, func(m any) bool {
					if x, ok := m.(*syntax.Call); ok {
						refCalls[x] = true
					}
					return true
				})
			}
		case *syntax.Call:
			c.atomicWarnings(n)
			c.redundantSpreads(n)
			if !refCalls[n] {
				c.conversionWarning(n)
			}
			if c.u.extraWarnings || c.u.experimentalWarnings {
				c.emptyRangeWarning(n)
			}
			if m, ok := n.Fun.(*syntax.Member); ok && !n.Invoke {
				callees[m] = true
				c.reflectionWarning(m, n)
			}
		case *syntax.ObjectExpr:
			if deadClasses != nil {
				c.classDeadCode(n.Decl, deadClasses)
			}
		case *syntax.LocalClass:
			if deadClasses != nil {
				c.classDeadCode(n.Decl, deadClasses)
			}
		case *syntax.As:
			c.castWarnings(n)
			c.uncheckedCast(n)
			c.castNeverSucceeds(n)
		case *syntax.Is:
			c.isWarnings(n, syntax.StartPos(n), n.Target, n.Not, false)
		case *syntax.When:
			c.whenIsWarnings(n)
			c.whenNullWarnings(n)
			c.redundantElseWarning(n)
			c.duplicateWhenConditions(n)
			c.whenEnumWarnings(n)
		case *syntax.Block:
			if c.u.extraWarnings {
				c.unreachableWarnings(n)
			}
		case *syntax.StringLit:
			if c.u.experimentalWarnings {
				c.prefixWarnings(n)
			}
		}
		return true
	})
}

// prefixWarnings reports the redundant interpolation prefixes of the
// string literals in x, an annotation argument or a literal itself, for
// kotlinc's experimental REDUNDANT_INTERPOLATION_PREFIX.
func (c *Checker) prefixWarnings(x syntax.Expr) {
	syntax.Inspect(x, func(n any) bool {
		if s, ok := n.(*syntax.StringLit); ok && s.RedundantPrefix() {
			c.warnf(s.P, "REDUNDANT_INTERPOLATION_PREFIX", "redundant interpolation prefix")
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

// unchecked returns e without the Checked nodes around it.
func unchecked(e syntax.Expr) syntax.Expr {
	for {
		x, ok := e.(*syntax.Checked)
		if !ok {
			return e
		}
		e = x.X
	}
}

// isNullLit reports whether e is the literal null.
func isNullLit(e syntax.Expr) bool {
	_, ok := unchecked(e).(*syntax.NullLit)
	return ok
}

// lambdaLike reports whether x is a lambda, which kotlinc resolves as
// an argument of !! and doesn't warn of (it does for ?:).
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
		eq := x.Op == syntax.EQ || x.Op == syntax.EQ_EQ_EQ
		// Of a value that can't be null, or of type Nothing? and so
		// always null.
		if t := other.Type(); definitelyNonNull(t) || t.K == types.Null {
			c.warnf(x.X.Pos(), "SENSELESS_COMPARISON", "condition is always '%v'", eq == (t.K == types.Null))
		}
	case syntax.ELVIS:
		if x.X == nil {
			return
		}
		// As kotlinc's FirUselessElvisChecker: null ?: y, then a left
		// operand that can't be null, then x ?: null.
		if isNullLit(x.X) {
			c.warnf(x.P, "USELESS_ELVIS_LEFT_IS_NULL", "Elvis operator (?:) is redundant if the left operand is always null.")
			return
		}
		// A left operand of type Nothing never gives a value: kotlinc
		// warns of the elvis too.
		if t := x.X.Type(); (definitelyNonNull(t) || t.K == types.Nothing && !t.N) && !c.maybeNull[x] {
			c.warnf(x.P, "USELESS_ELVIS", "elvis operator (?:) always returns the left operand of non-nullable type '%s'", t)
			return
		}
		if isNullLit(x.Y) && x.X.Type().K != types.Invalid && !lambdaLike(x.X) {
			c.warnf(x.P, "USELESS_ELVIS_RIGHT_IS_NULL", "Elvis operator (?:) is redundant if the right operand is always null.")
		}
	}
}

// castWarnings reports a cast x as T of a value that is already of type
// T, or x as? T of one of type T or, if x may be null, T?: as kotlinc, a
// safe cast of a nullable value to its type's non-null form only checks
// that it's not null.
func (c *Checker) castWarnings(x *syntax.As) {
	if x.X == nil {
		return
	}
	switch e := x.X.(type) {
	case *syntax.Lambda:
		// A cast that gives a lambda or a reference its type changes
		// what it is.
		return
	case *syntax.NullLit:
		// null as T? gives null its type, except null as Nothing?.
		if x.Target.K == types.Null {
			c.warnf(x.P, "USELESS_CAST", "cast is redundant")
		}
		return
	case *syntax.IntLit:
		// An integer literal takes the type it's cast to, 1 as Long,
		// unless written as a Long (1L, 1UL), which kotlinc reports as
		// useless. 1 as Int is INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL.
		if !e.Long || !e.Unsigned && e.Val != int64(int32(e.Val)) {
			return
		}
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
	if x.Safe && t.N {
		target = target.Nullable()
	}
	if t.N != target.N || t != target && !c.sameType(t, target) {
		return
	}
	c.warnf(x.P, "USELESS_CAST", "cast is redundant")
}

// uncheckedCast reports a cast x as T or x as? T that the JVM can't
// check: to a type parameter that isn't reified, or to a generic class's
// type with type arguments from Any or a star-projected supertype, which
// tell nothing of them. Other casts that may be unchecked, from types
// with type arguments, aren't reported.
func (c *Checker) uncheckedCast(x *syntax.As) {
	if x.X == nil {
		return
	}
	t, target := x.X.Type(), x.Target
	if t.P || hasInvalid(t, 0) || hasInvalid(target, 0) || c.subtype(t, target) {
		return
	}
	switch {
	case target.K == types.TypeParam:
		p := target.Param()
		if p == nil || p.Reified || p.Intersection || t.IsParam() {
			return
		}
		switch t.K {
		case types.Nothing:
			return
		case types.Null:
			// null as T of a T that can't be null never succeeds.
			if _, ok := x.X.(*syntax.NullLit); !ok || !target.N && !p.NullableBound() {
				return
			}
		}
	case target.K == types.Class && target.A != nil:
		if !argsTellMore(target) {
			return
		}
		switch {
		case t.K == types.Any && !ownParams(target):
		case target.IsFunc() && t.IsFunc() && t.C == target.C && !t.N && !lambdaLike(unchecked(x.X)):
			// (Int) -> Number as (Any) -> Int: the JVM checks only
			// that it's a Function1.
		case t.K == types.Class && t.C != nil && starArgsOnly(t) && target.C.IsSubclassOf(t.C):
		case t.K == types.Class && t.C != nil && starImplied(t) && target.C.IsSubclassOf(t.C) && x.Ref != nil && x.Ref.Args != nil:
			// From MutableList<out Any?>, which tells as little, to type
			// arguments written: those of a bare type follow from it.
		default:
			return
		}
	default:
		return
	}
	c.warnf(x.P, "UNCHECKED_CAST", "unchecked cast of '%s' to '%s'", t, target)
}

// argsTellMore reports whether some type argument of class type t
// tells more than a star projection would.
func argsTellMore(t types.Type) bool {
	ps := t.C.TypeParams
	for i := range t.A.Len() {
		a := t.A.Get(i)
		if a.V == types.Star || i >= len(ps) {
			continue
		}
		v := ps[i].Var
		if a.V != types.Inv {
			v = a.V
		}
		// out Any? and in Nothing take what * does.
		if v == types.Out && a.T.K == types.Any && a.T.N && len(ps[i].Bounds) == 0 || v == types.In && a.T.K == types.Nothing {
			continue
		}
		return true
	}
	return false
}

// starArgsOnly reports whether class type t has no type arguments other
// than star projections.
func starArgsOnly(t types.Type) bool {
	if t.A == nil {
		return true
	}
	for i := range t.A.Len() {
		if t.A.Get(i).V != types.Star {
			return false
		}
	}
	return true
}

// ownParams reports whether class type t is its class's own type, its
// type arguments its type parameters: Qwe<T> in class Qwe<T>, which
// kotlinc doesn't warn of a cast from Any to.
func ownParams(t types.Type) bool {
	for i := range t.A.Len() {
		a := t.A.Get(i)
		if a.V != types.Inv || a.T.K != types.TypeParam || a.T.Param() == nil || a.T.Param().Owner != any(t.C) {
			return false
		}
	}
	return t.A.Len() > 0
}
