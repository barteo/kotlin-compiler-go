package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The shapes of callable reference arguments, which choose between
// overloads that the other arguments can't (byRefShape).
//
// K2 resolves a reference argument against each candidate of the call it
// is passed to, and drops the candidates for which none of the callables
// it may name fits: foo(::bar) for bar() and bar(x: Double) is
// foo(x: () -> Int), not foo(x: (String) -> Int), and property(O::p) is
// property(KProperty1<T, R>), not property(KProperty0<R>). kc gives a
// reference the types of the callables its name may name, when it can
// list them without checking it: those of the functions of the name,
// or of the property, at the top level, or of a class without type
// parameters, C::m.

// refShapes returns the types reference l may have, nil if kc can't tell.
func (c *Checker) refShapes(l *syntax.Lambda) []types.Type {
	r := l.Ref
	if r == nil || r.Type != nil || r.Bound != nil {
		return nil
	}
	if ft, ok := c.eagerRefType(l); ok {
		return []types.Type{ft}
	}
	if r.X == nil {
		return c.topRefShapes(r.Name)
	}
	id, ok := r.X.(*syntax.Ident)
	if !ok || c.isVar(id.Name) {
		return nil // a bound reference, x::m
	}
	cls := c.lookupClass(id.Name)
	if cls == nil || len(cls.TypeParams) > 0 || cls.Object {
		return nil
	}
	return c.memberRefShapes(cls, r.Name)
}

// topRefShapes returns the types of ::name, naming top-level functions or
// a top-level property.
func (c *Checker) topRefShapes(name string) []types.Type {
	if c.lookupClass(name) != nil || c.cls != nil && c.hasCallable(c.cls, name) {
		return nil
	}
	if _, _, ok := c.scope.lookup(name); ok {
		return nil
	}
	fs, _ := c.scope.findFuns(name)
	if len(fs) == 0 {
		fs = c.topFuns(name)
	}
	v := c.topVar(name)
	switch {
	case v != nil && len(fs) > 0:
		return nil
	case v != nil:
		ensureVar(v)
		if v.Type.K == types.Invalid || v.Type.HasParams() {
			return nil
		}
		if t, ok := c.kpropShape(v.Mutable, nil, v.Type); ok {
			return []types.Type{t}
		}
		return nil
	}
	var out []types.Type
	for _, f := range fs {
		t, ok := c.funShape(f, nil)
		if !ok {
			return nil
		}
		out = append(out, t)
	}
	return out
}

// memberRefShapes returns the types of C::name for a class cls.
func (c *Checker) memberRefShapes(cls *types.ClassSym, name string) []types.Type {
	recv := cls.This()
	fs := c.javaMethods(cls, name)
	p := c.memberProp(cls, name)
	switch {
	case p != nil && len(fs) > 0:
		return nil
	case p != nil:
		ensureVar(p)
		if p.Type.K == types.Invalid || p.Type.HasParams() {
			return nil
		}
		if t, ok := c.kpropShape(p.Mutable, &recv, p.Type); ok {
			return []types.Type{t}
		}
		return nil
	}
	var out []types.Type
	for _, f := range fs {
		t, ok := c.funShape(f, &recv)
		if !ok {
			return nil
		}
		out = append(out, t)
	}
	return out
}

// funShape returns the function type of a reference to f, a function
// without type parameters or a vararg, that takes recv first, if any.
func (c *Checker) funShape(f *types.FunSym, recv *types.Type) (types.Type, bool) {
	ensureFun(f)
	if f.Ext || len(f.TypeParams) > 0 || f.Ret.K == types.Invalid || f.Ret.HasParams() || len(f.Contexts) > 0 || f.Hidden || f.Unsupported {
		return types.InvalidType, false
	}
	var ps []types.Type
	if recv != nil {
		ps = append(ps, *recv)
	}
	for _, q := range f.Params {
		if q.Vararg || q.Type.HasParams() {
			return types.InvalidType, false
		}
		ps = append(ps, q.Type)
	}
	if f.Suspend {
		return c.u.ext.funcs.SuspendType(ps, f.Ret), true
	}
	return c.u.ext.funcs.Type(ps, f.Ret), true
}

// kpropShape returns the type of a reference to a property of type t, of
// an instance of recv, if any.
func (c *Checker) kpropShape(mutable bool, recv *types.Type, t types.Type) (types.Type, bool) {
	name := "kotlin/reflect/KProperty"
	if mutable {
		name = "kotlin/reflect/KMutableProperty"
	}
	args := []types.Arg{{T: t}}
	if recv != nil {
		name += "1"
		args = append([]types.Arg{{T: *recv}}, args...)
	} else {
		name += "0"
	}
	cls := c.u.ext.class(name)
	if cls == nil {
		return types.InvalidType, false
	}
	return types.GenericType(cls, args), true
}

// refFits reports whether a reference of type ft fits parameter type p in
// system s, whose variables stay within their bounds, and if so returns
// the system with it.
func (c *Checker) refFits(s *system, ft, p types.Type) (*system, bool) {
	trial := s.clone()
	pt := trial.apply(p)
	if _, r, ok := funcParts(pt); ok && r.K == types.Unit && !r.N {
		if ps, _, ok := funcParts(ft); ok {
			// A function's result fits a parameter whose result is Unit.
			if ft.Flex().IsSuspendFunc() {
				ft = c.u.ext.funcs.SuspendType(ps, r)
			} else {
				ft = c.u.ext.funcs.Type(ps, r)
			}
		}
	}
	if trial.sub(ft, pt); !trial.ok || !trial.withinBounds() {
		return nil, false
	}
	return trial, true
}

// onlyRefFit returns system s with the type of reference l, for
// parameter type p, if only one of the types it may have fits, or nil:
// T and R of bar(f: (T) -> Inv<R>) for bar(::foo), where only
// foo(x: Int): Inv<Int> gives an R within its bound Number.
func (c *Checker) onlyRefFit(s *system, l *syntax.Lambda, p types.Type) *system {
	ts := c.refShapes(l)
	if len(ts) == 0 || !s.apply(p).HasVars() {
		return nil
	}
	var fit *system
	for _, ft := range ts {
		if trial, ok := c.refFits(s, ft, p); ok {
			if fit != nil {
				return nil
			}
			fit = trial
		}
	}
	return fit
}
