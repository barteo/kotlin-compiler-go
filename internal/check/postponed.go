package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Postponed arguments: lambdas whose expected type is an inference
// variable.

// funcUpper returns a function type that variable tv is below through
// the variables it is below, if it has no proper bound of its own: T of
// listOf(id { a -> }) for an expected List<(String) -> Unit> is below
// listOf's E, which is below (String) -> Unit. As in Kotlin, a lambda
// passed for T takes its parameter types from it, or from a declared
// bound, as A's of fun <A : (Int) -> Unit> take(x: A).
func (s *system) funcUpper(tv *tvar) (types.Type, bool) {
	if slices.ContainsFunc(tv.lower, s.proper) || slices.ContainsFunc(tv.upper, s.proper) {
		return types.InvalidType, false
	}
	seen := map[*tvar]bool{tv: true}
	queue := []*tvar{tv}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		for _, u := range slices.Concat(v.upper, v.v.Bounds) {
			u = s.apply(u)
			if w := s.unfixedVar(u); w != nil {
				if !seen[w] {
					seen[w] = true
					queue = append(queue, w)
				}
				continue
			}
			if !u.HasVars() && u.Flex().IsFunc() {
				return u, true
			}
		}
	}
	return types.InvalidType, false
}

// openVar reports whether t is an unfixed variable without proper
// bounds.
func (s *system) openVar(t types.Type) bool {
	tv := s.unfixedVar(s.apply(t).Flex())
	return tv != nil && !s.fixable(tv)
}

// untypedParams reports whether lambda l takes its parameter types from
// its expected type: it declares a parameter without a type, or may take
// it. A callable reference or an anonymous function whose parameters
// have types has a type of its own.
func untypedParams(l *syntax.Lambda) bool {
	if l.Ref != nil {
		return false
	}
	if !l.Arrow && !l.Anon {
		return true
	}
	return slices.ContainsFunc(l.Params, func(p *syntax.LambdaParam) bool { return p.Type == nil })
}
