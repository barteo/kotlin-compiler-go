package check

import "github.com/barteo/kotlin-compiler-go/internal/syntax"

// suppressBounds turns off the bound checks of a declaration's signature
// whose annotations @Suppress("UPPER_BOUND_VIOLATED"), as kotlinc does,
// and returns what turns them back on.
func (c *Checker) suppressBounds(anns []syntax.Annotation) func() {
	saved := c.noBoundChecks
	if suppresses(anns, "UPPER_BOUND_VIOLATED") {
		c.noBoundChecks = true
	}
	return func() { c.noBoundChecks = saved }
}
