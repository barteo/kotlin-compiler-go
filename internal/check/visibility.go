package check

import (
	"slices"
	"strings"

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
	if o := f.Owner; o != nil && o.CompanionOf != nil && !o.External && !c.classVisible(o) {
		// A member of a private companion object, outside its class.
		return false
	}
	if f.PackagePrivate {
		return c.samePackage(f.Owner)
	}
	if f.Protected && c.javaPackage(f.Owner) {
		return true
	}
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
	if p.PackagePrivate {
		// A Java field of package visibility, which its subclasses in
		// other packages don't see either.
		return c.samePackage(p.Owner)
	}
	if p.Protected && c.javaPackage(p.Owner) {
		return true
	}
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
	v := setterVis(p)
	return v == syntax.Public || c.canSee(v, p.Owner, p.Facade)
}

// setterVis returns the visibility of the setter of a property of the
// program: its modifier's, or, for an override without modifiers on it
// or its setter, the overridden setter's, as B5's set of override var
// attribute for A5's open var attribute protected set.
func setterVis(p *types.VarSym) syntax.Visibility {
	return setterVis1(p, 0)
}

func setterVis1(p *types.VarSym, depth int) syntax.Visibility {
	d, ok := p.Decl.(*syntax.PropDecl)
	if !ok || d.SetterVis != syntax.Public || d.SetterVisP.Line > 0 || !d.Override || explicitVisibility(p) || p.Owner == nil || depth > 20 {
		if ok {
			return d.SetterVis
		}
		return syntax.Public
	}
	v := syntax.Public
	p.Owner.Supertypes(func(s *types.ClassSym) bool {
		if s == p.Owner || s.External {
			return true
		}
		if q := s.Prop(p.Name); q != nil && q != p && q.Mutable {
			v = setterVis1(q, depth+1)
			return false
		}
		return true
	})
	return v
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
	if cls != nil && cls.External {
		if o := c.protectedOuter(cls); o != nil && !c.samePackage(cls) && !c.seesProtected(o) &&
			!(c.file != nil && c.file.SuppressInvisible) {
			return cls
		}
		return nil
	}
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
	if o := f.Owner; o != nil && o.CompanionOf != nil && !o.External && !c.classVisible(o) {
		c.errorf(p, "cannot access '%s': it is a member of a private companion object of '%s'", f.Name, o.CompanionOf.DisplayName())
		return
	}
	c.errorf(p, "cannot access '%s': it is %s", f.Name, visibilityOf(memberVisibility(f), f.Owner))
}

// invisibleProp reports a use of a property the code can't see.
func (c *Checker) invisibleProp(p syntax.Pos, v *types.VarSym) {
	vis, ok := propVisibility(v)
	if !ok {
		vis = syntax.Protected
	}
	if !ok && v.PackagePrivate {
		c.errorf(p, "cannot access '%s': it is package-private in '%s'", v.Name, v.Owner.DisplayName())
		return
	}
	c.errorf(p, "cannot access '%s': it is %s", v.Name, visibilityOf(vis, v.Owner))
}

// invisibleSetter reports an assignment of a property whose setter the
// code can't see.
func (c *Checker) invisibleSetter(p syntax.Pos, v *types.VarSym) {
	vis := setterVis(v)
	c.errorf(p, invisibleSetterMsg+"'%s': the setter is %s", v.Name, visibilityOf(vis, v.Owner))
}

// invisibleSetterMsg begins invisibleSetter's message.
const invisibleSetterMsg = "cannot assign to "

// checkClassVisible reports a use of a class of the program that the
// code can't see.
func (c *Checker) checkClassVisible(cls *types.ClassSym, p syntax.Pos) {
	if k := c.invisibleOuter(cls); k != nil {
		if k.External {
			c.errorf(p, "cannot access '%s': it is protected in '%s'", k.DisplayName(), c.protectedOuter(k).DisplayName())
			return
		}
		v, _ := declVisibility(k.Decl)
		c.errorf(p, "cannot access '%s': it is %s", k.DisplayName(), visibilityOf(v, k.Outer))
	}
}

// checkAliasVisible reports a use of a typealias of the program that the
// code can't see.
func (c *Checker) checkAliasVisible(a *types.AliasSym, p syntax.Pos) {
	if v, ok := declVisibility(a.Decl); ok && !c.canSee(v, a.Outer, c.aliasFileClass(a)) {
		c.errorf(p, "cannot access '%s': it is %s", a.Name, visibilityOf(v, a.Outer))
	}
}

// aliasFileClass returns the class of the file that declares typealias a.
func (c *Checker) aliasFileClass(a *types.AliasSym) string {
	if f := c.u.aliasFile[a]; f != nil {
		return f.file.Class()
	}
	return ""
}

// privateElsewhere reports whether k is a private top-level class or
// typealias of another file, which a simple name finds only if nothing
// else is found: then its use is an error.
func (c *Checker) privateElsewhere(k classifier) bool {
	if k.cls != nil {
		v, ok := declVisibility(k.cls.Decl)
		return ok && v == syntax.Private && k.cls.Outer == nil && !c.canSee(v, nil, k.cls.File)
	}
	v, ok := declVisibility(k.alias.Decl)
	return ok && v == syntax.Private && k.alias.Outer == nil && !c.canSee(v, nil, c.aliasFileClass(k.alias))
}

// checkPrivateToThis reports a use of a member of owner named name that
// is private to this on a receiver other than this: on another instance
// of its class, or on a lambda's receiver. x is the receiver, or nil for
// the implicit this of the class the code is in.
func (c *Checker) checkPrivateToThis(x syntax.Expr, name string, owner *types.ClassSym, p syntax.Pos) {
	switch x := x.(type) {
	case nil:
		return
	case *syntax.This:
		if c.classThis(x) {
			return
		}
	case *syntax.Ident:
		if slices.Contains(c.thisRefs, x.Sym) {
			return // the receiver of this::p
		}
	}
	if c.file.SuppressInvisible {
		return
	}
	c.errorf(p, "cannot access '%s': it is %s'%s'", name, privateToThisMsg, owner.DisplayName())
}

// privateToThisMsg is in checkPrivateToThis's message.
const privateToThisMsg = "private to this in "

// classThis reports whether this, checked, is the instance of the class
// the code is in or of an enclosing class, not a lambda's receiver.
func (c *Checker) classThis(x *syntax.This) bool {
	if x.Recv == nil {
		return true
	}
	r := c.receiverFor(x)
	return r != nil && r.class
}

// isEnumEntry reports whether name is an entry of enum class cls of the
// class path. An entry with a body has a class of its own, Foo$FOO, which
// isn't public, and which the name doesn't name: Foo.FOO is the entry.
func isEnumEntry(cls *types.ClassSym, name string) bool {
	cls.Ensure()
	return cls.Entries != nil && slices.Contains(cls.Entries, name)
}

// annotationScope returns the class whose nested classifiers the
// arguments of an annotation of a declaration in class owner see:
// @Ann(Nested::class) on a member or the constructor of Outer names
// Outer.Nested. A class's own annotations are outside it: they see its
// outer class's.
func annotationScope(owner, lookup *types.ClassSym, site annotSite) *types.ClassSym {
	if owner != nil && site.kind == siteClass {
		return owner.Outer
	}
	if owner != nil {
		return owner
	}
	return lookup
}

// protectedVia reports whether the code may use a protected member of
// owner on receiver x, by Kotlin's rule for protected members (Java's
// too): on an explicit receiver, the member is visible in a subclass K
// of owner only if the receiver is a K. Inside User : BaseClass,
// tree.ui of tree: BaseClass reads a field User can't, but this.ui
// can. The rule doesn't apply in owner's own package for a Java class,
// to a companion object's members, nor on this or super.
func (c *Checker) protectedVia(owner *types.ClassSym, x syntax.Expr) bool {
	if owner == nil || owner.CompanionOf != nil || owner.Object || c.javaPackage(owner) ||
		c.file != nil && c.file.SuppressInvisible {
		return true
	}
	switch x := x.(type) {
	case nil, *syntax.This, *syntax.Super:
		return true
	default:
		t := x.Type()
		if t.K == types.Invalid || t.IsIntersection() {
			return true
		}
		rc := c.classOf(t.NonNull())
		if rc == nil {
			return true
		}
		for k := c.visFirst(); k != nil; k = c.nextEnclosing(k) {
			if (k == owner || k.IsSubclassOf(owner) || owner.IsSubclassOf(k)) && (rc == k || rc.IsSubclassOf(k)) {
				return true
			}
			if k.Companion != nil && k.Companion.IsSubclassOf(owner) && (rc == k.Companion || rc.IsSubclassOf(k.Companion)) {
				return true
			}
		}
		return false
	}
}

// protectedFun reports whether f is a protected instance member, which
// protectedVia checks the receiver of.
func protectedFun(f *types.FunSym) bool {
	return !f.Static && !f.Ext && f.Owner != nil && memberVisibility(f) == syntax.Protected
}

// protectedMember is protectedFun for a property or a Java field.
func protectedMember(p *types.VarSym) bool {
	if p.Owner == nil || p.Kind != types.Prop {
		return false
	}
	v, ok := propVisibility(p)
	return ok && v == syntax.Protected || !ok && p.Protected
}

// javaSetterVisible reports whether the code may call setter, the setter
// of a Java synthetic property, on receiver x: J().x = 1 in a subclass
// of J doesn't call J's protected setX.
func (c *Checker) javaSetterVisible(setter *types.FunSym, x syntax.Expr) bool {
	if setter.Ext || setter.Owner == nil || !setter.Owner.External {
		return true
	}
	return c.funVisible(setter) && (!protectedFun(setter) || c.protectedVia(setter.Owner, x))
}

// protectedOuter returns, for a protected nested Java class, the class
// it is nested in, and nil for another class.
func (c *Checker) protectedOuter(cls *types.ClassSym) *types.ClassSym {
	if !cls.ProtectedNested {
		return nil
	}
	if cls.Outer != nil {
		return cls.Outer
	}
	i := strings.LastIndexByte(cls.Name, '$')
	if i < 0 {
		return nil
	}
	return c.u.ext.class(cls.Name[:i])
}

// companionProtected reports a use of a protected member of the
// companion object of class A, not @JvmStatic, in a subclass of A
// outside A, which the JVM forbids: the member is an instance member
// of A.Companion, which the subclass isn't.
func (c *Checker) companionProtected(owner *types.ClassSym, name string, jvmStatic bool, p syntax.Pos) {
	if owner == nil || owner.External || owner.CompanionOf == nil || jvmStatic || c.inside(owner.CompanionOf) {
		return
	}
	for k := c.visFirst(); k != nil; k = c.nextEnclosing(k) {
		if k.IsSubclassOf(owner) {
			return // the companion's own subclass, which sees it as its own
		}
	}
	c.errorf(p, "using non-JVM static members protected in the superclass companion is unsupported yet: '%s'", name)
}

// companionProtectedFun is companionProtected for a call of f.
func (c *Checker) companionProtectedFun(f *types.FunSym, p syntax.Pos) {
	if f.Owner == nil || f.Owner.CompanionOf == nil || memberVisibility(f) != syntax.Protected {
		return
	}
	d, _ := f.Decl.(*syntax.FunDecl)
	c.companionProtected(f.Owner, f.Name, d == nil || d.Find("", "JvmStatic") != nil, p)
}

// companionProtectedProp is companionProtected for a use of property v.
func (c *Checker) companionProtectedProp(v *types.VarSym, p syntax.Pos) {
	if v.Owner == nil || v.Owner.CompanionOf == nil || v.Kind != types.Prop || !protectedProp(v) {
		return
	}
	c.companionProtected(v.Owner, v.Name, v.JvmStatic || v.JvmField || v.Const, p)
}
