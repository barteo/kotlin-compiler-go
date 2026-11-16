package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// unitElvisWant reports whether elvis x, the last expression of a lambda
// that returns Unit, expects the Unit: as in K2, whose elvis is a call
// elvis(K?, K): K, K takes it if it can, if the right side is a Unit or
// Nothing. Its left side then expects a Unit?: myLet { ::foo } ?:
// unitFun() chooses foo's overload that returns Unit, not myLet { ::foo }
// ?: 1, whose K is above an Int.
func (c *Checker) unitElvisWant(x *syntax.Binary) bool {
	if c.unitElvis != x {
		return false
	}
	c.unitElvis = nil
	t, ok := c.elvisRightFirst(x.Y)
	if !ok || t.N {
		return false
	}
	return t.K == types.Unit || t.K == types.Nothing || t.K == types.Class && t.C.Name == "kotlin/Unit"
}
