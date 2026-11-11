package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// coerceCall checks a call of `kc$coerce`<R>(x), which only the code kc
// writes for compiler plugins calls (state.plugin): x, of a value class
// type held unboxed, as R, the type that represents it on the JVM, its
// underlying value's, or the other way round. kotlinc's parcelize plugin
// writes a UInt or a Duration as its underlying value so, with no call
// and whatever the visibility of the class's constructor and property.
func (c *Checker) coerceCall(x *syntax.Call) types.Type {
	x.Builtin = types.UnsafeCoerce
	x.Sym = &types.FunSym{Name: "kc$coerce", Resolved: true}
	if len(x.TypeArgs) != 1 || x.TypeArgs[0].Type == nil || len(x.Args) != 1 {
		c.errorf(x.P, "kc$coerce takes a type argument and an argument")
		c.args(x, nil)
		return types.InvalidType
	}
	to := c.resolveType(x.TypeArgs[0].Type)
	from := c.value(x.Args[0], noWant)
	if to.K == types.Invalid || from.K == types.Invalid {
		return types.InvalidType
	}
	if !(types.Unboxed(from) && c.sameType(types.Repr(from), to) || types.Unboxed(to) && c.sameType(types.Repr(to), from)) {
		c.errorf(x.P, "kc$coerce can't coerce %s to %s", from, to)
		return types.InvalidType
	}
	return to
}
