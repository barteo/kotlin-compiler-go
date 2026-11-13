package check

import (
	"slices"

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
	if rt, ok := c.peekRefRecv(r); ok {
		return c.boundRefShapes(rt, r.Name)
	}
	if r.X == nil {
		if ts := c.ctorRefShapes(r.Name); ts != nil {
			return ts
		}
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

// refKinds reports whether reference l, ::name, C::name or a.b.C::name, names a
// function and whether it names a property, and false for known when kc
// can't tell without checking it. A reference that names only properties
// isn't a KFunction, and one that names only functions isn't a
// KProperty: property(Price::cents) is property(KProperty1<T, G>), not
// property(KFunction<Unit>).
func (c *Checker) refKinds(l *syntax.Lambda) (fun, prop, known bool) {
	r := l.Ref
	if r == nil || r.Type != nil || r.Bound != nil {
		return false, false, false
	}
	if r.X == nil {
		if _, _, ok := c.scope.lookup(r.Name); ok || c.lookupClass(r.Name) != nil {
			return false, false, false
		}
		fs, _ := c.scope.findFuns(r.Name)
		return len(fs) > 0 || len(c.topFuns(r.Name)) > 0, c.topVar(r.Name) != nil, true
	}
	var cls *types.ClassSym
	switch x := r.X.(type) {
	case *syntax.Ident:
		if !c.isVar(x.Name) {
			cls = c.lookupClass(x.Name)
		}
	case *syntax.Member:
		cls = c.classRef(x) // a.b.C::name, C.Companion::name
	}
	if cls == nil {
		return false, false, false
	}
	cls.Ensure()
	if r.Name == "length" && c.isCharSequence(cls) || r.Name == "size" && c.hasSize(cls) {
		return false, true, true // Kotlin's properties of Java's methods
	}
	prop = c.memberProp(cls, r.Name) != nil
	static := false
	cls.Supertypes(func(s *types.ClassSym) bool {
		prop = prop || s.KProps[r.Name] != nil
		static = static || len(s.Statics[r.Name]) > 0 // a Java class's static method: UUID::randomUUID
		return true
	})
	return static || len(c.javaMethods(cls, r.Name)) > 0 || len(c.extCallees(r.Name, cls.This())) > 0, prop, true
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

// ctorRefShapes returns the types of ::name, naming only a class of the
// program without type parameters: those of its constructors.
func (c *Checker) ctorRefShapes(name string) []types.Type {
	cls := c.lookupClass(name)
	if cls == nil || cls.External || cls.Inner || cls.Interface || cls.Object || len(cls.TypeParams) > 0 || c.isVar(name) ||
		c.cls != nil && c.hasCallable(c.cls, name) {
		return nil
	}
	if fs, _ := c.scope.findFuns(name); len(fs) > 0 || len(c.topFuns(name)) > 0 {
		return nil
	}
	var out []types.Type
	for _, f := range userCtors(cls) {
		ensureFun(f)
		if f.Hidden || f.Unsupported || len(f.Contexts) > 0 {
			return nil
		}
		var ps []types.Type
		for _, q := range f.Params {
			if q.Vararg || q.Type.K == types.Invalid || q.Type.HasParams() {
				return nil
			}
			ps = append(ps, q.Type)
		}
		out = append(out, c.u.ext.funcs.Type(ps, cls.This()))
	}
	return out
}

// memberRefShapes returns the types of C::name for a class cls.
func (c *Checker) memberRefShapes(cls *types.ClassSym, name string) []types.Type {
	recv := cls.This()
	fs := c.javaMethods(cls, name)
	p := c.memberProp(cls, name)
	var out []types.Type
	if p != nil {
		ensureVar(p)
		if p.Type.K == types.Invalid || p.Type.HasParams() {
			return nil
		}
		t, ok := c.kpropShape(p.Mutable, &recv, p.Type)
		if !ok {
			return nil
		}
		out = append(out, t)
	}
	if len(fs) == 0 && p == nil {
		// Its extensions, which a member would shadow: B::test for
		// fun B.test(), not fun A.test().
		for _, f := range calleeFuns(c.extCallees(name, recv)) {
			ensureFun(f)
			if f.Recv.HasParams() || !c.subtype(recv, f.Recv) {
				return nil
			}
			g := *f
			g.Ext = false
			t, ok := c.funShape(&g, &recv)
			if !ok {
				return nil
			}
			out = append(out, t)
		}
	}
	for _, f := range fs {
		t, ok := c.funShape(f, &recv)
		if !ok {
			return nil
		}
		out = append(out, t)
	}
	return out
}

// boundRefShapes returns the types of x::name for an x of type rt: those
// of its class's member functions of the name, if it has no property of
// the name.
func (c *Checker) boundRefShapes(rt types.Type, name string) []types.Type {
	cls := c.classOf(rt.NonNull())
	if cls == nil || len(cls.TypeParams) > 0 || c.memberProp(cls, name) != nil {
		return nil
	}
	var out []types.Type
	for _, f := range c.accessible(cls, c.javaMethods(cls, name)) {
		if f.Static {
			return nil
		}
		t, ok := c.funShape(f, nil)
		if !ok {
			return nil
		}
		out = append(out, t)
	}
	return out
}

// refsFit reports whether candidate cd takes the arguments of call x
// that are callable references, one of the types each may have (see
// refShapes), where kc can tell: call(A::foo) isn't
// call(function: suspend T.(U) -> List<R>) for fun foo(input: String): String.
func (c *Checker) refsFit(cd *candidate, x *syntax.Call) bool {
	trial := cd.sys
	for i, a := range x.Args {
		l, ok := a.(*syntax.Lambda)
		if !ok || i >= len(cd.params) {
			continue
		}
		ts := c.refShapes(l)
		if len(ts) == 0 {
			continue
		}
		var fit *system
		for _, ft := range ts {
			if s, ok := c.refFits(trial, ft, cd.params[i]); ok {
				fit = s
				break
			}
		}
		if fit == nil {
			return false
		}
		trial = fit
	}
	return true
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
	return c.refFits1(s, ft, p, true)
}

// refFits1 is refFits, where a function's result fits a parameter whose
// result is Unit only if unit is set.
func (c *Checker) refFits1(s *system, ft, p types.Type, unit bool) (*system, bool) {
	trial := s.clone()
	pt := trial.apply(p)
	if v, ok := c.kfunWant(pt); ok && ft.Flex().IsFunc() {
		// A function's reference is a KFunctionN of its function type.
		pt = v
	} else if ft.Flex().IsFunc() && !pt.Flex().IsFunc() {
		// Or converts to a fun interface: Callable<V>'s () -> V.
		if m, f := c.samTarget(pt); m != nil && !m.Ext && f.K != types.Invalid {
			pt = f
		}
	}
	if _, r, ok := funcParts(pt); ok && unit && r.K == types.Unit && !r.N {
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
	if len(ts) == 0 {
		// As in Kotlin, only with a parameter of a known type: not for
		// (T, T) -> R, whose variables may as well be an array's.
		if ps, _, ok := funcParts(s.apply(p)); ok && slices.ContainsFunc(ps, func(t types.Type) bool { return !t.HasVars() }) {
			if ft, ok := c.varargShape(l, len(ps)); ok {
				ts = []types.Type{ft}
			}
		}
	}
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

// varargShape returns the type of ::f, a reference to the only function
// f, which has a vararg, adapted to a function type of n parameters: each
// one past f's others is an element of the vararg, as T of
// foo(b: (Any, T) -> String) is Any for foo(::of), fun of(vararg args: Any),
// unless there is one for it, which takes its array.
func (c *Checker) varargShape(l *syntax.Lambda, n int) (types.Type, bool) {
	r := l.Ref
	if r == nil || r.X != nil || r.Type != nil || r.Bound != nil {
		return types.InvalidType, false
	}
	name := r.Name
	if c.lookupClass(name) != nil || c.cls != nil && c.hasCallable(c.cls, name) || c.topVar(name) != nil {
		return types.InvalidType, false
	}
	if _, _, ok := c.scope.lookup(name); ok {
		return types.InvalidType, false
	}
	fs, _ := c.scope.findFuns(name)
	if len(fs) == 0 {
		fs = c.topFuns(name)
	}
	if len(fs) != 1 {
		return types.InvalidType, false
	}
	f := fs[0]
	ensureFun(f)
	if f.Ext || len(f.TypeParams) > 0 || f.Ret.K == types.Invalid || f.Ret.HasParams() || len(f.Contexts) > 0 || f.Hidden || f.Unsupported || n < len(f.Params)-1 {
		return types.InvalidType, false
	}
	var ps []types.Type
	varargs := 0
	for _, q := range f.Params {
		if q.Type.HasParams() || q.Default {
			return types.InvalidType, false
		}
		if !q.Vararg {
			ps = append(ps, q.Type)
			continue
		}
		varargs++
		if n == len(f.Params) {
			// One parameter for the vararg takes its array.
			ps = append(ps, q.Type)
			continue
		}
		for range n - (len(f.Params) - 1) {
			ps = append(ps, types.VarargElem(q.Type))
		}
	}
	if varargs != 1 {
		return types.InvalidType, false
	}
	if f.Suspend {
		return c.u.ext.funcs.SuspendType(ps, f.Ret), true
	}
	return c.u.ext.funcs.Type(ps, f.Ret), true
}

// refVarargPos returns the parameter that takes the vararg of the
// function a reference ::f names, if it is the only top-level function
// of the name and has a vararg.
func (c *Checker) refVarargPos(r *syntax.CallableRef) (int, bool) {
	if r == nil || r.X != nil || r.Type != nil || r.Bound != nil {
		return 0, false
	}
	name := r.Name
	if c.lookupClass(name) != nil || c.cls != nil && c.hasCallable(c.cls, name) || c.topVar(name) != nil {
		return 0, false
	}
	if _, _, ok := c.scope.lookup(name); ok {
		return 0, false
	}
	fs, _ := c.scope.findFuns(name)
	if len(fs) == 0 {
		fs = c.topFuns(name)
	}
	if len(fs) != 1 {
		return 0, false
	}
	ensureFun(fs[0])
	if fs[0].Ext {
		return 0, false
	}
	for i, q := range fs[0].Params {
		if q.Vararg {
			return i, true
		}
	}
	return 0, false
}

// refArgFit returns system s with the type of reference l, for parameter
// type p, when only one function the reference may name takes as many
// parameters as p's function type, or nil. As in Kotlin, a reference
// with one candidate is resolved with the call it is passed to, the
// type parameters of its function as the call's variables: S and T2 of
// bar(list, ::Foo), fun <T1, T2, S> bar(list: T2, fn: (S) -> B<T1, T2>),
// for a generic class Foo<T>(x: Int), are Int and Foo's B's, and T of
// myLet(::foo), fun <T> myLet(block: (T) -> Unit), for
// fun foo(vararg ints: Int), is an IntArray.
func (c *Checker) refArgFit(s *system, l *syntax.Lambda, p types.Type) *system {
	r := l.Ref
	if r == nil || r.Bound != nil {
		return nil
	}
	pt := s.apply(p)
	if v, ok := c.kfunWant(pt); ok {
		pt = v
	}
	ps, _, ok := funcParts(pt)
	if !ok || !pt.HasVars() || pt.Flex().Contexts() > 0 {
		return nil
	}
	var recv types.Type
	unbound, bound := false, false
	if t, ok := c.peekRefRecv(r); ok {
		recv, bound = t, true
	} else if r.X == nil && r.Type == nil {
		if c.isVar(r.Name) || c.topVar(r.Name) != nil {
			return nil
		}
		if _, _, ok := c.scope.lookup(r.Name); ok {
			return nil
		}
	} else {
		rt, ok := c.refRecvType(r)
		if !ok || rt.HasParams() || rt.HasVars() {
			return nil
		}
		cls := c.classOf(rt.NonNull())
		if cls == nil || c.memberProp(cls, r.Name) != nil || cls.Entries != nil || cls.External && len(javaStatics(cls, r.Name)) > 0 {
			return nil
		}
		recv, unbound = rt, true
	}
	var f *types.FunSym
	for _, g := range c.refFuns(r, recv, nil, false) {
		ensureFun(g)
		n := len(g.Params)
		if unbound {
			n++
		}
		if n != len(ps) || g.Hidden || g.Unsupported || len(g.Contexts) > 0 || g.Suspend != pt.IsSuspendFunc() || g.Ret.K == types.Invalid {
			continue
		}
		if f != nil {
			return nil
		}
		f = g
	}
	if f == nil || !unbound && !bound && f.Ext || f.Name == "<init>" && (f.Owner.Inner || f.Owner.Abstract || f.Owner.Interface) {
		return nil
	}
	tps := f.TypeParams
	if f.Name == "<init>" {
		tps = append(slices.Clone(f.Owner.TypeParams), tps...)
	} else if !unbound && !bound && f.Owner != nil && len(f.Owner.TypeParams) > 0 {
		return nil // an implicit receiver's member
	}
	off := 0
	if unbound {
		off = 1
	}
	for i, q := range f.Params {
		// A vararg takes an array only for a parameter type that nothing
		// else gives: myLet<Int>(::foo) passes the elements.
		if tv := s.unfixedVar(ps[off+i]); q.Vararg && (tv == nil || len(tv.lower) > 0 || len(tv.upper) > 0) {
			return nil
		}
	}
	trial := s.clone()
	var base types.Subst
	if unbound || bound {
		base = c.memberSubst(recv, f.Owner)
	}
	sub := trial.fresh(tps, base, l.P)
	var fps []types.Type
	if unbound {
		fps = append(fps, recv)
	}
	if (unbound || bound) && f.Ext {
		trial.sub(recv, sub.Apply(f.Recv))
	}
	for _, q := range f.Params {
		fps = append(fps, sub.ApplyIn(q.Type))
	}
	ret := sub.Apply(f.Ret)
	if f.Name == "<init>" {
		ret = sub.Apply(f.Owner.This())
	}
	ft := c.u.ext.funcs.Type(fps, ret)
	if f.Suspend {
		ft = c.u.ext.funcs.SuspendType(fps, ret)
	}
	if !trial.ok {
		return nil
	}
	fit, ok := c.refFits(trial, ft, p)
	if !ok {
		return nil
	}
	return fit
}

// peekRefRecv returns the type of the receiver of bound reference r,
// x::m, without checking it, if it is a simple expression (see peekType)
// or a call of the constructor of a class without type parameters,
// A()::baz, that nothing else of the name may call.
func (c *Checker) peekRefRecv(r *syntax.CallableRef) (types.Type, bool) {
	if r.X == nil || r.Type != nil {
		return types.InvalidType, false
	}
	if id, ok := r.X.(*syntax.Ident); ok && !c.isVar(id.Name) {
		return types.InvalidType, false // a class: C::m
	}
	if t, ok := c.peekType(r.X); ok {
		return t, !t.HasVars() && !t.HasParams() && t.K != types.Invalid
	}
	x, ok := r.X.(*syntax.Call)
	if !ok || len(x.TypeArgs) > 0 || hasLambda(x.Args) {
		return types.InvalidType, false
	}
	fn, ok := x.Fun.(*syntax.Ident)
	if !ok || c.isVar(fn.Name) {
		return types.InvalidType, false
	}
	cls := c.lookupClass(fn.Name)
	if cls == nil || len(cls.TypeParams) > 0 || cls.Interface || cls.Object || cls.Abstract || cls.Inner {
		return types.InvalidType, false
	}
	if fs, _ := c.scope.findFuns(fn.Name); len(fs) > 0 || len(c.topFuns(fn.Name)) > 0 || c.cls != nil && c.hasCallable(c.cls, fn.Name) {
		return types.InvalidType, false
	}
	return cls.This(), true
}
