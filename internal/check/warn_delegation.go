package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// delegationHidesWarnings reports DELEGATED_MEMBER_HIDES_SUPERTYPE_OVERRIDE
// for class d, as kotlinc: a member that delegation to an interface
// implements, where another supertype has an implementation of it that
// overrides the interface's (one of a class or interface that isn't the
// interface or its supertype). Delegation, not that override, wins. It is
// reported once, and only for members of the program's Kotlin classes.
func (c *Checker) delegationHidesWarnings(d *syntax.ClassDecl) {
	cls := d.Sym
	// hides reports whether, of the members over a delegated member of
	// interface iface overrides, as owners and whether they are abstract,
	// one that no other overrides is an implementation from elsewhere.
	// A final one is an error (OVERRIDING_FINAL_MEMBER_BY_DELEGATION).
	hides := func(iface *types.ClassSym, owners []*types.ClassSym, abstract, final []bool) bool {
		for i, o := range owners {
			if o.External || final[i] {
				return false
			}
			if abstract[i] || iface.IsSubclassOf(o) {
				continue
			}
			if !slices.ContainsFunc(owners, func(h *types.ClassSym) bool { return h != o && h.IsSubclassOf(o) }) {
				return true
			}
		}
		return false
	}
	report := func(kind, name string) {
		c.warnf(d.P, "DELEGATED_MEMBER_HIDES_SUPERTYPE_OVERRIDE", "Delegated member '%s %s' hides supertype override. Specify proper override explicitly.", kind, name)
	}
	for _, fs := range cls.Methods {
		for _, f := range fs {
			dl, ok := f.Decl.(*syntax.Delegated)
			if !ok || dl.Super == nil || dl.Super.T.K != types.Class || f.Ext || f.Static {
				continue
			}
			var owners []*types.ClassSym
			var abstract, final []bool
			for _, o := range c.overridden(cls, f) {
				owners = append(owners, o.Owner)
				abstract = append(abstract, o.Abstract)
				switch od := o.Decl.(type) {
				case *syntax.FunDecl:
					final = append(final, isFinal(od.Modifiers, o.Owner))
				case *syntax.Delegated:
					final = append(final, false)
				default:
					final = append(final, true) // unknown: report nothing
				}
			}
			if hides(dl.Super.T.C, owners, abstract, final) {
				report("fun", f.Name)
				return
			}
		}
	}
	for _, p := range cls.Props {
		dl, ok := p.Decl.(*syntax.Delegated)
		if !ok || dl.Super == nil || dl.Super.T.K != types.Class {
			continue
		}
		var owners []*types.ClassSym
		var abstract, final []bool
		for _, o := range c.overriddenProps(cls, p) {
			owners = append(owners, o.owner)
			abstract = append(abstract, o.abstract)
			final = append(final, o.final)
		}
		if hides(dl.Super.T.C, owners, abstract, final) {
			kind := "val"
			if p.Mutable {
				kind = "var"
			}
			report(kind, p.Name)
			return
		}
	}
}
