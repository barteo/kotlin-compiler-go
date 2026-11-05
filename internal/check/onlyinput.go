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
// kc checks a variable only when each of its constraints is a type
// without variables, type parameters (a star projection's), captures or
// a platform type's flexibility, which it compares as K2 does; it leaves
// the others be.

// onlyInputOK reports whether val, inferred for tv, is the type of one of
// its constraints, or kc can't tell.
func (s *system) onlyInputOK(tv *tvar, val types.Type) bool {
	if tv.v.Origin == nil || !tv.v.Origin.OnlyInput || tv.star || tv.proj || len(tv.lits) > 0 {
		return true
	}
	if val.K == types.Invalid || val.HasVars() || hasFlex(val) {
		return true
	}
	var inputs []types.Type
	for _, cs := range [][]types.Type{tv.lower, tv.upper} {
		for _, t := range cs {
			t = s.apply(t)
			if t.K == types.Invalid || t.HasVars() || t.HasParams() || hasFlex(t) || t.K == types.Nothing && !t.N {
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

// hasFlex reports whether t is or has a platform type, or a star.
func hasFlex(t types.Type) bool {
	return t.Has(func(u types.Type) bool {
		if u.P {
			return true
		}
		if u.A != nil {
			for _, a := range u.A.List {
				if a.V != types.Inv {
					return true // a projection, out T or *
				}
			}
		}
		return false
	})
}
