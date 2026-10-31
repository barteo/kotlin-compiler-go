package check

import (
	"fmt"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Delegated properties.
//
// The parser turns val x by d into a private property x$delegate = d and
// a property x whose getter calls x$delegate.getValue(this, ref), and for
// a var, whose setter calls x$delegate.setValue(this, ref, value): ref is
// x's KProperty, a PropRef. A local val x by d is a hidden local holding
// d; reading x calls getValue on it. The checker checks these like any
// code, then gives each PropRef its place in $$delegatedProperties.

// propRef checks a delegated property's KProperty, which is a
// KProperty<*>.
func (c *Checker) propRef(x *syntax.PropRef) types.Type {
	cls := c.u.ext.class("kotlin/reflect/KProperty")
	if cls == nil {
		c.errorf(x.P, "delegated properties need kotlin.reflect.KProperty: compile with the standard library")
		return types.InvalidType
	}
	return types.GenericType(cls, []types.Arg{{V: types.Star}})
}

// localDelegate checks a local delegated property, val x by d.
func (c *Checker) localDelegate(s *syntax.VarStmt) {
	c.localDelegates = true
	ref := &syntax.PropRef{Name: s.Name, Mutable: s.Mutable, Index: -1}
	ref.P = s.P
	want := c.kpropDelegateWant(s.Delegate)
	var t types.Type
	if s.Type != nil {
		t = c.resolveType(s.Type)
		if want.K == types.Invalid {
			// var x: T? by mutableStateOf(null) is a MutableState<T?>.
			want = c.delegateExprWant(s.Delegate, t, types.NothingType.Nullable())
		}
	}
	var dt types.Type
	provided := noWant
	if s.Type != nil && want.K == types.Invalid && t.K != types.Invalid {
		null := types.NothingType.Nullable()
		dt, provided = c.inferDelegate(s.Delegate, t, null, null, s.Mutable)
	} else {
		dt = c.value(s.Delegate, want)
	}
	if dt.K != types.Invalid {
		// The delegate is the result of its provideDelegate operator, if
		// it has one.
		null := &syntax.NullLit{}
		m := &syntax.Member{X: checked(s.Delegate, dt), Name: "provideDelegate"}
		call := &syntax.Call{Fun: m, Args: []syntax.Expr{null, ref}, Operator: true}
		null.P, m.P, call.P = s.P, s.P, s.P
		if c.hasFun(dt, "provideDelegate", call) {
			s.Delegate = call
			dt = c.value(call, provided)
		}
	}
	s.DelegateSym = &types.VarSym{Name: s.Name + "$delegate", Type: dt, Kind: types.Local, Decl: s}
	c.scope.declare(s.DelegateSym)
	id := &syntax.Ident{Name: s.DelegateSym.Name}
	null := &syntax.NullLit{}
	m := &syntax.Member{X: id, Name: "getValue"}
	s.Get = &syntax.Call{Fun: m, Args: []syntax.Expr{null, ref}}
	id.P, null.P, m.P, s.Get.P = s.P, s.P, s.P, s.P
	if s.Type != nil {
		c.assignable(c.value(s.Get, t), t, s.Delegate.Pos())
	} else {
		t = c.value(s.Get, noWant)
	}
	s.Sym = &types.VarSym{Name: s.Name, Type: t, Kind: types.Local, Mutable: s.Mutable, Decl: s, Delegate: s.DelegateSym}
	if s.Mutable && t.K != types.Invalid {
		// Assigning it calls setValue with the value, in a hidden local.
		s.ValueSym = &types.VarSym{Name: "<" + s.Name + "$value>", Type: t, Kind: types.Local}
		c.scope.declare(s.ValueSym)
		id := &syntax.Ident{Name: s.DelegateSym.Name}
		null := &syntax.NullLit{}
		v := &syntax.Ident{Name: s.ValueSym.Name}
		m := &syntax.Member{X: id, Name: "setValue"}
		s.Set = &syntax.Call{Fun: m, Args: []syntax.Expr{null, ref, v}}
		id.P, null.P, v.P, m.P, s.Set.P = s.P, s.P, s.P, s.P, s.P
		c.value(s.Set, noWant)
	}
	if !c.scope.declare(s.Sym) {
		c.errorf(s.P, "conflicting declarations: %s", s.Name)
	}
}

// inferDelegate checks delegate x of a property of type pt, whose
// provideDelegate takes a thisRef of type owner, and getValue and setValue
// one of type this, as kotlinc infers it: the type arguments that
// x leaves open, and those of its provideDelegate, are those for which
// getValue returns a pt, and, if the property is mutable, setValue takes
// one. Delegates.notNull() for a String is a ReadWriteProperty<Any?,
// String>. It returns x's type and what its provideDelegate returns, or
// noWant if it has none.
func (c *Checker) inferDelegate(x syntax.Expr, pt, owner, this types.Type, mutable bool) (types.Type, types.Type) {
	s := newSystem(c)
	c.sink = s
	t := c.value(x, noWant)
	// A constraint that can't hold is left out, for the operator calls'
	// own checks to report.
	try := func(a, b types.Type) {
		trial := s.clone()
		trial.sub(a, b)
		if trial.ok {
			s = trial
		}
	}
	pos := x.Pos()
	rt, provided := t, noWant
	if ps, r, ok := c.delegateOp(s, rt, "provideDelegate", pos); ok {
		try(owner, ps[0])
		rt, provided = r, r
	}
	if ps, r, ok := c.delegateOp(s, rt, "getValue", pos); ok {
		try(this, ps[0])
		try(r, pt)
	}
	if mutable {
		if ps, _, ok := c.delegateOp(s, rt, "setValue", pos); ok {
			try(this, ps[0])
			try(pt, ps[2])
		}
	}
	c.runLambdas(s, false)
	s.finish()
	t = s.final(t)
	x.SetType(t)
	if provided.K != types.Invalid {
		provided = s.final(provided)
	}
	return t, provided
}

// delegateOp returns the parameter and result types of operator name of a
// delegate of type rt, whose type arguments the variables of s may stand
// for, if it has one. The operator's own type parameters become
// variables of s which, as the call of it is checked on its own, need no
// value.
func (c *Checker) delegateOp(s *system, rt types.Type, name string, pos syntax.Pos) ([]types.Type, types.Type, bool) {
	if rt.K == types.Invalid {
		return nil, types.InvalidType, false
	}
	n := operatorArity[name]
	var fs []*types.FunSym
	if rt.K == types.Class {
		for _, f := range c.javaMethods(rt.C, name) {
			if ensureFun(f); len(f.Params) == n {
				fs = append(fs, f)
			}
		}
	}
	ext := len(fs) == 0
	if ext {
		for _, cl := range c.extCallees(name, rt) {
			if ensureFun(cl.f); len(cl.f.Params) == n && !cl.f.Recv.IsParam() {
				fs = append(fs, cl.f)
			}
		}
	}
	if len(fs) != 1 {
		return nil, types.InvalidType, false
	}
	f := fs[0]
	var base types.Subst
	if !ext {
		base = c.memberSubst(rt, f.Owner)
	}
	sub := s.fresh(f.TypeParams, base, pos)
	for _, p := range f.TypeParams {
		s.byVar[sub[p].T.Param()].star = true
	}
	if ext {
		trial := s.clone()
		if trial.sub(rt, sub.ApplyIn(f.Recv)); !trial.ok {
			return nil, types.InvalidType, false
		}
		*s = *trial
	}
	ps := make([]types.Type, len(f.Params))
	for i, p := range f.Params {
		ps[i] = sub.ApplyIn(p.Type)
	}
	return ps, sub.Apply(f.Ret), true
}

// kprops collects the KProperty objects of a class, or of a file's top
// level, in its $$delegatedProperties array.
type kprops struct {
	owner  string
	refs   []*syntax.PropRef
	locals int // the local delegated properties so far, which are <v#N>
}

// propRefs gives each delegated property's KProperty its place. As with
// kotlinc, a class's array holds its properties' and those of the locals
// in its functions, and the facade's the top-level ones and the locals
// in top-level functions, in order. Only the KProperty objects that the
// delegates use are created: when getValue and setValue are inline
// functions that don't read the parameter, as Lazy's getValue, null is
// passed instead.
func (c *Checker) propRefs() {
	k := &kprops{owner: c.file.Class()}
	for _, d := range c.file.Decls {
		c.declRefs(d, k)
	}
	c.file.KProps = k.refs
	for _, d := range c.file.Anon {
		c.declRefs(d, nil)
	}
}

func (c *Checker) declRefs(d syntax.Decl, k *kprops) {
	switch d := d.(type) {
	case *syntax.PropDecl:
		if r := d.Ref; r != nil && d.Sym != nil {
			r.Sig = types.GetterJVM(d.Sym) + "()" + descOf(d.Sym.Type)
			if g := d.Getter.Sym; r.Ext && g != nil {
				r.Sig = types.InstanceJVM(g) + memberDesc(g) // getX(LR;)T
			}
			calls := []syntax.Expr{d.Getter.ExprBody}
			if d.Setter != nil {
				calls = append(calls, d.Setter.ExprBody)
			}
			if f := d.Field; f != nil {
				if x, ok := f.Init.(*syntax.Call); ok && x.Operator && len(x.Args) == 2 && x.Args[1] == r {
					calls = append(calls, x) // provideDelegate(thisRef, property)
				}
			}
			k.use(r, calls...)
		}
		c.localRefs(d.Init, k)
		if d.Getter != nil && d.Ref == nil {
			c.localRefs(d.Getter.Body, k)
			c.localRefs(d.Getter.ExprBody, k)
		}
	case *syntax.FunDecl:
		for _, p := range d.Params {
			c.localRefs(p.Default, k)
		}
		c.localRefs(d.Body, k)
		c.localRefs(d.ExprBody, k)
	case *syntax.InitBlock:
		c.localRefs(d.Body, k)
	case *syntax.SecondaryCtor:
		c.localRefs(d.Body, k)
	case *syntax.ClassDecl:
		if d.Sym == nil {
			return
		}
		ck := &kprops{owner: d.Sym.Name}
		for _, m := range d.Members {
			c.declRefs(m, ck)
		}
		d.KProps = ck.refs
	}
}

// localRefs places the KProperty objects of the local delegated
// properties in a body or an expression.
func (c *Checker) localRefs(n any, k *kprops) {
	if n == nil || !c.localDelegates {
		return
	}
	syntax.Inspect(n, func(n any) bool {
		if vs, ok := n.(*syntax.VarStmt); ok && vs.Get != nil {
			r := vs.Get.Args[1].(*syntax.PropRef)
			r.Sig = fmt.Sprintf("<v#%d>", k.locals)
			k.locals++
			calls := []syntax.Expr{vs.Get}
			if vs.Set != nil {
				calls = append(calls, vs.Set)
			}
			if x, ok := vs.Delegate.(*syntax.Call); ok && x.Operator && len(x.Args) == 2 && x.Args[1] == r {
				calls = append(calls, x) // provideDelegate(null, property)
			}
			k.use(r, calls...)
		}
		return true
	})
}

// use places r in the array if one of the calls of getValue and setValue
// uses it.
func (k *kprops) use(r *syntax.PropRef, calls ...syntax.Expr) {
	for _, x := range calls {
		call, ok := x.(*syntax.Call)
		if !ok || call.Sym == nil {
			return // an error
		}
		if readsProperty(call.Sym) {
			r.Owner, r.Index = k.owner, len(k.refs)
			k.refs = append(k.refs, r)
			return
		}
	}
}

// readsProperty reports whether getValue or setValue f uses its
// KProperty parameter: unless it is an inline function of the class
// path whose body doesn't read it.
func readsProperty(f *types.FunSym) bool {
	if f.Inline == nil {
		return true
	}
	b, err := f.Inline.Body()
	if err != nil {
		return true
	}
	i := 1 // thisRef, then the property
	if f.Ext {
		i++ // after the receiver
	}
	if b.This {
		i++ // after the dispatch receiver
	}
	return i >= len(b.Params) || b.Reads(i)
}
