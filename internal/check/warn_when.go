package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// noteRedundantElse records, for localWarnings, whether the else branch
// of when x, whose subject is of type st and whose other branches cover
// cov, is redundant: kotlinc's REDUNDANT_ELSE_IN_WHEN, of a when that
// is exhaustive without it. Its subject's type is a Boolean, an enum,
// or a sealed class of the program or of Kotlin: kc stays silent of
// other whens kotlinc finds exhaustive, as one with is Any of an Any or
// a subject of a type parameter's type. A platform type is covered only
// with null, as kotlinc has it.
func (c *Checker) noteRedundantElse(x *syntax.When, st types.Type, cov *coverage, hasElse, covered bool) {
	redundant := hasElse && covered && x.Subject != nil && enumerable(st) && !hasInvalid(st, 0)
	if redundant && st.K == types.Class {
		cls := st.C
		redundant = !cls.External || cls.FromKotlin || cls.Entries != nil
		// covers takes an entry of another enum, of the same name, for
		// the subject's in code with an incompatible enum comparison.
		if cls.Entries != nil && !slices.ContainsFunc(cov.classes, cls.IsSubclassOf) {
			for _, e := range cls.Entries {
				redundant = redundant && cov.owned[cls][e]
			}
		}
	}
	if redundant && st.P {
		up := st
		up.P, up.N = false, true
		redundant = c.covers(up, cov)
	}
	if c.redundantElses == nil {
		c.redundantElses = map[*syntax.When]bool{}
	}
	c.redundantElses[x] = redundant
}

// redundantElseWarning reports the else branch of when x if
// noteRedundantElse found it redundant.
func (c *Checker) redundantElseWarning(x *syntax.When) {
	if !c.redundantElses[x] {
		return
	}
	for _, br := range x.Branches {
		if br.Else && br.Guard == nil {
			c.warnf(br.P, "REDUNDANT_ELSE_IN_WHEN", "'when' is exhaustive so 'else' is redundant here")
		}
	}
}
