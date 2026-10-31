package check

import (
	"slices"
	"sort"
	"strconv"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// delegations declares the members that class delegation implements, as
// in class A(b: I) : I by b: each member of I and its superinterfaces
// that A doesn't declare itself becomes a method of A that calls the
// member on the delegate. As in kotlinc, the members of Kotlin interfaces
// are delegated whether abstract or not, and of Java interfaces only the
// abstract ones. It runs after signatures are resolved, before omitted
// types are inferred, which may call the delegated members.
func (c *Checker) delegations() {
	for _, d := range c.classes {
		cls := d.Sym
		this := cls.This()
		// The members each delegation supplies, by name and parameters,
		// to report one that two supply.
		from := map[string]*syntax.SuperType{}
		n := 0
		for _, st := range d.Supers {
			if st.Delegate == nil || st.T.K != types.Class || !st.T.C.Interface {
				continue
			}
			st.Field = "$$delegate_" + strconv.Itoa(n)
			if cp := delegateProp(d, st.Delegate); cp != nil {
				st.Field = cp.Name // kotlinc reads the property's field
			}
			n++
			var ms []*types.FunSym // the interface's methods, the most derived first
			st.T.C.Supertypes(func(s *types.ClassSym) bool {
				if s.Interface {
					for _, g := range append(delegable(s), delegableExts(s)...) {
						if !slicesContainsSame(c, this, ms, g) {
							ms = append(ms, g)
						}
					}
				}
				return true
			})
			for _, g := range ms {
				if c.ownMethod(d, this, g) {
					continue
				}
				key := g.Name + paramKey(g)
				if g.Ext {
					key = "ext " + key + " " + g.Recv.String()
				}
				if other := from[key]; other != nil && other != st {
					c.errorf(d.P, "class '%s' must override '%s' because it inherits multiple implementations for it", cls.DisplayName(), g.Name)
					continue
				}
				from[key] = st
				f := c.delegatedFun(d, st, g)
				if f.Ext {
					if cls.ExtMethods == nil {
						cls.ExtMethods = map[string][]*types.FunSym{}
					}
					cls.ExtMethods[f.Name] = append(cls.ExtMethods[f.Name], f)
					continue
				}
				cls.Methods[f.Name] = append(cls.Methods[f.Name], f)
			}
			c.delegatedProps(d, st, from)
			c.delegatedExtProps(d, st, from)
		}
	}
}

// defaultImpls declares the methods that a class needs to inherit the
// members with bodies of Kotlin interfaces on the class path. Those
// compiled without JVM default methods, as the standard library's are,
// have abstract methods, and their bodies are static methods of a class
// I$DefaultImpls, which take the instance first: as with kotlinc, the
// class gets a method that calls it, unless it or a superclass implements
// the member. So do the program's interfaces compiled without JVM default
// methods (-jvm-default=disable). A member that an interface closer to
// the class overrides without a body has none.
func (c *Checker) defaultImpls() {
	for _, d := range c.classes {
		cls := d.Sym
		if cls.Interface {
			// Also an interface with JVM default methods: as with kotlinc,
			// its DefaultImpls class calls the bodies it inherits from
			// interfaces without them, such as CoroutineContext.Element's.
			if cls.HasDefaultImpls() || cls.JVMDefault {
				c.forwarders(d)
			}
			continue
		}
		this := cls.This()
		abstract := map[string]bool{} // overridden without a body
		cls.Supertypes(func(s *types.ClassSym) bool {
			program := !s.External && s.HasDefaultImpls()
			if !program && (!s.External || !s.Interface || s.KotlinName == "" || s.Java != nil || s.JVMDefault) {
				// A JVMDefault interface's bodies are default methods,
				// which classes inherit, also if it has a DefaultImpls
				// class (-jvm-default=enable).
				return true
			}
			var impls *types.ClassSym
			for _, g := range delegable(s) {
				key := g.Name + paramKey(g)
				if g.Abstract {
					abstract[key] = true
					continue
				}
				if abstract[key] || c.implemented(cls, this, g) || forwardedAbove(cls, s) {
					continue
				}
				name := s.Name + "$DefaultImpls"
				if !program {
					if impls == nil {
						if impls = c.u.ext.class(name); impls == nil {
							return true
						}
						impls.Ensure()
					}
					if !hasImpl(impls, g) {
						continue
					}
				}
				f := c.delegatedFun(d, nil, g)
				f.Decl.(*syntax.Delegated).Impls = name
				cls.Methods[f.Name] = append(cls.Methods[f.Name], f)
			}
			// And its properties' accessors.
			for _, p := range memberProps(s) {
				key := "prop " + p.name
				if p.getter.Abstract {
					abstract[key] = true
					continue
				}
				if abstract[key] || c.propImplemented(cls, s, p.name) || forwardedAbove(cls, s) {
					continue
				}
				if !program && !c.inImpls(s, &impls, p.getter) {
					continue
				}
				c.delegateProp(d, s, nil, s.Name+"$DefaultImpls", p, c.memberSubst(this, s).Apply(p.t))
			}
			for _, p := range delegableExtProps(s) {
				recv := c.memberSubst(this, s).Apply(p.getter.Recv)
				key := "ext prop " + p.name + " " + recv.String()
				if p.getter.Abstract {
					abstract[key] = true
					continue
				}
				if abstract[key] || c.extPropImplemented(cls, this, s, p.name, recv) || forwardedAbove(cls, s) {
					continue
				}
				if !program && !c.inImpls(s, &impls, p.getter) {
					continue
				}
				c.delegateExtProp(d, nil, s.Name+"$DefaultImpls", p)
			}
			return true
		})
	}
}

// inImpls reports whether the DefaultImpls class of class-path interface
// s, which *impls caches, has the body of its member g.
func (c *Checker) inImpls(s *types.ClassSym, impls **types.ClassSym, g *types.FunSym) bool {
	if *impls == nil {
		if *impls = c.u.ext.class(s.Name + "$DefaultImpls"); *impls == nil {
			return false
		}
		(*impls).Ensure()
	}
	return hasImpl(*impls, g)
}

// propImplemented reports whether class cls has a property name that
// isn't abstract, declared in a class that inherits interface s's or in
// a superclass, as implemented does for methods. A superclass's call of
// the body of an interface that s extends doesn't count: s's overrides it.
func (c *Checker) propImplemented(cls, s *types.ClassSym, name string) bool {
	found := false
	cls.Supertypes(func(o *types.ClassSym) bool {
		if o == s || o.Interface && !o.IsSubclassOf(s) {
			return true
		}
		if p := o.Prop(name); p != nil {
			found = !p.Abstract && !overriddenBody(p.Decl, s)
		} else if o.External {
			o.Ensure()
			if kp := o.KProps[name]; kp != nil {
				found = !kp.Getter.Abstract
			}
		}
		return !found
	})
	return found
}

// overriddenBody reports whether decl is a call of the body of a member
// of an interface that interface s extends.
func overriddenBody(decl any, s *types.ClassSym) bool {
	dl, ok := decl.(*syntax.Delegated)
	return ok && dl.Impls != "" && dl.Target.Owner != s && s.IsSubclassOf(dl.Target.Owner)
}

// extPropImplemented reports whether class cls, whose type is this, has
// an extension property name of receiver type recv that isn't abstract,
// as propImplemented does other properties.
func (c *Checker) extPropImplemented(cls *types.ClassSym, this types.Type, s *types.ClassSym, name string, recv types.Type) bool {
	found := false
	cls.Supertypes(func(o *types.ClassSym) bool {
		if o == s || o.Interface && !o.IsSubclassOf(s) {
			return true
		}
		o.Ensure()
		for _, g := range o.ExtProps[name] {
			if !g.Abstract && !overriddenBody(g.Decl, s) && c.sameType(c.memberSubst(this, o).Apply(g.Recv), recv) {
				found = true
			}
		}
		return !found
	})
	return found
}

// forwardedAbove reports whether a superclass of class cls, a Kotlin
// class, implements interface s: it has the methods that call the bodies
// in s's DefaultImpls, which the metadata of one on the class path
// doesn't list, as AbstractCoroutineContextElement's get, fold, minusKey
// and plus, and which one of the program may not have been given yet.
func forwardedAbove(cls *types.ClassSym, s *types.ClassSym) bool {
	for sc := cls.Super; sc != nil; sc = sc.Super {
		if (!sc.External || sc.KotlinName != "") && sc.IsSubclassOf(s) {
			return true
		}
	}
	return false
}

// forwarders finds the members with bodies that interface d, compiled
// without JVM default methods, inherits from other such interfaces and
// doesn't override: its DefaultImpls class has static methods that call
// theirs.
func (c *Checker) forwarders(d *syntax.ClassDecl) {
	cls := d.Sym
	seen := map[string]bool{} // overridden, or forwarded
	this := cls.This()
	for _, g := range delegable(cls) {
		seen[g.Name+paramKey(g)] = true
	}
	for _, p := range memberProps(cls) {
		seen["prop "+p.name] = true
	}
	for _, p := range delegableExtProps(cls) {
		seen["ext prop "+p.name+" "+p.getter.Recv.String()] = true
	}
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s == cls || !s.HasDefaultImpls() {
			return true
		}
		name := s.Name + "$DefaultImpls"
		var impls *types.ClassSym
		if s.External {
			if impls = c.u.ext.class(name); impls == nil {
				return true
			}
			impls.Ensure()
		}
		forward := func(key string, gs ...*types.FunSym) {
			if seen[key] {
				return
			}
			seen[key] = true
			if gs[0].Abstract || impls != nil && !hasImpl(impls, gs[0]) {
				return
			}
			for _, g := range gs {
				if g != nil {
					d.Forwarders = append(d.Forwarders, syntax.ImplsForwarder{Fun: g, Of: s, Impls: name})
				}
			}
		}
		for _, g := range delegable(s) {
			forward(g.Name+paramKey(g), g)
		}
		for _, p := range memberProps(s) {
			forward("prop "+p.name, p.getter, p.setter)
		}
		for _, p := range delegableExtProps(s) {
			forward("ext prop "+p.name+" "+c.memberSubst(this, s).Apply(p.getter.Recv).String(), p.getter, p.getter.ExtSetter)
		}
		return true
	})
}

// noDefaultsChecks reports the members of interface d, compiled without
// JVM default methods, whose bodies kc can't move to its DefaultImpls
// class: private property accessors and suspend functions.
func (c *Checker) noDefaultsChecks(d *syntax.ClassDecl) {
	for _, m := range d.Members {
		switch m := m.(type) {
		case *syntax.PropDecl:
			if m.Visibility == syntax.Private && (m.Getter != nil || m.Setter != nil) {
				c.unsupported(m.P, "interface property body without default methods", "private property accessors with bodies in interfaces aren't supported with -jvm-default=disable")
			}
		case *syntax.FunDecl:
			if m.Suspend && (m.Body != nil || m.ExprBody != nil) {
				c.unsupported(m.P, "interface suspend body without default methods", "suspend functions with bodies in interfaces aren't supported with -jvm-default=disable")
			}
		}
	}
}

// hasImpl reports whether DefaultImpls class impls has the body of
// interface member g: a static method with its name that takes the
// instance and its parameters.
func hasImpl(impls *types.ClassSym, g *types.FunSym) bool {
	for _, h := range impls.Statics[g.JVM()] {
		if len(h.Params) == len(g.Params)+1 {
			return true
		}
	}
	return false
}

// delegateProp returns the constructor property that a delegate is, if
// it is a val: class A(val b: I) : I by b.
func delegateProp(d *syntax.ClassDecl, x syntax.Expr) *syntax.CtorParam {
	id, ok := x.(*syntax.Ident)
	if !ok {
		return nil
	}
	for _, cp := range d.CtorParams {
		if cp.Name == id.Name && cp.IsProp && !cp.Mutable {
			return cp
		}
	}
	return nil
}

// delegable returns the methods of interface s that delegation
// implements, in a stable order: declaration order for the program's
// interfaces.
func delegable(s *types.ClassSym) []*types.FunSym {
	var out []*types.FunSym
	if cd, ok := s.Decl.(*syntax.ClassDecl); ok && !s.External {
		for _, m := range cd.Members {
			if fd, ok := m.(*syntax.FunDecl); ok && fd.Recv == nil && fd.Sym != nil && fd.Visibility != syntax.Private {
				ensureFun(fd.Sym)
				out = append(out, fd.Sym)
			}
		}
		return out
	}
	s.Ensure()
	names := make([]string, 0, len(s.Methods))
	for name := range s.Methods {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, g := range s.Methods[name] {
			switch {
			case g.Static, g.Unsupported, g.Platform, isAnyMember(g):
			case s.KotlinName == "" && !g.Abstract: // a Java default method
			default:
				out = append(out, g)
			}
		}
	}
	return out
}

// delegableExts returns the extension functions declared in interface s
// that delegation implements, as delegable does its other methods.
func delegableExts(s *types.ClassSym) []*types.FunSym {
	var out []*types.FunSym
	if cd, ok := s.Decl.(*syntax.ClassDecl); ok && !s.External {
		for _, m := range cd.Members {
			if fd, ok := m.(*syntax.FunDecl); ok && fd.Recv != nil && fd.Sym != nil && fd.Visibility != syntax.Private {
				ensureFun(fd.Sym)
				out = append(out, fd.Sym)
			}
		}
		return out
	}
	s.Ensure()
	names := make([]string, 0, len(s.ExtMethods))
	for name := range s.ExtMethods {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, g := range s.ExtMethods[name] {
			if !g.Static && !g.Unsupported {
				out = append(out, g)
			}
		}
	}
	return out
}

// isAnyMember reports whether g is equals, hashCode or toString, which
// delegation leaves to the class.
func isAnyMember(g *types.FunSym) bool {
	switch g.Name {
	case "equals":
		return len(g.Params) == 1
	case "hashCode", "toString":
		return len(g.Params) == 0
	}
	return false
}

// slicesContainsSame reports whether ms has a method that g overrides or
// is overridden by: one with its name and parameters.
func slicesContainsSame(c *Checker, this types.Type, ms []*types.FunSym, g *types.FunSym) bool {
	for _, f := range ms {
		if f.Name == g.Name && c.sameParams(this, f, g) {
			return true
		}
	}
	return false
}

// ownMethod reports whether class d declares a method that overrides g.
func (c *Checker) ownMethod(d *syntax.ClassDecl, this types.Type, g *types.FunSym) bool {
	for _, f := range methodsNamed(d.Sym, g) {
		if fd, ok := f.Decl.(*syntax.FunDecl); ok && (fd.Recv != nil) == g.Ext && c.sameParams(this, f, g) {
			return true
		}
	}
	return false
}

// delegatedFun creates the method of class d that calls g, a member of a
// supertype delegated by st, with g's signature as d sees it.
func (c *Checker) delegatedFun(d *syntax.ClassDecl, st *syntax.SuperType, g *types.FunSym) *types.FunSym {
	cls := d.Sym
	if g.Composable {
		c.unsupported(d.P, "compose", "delegating composable function '%s' is not supported", g.Name)
	}
	sub := c.memberSubst(cls.This(), g.Owner)
	dl := &syntax.Delegated{Target: g, Super: st}
	f := &types.FunSym{Name: g.Name, JVMName: g.JVMName, TypeParams: g.TypeParams, Owner: cls, Decl: dl, Resolved: true,
		Ret: sub.Apply(g.Ret), Operator: g.Operator, Infix: g.Infix, Suspend: g.Suspend, Hidden: g.Hidden, Ext: g.Ext}
	if g.Ext {
		f.Recv = sub.Apply(g.Recv)
	}
	if g.Owner.Java != nil {
		// A Kotlin collection type's removeAt, Java's remove(int): as
		// with kotlinc, the class's member has the Kotlin name, which
		// codegen's collection bridges call.
		f.JVMName = ""
	}
	defaults := false
	for _, p := range g.Params {
		f.Params = append(f.Params, &types.VarSym{Name: p.Name, Type: sub.Apply(p.Type), Kind: types.Param, Resolved: true,
			Vararg: p.Vararg, Default: p.Default})
		defaults = defaults || p.Default
	}
	if defaults {
		// Omitted arguments go through the interface's name$default method.
		f.DefaultsOf = g
		if g.DefaultsOf != nil {
			f.DefaultsOf = g.DefaultsOf
		}
	}
	dl.Sym = f
	d.Delegated = append(d.Delegated, dl)
	c.delegatedBridges(cls, f)
	return f
}

// delegatedBridges records the bridges that a delegated method needs, as
// an override's: one for each overridden method whose erasure differs.
func (c *Checker) delegatedBridges(cls *types.ClassSym, f *types.FunSym) {
	own := types.InstanceJVM(f) + memberDesc(f)
	for _, o := range c.overridden(cls, f) {
		ensureFun(o) // its descriptor needs its return type
		if od := o.JVM() + memberDesc(o); od != own && !contains(f.Bridges, od) {
			f.Bridges = append(f.Bridges, od)
		}
	}
}

// delegatedProps declares the properties of the interface delegated by
// st that class d doesn't declare: each gets a getter, and a var a
// setter, that call the delegate's.
func (c *Checker) delegatedProps(d *syntax.ClassDecl, st *syntax.SuperType, from map[string]*syntax.SuperType) {
	cls := d.Sym
	this := cls.This()
	seen := map[string]bool{}
	st.T.C.Supertypes(func(s *types.ClassSym) bool {
		if !s.Interface {
			return true
		}
		for _, p := range memberProps(s) {
			if seen[p.name] {
				continue // overridden in a subinterface
			}
			seen[p.name] = true
			if own := cls.Prop(p.name); own != nil {
				if _, ok := own.Decl.(*syntax.Delegated); !ok {
					continue // declared by the class
				}
			}
			key := "prop " + p.name
			if other := from[key]; other != nil && other != st {
				c.errorf(d.P, "class '%s' must override '%s' because it inherits multiple implementations for it", cls.DisplayName(), p.name)
				continue
			}
			from[key] = st
			c.delegateProp(d, s, st, "", p, c.memberSubst(this, s).Apply(p.t))
		}
		return true
	})
}

// A memberProp is a property declared in a class, not an extension, by
// its name, type and accessors.
type memberProp struct {
	name           string
	t              types.Type
	getter, setter *types.FunSym
}

// memberProps returns the properties declared in interface s, the
// program's in declaration order.
func memberProps(s *types.ClassSym) []memberProp {
	var props []memberProp
	if cd, ok := s.Decl.(*syntax.ClassDecl); ok && !s.External {
		for _, m := range cd.Members {
			pd, ok := m.(*syntax.PropDecl)
			if !ok || pd.Recv != nil || pd.Sym == nil {
				continue
			}
			p := pd.Sym
			ensureVar(p)
			get := &types.FunSym{Name: types.GetterName(p.Name), Owner: s, Ret: p.Type, Resolved: true, Abstract: p.Abstract}
			var set *types.FunSym
			if p.Mutable {
				set = &types.FunSym{Name: types.SetterName(p.Name), Owner: s, Ret: types.UnitType, Resolved: true, Abstract: p.Abstract,
					Params: []*types.VarSym{{Name: "value", Type: p.Type, Kind: types.Param, Resolved: true}}}
			}
			props = append(props, memberProp{p.Name, p.Type, get, set})
		}
		return props
	}
	s.Ensure()
	names := make([]string, 0, len(s.KProps))
	for name := range s.KProps {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		kp := s.KProps[name]
		props = append(props, memberProp{name, kp.Getter.Ret, kp.Getter, kp.Setter})
	}
	return props
}

// delegateProp declares property p of interface s in class d, of type t
// there: its getter, and a var's setter, call the delegate's of st, or
// the bodies in DefaultImpls class impls.
func (c *Checker) delegateProp(d *syntax.ClassDecl, s *types.ClassSym, st *syntax.SuperType, impls string, p memberProp, t types.Type) {
	cls := d.Sym
	get := &types.FunSym{Name: p.getter.Name, JVMName: p.getter.JVMName, Owner: cls, Ret: t, Resolved: true}
	mapped := s.Java != nil // Kotlin's collection type: size is getSize()
	if mapped {
		get.Name, get.JVMName = types.GetterName(p.name), ""
	}
	dl := &syntax.Delegated{Sym: get, Target: p.getter, Super: st, Impls: impls}
	get.Decl = dl
	d.Delegated = append(d.Delegated, dl)
	// A collection type's size() is a bridge that codegen adds.
	if od := p.getter.JVM() + types.FunDesc(p.getter); od != types.InstanceJVM(get)+types.FunDesc(get) && !mapped {
		get.Bridges = append(get.Bridges, od)
	}
	v := &types.VarSym{Name: p.name, Type: t, Kind: types.Prop, Mutable: p.setter != nil, Owner: cls, Decl: dl,
		Resolved: true, Getter: get}
	cls.Props = append(cls.Props, v)
	if p.setter != nil {
		set := &types.FunSym{Name: p.setter.Name, JVMName: p.setter.JVMName, Owner: cls, Ret: types.UnitType, Resolved: true,
			Params: []*types.VarSym{{Name: "value", Type: t, Kind: types.Param, Resolved: true}}}
		sdl := &syntax.Delegated{Sym: set, Target: p.setter, Super: st, Impls: impls}
		set.Decl = sdl
		v.Setter = set // and no field
		d.Delegated = append(d.Delegated, sdl)
		if od := p.setter.JVM() + types.FunDesc(p.setter); od != types.InstanceJVM(set)+types.FunDesc(set) {
			set.Bridges = append(set.Bridges, od)
		}
	}
}

// delegatedExtProps declares the extension properties of the interface
// delegated by st that class d doesn't declare, as delegatedProps does
// its other properties: each gets a getter, and a var a setter, that call
// the delegate's, in the class's ExtProps.
func (c *Checker) delegatedExtProps(d *syntax.ClassDecl, st *syntax.SuperType, from map[string]*syntax.SuperType) {
	cls := d.Sym
	this := cls.This()
	recv := func(g *types.FunSym) types.Type { return c.memberSubst(this, g.Owner).Apply(g.Recv) }
	declared := func(gs []*types.FunSym, g *types.FunSym) bool {
		return slices.ContainsFunc(gs, func(f *types.FunSym) bool { return c.sameType(recv(f), recv(g)) })
	}
	var props []extProp // the most derived first
	seen := map[string][]*types.FunSym{}
	st.T.C.Supertypes(func(s *types.ClassSym) bool {
		if !s.Interface {
			return true
		}
		for _, p := range delegableExtProps(s) {
			if !declared(seen[p.name], p.getter) {
				seen[p.name] = append(seen[p.name], p.getter)
				props = append(props, p)
			}
		}
		return true
	})
	for _, p := range props {
		own := slices.DeleteFunc(slices.Clone(cls.ExtProps[p.name]), func(f *types.FunSym) bool {
			_, ok := f.Decl.(*syntax.Delegated)
			return ok
		})
		if declared(own, p.getter) {
			continue
		}
		key := "ext prop " + p.name + " " + recv(p.getter).String()
		if other := from[key]; other != nil && other != st {
			c.errorf(d.P, "class '%s' must override '%s' because it inherits multiple implementations for it", cls.DisplayName(), p.name)
			continue
		}
		from[key] = st
		c.delegateExtProp(d, st, "", p)
	}
}

// delegateExtProp declares extension property p of an interface in class
// d, as delegateProp does a property that isn't an extension, in the
// class's ExtProps.
func (c *Checker) delegateExtProp(d *syntax.ClassDecl, st *syntax.SuperType, impls string, p extProp) {
	cls := d.Sym
	get := c.delegatedFun(d, st, p.getter)
	dl := d.Delegated[len(d.Delegated)-1]
	dl.ExtProp, dl.Impls = p.name, impls
	if p.getter.ExtSetter != nil {
		get.ExtSetter = c.delegatedFun(d, st, p.getter.ExtSetter)
		dl := d.Delegated[len(d.Delegated)-1]
		dl.ExtProp, dl.Impls = p.name, impls
	}
	for _, o := range c.overriddenExtProps(cls, p.name, get) {
		if od := o.JVM() + memberDesc(o); od != get.JVM()+memberDesc(get) && !contains(get.Bridges, od) {
			get.Bridges = append(get.Bridges, od)
		}
		if set, ost := get.ExtSetter, o.ExtSetter; set != nil && ost != nil {
			if od := ost.JVM() + memberDesc(ost); od != set.JVM()+memberDesc(set) && !contains(set.Bridges, od) {
				set.Bridges = append(set.Bridges, od)
			}
		}
	}
	if cls.ExtProps == nil {
		cls.ExtProps = map[string][]*types.FunSym{}
	}
	cls.ExtProps[p.name] = append(cls.ExtProps[p.name], get)
}

// An extProp is an extension property declared in a class, by its name
// and getter.
type extProp struct {
	name   string
	getter *types.FunSym
}

// delegableExtProps returns the extension properties declared in
// interface s that delegation implements, as delegable does its methods.
func delegableExtProps(s *types.ClassSym) []extProp {
	var out []extProp
	if cd, ok := s.Decl.(*syntax.ClassDecl); ok && !s.External {
		for _, m := range cd.Members {
			if pd, ok := m.(*syntax.PropDecl); ok && pd.Recv != nil && pd.Getter != nil && pd.Getter.Sym != nil && pd.Visibility != syntax.Private {
				ensureFun(pd.Getter.Sym)
				out = append(out, extProp{pd.Name, pd.Getter.Sym})
			}
		}
		return out
	}
	s.Ensure()
	names := make([]string, 0, len(s.ExtProps))
	for name := range s.ExtProps {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, g := range s.ExtProps[name] {
			out = append(out, extProp{name, g})
		}
	}
	return out
}

// checkDelegates checks the delegates of a class's delegated interfaces:
// like the arguments of the superclass's constructor, they see the
// primary constructor's parameters but not the class's members.
func (c *Checker) checkDelegates(d *syntax.ClassDecl) {
	for _, st := range d.Supers {
		if st.Delegate == nil {
			continue
		}
		c.state = state{scope: c.ctorScope(d.Sym), tparams: d.Sym.TypeParams, staticCls: d.Sym}
		c.inAnon(d.Sym)
		want := st.T
		if want.K != types.Class || !want.C.Interface {
			want = noWant
		}
		t := c.value(st.Delegate, want)
		if want.K != types.Invalid {
			c.assignable(t, want, st.Delegate.Pos())
		}
	}
}
