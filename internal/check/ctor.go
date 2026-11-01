package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// userCtors returns the constructors of one of the program's classes: the
// primary constructor, if the class has one, then the secondary ones.
func userCtors(cls *types.ClassSym) []*types.FunSym {
	var out []*types.FunSym
	if cls.Ctor != nil {
		out = append(out, cls.Ctor)
	}
	return append(out, cls.Ctors...)
}

// ctorVisibility returns the visibility of a constructor of the program.
func ctorVisibility(f *types.FunSym) syntax.Visibility {
	switch d := f.Decl.(type) {
	case *syntax.ClassDecl:
		return d.CtorMods.Visibility
	case *syntax.SecondaryCtor:
		return d.Visibility
	}
	return syntax.Public
}

// visibleCtors returns the constructors of one of the program's classes
// that code here can call: a private or protected one only in the class,
// and the classes nested in it.
func (c *Checker) visibleCtors(cls *types.ClassSym) []*types.FunSym {
	all := userCtors(cls)
	for k := c.cls; k != nil; k = k.Outer {
		if k == cls {
			return all
		}
	}
	var out []*types.FunSym
	for _, f := range all {
		vis := ctorVisibility(f)
		// A protected constructor is only for the subclasses' super(...)
		// calls, which don't look here.
		if vis != syntax.Private && vis != syntax.Protected {
			out = append(out, f)
		}
	}
	return out
}

// resolveSecondaries declares a class's secondary constructors, with the
// errors that don't need their bodies.
func (c *Checker) resolveSecondaries(d *syntax.ClassDecl) {
	cls := d.Sym
	secs := d.Secondaries()
	if len(secs) == 0 {
		return
	}
	switch {
	case d.Interface:
		c.errorf(secs[0].P, "an interface may not have a constructor")
		return
	case d.Object:
		c.errorf(secs[0].P, "constructors are not allowed for objects")
		return
	}
	// The constructors by erased parameters, to report conflicts.
	seen := map[string]syntax.Pos{}
	reported := map[string]bool{}
	if cls.Ctor != nil {
		seen[paramKey(cls.Ctor)] = d.P
	}
	for _, sc := range secs {
		if sc.Serial != nil {
			continue // see serialFill
		}
		f := &types.FunSym{Name: "<init>", Owner: cls, Ret: cls.This(), Resolved: true, Decl: sc, Hidden: sc.Hidden(),
			Private: privateCtor(d, &sc.Modifiers)}
		sc.Sym = f
		if sc.External {
			c.errorf(sc.ExternalP, "modifier 'external' is not applicable to 'constructor'")
		}
		c.tparams = cls.TypeParams
		names := map[string]bool{}
		for _, p := range sc.Params {
			if names[p.Name] {
				c.errorf(p.P, "conflicting declarations: %s", p.Name)
			}
			names[p.Name] = true
			p.Sym = c.paramSym(p, c.resolveType(p.Type), p)
			f.Params = append(f.Params, p.Sym)
		}
		c.tparams = nil
		c.checkVarargs(sc.Params)
		if d.Enum && sc.Explicit && sc.Visibility != syntax.Private {
			c.errorf(sc.P, "constructor must be private in enum class")
		}
		if k := paramKey(f); seen[k] != (syntax.Pos{}) {
			if !reported[k] {
				reported[k] = true
				c.errorf(seen[k], "conflicting overloads: constructor%s", k)
			}
			c.errorf(sc.P, "conflicting overloads: constructor%s", k)
		} else {
			seen[k] = sc.P
		}
		cls.Ctors = append(cls.Ctors, f)
	}
}

// superCands returns the constructors of cls's superclass that its
// constructors can call.
func superCands(cls *types.ClassSym) []callee {
	sc := cls.Super
	var cands []callee
	if sc == nil {
		return nil // java.lang.Object, without a JDK
	}
	if !sc.External {
		for _, f := range userCtors(sc) {
			cands = append(cands, callee{f: f, tparams: sc.TypeParams})
		}
		return cands
	}
	for _, f := range sc.Ctors {
		cands = append(cands, callee{f: f, tparams: append(sc.TypeParams[:len(sc.TypeParams):len(sc.TypeParams)], f.TypeParams...)})
	}
	return cands
}

// checkSecondaries checks a class's secondary constructors: the
// constructor each one calls, default values and the body.
func (c *Checker) checkSecondaries(d *syntax.ClassDecl) {
	cls := d.Sym
	secs := d.Secondaries()
	for _, sc := range secs {
		if sc.Sym == nil || sc.Serial != nil {
			continue // reported by resolveSecondaries, or the serialization plugin's
		}
		c.checkDefaults(sc.Params, cls, sc.Sym)
		params := func() *scope {
			s := &scope{}
			for _, p := range sc.Sym.Params {
				s.declare(p)
			}
			return s
		}
		x := sc.Delegation
		if d.Enum && x != nil && !sc.ToThis {
			c.errorf(x.P, "call to super is not allowed in enum constructor")
		}
		switch {
		case d.HasPrimary && (x == nil || !sc.ToThis):
			c.errorf(sc.P, "primary constructor call expected")
			if x != nil {
				c.state = state{scope: params(), tparams: cls.TypeParams}
				c.args(x, nil)
			}
			sc.Delegation = nil
		case d.Enum && !sc.ToThis:
			// It calls Enum's constructor, with the entry's name and
			// ordinal.
			if x != nil {
				c.state = state{scope: params(), tparams: cls.TypeParams}
				c.args(x, nil)
			}
			sc.Delegation = nil
		case !sc.ToThis && cls.Super == nil:
			// Without a JDK, java.lang.Object has no class: the
			// constructor calls its constructor, which takes nothing.
			if x != nil && len(x.Args) > 0 {
				c.errorf(x.P, "too many arguments for public constructor Any()")
			}
			sc.Delegation = nil
		default:
			implicit := x == nil
			if implicit {
				id := &syntax.Ident{Name: "super"}
				id.P = sc.P
				x = &syntax.Call{Fun: id}
				x.P = sc.P
				sc.Delegation = x
			}
			c.state = state{scope: params(), tparams: cls.TypeParams, staticCls: cls}
			c.inAnon(cls)
			c.lookupCls = cls // its nested classes
			var cands []callee
			if sc.ToThis {
				x.Ctor = cls
				for _, f := range userCtors(cls) {
					if f != sc.Sym {
						cands = append(cands, callee{f: f})
					}
				}
			} else {
				x.Ctor = cls.Super
				cands = superCands(cls)
			}
			if implicit && !hasNoArgCtor(cands) {
				c.errorf(sc.P, "explicit 'this' or 'super' call is required: there is no constructor in the superclass that can be called without arguments")
				sc.Delegation = nil
				break
			}
			if len(cands) == 0 {
				c.errorf(x.P, "%s has no constructor that can be called here", x.Ctor.DisplayName())
				c.args(x, nil)
				sc.Delegation = nil
				break
			}
			if !sc.ToThis && !c.superOuter(cls, x) {
				sc.Delegation = nil
				break
			}
			// The superclass's type arguments are the supertype's:
			// super() for O<Comparable<*>> is O<Comparable<*>>'s.
			want := noWant
			if sup, ok := c.asSuper(cls.This(), cls.Super); ok && !sc.ToThis {
				want = sup
			}
			c.resolve(x, cands, want, nil)
			if !sc.ToThis && cls.Super.Local {
				c.localConstructed(cls.Super) // the class passes its superclass's captures
			}
		}
		c.lookupCls = nil
		if sc.Body != nil {
			c.state = state{scope: params(), cls: cls, this: newThis(cls), fn: sc.Sym, retType: types.UnitType, retAllowed: true,
				tparams: cls.TypeParams, init: true}
			c.inAnon(cls)
			c.block(sc.Body, noWant, false)
		}
	}
	// A constructor can't call itself through this(...), directly or not.
	for _, sc := range secs {
		seen := map[*syntax.SecondaryCtor]bool{}
		for t := sc; t != nil && t.ToThis && t.Delegation != nil && t.Delegation.Sym != nil; {
			if seen[t] {
				break
			}
			seen[t] = true
			next, _ := t.Delegation.Sym.Decl.(*syntax.SecondaryCtor)
			if next == sc {
				c.errorf(sc.P, "there's a cycle in the delegation calls chain")
				break
			}
			t = next
		}
	}
}

// hasNoArgCtor reports whether one of the constructors can be called
// without arguments.
func hasNoArgCtor(cands []callee) bool {
	for _, cl := range cands {
		ok := true
		for _, p := range cl.f.Params {
			ok = ok && (p.Default || p.Vararg)
		}
		if ok {
			return true
		}
	}
	return false
}
