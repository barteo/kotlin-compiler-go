package check

import (
	"slices"

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
	u := newUnusedWarnings(c.file.Decls)
	c.shadowWarnings()
	c.dangerousNames()
	c.inPlaceWarnings()
	c.redundantBackingFields()
	var deadClasses map[*syntax.ClassDecl]bool
	if c.u.extraWarnings {
		c.unusedVarWarnings()
		c.unusedLambdaParams()
		c.redundantVisibilities()
		// Before the walk, so that unreachableWarnings skips the blocks
		// reported here.
		deadClasses = map[*syntax.ClassDecl]bool{}
		for _, d := range c.classes {
			c.classDeadCode(d, deadClasses)
		}
	}
	if c.u.experimentalWarnings {
		// Those of a file with syntax errors, read from its tree: the
		// literals that are kept are reported once, by the walk too.
		for _, p := range c.file.RedundantPrefixes {
			c.warnf(p, "REDUNDANT_INTERPOLATION_PREFIX", "redundant interpolation prefix")
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
			if c.u.extraWarnings {
				c.redundantDeclTypes(lf.Decl)
			}
		}
		if s, ok := n.(*syntax.StringLit); ok && c.u.extraWarnings {
			c.redundantTemplate(s)
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
			// As for ?:, a receiver of type Nothing too.
			if t := n.X.Type(); (definitelyNonNull(t) || t.K == types.Nothing && !t.N) && !c.maybeNull[n] && !lambdaLike(n.X) {
				c.warnf(n.P, "UNNECESSARY_NOT_NULL_ASSERTION", "unnecessary non-null assertion (!!) on a non-null receiver of type '%s'", t)
			}
		case *syntax.Member:
			c.platformArgWarning(n)
			c.javaClassOnCompanion(n)
			if !callees[n] {
				c.reflectionWarning(n, nil)
			}
			if n.Safe && n.X != nil {
				// As for ?:, a receiver of type Nothing too.
				if t := n.X.Type(); definitelyNonNull(t) || t.K == types.Nothing && !t.N {
					c.warnf(n.P, "UNNECESSARY_SAFE_CALL", "unnecessary safe call on a non-null receiver of type '%s'", t)
				} else if c.qualifierSafe[n] {
					c.warnf(n.P, "UNNECESSARY_SAFE_CALL", "unnecessary safe call on a non-null receiver")
				}
			}
		case *syntax.Binary:
			c.arrayEqualityWarning(n)
			if c.u.experimentalWarnings {
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
				if n.Body != nil {
					c.markBlockDead(n.Body) // not written: unreachableWarnings skips it
				}
				syntax.Inspect(n.Body, func(m any) bool {
					if x, ok := m.(*syntax.Call); ok {
						refCalls[x] = true
					}
					return true
				})
			}
		case *syntax.VarStmt:
			if n.Get != nil {
				// x$delegate.getValue(null, ref), which isn't written:
				// reported at the delegate.
				c.emptyIntersectionWarningAt(n.Get, syntax.StartPos(n.Delegate))
			}
		case *syntax.Call:
			c.platformArgWarning(n)
			c.emptyIntersectionWarningAt(n, delegateAt(n, calleePos(n)))
			c.inferredAliasBounds(n)
			c.atomicWarnings(n)
			c.redundantSpreads(n)
			c.varargNothing(n)
			if !refCalls[n] {
				c.trimMarginWarning(n)
				c.conversionWarning(n)
				c.inferredInvisibleCall(n)
				if c.u.extraWarnings {
					c.uselessCallOnNotNull(n)
				}
			}
			if c.u.experimentalWarnings {
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
			c.inferredInvisibleBranches(n)
		case *syntax.If:
			c.inferredInvisibleBranches(n)
		case *syntax.Block:
			c.sameLineAnnotations(n)
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
	if definitelyNonNull(t) || t.K == types.Nothing && !t.N || t.HasVars() {
		// A type with variables not inferred yet says nothing: the
		// warning judges the type they are inferred to.
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

// equalityWarning reports comparison x as kotlinc's
// EQUALITY_NOT_APPLICABLE_WARNING: its operands' types are unrelated, but
// K1 let both have subtypes (see equalityCompat). It follows the path of
// equality to that check.
func (c *Checker) equalityWarning(x *syntax.Binary) {
	xt, yt := x.X.Type(), x.Y.Type()
	for _, t := range []types.Type{xt, yt} {
		switch {
		case t.K == types.Invalid, t.K == types.Unit, t.K == types.Null, t.K == types.Nothing, t.IsParam(), t.HasVars(), types.IsValue(t):
			return
		}
	}
	if xt.K == types.Class && yt.K == types.Class && xt.C == yt.C {
		return
	}
	xn, yn := xt.NonNull(), yt.NonNull()
	if xn == yn || c.subtype(xn, yn) || c.subtype(yn, xn) || c.mayBeSame(xn, yn) {
		return
	}
	if c.equalityCompat(x.Op, xt, yt) == eqWarning {
		c.warnf(syntax.StartPos(x), "EQUALITY_NOT_APPLICABLE_WARNING", "operator '%s' cannot be applied to '%s' and '%s'", x.Op, xt, yt)
	}
}

func (c *Checker) binaryWarnings(x *syntax.Binary) {
	switch x.Op {
	case syntax.EQ, syntax.NE, syntax.EQ_EQ_EQ, syntax.NOT_EQ_EQ:
		if x.X == nil || x.Y == nil {
			return
		}
		c.equalityWarning(x)
		// As kotlinc's checkSenselessness, by the operands' types: one
		// is always null, the literal or a value of type Nothing?, and
		// the other always is too, or can't be, a value of type Nothing
		// among those.
		other := x.X
		if _, ok := x.X.(*syntax.NullLit); ok {
			other = x.Y
		} else if _, ok := x.Y.(*syntax.NullLit); !ok {
			switch {
			case x.Y.Type().K == types.Null:
			case x.X.Type().K == types.Null:
				other = x.Y
			default:
				return
			}
		}
		if _, ok := other.(*syntax.NullLit); ok {
			return
		}
		eq := x.Op == syntax.EQ || x.Op == syntax.EQ_EQ_EQ
		if t := other.Type(); definitelyNonNull(t) || t.K == types.Null || t.K == types.Nothing && !t.N {
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
		// A Long literal too large for an Int may be written without
		// the L that kotlinc tells it by.
		if e.Long && !e.Unsigned && e.Val != int64(int32(e.Val)) {
			return
		}
	case *syntax.Call:
		// A call of a function whose result is its only type parameter,
		// which its parameters don't mention, takes its type from the
		// type the cast expects: foo() as String, for a fun <T> foo(): T.
		// Not a safe call's, which isn't a function call to kotlinc's
		// checker: the cast still gives it its type argument, as kc
		// doesn't, so that x?.foo() as? String is useless.
		if m, ok := e.Fun.(*syntax.Member); e.Ctor == nil && expectsTypeFromCast(e.Sym) {
			if !ok || !m.Safe || e.Invoke {
				return
			}
			if len(e.TypeArgs) == 0 {
				if x.Safe && c.meetsBounds(x.Target.NonNull(), e.Sym.TypeParams[0]) {
					c.warnf(x.P, "USELESS_CAST", "cast is redundant")
				}
				return
			}
		}
	}
	target := x.Target
	if target.K == types.Invalid || target.K == types.TypeVar {
		return
	}
	exact := func(t types.Type) bool {
		if t.K == types.Invalid || t.K == types.TypeVar {
			return false
		}
		if t.P {
			t = platformUpper(t) // a platform type's upper bound, T?
		}
		target := target
		if x.Safe && t.N {
			target = target.Nullable()
		}
		return t.N == target.N && (t == target || c.sameType(t, target))
	}
	t := x.X.Type()
	switch {
	case exact(t):
	case x.Safe && t.K == types.Null && !types.MayBeNull(target):
		// A value that is always null, as? a type that excludes it.
	default:
		return
	}
	if intLiteral(x.X) {
		// As kotlinc, 1 as Int, which may be meant as a conversion.
		c.warnf(x.P, "INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL", "use 'to%s' to give the type explicitly", target)
		return
	}
	c.warnf(x.P, "USELESS_CAST", "cast is redundant")
}

// meetsBounds reports whether type t, which has no type variables,
// meets the bounds of type parameter p, which mention no type
// parameters.
func (c *Checker) meetsBounds(t types.Type, p *types.TypeParamSym) bool {
	if !certainCastType(t) {
		return false
	}
	for _, b := range p.Bounds {
		if b.P {
			b = platformUpper(b)
		}
		if mentionsParam(b, p) || !certainCastType(b) || !c.subtype(t, b) {
			return false
		}
	}
	return true
}

// expectsTypeFromCast reports whether f, a function with a single type
// parameter that is its result type and that no parameter, receiver or
// context parameter mentions, may take its type argument from a cast of
// its call, as kotlinc's isFunctionForExpectTypeFromCastFeature: the cast
// isn't useless then.
func expectsTypeFromCast(f *types.FunSym) bool {
	if f == nil || len(f.TypeParams) != 1 {
		return false
	}
	tp := f.TypeParams[0]
	if r := f.Ret; r.K != types.TypeParam || r.Param() != tp || r.D {
		return false // nor a T & Any, as a Java @NotNull T
	}
	if f.Ext && mentionsParam(f.Recv, tp) {
		return false
	}
	for _, ps := range [][]*types.VarSym{f.Params, f.Contexts} {
		for _, p := range ps {
			if mentionsParam(p.Type, tp) {
				return false
			}
		}
	}
	return true
}

// intLiteralOps are the operators of the integer literal operations
// whose type kotlinc takes from where they're used, as of literals.
var intLiteralOps = map[string]bool{
	"plus": true, "minus": true, "times": true, "div": true, "rem": true,
	"shl": true, "shr": true, "ushr": true, "and": true, "or": true, "xor": true,
	"inv": true, "unaryMinus": true, "unaryPlus": true,
}

// intLiteral reports whether x is an integer literal of type Int or
// UInt, 1 or 1u, or an operation of those, -1 or 1 shl 2: whose type
// kotlinc's hasIntegerLiteralTypeAmbiguity says depends on where it's
// used.
func intLiteral(x syntax.Expr) bool {
	switch e := x.(type) {
	case *syntax.IntLit:
		return !e.Long
	case *syntax.Unary:
		return (e.Op == syntax.MINUS || e.Op == syntax.PLUS) && intLiteral(e.X)
	case *syntax.Binary:
		switch e.Op {
		case syntax.PLUS, syntax.MINUS, syntax.STAR, syntax.SLASH, syntax.PERCENT:
			return intLiteral(e.X) && intLiteral(e.Y)
		}
	case *syntax.Call:
		m, ok := e.Fun.(*syntax.Member)
		if !ok || m.Safe || e.Invoke || len(e.TypeArgs) > 0 || !intLiteralOps[m.Name] || !intLiteral(m.X) {
			return false
		}
		for _, a := range e.Args {
			if !intLiteral(a) {
				return false
			}
		}
		return len(e.Args) <= 1
	}
	return false
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
	if hasInvalid(t, 0) || hasInvalid(target, 0) || c.subtype(t, target) {
		return
	}
	if t.P {
		// Of a platform type, only what tells nothing of the target's
		// type arguments: a Class<*>! as Class<T>.
		t = platformUpper(t)
		if target.K != types.Class || target.A == nil || !argsTellMore(target) || t.K != types.Class || t.C == nil || t.A == nil || !starArgsOnly(t) || !target.C.IsSubclassOf(t.C) {
			return
		}
	}
	switch {
	case target.K == types.TypeParam:
		p := target.Param()
		if p == nil || p.Reified || p.Intersection {
			return
		}
		if t.IsParam() {
			// Of another type parameter, or the same one nullable (V? as
			// V), that isn't a subtype of it: not of an intersection or a
			// smart-cast value, which kc's types may not tell as kotlinc's.
			// as? T is a cast to T?.
			if f, ok := c.typeFacts[x]; t.IsIntersection() || !ok || len(f.casts) > 0 || !certainCastType(t) || x.Safe && c.subtype(t, target.Nullable()) {
				return
			}
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
		case (t.K == types.Class || t.K == types.TypeParam) && c.castErased(x, t, target):
		default:
			return
		}
	default:
		return
	}
	c.warnf(x.P, "UNCHECKED_CAST", "unchecked cast of '%s' to '%s'", t, target)
}

// castErased reports whether cast x of a value of class type t to class
// type target, which has type arguments, is unchecked, as kotlinc's
// isCastErased judges it: the type of target's class that t tells
// statically (findStaticallyKnownSubtype), whose type arguments are
// those that t's determine and the class's own type parameters, isn't a
// subtype of target: Class<T> as Class<Any>, or MutableCollection<String>
// as MutableList<Int>. Of a smart-cast operand, t is the intersection of
// its declared type and its smart casts, whose parts together determine
// the type arguments. kc judges only casts that can succeed, and where
// the parts' type arguments map to the class's type parameters one by
// one, without projections.
func (c *Checker) castErased(x *syntax.As, t, target types.Type) bool {
	f, ok := c.typeFacts[x]
	if !ok || !f.certain() || !certainCastType(target) {
		return false
	}
	if lambdaLike(unchecked(x.X)) && (target.C.Interface || target.IsFunc()) {
		// A lambda or a reference is of a function type, which tells
		// nothing of a class's type arguments, but kc's may not be
		// kotlinc's to compare with an interface's.
		return false
	}
	if c.castImpossible(f.orig, []types.Type{f.orig}, target, x.Safe) {
		return false // CAST_NEVER_SUCCEEDS
	}
	known := []types.Type{t}
	if len(f.casts) > 0 {
		if c.castImpossible(f.casts[len(f.casts)-1], f.all(), target, x.Safe) {
			return false
		}
		known = f.all()
	}
	// The parts: an intersection's components, but not type parameters,
	// which tell nothing of the type arguments, as kotlinc judges them,
	// whatever their bounds: a T : List<String> as ArrayList<String> is
	// unchecked.
	var parts []types.Type
	for _, t := range known {
		switch {
		case t.IsIntersection():
			for _, b := range t.Param().Bounds {
				if !b.IsParam() {
					parts = append(parts, b)
				}
			}
		case !t.IsParam():
			parts = append(parts, t)
		}
	}
	target = target.NonNull()
	k := target.C
	if k.Local {
		return false
	}
	args := make([]types.Arg, len(k.TypeParams))
	set := make([]bool, len(k.TypeParams))
	conflict := false
	for _, t := range parts {
		t = t.NonNull()
		if t.K == types.Any {
			continue
		}
		if t.K != types.Class || t.C == nil || platformArgs(t) {
			return false
		}
		if c.subtype(t, target) {
			return false
		}
		if t.C == k {
			if len(parts) == 1 {
				return true // not a subtype of target, of its own class
			}
			if t.A == nil || t.A.Len() != len(k.TypeParams) {
				return false
			}
			for j, ta := range t.A.List {
				if ta.V != types.Inv && ta.V != types.Star {
					return false
				}
				if ta.V == types.Star {
					ta = target.A.List[j]
				}
				if set[j] && args[j] != ta {
					conflict = true
				}
				args[j], set[j] = ta, true
			}
			continue
		}
		sup, ok := c.asSuper(k.This(), t.C)
		if !ok || sup.A == nil {
			continue // a part that tells nothing of k's type arguments
		}
		if t.A == nil || t.A.Len() != sup.A.Len() {
			return false
		}
		for i, a := range sup.A.List {
			ta := t.A.List[i]
			j := -1
			if a.V == types.Inv && a.T.K == types.TypeParam && !a.T.N {
				j = slices.Index(k.TypeParams, a.T.Param())
			}
			if j < 0 || ta.V != types.Inv && ta.V != types.Star {
				return false // not one of k's parameters, or projected
			}
			arg := ta
			if ta.V == types.Star {
				// What a star tells fits anything.
				arg = target.A.List[j]
			}
			if set[j] && args[j] != arg {
				conflict = true
			}
			args[j], set[j] = arg, true
		}
	}
	if conflict {
		// Parts that give a type argument different types, Array<Any?>
		// and Array<Int>, none of them a subtype of target.
		return true
	}
	for j, p := range k.TypeParams {
		if !set[j] {
			args[j] = types.Arg{T: types.ParamType(p)}
		}
	}
	return !c.subtype(types.GenericType(k, args), target)
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

// sameLineAnnotations warns of the statements of block b that begin with
// an annotation followed, on its line, by a binary operator, an infix
// call or an assignment, @A x + y, as kotlinc's
// ANNOTATIONS_ON_BLOCK_LEVEL_EXPRESSION_ON_THE_SAME_LINE: a line break
// after it makes it the whole statement's, and otherwise it is x's.
func (c *Checker) sameLineAnnotations(b *syntax.Block) {
	if b == nil || len(c.file.Annotated) == 0 {
		return
	}
	for _, st := range b.Stmts {
		var n any
		var op syntax.Pos // the outermost operator
		switch st := st.(type) {
		case *syntax.ExprStmt:
			n = st.X
			switch x := st.X.(type) {
			case *syntax.Binary:
				if !x.Paren {
					op = x.P
				}
			case *syntax.Call:
				if m, ok := x.Fun.(*syntax.Member); ok && x.Infix && !x.Paren {
					op = m.P
				}
			}
		case *syntax.Assign:
			if st.Op == syntax.ASSIGN {
				// Not y += 1, of which kotlinc reports the wrapped
				// left-hand side alone.
				n, op = st, st.P
			}
		}
		if op == (syntax.Pos{}) {
			continue
		}
		if as := c.file.Annotated[n]; len(as) > 0 && as[0].P.Line == op.Line {
			c.warnf(as[0].P, "ANNOTATIONS_ON_BLOCK_LEVEL_EXPRESSION_ON_THE_SAME_LINE", "annotations on block-level expressions are being parsed differently depending on presence of a new line after them. Use new line if whole block-level expression must be annotated or wrap annotated expression in parentheses")
		}
	}
}
