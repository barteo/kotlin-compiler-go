package check

import "github.com/barteo/kotlin-compiler-go/internal/types"

// disjointClasses reports whether class types a and b, neither an
// interface, are of classes neither of which extends the other: as in K2,
// a value of a type smart cast to the other is of both, an impossible
// intersection whose members are each's: x of En & En2 after x is En &&
// x is En2.
func (c *Checker) disjointClasses(a, b types.Type) bool {
	return a.A == nil && b.A == nil && !c.subtype(a, b) && !c.subtype(b, a)
}
