package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Inner classes.
//
// An inner class's instances hold their outer class's instance, in a
// field this$0, as kotlinc writes it. Its code sees the outer class's
// members and this@Outer as an object expression's sees the enclosing
// class's: as an implicit receiver after its own members, the capture
// this$0, which its constructor takes first. Its type parameters are its
// own, then its outer class's, so that Outer<Int>.Inner's members see the
// outer class's arguments.

// declareInner makes class sym, declared by d in class outer, an inner
// class.
func (c *Checker) declareInner(d *syntax.ClassDecl, sym, outer *types.ClassSym) {
	switch {
	case outer == nil:
		c.errorf(d.P, "modifier 'inner' is not applicable to 'top level class'")
		return
	case outer.Interface || outer.Object:
		c.errorf(d.P, "modifier 'inner' is not applicable inside an object or an interface")
		return
	case d.Interface || d.Object || d.Enum || d.Annotation:
		c.errorf(d.P, "modifier 'inner' is only applicable to classes")
		return
	}
	sym.Inner = true
	sym.OuterParams = len(outer.TypeParams)
	sym.TypeParams = append(sym.TypeParams[:len(sym.TypeParams):len(sym.TypeParams)], outer.TypeParams...)
	this := &types.VarSym{Name: "this$0", Kind: types.Local, Type: outer.This()}
	d.Captures, d.OuterThis = []*types.VarSym{this}, this
	name := outer.DisplayName()
	if c.anon == nil {
		c.anon = map[*types.ClassSym]*anonCtx{}
	}
	// An inner class of an inner class also sees the outer classes of its
	// outer class, through this$0.this$0.
	var recvs []implicitRecv
	if ctx := c.anon[outer]; ctx != nil && outer.Inner {
		// Each is read as the outer class reads it, from this$0.this$0
		// instead of its this$0.
		var rebase func(v *types.VarSym) *types.VarSym
		rebase = func(v *types.VarSym) *types.VarSym {
			if v.OuterOf == nil {
				return &types.VarSym{Name: v.Name, Kind: types.Local, Type: v.Type, OuterOf: this, OuterCls: outer}
			}
			return &types.VarSym{Name: v.Name, Kind: types.Local, Type: v.Type, OuterOf: rebase(v.OuterOf), OuterCls: v.OuterCls}
		}
		for _, r := range ctx.recvs {
			recvs = append(recvs, implicitRecv{sym: rebase(r.sym), label: r.label, outer: true, class: r.class})
		}
	}
	recvs = append(recvs, implicitRecv{sym: this, label: name[strings.LastIndexByte(name, '.')+1:], outer: true, class: true})
	c.anon[sym] = &anonCtx{scope: &scope{}, recvs: recvs}
}

// implicitOuter returns this for the innermost implicit receiver that is
// an instance of class outer, which Inner() passes to the constructor of
// an inner class of outer, checked, and its type; or nil if there is none.
func (c *Checker) implicitOuter(outer *types.ClassSym, p syntax.Pos) (syntax.Expr, types.Type) {
	own := func() (syntax.Expr, types.Type) {
		th := &syntax.This{Class: true} // not an extension's receiver
		th.P = p
		return th, c.value(th, noWant)
	}
	for i := len(c.recvs) - 1; i >= 0; i-- {
		r := &c.recvs[i]
		if r.outer && c.cls != nil && c.cls.IsSubclassOf(outer) {
			return own() // the class's own this comes before outer ones
		}
		if t := r.sym.Type.NonNull(); t.K == types.Class && t.C.IsSubclassOf(outer) {
			th := recvThis(r, p)
			return th, c.value(th, noWant)
		}
	}
	if c.cls != nil && c.cls.IsSubclassOf(outer) {
		return own()
	}
	return nil, types.InvalidType
}

// outerRecv returns the receiver this@label refers to around the code of
// an inner class, a local class or an object expression, if it is an
// outer class's this; or nil.
func (c *Checker) outerRecv(label string) *implicitRecv {
	for i := len(c.recvs) - 1; i >= 0; i-- {
		if r := &c.recvs[i]; r.outer && r.label == label && r.sym.Type.K == types.Class {
			return r
		}
	}
	return nil
}

// superOuter finds the outer instance of x, cls's call of its
// superclass's constructor, if the superclass is an inner class: the
// innermost implicit receiver that is an instance of its outer class, as
// for object : Inner() in the outer class's code. An inner class of that
// outer class passes its own, this$0. It reports false if there is none.
func (c *Checker) superOuter(cls *types.ClassSym, x *syntax.Call) bool {
	sc := x.Ctor
	if !sc.Inner || cls.Inner && cls.Outer.IsSubclassOf(sc.Outer) {
		return true
	}
	outer, _ := c.implicitOuter(sc.Outer, x.P)
	if outer == nil {
		c.errorf(x.P, "constructor of inner class %s can be called only with receiver of containing class", sc.DisplayName())
		c.args(x, nil)
		return false
	}
	x.Outer = outer
	return true
}

// innerConstruct checks a call of the constructor of inner class cls on
// outer, an instance of its outer class of type ot: outer.Inner(x), or
// Inner(x) in the outer class's code. The outer class's type arguments
// are ot's.
func (c *Checker) innerConstruct(x *syntax.Call, cls *types.ClassSym, outer syntax.Expr, ot types.Type, p syntax.Pos, want types.Type, sink *system) types.Type {
	x.Outer = outer
	sub := types.Subst{}
	if sup, ok := c.asSuper(ot, cls.Outer); ok {
		own := len(cls.TypeParams) - cls.OuterParams
		for i, tp := range cls.TypeParams[own:] {
			if i < sup.A.Len() {
				sub[tp] = sup.A.Get(i)
			}
		}
	}
	saved := c.outerSubst
	c.outerSubst = sub
	defer func() { c.outerSubst = saved }()
	return c.construct(x, cls, p, want, sink)
}

// innerOf returns the inner class name of program class cls or of one of
// its program superclasses, or nil: outer.Inner() for an outer whose
// superclass declares Inner.
func innerOf(cls *types.ClassSym, name string) *types.ClassSym {
	var in *types.ClassSym
	cls.Supertypes(func(s *types.ClassSym) bool {
		if n := s.Nested[name]; !s.External && n != nil && n.Inner {
			in = n
		}
		return in == nil
	})
	return in
}

// innerNamed returns the inner class name of program class cls or of one
// of its program superclasses, as innerOf does, or the one a typealias
// name expands to: outer.Alias() for typealias Alias = Outer.Inner. It
// returns the alias too, or nil.
func (c *Checker) innerNamed(cls *types.ClassSym, name string) (*types.ClassSym, *types.AliasSym) {
	if n := innerOf(cls, name); n != nil {
		return n, nil
	}
	a := c.lookupAlias(name)
	if a == nil {
		return nil, nil
	}
	if n := c.aliasClass(a); n != nil && !n.External && n.Inner && cls.IsSubclassOf(n.Outer) {
		return n, a
	}
	return nil, nil
}

// enclosingSubclass returns the innermost class whose code is being
// checked that is a subclass of outer, through inner classes, or nil.
func (c *Checker) enclosingSubclass(outer *types.ClassSym) *types.ClassSym {
	for _, k := range []*types.ClassSym{c.lookupCls, c.cls} {
		for ; k != nil; k = k.Outer {
			if k.IsSubclassOf(outer) {
				return k
			}
			if !k.Inner {
				break
			}
		}
	}
	return nil
}

// innerType resolves a type of an inner class of a generic class: its own
// type arguments, then the outer class's, of the type written before it,
// Outer<A>.Inner<B>, or in the outer class's code, its own type
// parameters.
func (c *Checker) innerType(ref *syntax.TypeRef) types.Type {
	cls := c.lookupClass(ref.Name)
	if cls == nil {
		c.errorf(ref.P, "unresolved or unsupported type: %s", ref.Name)
		return types.InvalidType
	}
	if !cls.Inner {
		c.errorf(ref.P, "type arguments are only allowed on an outer class of an inner class: %s", ref.Name)
		return types.InvalidType
	}
	own := len(cls.TypeParams) - cls.OuterParams
	args := make([]types.Arg, 0, len(cls.TypeParams))
	if len(ref.Args) != own {
		c.errorf(ref.P, "%s expected for class %s", countArgs(own), cls.DisplayName())
		return types.InvalidType
	}
	for _, a := range ref.Args {
		arg := types.Arg{V: a.Variance}
		if a.Variance != types.Star {
			if arg.T = c.resolveType(a.Type); arg.T.K == types.Invalid {
				return arg.T
			}
		}
		args = append(args, arg)
	}
	var outer types.Type
	if ref.Outer != nil {
		outer = c.resolveType(ref.Outer)
		if outer.K == types.Invalid {
			return outer
		}
	} else if k := c.enclosingSubclass(cls.Outer); k != nil && k != cls.Outer {
		// In a subclass's code, with the arguments of its supertype:
		// ClauseData in an UnbiasedSelectImplementation<R> :
		// SelectImplementation<R> is a SelectImplementation<R>.ClauseData.
		outer = k.This()
	} else {
		// In the outer class's code, where its type parameters are.
		for _, p := range cls.TypeParams[own:] {
			if c.lookupTypeParam(p.Name) == p {
				continue
			}
			if strings.Contains(ref.Name, ".") { // G.I for G<T>.I
				c.errorf(ref.P, "%s expected for class %s", countArgs(cls.OuterParams), cls.Outer.DisplayName())
			} else {
				c.errorf(ref.P, "type arguments of the outer class of %s are needed here: write %s<...>.%s", cls.DisplayName(), cls.Outer.DisplayName(), ref.Name)
			}
			return types.InvalidType
		}
		outer = cls.Outer.This()
	}
	sup, ok := c.asSuper(outer, cls.Outer)
	if !ok {
		c.errorf(ref.P, "%s is not an inner class of %s", ref.Name, outer)
		return types.InvalidType
	}
	for i := range cls.TypeParams[own:] {
		args = append(args, sup.A.Get(i))
	}
	return c.nullable(types.GenericType(cls, args), ref)
}
