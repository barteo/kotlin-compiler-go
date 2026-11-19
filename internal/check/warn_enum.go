package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// enumComparisonWarning reports x == y and x != y of x, a value of a
// nullable enum, and y, a value of another nullable enum: kotlinc's
// INCOMPATIBLE_ENUM_COMPARISON, which it reports where K1 reported
// nothing, as the values are equal if both are null. Of a non-null enum
// its error is INCOMPATIBLE_ENUM_COMPARISON_ERROR. kc doesn't report the
// warnings kotlinc reports of an enum compared with a value of a type
// parameter or one smart cast to another enum. The operands' types are
// those they are declared with, as kotlinc's are, but in its message.
func (c *Checker) enumComparisonWarning(x *syntax.Binary) {
	if (x.Op != syntax.EQ && x.Op != syntax.NE) || x.X == nil || x.Y == nil {
		return
	}
	if c.nullableEnums(originalType(x.X, x.X.Type()), originalType(x.Y, x.Y.Type())) {
		c.warnf(syntax.StartPos(x), "INCOMPATIBLE_ENUM_COMPARISON", "comparison of incompatible enums '%s' and '%s' is always unsuccessful", x.X.Type(), x.Y.Type())
	}
}

// whenEnumWarnings reports the conditions of when x that compare its
// subject of a nullable enum with a value of another nullable enum, as
// enumComparisonWarning.
func (c *Checker) whenEnumWarnings(x *syntax.When) {
	if x.Subject == nil {
		return
	}
	st := x.Subject.Type()
	for _, br := range x.Branches {
		for _, cond := range br.Conds {
			if cond.X == nil || cond.In || cond.NotIn || cond.Is || cond.NotIs || cond.SubjType.K != types.Invalid {
				continue
			}
			if ct := cond.X.Type(); c.nullableEnums(st, ct) {
				c.warnf(syntax.StartPos(cond.X), "INCOMPATIBLE_ENUM_COMPARISON", "comparison of incompatible enums '%s' and '%s' is always unsuccessful", st, ct)
			}
		}
	}
}

// nullableEnums reports whether xt and yt are nullable types, not
// platform ones, of two enum classes.
func (c *Checker) nullableEnums(xt, yt types.Type) bool {
	if !xt.N || !yt.N || xt.P || yt.P || xt.K != types.Class || yt.K != types.Class || xt.C == yt.C {
		return false
	}
	return c.enumType(xt.NonNull()) && c.enumType(yt.NonNull())
}
