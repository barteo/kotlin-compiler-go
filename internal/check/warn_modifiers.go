package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// This file reports the warnings about modifiers that kotlinc's
// declaration checkers report: NON_FINAL_MEMBER_IN_FINAL_CLASS and
// NON_FINAL_MEMBER_IN_OBJECT (FirOpenMemberChecker),
// REDUNDANT_OPEN_IN_INTERFACE (FirMemberFunctionsChecker and
// FirMemberPropertiesChecker) and REDUNDANT_MODIFIER (of kotlinc's
// modifier compatibility table).

// openMembers reports, in final class d, the members written open, which
// no subclass can override: as NON_FINAL_MEMBER_IN_OBJECT in an object,
// NON_FINAL_MEMBER_IN_FINAL_CLASS otherwise (an enum entry's body and an
// object expression too), on the modifier. An enum class can have open
// members, which its entries override.
func (c *Checker) openMembers(d *syntax.ClassDecl) {
	if d.Interface {
		c.openInInterface(d)
		return
	}
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

// openInInterface reports, in interface d, the abstract members written
// open but not abstract, which are open anyway, as
// REDUNDANT_OPEN_IN_INTERFACE on the modifier: functions without a body
// and properties without an initializer, a delegate or accessors with
// bodies. An expect interface's members aren't checked.
func (c *Checker) openInInterface(d *syntax.ClassDecl) {
	if d.Expect || d.Generated {
		return
	}
	hasBody := func(f *syntax.FunDecl) bool { return f != nil && (f.Body != nil || f.ExprBody != nil) }
	for _, m := range d.Members {
		var mods *syntax.Modifiers
		switch m := m.(type) {
		case *syntax.FunDecl:
			if !m.Generated && !hasBody(m) {
				mods = &m.Modifiers
			}
		case *syntax.PropDecl:
			if !m.Generated && m.Init == nil && m.Field == nil && !hasBody(m.Getter) && !hasBody(m.Setter) {
				mods = &m.Modifiers
			}
		}
		if mods == nil || mods.Has("abstract") {
			continue
		}
		for _, w := range mods.Written {
			if w.Name == "open" {
				c.warnf(w.P, "REDUNDANT_OPEN_IN_INTERFACE", "modifier 'open' is redundant for abstract interface members")
				break
			}
		}
	}
}

// redundantModifiers reports, of the modifiers written on a declaration,
// one that another makes redundant, as REDUNDANT_MODIFIER: open with
// abstract, abstract with sealed.
func (c *Checker) redundantModifiers(written []syntax.Mod) {
	has := func(name string) bool {
		return slices.ContainsFunc(written, func(w syntax.Mod) bool { return w.Name == name })
	}
	for _, p := range [][2]string{{"abstract", "open"}, {"sealed", "abstract"}} {
		if !has(p[0]) {
			continue
		}
		for _, w := range written {
			if w.Name == p[1] {
				c.warnf(w.P, "REDUNDANT_MODIFIER", "modifier '%s' is redundant in presence of '%s'", p[1], p[0])
				break
			}
		}
	}
}

// uselessVarargs reports the parameters of anonymous function l written
// vararg, which it ignores, as USELESS_VARARG_ON_PARAMETER.
func uselessVarargs(c *Checker, l *syntax.Lambda) {
	for _, p := range l.Params {
		for _, w := range p.Mods {
			if w.Name == "vararg" {
				c.warnf(w.P, "USELESS_VARARG_ON_PARAMETER", "the 'vararg' modifier is ignored on parameters of anonymous functions")
				break
			}
		}
	}
}
