package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports the warnings of kotlinc's
// FirExtensionShadowedByMemberChecker about an extension that a member of
// its receiver's class always wins over, so that no call of it on a
// receiver of that class can call it: EXTENSION_SHADOWED_BY_MEMBER, and
// EXTENSION_FUNCTION_SHADOWED_BY_MEMBER_PROPERTY_WITH_INVOKE for a member
// property whose value's invoke operator wins over an extension
// function. As kotlinc, it judges the receiver's class's members as its
// own declarations give them, and its supertypes' as it inherits them,
// not by the receiver's type arguments. It stays silent where kc may not
// know the members as kotlinc does: those of Java classes and of
// Kotlin's built-in types, but Any's, and private and protected ones.

// shadowWarnings reports the extensions declared in the file, in its
// classes and, through localWarnings, in its functions that members
// shadow.
func (c *Checker) shadowWarnings() {
	for _, d := range c.file.Decls {
		c.extShadowed(d)
	}
	for _, cd := range c.classes {
		for _, m := range cd.Members {
			c.extShadowed(m)
		}
	}
}

// extShadowed reports extension d, a function or a property, if a member
// shadows it.
func (c *Checker) extShadowed(d syntax.Decl) {
	switch d := d.(type) {
	case *syntax.FunDecl:
		if d.Recv == nil || d.Sym == nil || !d.Sym.Ext || d.Generated || d.Name == "" {
			return
		}
		if !c.shadowable(&d.Modifiers, d.Sym.Recv) {
			return
		}
		c.extFunShadowed(d)
	case *syntax.PropDecl:
		if d.Recv == nil || d.Getter == nil || d.Getter.Sym == nil || d.Generated {
			return
		}
		if !c.shadowable(&d.Modifiers, d.Getter.Sym.Recv) {
			return
		}
		if _, ok := c.shadowingProp(d.Getter.Sym.Recv, d.Name); ok {
			c.warnf(d.P, "EXTENSION_SHADOWED_BY_MEMBER", "extension is shadowed by a member: %s", d.Name)
		}
	}
}

// shadowable reports whether an extension with modifiers m and receiver
// type recv may be shadowed: it isn't expect, actual or an override, nor
// annotated @HidesMembers, and its receiver is a class's type, not
// nullable, of the number of type arguments its class takes.
func (c *Checker) shadowable(m *syntax.Modifiers, recv types.Type) bool {
	if m.Expect || m.Actual || m.Override || m.Find("", "internal.HidesMembers") != nil || m.Find("", "HidesMembers") != nil {
		return false
	}
	if !certainCastType(recv) || types.MayBeNull(recv) || recv.IsParam() {
		return false
	}
	if recv.K == types.Class && (recv.C == nil || recv.A.Len() != len(recv.C.TypeParams)) {
		return false
	}
	return true
}

// shadowClass returns the class whose members kc judges for receiver
// type recv: one of the program's or a Kotlin one of the class path, but
// not one that is a Java class on the JVM, or nil.
func shadowClass(recv types.Type) *types.ClassSym {
	cls := recv.C
	if recv.K != types.Class || cls == nil {
		return nil
	}
	cls.Ensure()
	if cls.External && !cls.FromKotlin || cls.Java != nil {
		return nil
	}
	return cls
}

// ownType returns cls's type with its own type parameters as arguments,
// the type its members are declared on.
func ownType(cls *types.ClassSym) types.Type {
	args := make([]types.Arg, len(cls.TypeParams))
	for i, p := range cls.TypeParams {
		args[i] = types.Arg{T: types.ParamType(p)}
	}
	return types.GenericType(cls, args)
}

// shadowProp is a member property that shadows extensions: its type,
// the class that declares it, and whether it has context parameters.
type shadowProp struct {
	typ   types.Type
	owner *types.ClassSym
	ctx   bool
}

// shadowingProp returns a property of recv's class, declared or
// inherited, that is visible everywhere and isn't an extension: a
// property of name that an extension of recv can never be called over.
func (c *Checker) shadowingProp(recv types.Type, name string) (shadowProp, bool) {
	cls := shadowClass(recv)
	if cls == nil {
		return shadowProp{}, false
	}
	var found shadowProp
	ok := false
	cls.Supertypes(func(s *types.ClassSym) bool {
		switch {
		case s.Name == "java/lang/Enum" || s.Name == "kotlin/Enum":
			// An enum class's name and ordinal.
			if name == "name" || name == "ordinal" {
				found, ok = shadowProp{typ: types.StringType, owner: s}, true
				if name == "ordinal" {
					found.typ = types.IntType
				}
			}
		case s.External && !s.FromKotlin || s.Java != nil:
		case s.External:
			if p := s.KProps[name]; p != nil && p.Getter != nil && !p.Getter.Ext && !p.Getter.Hidden && !p.Getter.Protected && p.Getter.Module == "" {
				found, ok = shadowProp{typ: p.Getter.Ret, owner: s}, true
			}
		default:
			if p := s.Prop(name); p != nil && p.Kind == types.Prop && everywhereProp(p) {
				found, ok = shadowProp{typ: p.Type, owner: s, ctx: len(p.Contexts) > 0}, true
			}
		}
		return !ok
	})
	return found, ok
}

// everywhereProp reports whether property p of the program is public or
// internal and not deprecated with level HIDDEN.
func everywhereProp(p *types.VarSym) bool {
	v, ok := propVisibility(p)
	if !ok || v != syntax.Public && v != syntax.Internal || p.InheritsProtected {
		return false
	}
	switch d := p.Decl.(type) {
	case *syntax.PropDecl:
		return d.Recv == nil && !d.Hidden()
	case *syntax.CtorParam:
		return !d.Hidden()
	}
	return false
}

// extFunShadowed reports extension function d if a member function of
// its receiver's class shadows it, or else a member property whose value
// has an invoke operator that does.
func (c *Checker) extFunShadowed(d *syntax.FunDecl) {
	e := d.Sym
	ensureFun(e)
	if c.anyMemberShadows(e) {
		c.warnf(d.P, "EXTENSION_SHADOWED_BY_MEMBER", "extension is shadowed by a member: %s", d.Name)
		return
	}
	cls := shadowClass(e.Recv)
	if cls == nil {
		return
	}
	this := ownType(cls)
	for _, m := range c.inheritedFuns(cls, this, d.Name) {
		if c.funShadows(m.f, m.subst, e) {
			c.warnf(d.P, "EXTENSION_SHADOWED_BY_MEMBER", "extension is shadowed by a member: %s", d.Name)
			return
		}
	}
	p, ok := c.shadowingProp(e.Recv, d.Name)
	if !ok || p.ctx {
		return
	}
	pc := shadowClass(c.memberSubst(this, p.owner).Apply(p.typ))
	if pc == nil {
		return
	}
	for _, m := range c.inheritedFuns(pc, ownType(pc), "invoke") {
		if m.f.Operator && c.funShadows(m.f, m.subst, e) {
			c.warnf(d.P, "EXTENSION_FUNCTION_SHADOWED_BY_MEMBER_PROPERTY_WITH_INVOKE", "extension is shadowed by a member property %s with an invoke operator", d.Name)
			return
		}
	}
}

// memberFun is a member function, with the substitution of its owner's
// type parameters that the class it is a member of inherits it with.
type memberFun struct {
	f     *types.FunSym
	subst types.Subst
}

// inheritedFuns returns the member functions name of cls, whose type is
// this, declared or inherited, that are visible everywhere and not
// deprecated with level HIDDEN, of the program's classes and the Kotlin
// ones of the class path.
func (c *Checker) inheritedFuns(cls *types.ClassSym, this types.Type, name string) []memberFun {
	var out []memberFun
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s.External && !s.FromKotlin || s.Java != nil {
			return true
		}
		for _, f := range s.Methods[name] {
			if f.Static || f.Ext || f.Hidden || f.Unsupported || f.Protected || f.Private || f.PackagePrivate {
				continue
			}
			if v := memberVisibility(f); v != syntax.Public && v != syntax.Internal {
				continue
			}
			if s.External && f.Module != "" {
				continue // internal to another module
			}
			out = append(out, memberFun{f, c.memberSubst(this, s)})
		}
		return true
	})
	return out
}

// anyMemberShadows reports whether one of Any's members, which every
// class has, shadows extension function e: equals(other: Any?),
// hashCode() and toString().
func (c *Checker) anyMemberShadows(e *types.FunSym) bool {
	if len(e.TypeParams) > 0 || e.Infix || slices.ContainsFunc(e.Params, func(p *types.VarSym) bool { return p.Default || p.Vararg }) {
		return false
	}
	switch e.Name {
	case "equals":
		return len(e.Params) == 1 && e.Params[0].Name == "other" && certainCastType(e.Params[0].Type)
	case "hashCode", "toString":
		return len(e.Params) == 0 && !e.Operator
	}
	return false
}

// funShadows reports whether member function m, its owner's type
// parameters substituted by sub, shadows extension function e, as
// kotlinc's shadows decides: m takes the parameters e does, by name,
// default values and varargs, at least as generally, and needs no
// context parameter e doesn't have, and m is an operator and infix if e
// is.
func (c *Checker) funShadows(m *types.FunSym, sub types.Subst, e *types.FunSym) bool {
	ensureFun(m)
	switch {
	case m.Ext, len(m.Params) != len(e.Params), len(m.TypeParams) != len(e.TypeParams),
		e.Operator && !m.Operator, e.Infix && !m.Infix:
		return false
	}
	for i, ep := range e.Params {
		mp := m.Params[i]
		if ep.Vararg != mp.Vararg || ep.Default && !mp.Default || ep.Name != mp.Name || ep.Name == "" {
			return false
		}
		if !certainCastType(ep.Type) || hasInvalid(mp.Type, 0) || mp.Type.K == types.Invalid {
			return false
		}
	}
	sys := newSystem(c)
	vars := sys.fresh(m.TypeParams, sub, syntax.Pos{})
	for _, p := range m.TypeParams {
		v := vars[p].T
		for _, b := range p.Bounds {
			sys.sub(v, vars.ApplyIn(b))
		}
	}
	for i, ep := range e.Params {
		sys.sub(ep.Type, vars.ApplyIn(m.Params[i].Type))
	}
	if !sys.ok {
		return false
	}
	// Each of m's context parameters needs one of e's that is at least as
	// specific.
	for _, mc := range m.Contexts {
		mt := vars.ApplyIn(mc.Type)
		if mt.Has(func(t types.Type) bool { return t.K == types.TypeVar }) {
			return false
		}
		if !slices.ContainsFunc(e.Contexts, func(ec *types.VarSym) bool { return c.subtype(ec.Type, mt) }) {
			return false
		}
	}
	return true
}
