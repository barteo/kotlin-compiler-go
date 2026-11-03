package check

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Coroutines.
//
// A suspend function can only be called where the code can suspend: in a
// suspend function, or in a suspend lambda, the body of a lambda whose
// expected type is a suspend function type. A lambda inlined into its
// caller, which isn't crossinline, is part of the code around it.

// suspendScope returns the innermost lambda whose body is code of its
// own, or nil for the function being checked, whether that code can
// suspend, and whether an inlined lambda passed to synchronized is
// between the two: a suspension point there would hold the monitor.
func (c *Checker) suspendScope() (l *syntax.Lambda, ok, critical bool) {
	for i := len(c.lambdas) - 1; i >= c.base; i-- {
		l := c.lambdas[i]
		j := c.jumps[l]
		switch {
		case l.Suspend:
			return l, true, critical
		case l.Inline && !j.crossinline:
			critical = critical || j.callee == "synchronized"
			continue
		}
		return l, false, critical
	}
	return nil, c.fn != nil && c.fn.Suspend, critical
}

// suspendCall checks a call at p of f, if it is a suspend function.
func (c *Checker) suspendCall(p syntax.Pos, f *types.FunSym) {
	if f == nil || !f.Suspend {
		return
	}
	_, ok, critical := c.suspendScope()
	switch {
	case !ok:
		c.errorf(p, "suspend function '%s' can only be called from a coroutine or another suspend function", f.Name)
	case critical:
		c.errorf(p, "the '%s' suspension point is inside a critical section", f.Name)
	}
}

// suspendFunCall checks call x of f, if it is a suspend function: as
// suspendCall, and as restrictedCall.
func (c *Checker) suspendFunCall(x *syntax.Call, f *types.FunSym) {
	c.suspendCall(x.P, f)
	if f != nil && f.Suspend && !c.restrictedCall(x, f) {
		c.errorf(x.P, "restricted suspending functions can invoke member or extension suspending functions only on their restricted coroutine scope")
	}
}

// Restricted suspension: in a suspend function or lambda with a receiver
// or a context parameter of a class annotated @RestrictsSuspension, a
// suspend call must pass that value, the same instance, as its dispatch
// receiver or as an extension receiver or a context argument of the
// restricted type; and a value of a restricted type a call passes must
// be the enclosing function's own, as kotlinc's FirSuspendCallChecker.

// suspendRecv is a value a call passes as a receiver or a context
// argument: of type t, and the enclosing function's this (sym nil) or
// receiver or context parameter sym if it is one.
type suspendRecv struct {
	ok, this bool
	sym      *types.VarSym
	t        types.Type
	decl     types.Type // the parameter's type, or Invalid
}

// recvOf returns the suspendRecv of x, passed where decl is declared.
func (c *Checker) recvOf(x syntax.Expr, decl types.Type) suspendRecv {
	r := suspendRecv{ok: true, t: x.Type(), decl: decl}
	switch x := x.(type) {
	case *syntax.This:
		r.this, r.sym = x.Label == "" || x.Recv != nil, x.Recv
		if x.Recv == nil && x.Label != "" && c.cls != nil {
			// this@C of the class the code is in.
			name := c.cls.DisplayName()
			r.this = x.Label == name[strings.LastIndexByte(name, '.')+1:]
		}
		if r.t.K == types.Invalid && x.Recv != nil {
			r.t = x.Recv.Type
		}
	case *syntax.Ident:
		if x.Sym != nil && x.Member == nil {
			r.sym = x.Sym // a named context parameter
		}
	}
	return r
}

// restricted reports whether r is of a restricted type.
func (r suspendRecv) restricted() bool {
	return r.ok && (restrictedClass(r.t) != nil || restrictedClass(r.decl) != nil)
}

// suspendRecvs returns the values call x of f passes as its dispatch
// receiver, its extension receiver and its context arguments, and false
// if it calls a function value with a receiver whose receiver is passed
// as an argument, f(r), which kotlinc doesn't check.
func (c *Checker) suspendRecvs(x *syntax.Call, f *types.FunSym) (disp suspendRecv, ext suspendRecv, ctx []suspendRecv, ok bool) {
	if x.Invoke {
		ft := x.Fun.Type().Flex().NonNull()
		k := ft.Contexts()
		if !ft.IsFunc() || !ft.IsExtFunc() && k == 0 {
			return disp, ext, nil, true
		}
		for i := range min(k, len(x.Args)) {
			ctx = append(ctx, c.recvOf(x.Args[i], ft.A.Get(i).T))
		}
		if ft.IsExtFunc() && k < len(x.Args) {
			a := x.Args[k]
			if th, _ := a.(*syntax.This); (th == nil || !th.Implicit) && !x.Recv && !posBefore(a.Pos(), x.Fun.Pos()) {
				return disp, ext, nil, false // f(r), not r.f()
			}
			ext = c.recvOf(a, ft.A.Get(k).T)
		}
		return disp, ext, ctx, true
	}
	if !f.Static && f.Name != "<init>" {
		var owner types.Type
		if f.Owner != nil {
			owner = f.Owner.This()
		}
		switch fn := x.Fun.(type) {
		case *syntax.Member:
			if !f.Ext {
				disp = c.recvOf(fn.X, owner)
				break
			}
			ext = c.recvOf(fn.X, f.Recv)
			switch {
			case f.Owner == nil:
			case fn.DispatchThis != nil:
				disp = c.recvOf(fn.DispatchThis, owner)
			case fn.Dispatch != nil:
				disp = suspendRecv{ok: true, t: fn.Dispatch.This(), decl: owner}
			case c.cls != nil && c.cls.IsSubclassOf(f.Owner):
				disp = suspendRecv{ok: true, this: true, t: c.cls.This(), decl: owner}
			}
		case *syntax.Ident:
			// Implicit receivers the call doesn't name: a member's, or a
			// member extension's two.
			if f.Ext {
				ext = c.implicitRecvOf(f.Recv)
			}
			if f.Owner != nil {
				disp = c.implicitRecvOf(owner)
			}
		}
	}
	for i, a := range x.Ctx {
		var decl types.Type
		if i < len(f.Contexts) {
			decl = f.Contexts[i].Type
		}
		ctx = append(ctx, c.recvOf(a, decl))
	}
	return disp, ext, ctx, true
}

// implicitRecvOf returns the nearest implicit receiver of class t: a
// lambda's or a function's, or the class's this.
func (c *Checker) implicitRecvOf(t types.Type) suspendRecv {
	t = t.Flex().NonNull()
	if t.K != types.Class {
		return suspendRecv{}
	}
	for i := len(c.recvs) - 1; i >= 0; i-- {
		if rt := c.scope.narrowed(c.recvs[i].sym); rt.K == types.Class && rt.C.IsSubclassOf(t.C) {
			return suspendRecv{ok: true, this: true, sym: c.recvs[i].sym, t: rt, decl: t}
		}
	}
	if c.cls != nil && c.cls.IsSubclassOf(t.C) {
		return suspendRecv{ok: true, this: true, t: c.cls.This(), decl: t}
	}
	return suspendRecv{}
}

// restrictedCall reports whether suspend call x of f may be made in the
// innermost suspend function or lambda around it.
func (c *Checker) restrictedCall(x *syntax.Call, f *types.FunSym) bool {
	l, ok, _ := c.suspendScope()
	if !ok || l != nil && l.Ref != nil {
		return true // a callable reference, ::f, isn't a call
	}
	// The enclosing function's this, receiver and context parameters.
	var member bool
	var recv *types.VarSym
	var ctxs []*types.VarSym
	if l != nil {
		recv, ctxs = l.Recv, l.Ctx
	} else {
		member = c.fn.Owner != nil && !c.fn.Static && c.cls == c.fn.Owner
		for _, r := range c.recvs {
			if d, _ := r.owner.(*syntax.FunDecl); d != nil && d.Sym == c.fn {
				recv = r.sym
			}
		}
		for _, r := range c.ctxs {
			if d, _ := r.owner.(*syntax.FunDecl); d != nil && d.Sym == c.fn {
				ctxs = append(ctxs, r.sym)
			}
		}
	}
	same := func(r suspendRecv, sym *types.VarSym) bool {
		return r.ok && sym != nil && r.sym == sym && (r.this || slices.Contains(ctxs, sym))
	}
	disp, ext, ctx, ok := c.suspendRecvs(x, f)
	if !ok {
		return true
	}
	for _, r := range append([]suspendRecv{disp, ext}, ctx...) {
		if !r.restricted() {
			continue
		}
		if member && r.this && r.sym == nil || same(r, recv) || slices.ContainsFunc(ctxs, func(s *types.VarSym) bool { return same(r, s) }) {
			continue
		}
		return false
	}
	var chosen []*types.VarSym
	if recv != nil && restrictedClass(recv.Type) != nil {
		chosen = append(chosen, recv)
	}
	for _, s := range ctxs {
		if restrictedClass(s.Type) != nil {
			chosen = append(chosen, s)
		}
	}
	switch len(chosen) {
	case 0:
		return true
	case 1:
	default:
		return false
	}
	if same(disp, chosen[0]) {
		return true
	}
	for _, r := range append([]suspendRecv{ext}, ctx...) {
		if same(r, chosen[0]) && restrictedClass(r.decl) != nil {
			return true
		}
	}
	return false
}

// restrictedClass returns the class of t if it or a supertype is
// annotated @RestrictsSuspension, or a type parameter's bound is.
func restrictedClass(t types.Type) *types.ClassSym {
	t = t.Flex().NonNull()
	switch t.K {
	case types.TypeParam, types.TypeVar:
		for _, b := range t.Param().Bounds {
			if cls := restrictedClass(b); cls != nil {
				return cls
			}
		}
		return nil
	case types.Class:
	default:
		return nil
	}
	var found bool
	t.C.Supertypes(func(s *types.ClassSym) bool {
		found = s.RestrictsSuspension
		return !found
	})
	if !found {
		return nil
	}
	return t.C
}

// posBefore reports whether p is before q.
func posBefore(p, q syntax.Pos) bool {
	return p.Line < q.Line || p.Line == q.Line && p.Col < q.Col
}
