package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The standard library's @kotlin.internal.OnlyInputTypes, on T of
// Iterable<T>.contains(element: T), Map<out K, V>.get(key: K) and
// assertEquals(expected: T, actual: T): what a call infers for the
// variable must be the type of one of the call's inputs, the types it is
// constrained by, not a supertype of them. A List<A>'s contains(b) for
// a B : A infers A, the list's, and its contains(b) for a B? infers A?,
// which neither is: kotlinc reports that it can't infer T.
//
// An integer literal's inputs are the types it can have: Int, and Long,
// Short and Byte where it fits, as K2's integer literal type's possible
// types are.
//
// kc checks a variable only when each of its constraints is a type
// without variables, star projections (which stand for captured types,
// which kc doesn't have) or type parameters other than those in scope at
// the call, as T of c.contains(1) in fun <T> f(c: Collection<T>); it
// leaves the others be. A platform type is compared as K2 compares
// flexible types.

// onlyInputOK reports whether val, inferred for tv, is the type of one of
// its constraints, or kc can't tell.
func (s *system) onlyInputOK(tv *tvar, val types.Type) bool {
	if tv.v.Origin == nil || !tv.v.Origin.OnlyInput || tv.star || tv.proj {
		return true
	}
	// A star projection in val is a common supertype's, InvB<*> of
	// InvB<First> and InvB<Bound>, which stands for K2's InvB<out Bound>:
	// only those of the constraints are captured types. A platform type
	// in val is compared as K2 compares flexible types, by subtyping both
	// ways: Any! of a String! and an Int is neither.
	if val.K == types.Invalid || val.HasVars() {
		return true
	}
	var inputs []types.Type
	for _, lit := range tv.lits {
		if lit.Unsigned {
			return true
		}
		if !lit.Long {
			inputs = append(inputs, types.IntType)
		}
		for _, t := range []types.Type{types.LongType, types.ShortType, types.ByteType} {
			if fitsLit(lit, t) {
				inputs = append(inputs, t)
			}
		}
	}
	coerced := tv.coercedUnits
	for i, cs := range [][]types.Type{tv.lower, tv.upper} {
		for _, t := range cs {
			if w := s.unfixedVar(t); w != nil && len(w.lits) > 0 && len(w.lower) == 0 {
				continue // its literals, passed on to tv's (see fix)
			}
			if i == 0 && coerced > 0 && t == types.UnitType {
				// The Unit of an empty lambda isn't an input, as in K2:
				// expect {} of expect(block: () -> T) infers a Unit that
				// no input has.
				coerced--
				continue
			}
			if i == 0 && slices.Contains(tv.inCapt, t) {
				// The capture of an argument's in projection, which a
				// common supertype never is: as K2, list.contains(x) for
				// an x: Inv<in Number> of a List<Inv<Number>>.
				continue
			}
			t = s.apply(t)
			if t.K == types.Invalid || t.HasVars() || t.Has(hasStar) || t.Has(s.outOfScope) {
				return true
			}
			inputs = append(inputs, t)
		}
	}
	if len(inputs) == 0 {
		return tv.coercedUnits == 0
	}
	for _, t := range inputs {
		if s.c.subtype(t, val) && s.c.subtype(val, t) {
			return true
		}
	}
	return false
}

// outOfScope reports whether t is a type parameter that isn't in scope
// at the call, as one that stands for a captured type, P of a Foo<*>'s
// Map<P, Any?>, or an intersection type.
func (s *system) outOfScope(t types.Type) bool {
	return t.IsParam() && s.c.lookupTypeParam(t.Param().Name) != t.Param()
}
