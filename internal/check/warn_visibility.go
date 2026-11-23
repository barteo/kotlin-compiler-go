package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports kotlinc's extra check REDUNDANT_VISIBILITY_MODIFIER
// (RedundantVisibilityModifierSyntaxChecker), with -Wextra: a visibility
// modifier that gives a declaration the visibility it has without it,
// or one that a private or local class it is in hides. It is reported
// where that is certain: not on an override of a member whose visibility
// is itself inherited, on an override var whose modifier may widen a
// setter it inherits, or on an override property's accessors.

// visibilityKeywords are the visibility modifiers by keyword.
var visibilityKeywords = map[string]syntax.Visibility{
	"public": syntax.Public, "private": syntax.Private, "protected": syntax.Protected, "internal": syntax.Internal,
}

// writtenVisibility returns the visibility modifier among written, if
// there is one.
func writtenVisibility(written []syntax.Mod) (syntax.Mod, syntax.Visibility, bool) {
	for _, m := range written {
		if v, ok := visibilityKeywords[m.Name]; ok {
			return m, v, true
		}
	}
	return syntax.Mod{}, 0, false
}

// redundantVisibilities reports the redundant visibility and modality
// modifiers of the file's declarations (see redundantModality).
func (c *Checker) redundantVisibilities() {
	if !c.u.extraWarnings {
		return
	}
	done := map[*syntax.ClassDecl]bool{}
	var class func(d *syntax.ClassDecl)
	class = func(d *syntax.ClassDecl) {
		if d == nil || done[d] || d.Sym == nil || d.Generated || d.Synthetic || d.EnumEntry || d.Expect {
			return
		}
		done[d] = true
		for _, cp := range d.CtorParams {
			if cp.IsProp && !cp.Override {
				c.visibilityWarning(cp.Written, syntax.Public, d)
			}
			c.ctorPropModality(cp, d)
		}
		if !d.Interface && !d.Object && !d.Anonymous {
			c.visibilityWarning(d.CtorMods.Written, ctorVisibilityOf(d), d)
		}
		for _, m := range d.Members {
			c.redundantVisibility(m, d)
			c.redundantModality(m, d)
			c.redundantDeclTypes(m)
			if m, ok := m.(*syntax.ClassDecl); ok {
				class(m)
			}
		}
	}
	for _, d := range c.file.Decls {
		c.redundantVisibility(d, nil)
		c.redundantModality(d, nil)
		c.redundantDeclTypes(d)
		if d, ok := d.(*syntax.ClassDecl); ok {
			class(d)
		}
	}
	// The members of local classes and object expressions.
	walkCode(c.file.Decls, func(n any) bool {
		switch n := n.(type) {
		case *syntax.LocalClass:
			class(n.Decl)
		case *syntax.ObjectExpr:
			class(n.Decl)
		}
		return true
	})
}

// ctorVisibilityOf returns the visibility of class d's constructors
// without a modifier.
func ctorVisibilityOf(d *syntax.ClassDecl) syntax.Visibility {
	switch {
	case d.Sealed:
		return syntax.Protected
	case d.Enum:
		return syntax.Private
	}
	return syntax.Public
}

// redundantVisibility reports the visibility modifiers of declaration d
// of class owner, or of the top level, that are redundant.
func (c *Checker) redundantVisibility(d syntax.Decl, owner *syntax.ClassDecl) {
	switch d := d.(type) {
	case *syntax.FunDecl:
		if d.Generated || d.Sym == nil || d.Expect {
			return
		}
		implicit := syntax.Public
		if d.Override {
			if owner == nil || owner.Sym == nil {
				return
			}
			v, ok := c.overriddenVisibility(owner.Sym, d.Sym)
			if !ok {
				return
			}
			implicit = v
		}
		c.visibilityWarning(d.Written, implicit, owner)
	case *syntax.PropDecl:
		if d.Generated || d.Plugin || d.Sym == nil || d.DelegateOf != nil || d.Expect {
			return
		}
		if d.Override {
			// Without a modifier, it has the visibility of the
			// properties it overrides; its accessors aren't judged.
			if owner == nil || owner.Sym == nil || d.Recv != nil {
				return
			}
			if v, ok := c.overriddenPropVisibility(owner.Sym, d.Sym); ok {
				if _, set := writtenSetterVisibility(d); d.Sym.Mutable && !set && !c.overriddenSettersAre(owner.Sym, d.Sym, v) {
					// Its modifier is its setter's too, which it may
					// widen: public over a protected set.
					return
				}
				c.visibilityWarning(d.Written, v, owner)
			}
			return
		}
		vis := d.Visibility
		for _, mods := range d.AccessorMods {
			// An accessor has its property's visibility.
			m, v, ok := writtenVisibility(mods)
			if ok && v == vis && (v != syntax.Public || c.u.explicitAPI == APIDisabled) {
				c.warnf(m.P, "REDUNDANT_VISIBILITY_MODIFIER", "redundant visibility modifier")
			}
		}
		c.visibilityWarning(d.Written, syntax.Public, owner)
	case *syntax.SecondaryCtor:
		if owner != nil {
			c.visibilityWarning(d.Written, ctorVisibilityOf(owner), owner)
		}
	case *syntax.ClassDecl:
		if !d.Generated && !d.Synthetic && !d.EnumEntry && !d.Expect {
			c.visibilityWarning(d.Written, syntax.Public, owner)
		}
	case *syntax.TypeAliasDecl:
		c.visibilityWarning(d.Written, syntax.Public, owner)
	}
}

// visibilityWarning reports the visibility modifier among written of a
// declaration in class owner, or at the top level, if it is redundant:
// the declaration's visibility without one, implicit, or one that owner
// hides.
func (c *Checker) visibilityWarning(written []syntax.Mod, implicit syntax.Visibility, owner *syntax.ClassDecl) {
	m, v, ok := writtenVisibility(written)
	if !ok {
		return
	}
	if owner != nil && v != syntax.Protected && implicit != syntax.Protected {
		// Hidden by owner: v gives no more than the implicit visibility
		// does within owner's, which is lower than v.
		local, cv, known := containerVisibility(owner)
		switch {
		case local && v == syntax.Internal:
			c.warnf(m.P, "REDUNDANT_VISIBILITY_MODIFIER", "redundant visibility modifier")
			return
		case !local && known:
			d, ok := compareVisibility(v, cv)
			di, oki := compareVisibility(cv, implicit)
			if ok && d > 0 && oki && di <= 0 {
				c.warnf(m.P, "REDUNDANT_VISIBILITY_MODIFIER", "redundant visibility modifier")
				return
			}
		}
	}
	if c.u.explicitAPI != APIDisabled && v == syntax.Public {
		return // required in explicit API mode
	}
	if v == implicit {
		c.warnf(m.P, "REDUNDANT_VISIBILITY_MODIFIER", "redundant visibility modifier")
	}
}

// containerVisibility returns the effective visibility of the members of
// class d: local, of a local class or an object expression's, or the
// lowest of the visibilities of d and the classes it is nested in, if
// they are comparable.
func containerVisibility(d *syntax.ClassDecl) (local bool, v syntax.Visibility, known bool) {
	v, known = syntax.Public, true
	for k := d.Sym; k != nil; k = k.Outer {
		if k.Local || k.Anonymous {
			return true, 0, false
		}
		kd, ok := k.Decl.(*syntax.ClassDecl)
		if !ok {
			return false, 0, false
		}
		switch d, ok := compareVisibility(kd.Visibility, v); {
		case !ok:
			known = false
		case d < 0:
			v = kd.Visibility
		}
	}
	return false, v, known
}

// overriddenPropVisibility is overriddenVisibility of override property
// p of class cls: the highest visibility of the properties it overrides.
func (c *Checker) overriddenPropVisibility(cls *types.ClassSym, p *types.VarSym) (syntax.Visibility, bool) {
	v, found := syntax.Private, false
	for _, o := range c.overriddenProps(cls, p) {
		if o.inherits {
			// Its visibility is that of a property it overrides,
			// which is among the others.
			continue
		}
		found = true
		d, ok := compareVisibility(o.vis, v)
		if !ok {
			return 0, false
		}
		if d > 0 {
			v = o.vis
		}
	}
	return v, found
}

// overriddenSettersAre reports whether the setters of the properties
// that mutable property p of class cls overrides all have visibility v,
// which a setter without a modifier of its own inherits.
func (c *Checker) overriddenSettersAre(cls *types.ClassSym, p *types.VarSym, v syntax.Visibility) bool {
	over := c.overriddenProps(cls, p)
	for _, o := range over {
		if !o.mutable || slices.ContainsFunc(over, func(n superProp) bool { return n.owner != o.owner && n.owner.IsSubclassOf(o.owner) }) {
			// Overridden itself by another: only the nearest count.
			continue
		}
		sv := o.setterVis
		if o.inheritsSetter {
			// Unless its setter has a modifier, as public set, its
			// visibility is that of a setter it overrides.
			q := c.memberProp(o.owner, p.Name)
			if q == nil {
				return false
			}
			d, ok := q.Decl.(*syntax.PropDecl)
			if !ok {
				return false
			}
			w, set := writtenSetterVisibility(d)
			if !set {
				return false
			}
			sv = w
		}
		if sv != v {
			return false
		}
	}
	return true
}

// writtenSetterVisibility returns the visibility modifier written on the
// setter of property d, if it has one.
func writtenSetterVisibility(d *syntax.PropDecl) (syntax.Visibility, bool) {
	for i, mods := range d.AccessorMods {
		if i < len(d.AccessorSets) && d.AccessorSets[i] {
			if _, v, ok := writtenVisibility(mods); ok {
				return v, true
			}
		}
	}
	return 0, false
}

// overriddenVisibility returns the visibility override f of class cls has
// without a modifier: the highest of the members it overrides. ok is
// false if one of them inherits its own, or isn't Kotlin's or Java's
// plain visibility.
func (c *Checker) overriddenVisibility(cls *types.ClassSym, f *types.FunSym) (syntax.Visibility, bool) {
	over := c.overridden(cls, f)
	if len(over) == 0 {
		return 0, false
	}
	v := syntax.Private
	for _, o := range over {
		if o.PackagePrivate || inheritsVisibility(o.Decl) {
			return 0, false
		}
		ov := memberVisibility(o)
		d, ok := compareVisibility(ov, v)
		if !ok {
			return 0, false
		}
		if d > 0 {
			v = ov
		}
	}
	return v, true
}
