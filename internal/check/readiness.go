package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// K2's readiness of a variable for fixation
// (VariableReadinessCalculator.TypeVariableFixationReadinessQuality):
// qualities in priority order, each a bit, earlier ones higher.
const (
	rdProper      = 1 << (12 - iota) // a proper constraint (not only declared bounds)
	rdNoDeps                         // no constraint with another variable in a type argument
	rdNonTrivial                     // a proper constraint other than >: Nothing
	rdNotDeclared                    // one not incorporated from a declared bound
	rdReified                        // a reified type parameter's
	rdFlexLower                      // a proper flexible lower constraint
	rdNonLit                         // a proper constraint other than an integer literal's
	rdNoNothing                      // no explicit lower Nothing
	rdEquality                       // a proper equality: the same type above and below
	rdLower                          // a proper lower constraint other than Nothing
)

// readiness returns tv's readiness for fixation as K2 orders it: a
// variable with a greater one is fixed first, and of equal ones the
// first in the call.
func (s *system) readiness(tv *tvar) int {
	r := 0
	declared := func(u types.Type) bool { return slices.Contains(tv.declUp, u) }
	incorporated := func(u types.Type) bool { return declared(u) || slices.Contains(tv.boundUp, u) }
	proper, nonTrivial, notDecl, nonLit, lower := false, false, false, false, false
	for _, l := range tv.lower {
		if !s.proper(l) {
			continue
		}
		proper, nonLit = true, true
		l = s.apply(l)
		if l.K != types.Nothing && l.K != types.Null {
			nonTrivial, notDecl, lower = true, true, true
		}
		if l.P {
			r |= rdFlexLower
		}
	}
	for _, u := range tv.upper {
		if !s.proper(u) || declared(u) {
			continue
		}
		proper, nonLit = true, true
		a := s.apply(u)
		if !(a.K == types.Any && a.N) {
			nonTrivial = true
			if !incorporated(u) {
				notDecl = true
			}
		}
	}
	if len(tv.lits) > 0 {
		proper, nonTrivial, notDecl, lower = true, true, true, true
	}
	if tv.starArg && (len(tv.lower) > 0 || len(tv.upper) > 0 || s.isUpperOf(tv)) {
		// A captured star of an argument's type: K2's equality with the
		// captured type, a proper one. Not one only in another's bound,
		// ER of ED : EventData<ER>, which that one's fixing gives.
		proper, nonTrivial, notDecl, nonLit = true, true, true, true
		r |= rdEquality
	}
	if proper {
		r |= rdProper
	}
	if !slices.ContainsFunc(append(slices.Clip(tv.lower), tv.upper...), s.dependsOnOthers(tv)) && !slices.ContainsFunc(tv.lower, s.safeResult(tv)) {
		r |= rdNoDeps
	}
	if nonTrivial {
		r |= rdNonTrivial
	}
	if notDecl || !nonTrivial {
		r |= rdNotDeclared
	}
	if tv.v.Origin != nil && tv.v.Origin.Reified {
		r |= rdReified
	}
	if nonLit {
		r |= rdNonLit
	}
	if !slices.ContainsFunc(tv.lower, func(l types.Type) bool { l = s.apply(l); return l.K == types.Nothing && !l.N }) {
		r |= rdNoNothing
	}
	for _, l := range tv.lower {
		if s.proper(l) && slices.ContainsFunc(tv.upper, func(u types.Type) bool { return s.apply(u) == s.apply(l) }) {
			r |= rdEquality
			break
		}
	}
	if lower {
		r |= rdLower
	}
	return r
}

// dependsOnOthers returns whether a constraint of tv has another unfixed
// variable in a type argument (hasDependencyToOtherTypeVariable).
func (s *system) dependsOnOthers(tv *tvar) func(types.Type) bool {
	return func(t types.Type) bool {
		t = s.apply(t)
		if t.A == nil || t.A.Len() == 0 {
			return false
		}
		return t.Has(func(u types.Type) bool {
			if u.K != types.TypeVar || u.Param() == tv.v {
				return false
			}
			o := s.byVar[u.Param()]
			return o != nil && !o.fixed
		})
	}
}

// safeResult returns whether a lower constraint of tv is another unfixed
// variable made nullable, the result of a safe call: K2 completes a safe
// call's selector on its own, before the call it is an argument of.
func (s *system) safeResult(tv *tvar) func(types.Type) bool {
	return func(t types.Type) bool {
		t = s.apply(t)
		if t.K != types.TypeVar || !t.N || t.Param() == tv.v {
			return false
		}
		o := s.byVar[t.Param()]
		return o != nil && !o.fixed
	}
}
