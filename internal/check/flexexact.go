package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// flexExact returns the platform type among the exact bounds of a
// variable, if its proper lower bounds are all that type, nullable or
// not, and it is one of the upper bounds too: as in K2, M of
// ResolverForProjectImpl(MyMap<M2, String>(), r) for an r of
// ResolverForProject<M2?> is M2!, which the Java supertype Map<M2!,
// String!> gives, not M2?. Otherwise val, the lower bounds' join.
func (s *system) flexExact(val types.Type, lowers, uppers []types.Type) types.Type {
	if len(lowers) < 2 {
		return val
	}
	for _, u := range uppers {
		if !u.P || !slices.Contains(lowers, u) {
			continue
		}
		base := u.Flex().NonNull()
		if !slices.ContainsFunc(lowers, func(l types.Type) bool { return l.Flex().NonNull() != base }) {
			return u
		}
	}
	return val
}
