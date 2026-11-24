package check

import "github.com/barteo/kotlin-compiler-go/internal/syntax"

// This file reports kotlinc's extra check REDUNDANT_MODALITY_MODIFIER
// (RedundantModalityModifierSyntaxChecker), with -Wextra: a modality
// modifier, final, open or abstract, that gives a declaration the
// modality it has without it. As kotlinc, it judges that by the syntax
// alone: a class is final, and an interface abstract (and open, which
// adds nothing to it); a member of an interface is open if it has a
// body and else abstract; an override is open in a class declared open,
// abstract or sealed, and else final; any other member is final.
// Top-level functions and properties, whose modality modifiers are
// errors, local classes, accessors and expect declarations aren't judged.

// modalityKeywords are the modality modifiers kotlinc judges.
var modalityKeywords = map[string]bool{"final": true, "open": true, "abstract": true}

// redundantModality reports the modality modifier of declaration d of
// class owner, or of the top level, if it is redundant.
func (c *Checker) redundantModality(d syntax.Decl, owner *syntax.ClassDecl) {
	if owner != nil && owner.Expect {
		return
	}
	switch d := d.(type) {
	case *syntax.ClassDecl:
		if d.Generated || d.Synthetic || d.EnumEntry || d.Anonymous || d.Expect {
			return
		}
		if d.Interface {
			c.modalityWarning(d.Written, "abstract", "open")
		} else {
			c.modalityWarning(d.Written, "final")
		}
	case *syntax.FunDecl:
		if owner == nil || d.Generated || d.Sym == nil || d.Expect || d.AccessorOf != nil {
			return
		}
		c.modalityWarning(d.Written, implicitModality(owner, d.Override, d.Body != nil || d.ExprBody != nil))
	case *syntax.PropDecl:
		if owner == nil || d.Generated || d.Plugin || d.Sym == nil || d.DelegateOf != nil || d.Expect {
			return
		}
		c.modalityWarning(d.Written, implicitModality(owner, d.Override, accessorBody(d.Getter) || accessorBody(d.Setter)))
	}
}

// ctorPropModality reports the modality modifier of property cp of class
// owner's primary constructor if it is redundant.
func (c *Checker) ctorPropModality(cp *syntax.CtorParam, owner *syntax.ClassDecl) {
	if cp.IsProp && !owner.Expect {
		c.modalityWarning(cp.Written, implicitModality(owner, cp.Override, false))
	}
}

// implicitModality returns the modality, as its keyword, of a member of
// class owner without a modality modifier, as kotlinc infers it from the
// syntax: an override, or a member with a body or without one.
func implicitModality(owner *syntax.ClassDecl, override, body bool) string {
	switch {
	case owner.Interface && body:
		return "open"
	case owner.Interface:
		return "abstract"
	case override && (owner.Open || owner.Abstract || owner.Sealed):
		return "open"
	}
	return "final"
}

// accessorBody reports whether accessor f, if any, has a body.
func accessorBody(f *syntax.FunDecl) bool {
	return f != nil && (f.Body != nil || f.ExprBody != nil)
}

// modalityWarning reports the modality modifier among written if it is
// one of implicit.
func (c *Checker) modalityWarning(written []syntax.Mod, implicit ...string) {
	for _, m := range written {
		if !modalityKeywords[m.Name] {
			continue
		}
		for _, k := range implicit {
			if m.Name == k {
				c.warnf(m.P, "REDUNDANT_MODALITY_MODIFIER", "redundant modality modifier")
			}
		}
		return
	}
}
