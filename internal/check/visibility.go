package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Visibility, as Kotlin checks it for the program's declarations. The
// class path's private and internal declarations aren't loaded (but in
// friend paths), and its protected members are marked Protected, so only
// the program's need their declarations' modifiers:
//
//   - A private member is visible in its class, the classes nested in it
//     and, for a companion object's member, the class the companion is
//     in. A private top-level declaration is visible in its file.
//   - A protected member is visible where a private one is, and in the
//     subclasses of its class (of the class a companion is in) and the
//     classes nested in them.
//   - Public and internal declarations are visible everywhere: the
//     program is one module.
//
// Lookups leave out the declarations the code can't see, so that a name
// resolves to what it would without them (an extension rather than a
// private member of the name), and fall back to them only if nothing
// else is found: then the reference is an error.

// declVisibility returns the visibility of a declaration of the program,
// and false for another (a local, a parameter, the class path's).
func declVisibility(decl any) (syntax.Visibility, bool) {
	switch d := decl.(type) {
	case *syntax.FunDecl:
		return d.Visibility, true
	case *syntax.PropDecl:
		return d.Visibility, true
	case *syntax.CtorParam:
		return d.Visibility, true
	case *syntax.ClassDecl:
		if d.Anonymous || d.Local {
			return syntax.Public, false
		}
		return d.Visibility, true
	case *syntax.TypeAliasDecl:
		return d.Visibility, true
	}
	return syntax.Public, false
}

// funVisibility returns the visibility of a function of the program,
// which an override without a visibility modifier inherits.
func funVisibility(f *types.FunSym) (syntax.Visibility, bool) {
	d, ok := f.Decl.(*syntax.FunDecl)
	if !ok {
		return syntax.Public, false
	}
	if d.AccessorOf != nil {
		// A getter has its property's visibility; a setter's is checked
		// with setterVisible.
		return propVisibility(d.AccessorOf)
	}
	if !d.Explicit && f.Inherited == types.InheritProtected {
		return syntax.Protected, true
	}
	return d.Visibility, true
}

// memberVisibility returns the visibility of any function: of the
// program's by its declaration, Public for the class path's but its
// protected members, whose private ones aren't loaded.
func memberVisibility(f *types.FunSym) syntax.Visibility {
	v, ok := funVisibility(f)
	switch {
	case ok:
		return v
	case f.Private && f.Owner != nil && !f.Owner.External && f.Name != "<init>":
		// Declared by the checker for a private property:
		// componentN() of a data class's private val.
		return syntax.Private
	case f.Protected:
		return syntax.Protected
	}
	return syntax.Public
}

// propVisibility returns the visibility of a property of the program.
func propVisibility(p *types.VarSym) (syntax.Visibility, bool) {
	v, ok := declVisibility(p.Decl)
	if !ok {
		return v, false
	}
	if p.InheritsProtected && !explicitVisibility(p) {
		return syntax.Protected, true
	}
	return v, true
}

// canSee reports whether the code being checked may use a declaration of
// the program of visibility v that is a member of owner, or top-level in
// the file whose class is file if owner is nil.
func (c *Checker) canSee(v syntax.Visibility, owner *types.ClassSym, file string) bool {
	if c.file != nil && c.file.SuppressInvisible {
		// Code under @Suppress("INVISIBLE_REFERENCE") sees everything,
		// as the class path's internal declarations.
		return true
	}
	switch v {
	case syntax.Private:
		if owner == nil {
			return file == "" || c.file == nil || file == c.file.Class()
		}
		return c.inside(owner)
	case syntax.Protected:
		return owner == nil || c.seesProtected(owner)
	}
	return true
}

// seesProtected reports whether the code may use the protected members
// of class owner: in it, a subclass or a class nested in one, or a class
// whose companion is a subclass, or, for a companion object's, the
// subclasses of the class it is in.
func (c *Checker) seesProtected(owner *types.ClassSym) bool {
	for k := c.visFirst(); k != nil; k = c.nextEnclosing(k) {
		if k == owner || owner.CompanionOf == k || k.IsSubclassOf(owner) || owner.CompanionOf != nil && k.IsSubclassOf(owner.CompanionOf) ||
			k.Companion != nil && k.Companion.IsSubclassOf(owner) {
			// The last: B's code calls the protected members its
			// companion inherits, B { companion object : A() }.
			return true
		}
	}
	return false
}

// seesOverridden reports whether the code may use a protected member
// named name of owner because it overrides one that the code sees: as
// in K2, A's code may call other.f() on other: B, B overriding A's
// protected f.
func (c *Checker) seesOverridden(owner *types.ClassSym, name string, fun bool) bool {
	found := false
	owner.Supertypes(func(s *types.ClassSym) bool {
		if s != owner && (fun && len(s.Methods[name]) > 0 || !fun && (s.Prop(name) != nil || s.KProps[name] != nil)) {
			found = c.seesProtected(s)
		}
		return !found
	})
	return found
}

// visFirst returns the innermost class the code is in, as
// firstEnclosing, or in the declaration phase, the class whose
// signatures or supertypes are being resolved.
func (c *Checker) visFirst() *types.ClassSym {
	if k := c.firstEnclosing(); k != nil {
		return k
	}
	if c.supersOf != nil {
		return c.supersOf
	}
	return c.lookupCls
}

// inside reports whether the code is in class cls or a class nested in
// it, or, for a companion object, in the class it is the companion of.
func (c *Checker) inside(cls *types.ClassSym) bool {
	for k := c.visFirst(); k != nil; k = c.nextEnclosing(k) {
		if k == cls || cls.CompanionOf == k {
			return true
		}
	}
	return false
}

// funVisible reports whether the code may call function f: one of the
// program's by its visibility, a protected one of the class path's in
// subclasses of its class.
func (c *Checker) funVisible(f *types.FunSym) bool {
	v := memberVisibility(f)
	if v == syntax.Public {
		return true
	}
	if c.canSee(v, f.Owner, f.Facade) {
		return true
	}
	return v == syntax.Protected && c.seesOverridden(f.Owner, f.Name, true)
}

// notInherited reports whether f is a private member of a supertype of
// class on, which on doesn't inherit: b.prv() for B : A, in A's code,
// doesn't call A's private prv, and in an object : A() in A, prv() is
// this@A.prv().
func notInherited(f *types.FunSym, on *types.ClassSym) bool {
	if on == nil || f.Owner == nil || f.Owner == on || f.Owner.External {
		return false
	}
	return memberVisibility(f) == syntax.Private
}

// declaredClass returns, for a variable x smart-cast to class cls, the
// class of its declared type, whose members it has too, or nil. As in
// K2, in A's code, a: A smart-cast to B has A's private members, but a:
// Any smart-cast to A, then B, doesn't.
func (c *Checker) declaredClass(x syntax.Expr, cls *types.ClassSym) *types.ClassSym {
	id, ok := x.(*syntax.Ident)
	if !ok || id.Sym == nil {
		return nil
	}
	if d := c.classOf(id.Sym.Type.NonNull()); d != cls {
		return d
	}
	return nil
}

// propNotInherited is notInherited for a property.
func propNotInherited(p *types.VarSym, on *types.ClassSym) bool {
	if on == nil || p.Owner == nil || p.Owner == on || p.Owner.External {
		return false
	}
	v, ok := propVisibility(p)
	return ok && v == syntax.Private
}

// propVisible reports whether the code may read property p.
func (c *Checker) propVisible(p *types.VarSym) bool {
	v, ok := propVisibility(p)
	if !ok {
		if !p.Protected {
			return true
		}
		v = syntax.Protected
	}
	if c.canSee(v, p.Owner, p.Facade) {
		return true
	}
	return v == syntax.Protected && c.seesOverridden(p.Owner, p.Name, false)
}

// setterVisible reports whether the code may assign property p: its
// setter's visibility, private set, is at most the property's.
func (c *Checker) setterVisible(p *types.VarSym) bool {
	if !c.propVisible(p) {
		return false
	}
	d, ok := p.Decl.(*syntax.PropDecl)
	if !ok || d.SetterVis == syntax.Public {
		return true
	}
	return c.canSee(d.SetterVis, p.Owner, p.Facade)
}

// classVisible reports whether the code may name class cls, and the
// classes it is nested in: Private.Public isn't visible outside the file
// of private class Private.
func (c *Checker) classVisible(cls *types.ClassSym) bool {
	return c.invisibleOuter(cls) == nil
}

// invisibleOuter returns cls, or the class it is nested in, that the code
// can't see, or nil.
func (c *Checker) invisibleOuter(cls *types.ClassSym) *types.ClassSym {
	for k := cls; k != nil && !k.External; k = k.Outer {
		v, ok := declVisibility(k.Decl)
		if !ok {
			return nil // a local class
		}
		if k.Outer == nil && !c.canSee(v, nil, k.File) || k.Outer != nil && !c.canSee(v, k.Outer, "") {
			return k
		}
	}
	return nil
}

// visibilityOf describes where a declaration of visibility v, a member
// of owner or top-level, is visible, for messages.
func visibilityOf(v syntax.Visibility, owner *types.ClassSym) string {
	what := "private"
	if v == syntax.Protected {
		what = "protected"
	}
	if owner == nil {
		return what + " in file"
	}
	return what + " in '" + owner.DisplayName() + "'"
}

// invisibleFun reports a call of a function the code can't see.
func (c *Checker) invisibleFun(p syntax.Pos, f *types.FunSym) {
	c.errorf(p, "cannot access '%s': it is %s", f.Name, visibilityOf(memberVisibility(f), f.Owner))
}

// invisibleProp reports a use of a property the code can't see.
func (c *Checker) invisibleProp(p syntax.Pos, v *types.VarSym) {
	vis, ok := propVisibility(v)
	if !ok {
		vis = syntax.Protected
	}
	c.errorf(p, "cannot access '%s': it is %s", v.Name, visibilityOf(vis, v.Owner))
}

// invisibleSetter reports an assignment of a property whose setter the
// code can't see.
func (c *Checker) invisibleSetter(p syntax.Pos, v *types.VarSym) {
	vis := v.Decl.(*syntax.PropDecl).SetterVis
	c.errorf(p, invisibleSetterMsg+"'%s': the setter is %s", v.Name, visibilityOf(vis, v.Owner))
}

// invisibleSetterMsg begins invisibleSetter's message.
const invisibleSetterMsg = "cannot assign to "

// checkClassVisible reports a use of a class of the program that the
// code can't see.
func (c *Checker) checkClassVisible(cls *types.ClassSym, p syntax.Pos) {
	if k := c.invisibleOuter(cls); k != nil {
		v, _ := declVisibility(k.Decl)
		c.errorf(p, "cannot access '%s': it is %s", k.DisplayName(), visibilityOf(v, k.Outer))
	}
}
