package check

import "github.com/barteo/kotlin-compiler-go/internal/types"

// funcSubNarrow reports whether a value of function type t that is
// identical to one of function type ot is then of ot, which is a subtype
// of t: c of (Int) -> Number is an (Any) -> Int after c === b for an
// (Any) -> Int b, as K2's intersection of the two is.
func (c *Checker) funcSubNarrow(t, ot types.Type) bool {
	return t.K == types.Class && ot.K == types.Class && t.C.Func && ot.C.Func && !types.MayBeNull(t) && c.subtype(ot, t)
}
