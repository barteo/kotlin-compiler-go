package check

import "github.com/barteo/kotlin-compiler-go/internal/syntax"

// This file reports the warnings about modifiers that kotlinc's
// declaration checkers report: NON_FINAL_MEMBER_IN_FINAL_CLASS and
// NON_FINAL_MEMBER_IN_OBJECT (FirOpenMemberChecker).

// openMembers reports, in final class d, the members written open, which
// no subclass can override: as NON_FINAL_MEMBER_IN_OBJECT in an object,
// NON_FINAL_MEMBER_IN_FINAL_CLASS otherwise (an enum entry's body and an
// object expression too), on the modifier. An enum class can have open
// members, which its entries override.
func (c *Checker) openMembers(d *syntax.ClassDecl) {
	if d.Open || d.Abstract || d.Sealed || d.Interface || d.Enum || d.Expect || d.Generated {
		return
	}
	name, msg := "NON_FINAL_MEMBER_IN_FINAL_CLASS", "'open' has no effect on a final class"
	if d.Object {
		name, msg = "NON_FINAL_MEMBER_IN_OBJECT", "'open' has no effect on object"
	}
	report := func(m *syntax.Modifiers) {
		for _, w := range m.Written {
			if w.Name == "open" {
				c.warnf(w.P, name, "%s", msg)
				return
			}
		}
	}
	for _, p := range d.CtorParams {
		if p.IsProp {
			report(&p.Modifiers)
		}
	}
	for _, m := range d.Members {
		switch m := m.(type) {
		case *syntax.FunDecl:
			if !m.Generated {
				report(&m.Modifiers)
			}
		case *syntax.PropDecl:
			if !m.Generated {
				report(&m.Modifiers)
			}
		}
	}
}
