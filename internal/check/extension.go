package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Extension functions.
//
// A top-level extension function, fun R.f(...), is a top-level function
// with Ext set and its receiver type in Recv; its Params don't include the
// receiver, as for the class path's extensions. Its body has the receiver
// as an implicit receiver, as a lambda with a receiver does: a parameter
// $this$f, as kotlinc names it, which this and this@f mean, and whose
// members and extensions can be used unqualified. On the JVM, it is a
// static method of the file's facade that takes the receiver first.
//
// Calls find extensions through extCallees, after the receiver's members.
//
// An extension function declared in a class, fun R.f(), has two receivers:
// the extension receiver, an implicit receiver as above, and the class's
// this, which this@Class names. It is a method of the class, in ExtMethods
// rather than Methods, that takes the extension receiver first. Code in
// the class (or its subclasses) can call it, on its own this. One declared
// in an object can also be called where the object's members are in
// scope, in the class of a companion, or where it is imported: the object
// is then its dispatch receiver, Member.Dispatch. An extension property
// declared in a class, val R.x get() = ..., is the same, with its getter
// in ExtProps. In an interface, one without a getter is abstract, and
// class delegation implements it, as the interface's other members. In a
// value class, either is a static method that takes the value first.

// dropExtensions reports the extension properties declared in classes
// that kc doesn't support, and removes them, so that the rest of the
// checker doesn't see them: those annotated @JvmStatic.
func (c *Checker) dropExtensions() {
	c.file.Decls = c.dropExt(c.file.Decls, nil)
}

func (c *Checker) dropExt(decls []syntax.Decl, cls *syntax.ClassDecl) []syntax.Decl {
	member := cls != nil
	out := decls[:0]
	for _, d := range decls {
		switch d := d.(type) {
		case *syntax.PropDecl:
			if d.Recv != nil && member && d.Find("", "JvmStatic") != nil {
				c.unsupported(d.P, "@JvmStatic extension property", "@JvmStatic is not supported on extension properties")
				continue
			}
		case *syntax.ClassDecl:
			d.Members = c.dropExt(d.Members, d)
		}
		out = append(out, d)
	}
	return out
}

// checkInfix checks an infix function: a member or an extension with one
// parameter, which is not a vararg. It may have a default value.
func (c *Checker) checkInfix(d *syntax.FunDecl, owner *types.ClassSym) {
	switch {
	case owner == nil && d.Recv == nil:
		c.errorf(d.P, "'infix' modifier is inapplicable on this function: must be a member or an extension function")
	case len(d.Params) != 1 || d.Params[0].Vararg:
		c.errorf(d.P, "'infix' modifier is inapplicable on this function: must have a single value parameter")
	}
}

// funModifiers reports the modifiers only functions take on another
// declaration, what.
func (c *Checker) funModifiers(m syntax.Modifiers, p syntax.Pos, what string) {
	if m.External {
		c.errorf(m.ExternalP, "modifier 'external' is not applicable to '%s'", what)
	}
	if m.Operator {
		c.errorf(p, "modifier 'operator' is not applicable to '%s'", what)
	}
	if m.Infix {
		c.errorf(p, "modifier 'infix' is not applicable to '%s'", what)
	}
	if m.Annotation && what != "class" {
		c.errorf(p, "modifier 'annotation' is not applicable to '%s'", what)
	}
}

// declareRecv resolves the receiver type of an extension function, with
// the function's type parameters in scope, and declares its receiver.
// A local function declared again, as in a lambda that stubInference
// checks first, keeps its receiver's symbol, which the calls rewritten
// onto the receiver (see recvCall) refer to.
func (c *Checker) declareRecv(d *syntax.FunDecl) {
	sym := d.Sym
	sym.Ext = true
	sym.Recv = c.resolveType(d.Recv)
	recv := types.VarSym{Name: "$this$" + d.Name, Type: sym.Recv, Kind: types.Param, Decl: d}
	if d.RecvSym != nil {
		*d.RecvSym = recv
		return
	}
	d.RecvSym = &recv
}

// extRecvs returns the implicit receivers in the body of a function: an
// extension function's or extension property getter's receiver, labeled
// with the function's or the property's name, or none.
func extRecvs(d *syntax.FunDecl) []implicitRecv {
	if d.RecvSym == nil {
		return nil
	}
	return []implicitRecv{{sym: d.RecvSym, label: strings.TrimPrefix(d.RecvSym.Name, "$this$")}}
}

// Extension properties.
//
// An extension property, val R.x: T get() = ..., has no field: it is its
// getter, an extension function getX(R) of the facade whose symbol the
// package lists in ExtProps under the property's name. member finds it
// through extProp, as it finds the class path's.

// declareExtProp enters an extension property into its package.
func (c *Checker) declareExtProp(d *syntax.PropDecl) {
	d.Sym = &types.VarSym{Name: d.Name, Kind: types.Global, Mutable: d.Mutable, Facade: c.declFacade(d.Visibility), Decl: d}
	if g := d.Getter; g != nil {
		g.Sym = &types.FunSym{Name: types.GetterName(d.Name), JVMName: jvmName(&d.Modifiers, "get"), Facade: c.declFacade(d.Visibility), Decl: g, Ext: true, Final: true,
			Private: privateMethod(&d.Modifiers, nil), Module: c.partSuffix(d.Visibility), Hidden: d.Hidden()}
		c.pkg.ExtProps[d.Name] = append(c.pkg.ExtProps[d.Name], g.Sym)
		c.declareExtSetter(d, nil, "")
		c.composeGetter(d, d.Sym, nil)
	}
}

// declareExtSetter creates the symbol of a var extension property's
// setter, which its getter's symbol links to.
func (c *Checker) declareExtSetter(d *syntax.PropDecl, cls *types.ClassSym, module string) {
	st := d.Setter
	if st == nil {
		return
	}
	setter := d.Modifiers
	if d.SetterVis != syntax.Public {
		setter.Visibility = d.SetterVis
	}
	value := &types.VarSym{Name: st.Params[0].Name, Kind: types.Param, Decl: st.Params[0]}
	st.Params[0].Sym = value
	st.Visibility = d.SetterVis
	st.Sym = &types.FunSym{Name: types.SetterName(d.Name), JVMName: jvmName(&d.Modifiers, "set"), Owner: cls, Decl: st, Ext: true, Final: cls == nil || isFinal(d.Modifiers, cls),
		Abstract: d.Abstract && cls != nil, Module: module, Private: privateMethod(&setter, cls), Params: []*types.VarSym{value}, Ret: types.UnitType, Resolved: true,
		Hidden: d.Hidden()}
	if cls == nil {
		st.Sym.Facade = c.declFacade(setter.Visibility)
		st.Sym.Module = c.partSuffix(setter.Visibility)
	}
	d.Getter.Sym.ExtSetter = st.Sym
}

// resolveExtProp resolves an extension property's receiver and type. It
// must have a getter and can't have an initializer: there is no field.
func (c *Checker) resolveExtProp(d *syntax.PropDecl) {
	sym := d.Sym
	c.funModifiers(d.Modifiers, d.P, "property")
	switch {
	case d.Init != nil:
		c.errorf(d.Init.Pos(), "extension property cannot be initialized because it has no backing field")
	case d.Getter == nil, d.Mutable && d.Setter == nil:
		c.errorf(d.P, "extension property must have accessors or be abstract")
	}
	g := d.Getter
	if g == nil {
		sym.Type, sym.Resolved = types.InvalidType, true
		return
	}
	gs := g.Sym
	c.externalAccessors(d, gs.Owner)
	for _, tp := range d.TypeParams {
		switch {
		case !tp.Reified:
		case !d.Inline && !d.InlineGetter || d.Setter != nil && !d.Inline && !d.InlineSetter:
			c.errorf(tp.P, "only type parameters of inline functions can be reified")
		case gs.Owner != nil:
			c.unsupported(tp.P, "reified member property type parameter", "reified type parameters of member extension properties are not supported")
		default:
			// Reads copy the getter and writes the setter, as calls of
			// an inline function do.
			g.Inline = true
			if d.Setter != nil {
				d.Setter.Inline = true
			}
		}
	}
	gs.TypeParams = c.declareTypeParams(d.TypeParams, gs, false)
	c.tparams = funTypeParams(gs, gs.Owner)
	defer func() { c.tparams = nil }()
	c.resolveBounds(d.TypeParams)
	gs.Recv = c.resolveType(d.Recv)
	g.RecvSym = &types.VarSym{Name: "$this$" + d.Name, Type: gs.Recv, Kind: types.Param, Decl: g}
	sym.Getter = gs
	if st := d.Setter; st != nil && st.Sym != nil {
		ss := st.Sym
		ss.TypeParams, ss.Recv = gs.TypeParams, gs.Recv
		st.RecvSym = &types.VarSym{Name: "$this$" + d.Name, Type: gs.Recv, Kind: types.Param, Decl: st}
		sym.Setter = ss
	}
	switch {
	case d.Type != nil:
		sym.Type, sym.Resolved = c.resolveType(d.Type), true
		gs.Ret, gs.Resolved = sym.Type, true
		unitGetter(gs)
		if d.Setter != nil && d.Setter.Sym != nil {
			d.Setter.Sym.Params[0].Type = sym.Type
			c.setterParamType(d)
		}
	case g.ExprBody == nil:
		c.errorf(d.P, "this property must have a type, be initialized, or have a getter with an expression body")
		sym.Type, sym.Resolved = types.InvalidType, true
		gs.Ret, gs.Resolved = types.InvalidType, true
	default:
		sym.Infer = func() { c.inferProp(d) }
		gs.Infer = func() { ensureVar(sym) }
	}
	if g.Inline {
		c.inlineFun(g, nil)
		if st := d.Setter; st != nil && st.Sym != nil {
			c.inlineFun(st, nil)
		}
	}
}

// declareMemberExtProp enters an extension property declared in a class
// into the class's ExtProps, and resolves it.
func (c *Checker) declareMemberExtProp(d *syntax.PropDecl, cls *types.ClassSym) {
	if cls.Interface && d.Getter == nil && d.Setter == nil && d.Init == nil {
		d.Abstract = true // implicitly, as an interface's other members
	}
	d.Sym = &types.VarSym{Name: d.Name, Kind: types.Prop, Mutable: d.Mutable, Owner: cls, Decl: d, Abstract: d.Abstract}
	if d.Abstract && d.Getter == nil && d.Init == nil {
		// An abstract one's accessors have no bodies.
		d.Getter = &syntax.FunDecl{P: d.P, Name: "get", Property: d.Name, Ret: d.Type}
		if d.Mutable && d.Setter == nil {
			d.Setter = &syntax.FunDecl{P: d.P, Name: "set", Property: d.Name, Params: []*syntax.Param{{P: d.P, Name: "value"}}}
		}
	}
	if g := d.Getter; g != nil {
		if d.Visibility == syntax.Protected && !g.Explicit {
			g.Visibility = d.Visibility // for its method's access
		}
		final := isFinal(d.Modifiers, cls)
		g.Sym = &types.FunSym{Name: types.GetterName(d.Name), JVMName: jvmName(&d.Modifiers, "get"), Owner: cls, Decl: g, Ext: true, Final: final,
			Abstract: d.Abstract, Module: c.internal(&d.Modifiers, cls, ""), Private: privateMethod(&d.Modifiers, cls), Hidden: d.Hidden()}
		if cls.ExtProps == nil {
			cls.ExtProps = map[string][]*types.FunSym{}
			c.u.memberExts = true
		}
		cls.ExtProps[d.Name] = append(cls.ExtProps[d.Name], g.Sym)
		c.declareExtSetter(d, cls, g.Sym.Module)
		c.composeGetter(d, d.Sym, cls)
	}
	c.resolveExtProp(d)
}

// overriddenExtProps returns the getters of the extension properties of
// cls's supertypes that f, the getter of extension property name of cls,
// overrides: those with its name, whose receiver is its receiver's type.
func (c *Checker) overriddenExtProps(cls *types.ClassSym, name string, f *types.FunSym) []*types.FunSym {
	this := cls.This()
	var out []*types.FunSym
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s == cls {
			return true
		}
		for _, g := range s.ExtProps[name] {
			ensureFun(g)
			if r := c.memberSubst(this, g.Owner).Apply(g.Recv); c.sameType(r, f.Recv) {
				out = append(out, g)
			}
		}
		return true
	})
	return out
}

// checkExtPropOverrides checks the overrides among the extension
// properties declared in class d, and records the bridges they need: an
// override of val R.key, getKey(Object), is getKey(Map.Entry) too.
func (c *Checker) checkExtPropOverrides(d *syntax.ClassDecl) {
	for _, m := range d.Members {
		pd, ok := m.(*syntax.PropDecl)
		if !ok || pd.Recv == nil || pd.Getter == nil || pd.Getter.Sym == nil {
			continue
		}
		over := c.overriddenExtProps(d.Sym, pd.Name, pd.Getter.Sym)
		switch {
		case len(over) == 0 && pd.Override:
			c.errorf(pd.P, "'%s' overrides nothing", pd.Name)
		case len(over) > 0 && !pd.Override:
			c.errorf(pd.P, "'%s' hides member of supertype '%s' and needs 'override' modifier", pd.Name, over[0].Owner.DisplayName())
		}
		f := pd.Getter.Sym
		own := types.InstanceJVM(f) + memberDesc(f)
		for _, o := range over {
			if o.Final {
				c.errorf(pd.P, "'%s' in '%s' is final and cannot be overridden", pd.Name, o.Owner.DisplayName())
				continue
			}
			if od, ok := o.Decl.(*syntax.FunDecl); ok && od.Visibility == syntax.Protected && !pd.Explicit {
				f.Inherited = types.InheritProtected
			}
			if od := o.JVM() + memberDesc(o); od != own && !contains(f.Bridges, od) {
				f.Bridges = append(f.Bridges, od)
			}
			if st, ost := pd.Setter, o.ExtSetter; st != nil && st.Sym != nil && ost != nil {
				if od := ost.JVM() + memberDesc(ost); od != types.InstanceJVM(st.Sym)+memberDesc(st.Sym) && !contains(st.Sym.Bridges, od) {
					st.Sym.Bridges = append(st.Sym.Bridges, od)
				}
			}
		}
	}
}

// memberExtProps returns the getters of the extension properties named
// name that a class declares or inherits.
func memberExtProps(cls *types.ClassSym, name string) []*types.FunSym {
	var gs []*types.FunSym
	cls.Supertypes(func(s *types.ClassSym) bool {
		gs = append(gs, s.ExtProps[name]...)
		return true
	})
	return gs
}

// memberExtMethods returns the extension functions named name that a
// class declares or inherits.
func memberExtMethods(cls *types.ClassSym, name string) []*types.FunSym {
	var fs []*types.FunSym
	cls.Supertypes(func(s *types.ClassSym) bool {
		fs = append(fs, s.ExtMethods[name]...)
		return true
	})
	return dropOverridden(fs)
}

// dropOverridden drops the member extensions that one before them, of a
// subtype, overrides: they have the same receiver and parameters, and JVM
// name, which tells apart those for value classes of one underlying type.
func dropOverridden(fs []*types.FunSym) []*types.FunSym {
	seen := map[string]bool{}
	out := fs[:0:0]
	for _, f := range fs {
		d := types.FunDesc(f)
		if i := strings.IndexByte(d, ')'); i >= 0 {
			d = d[:i]
		}
		if r, ok := types.ParamDesc(f.Recv); ok && f.Desc == "" {
			d = r + d // the program's: its receiver isn't in its parameters
		}
		d = f.JVM() + d
		if seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, f)
	}
	return out
}

// nonExt returns the functions that aren't extensions.
func nonExt(fs []*types.FunSym) []*types.FunSym {
	if !hasExt(fs) {
		return fs
	}
	var out []*types.FunSym
	for _, f := range fs {
		if !f.Ext {
			out = append(out, f)
		}
	}
	return out
}

// hasExt reports whether some of functions are extensions.
func hasExt(fs []*types.FunSym) bool {
	for _, f := range fs {
		if f.Ext {
			return true
		}
	}
	return false
}

// thisExtProp checks a name that is nothing else as an extension property
// of the enclosing class's this: x is this.x.
func (c *Checker) thisExtProp(x *syntax.Ident) (types.Type, bool) {
	if c.cls == nil {
		return types.InvalidType, false
	}
	t := c.cls.This()
	if n, ok := c.narrowedThis(); ok {
		t = n
	}
	if _, ok := c.extProp(&syntax.Member{Name: x.Name}, t); !ok {
		return types.InvalidType, false
	}
	this := &syntax.This{}
	this.P = x.P
	m := &syntax.Member{X: this, Name: x.Name}
	m.P = x.P
	x.Member = m
	return c.expr(m, noWant), true
}
