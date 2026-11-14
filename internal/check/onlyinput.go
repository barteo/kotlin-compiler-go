package check

import "github.com/barteo/kotlin-compiler-go/internal/types"

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
// without variables, type parameters, star projections (which stand for
// captured types, which kc doesn't have); it leaves the others be. A
// platform type is compared as K2 compares flexible types.

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
	for _, cs := range [][]types.Type{tv.lower, tv.upper} {
		for _, t := range cs {
			if w := s.unfixedVar(t); w != nil && len(w.lits) > 0 && len(w.lower) == 0 {
				continue // its literals, passed on to tv's (see fix)
			}
			t = s.apply(t)
			if t.K == types.Invalid || t.HasVars() || t.HasParams() || t.Has(hasStar) || t.K == types.Nothing && !t.N {
				return true
			}
			inputs = append(inputs, t)
		}
	}
	if len(inputs) == 0 {
		return true
	}
	for _, t := range inputs {
		if s.c.subtype(t, val) && s.c.subtype(val, t) {
			return true
		}
	}
	return false
}
