package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// propBound returns the type of receiver type t, or of one of the types
// of an intersection t, whose property x reads: the first that has it,
// or, for an assignment's target, the first whose property is a var, as
// K2's intersection scope has the var: a.foo = null for an a of C & B, C's
// foo a val String and B's a var String?.
func (c *Checker) propBound(x *syntax.Member, t types.Type) types.Type {
	has := func(cls *types.ClassSym) bool { return c.hasProp(cls, x.Name) }
	if c.varTarget == x {
		c.varTarget = nil
		if b := c.boundWith(t, func(cls *types.ClassSym) bool { return c.hasVarProp(cls, x.Name) }); b != t {
			return b
		}
	}
	return c.boundWith(t, has)
}

// hasVarProp reports whether class cls has a var property named name.
func (c *Checker) hasVarProp(cls *types.ClassSym, name string) bool {
	if p := c.userProp(cls, name); p != nil {
		return p.Mutable
	}
	found := false
	cls.Supertypes(func(s *types.ClassSym) bool {
		kp := s.KProps[name]
		found = kp != nil && kp.Setter != nil
		return kp == nil
	})
	return found
}
