package check

import (
	"maps"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Context parameters.
//
// A function with context parameters, context(a: A, _: B) fun f(), takes
// them implicitly: a call passes, for each, the nearest implicit value of
// its type, as an argument before the extension receiver. The implicit
// values are, innermost first, the context parameters and receivers of
// the lambdas and functions the call is in, then the class's this. Its
// FunSym has them in Contexts, apart from Params, as Recv is, and the
// call the chosen values in Ctx, as expressions this that name them. On
// the JVM, the function takes them first, before its receiver.
//
// In the function's body, a named context parameter is a local, and each
// is an implicit value for the calls in it, but not an implicit
// receiver: its members aren't in scope. A lambda whose expected type is
// a context function type, context(A) B.(C) -> R, which is the function
// type (A, B, C) -> R with its first parameter marked as a context (see
// types.Type.X), has its context parameters as implicit values the same
// way, in Lambda.Ctx. A call of a value of a context function type that
// leaves its contexts out, f(c) for f: context(A) (C) -> R, passes them as
// its first arguments.

// declareContexts resolves the types of a function's context parameters
// and declares them.
func (c *Checker) declareContexts(d *syntax.FunDecl, sym *types.FunSym) {
	sym.Contexts = c.contextSyms(d.Contexts)
}

// contextSyms resolves the types of context parameters ps and returns
// their symbols.
func (c *Checker) contextSyms(ps []*syntax.Param) []*types.VarSym {
	var out []*types.VarSym
	seen := map[string]bool{}
	for _, p := range ps {
		if p.Name != "_" {
			if seen[p.Name] {
				c.errorf(p.P, "conflicting declarations: %s", p.Name)
			}
			seen[p.Name] = true
		}
		name := p.Name
		if name == "_" {
			// As kotlinc names its local: $context-Logger.
			name = "$context-" + p.Type.Name[strings.LastIndexByte(p.Type.Name, '.')+1:]
		}
		p.Sym = &types.VarSym{Name: name, Type: c.resolveType(p.Type), Kind: types.Param, Decl: p}
		out = append(out, p.Sym)
	}
	return out
}

// propContexts declares the context parameters of property d, of symbol
// sym, which its accessors take, and declare in their bodies. A property
// with them has no field: it needs accessors unless abstract.
func (c *Checker) propContexts(d *syntax.PropDecl, sym *types.VarSym) {
	if len(d.Contexts) == 0 {
		return
	}
	switch {
	case d.Recv != nil, d.Init != nil:
		// Reported as an extension property's, or as an initializer
		// without a field.
	case !sym.Abstract && (d.Getter == nil || d.Mutable && d.Setter == nil):
		c.errorf(d.P, "a property with context parameters must have accessors or be abstract")
	}
	sym.Contexts = c.contextSyms(d.Contexts)
	for _, a := range []*syntax.FunDecl{d.Getter, d.Setter} {
		if a != nil && a.Sym != nil {
			a.Contexts = d.Contexts
			a.Sym.Contexts = sym.Contexts
		}
	}
}

// funContexts returns the context parameters in the body of a function,
// as implicit values.
func funContexts(d *syntax.FunDecl) []implicitRecv {
	var out []implicitRecv
	for _, p := range d.Contexts {
		out = append(out, implicitRecv{sym: p.Sym})
	}
	return out
}

// declareNamedContexts declares a function's named context parameters in
// scope s.
func declareNamedContexts(s *scope, d *syntax.FunDecl) {
	for _, p := range d.Contexts {
		if p.Name != "_" {
			s.declare(p.Sym)
		}
	}
}

// pushContexts makes the context parameters of a lambda implicit values
// in its body, nearer than the receivers around it.
func (c *Checker) pushContexts(syms []*types.VarSym) {
	if len(syms) == 0 {
		return
	}
	c.ctxs = c.ctxs[:len(c.ctxs):len(c.ctxs)]
	for _, s := range syms {
		c.ctxs = append(c.ctxs, implicitRecv{sym: s, depth: len(c.lambdas), at: len(c.recvs)})
	}
}

// contextValue returns the nearest implicit value that fits, as an
// expression at p, and its type: a context parameter or a receiver of the
// lambdas and functions around, or the class's this.
func (c *Checker) contextValue(p syntax.Pos, fits func(types.Type) bool) (syntax.Expr, types.Type, bool) {
	i, j := len(c.recvs)-1, len(c.ctxs)-1
	for i >= 0 || j >= 0 {
		var r *implicitRecv
		if j >= 0 && (i < 0 || c.ctxs[j].at > i) {
			r = &c.ctxs[j]
			j--
		} else {
			r = &c.recvs[i]
			i--
		}
		if t := c.scope.narrowed(r.sym); fits(t) {
			return recvThis(r, p), t, true
		}
	}
	if c.cls != nil {
		t := c.cls.This()
		if n, ok := c.narrowedThis(); ok {
			t = n
		}
		if fits(t) {
			this := &syntax.This{}
			this.P = p
			return this, t, true
		}
	}
	return nil, types.InvalidType, false
}

// contextArg is a context argument a call of a callee passes: an implicit
// value, and its type.
type contextArg struct {
	x syntax.Expr
	t types.Type
}

// contextArgs chooses the context arguments of call x of callee cl, the
// nearest implicit value for each of its context parameters, of a type
// that fits the parameter's with the call's type arguments, or some. It
// reports false if one has none.
func (c *Checker) contextArgs(cl *callee, x *syntax.Call) ([]contextArg, bool) {
	ctx := cl.f.Contexts
	if len(ctx) == 0 {
		return nil, true
	}
	sys := newSystem(c)
	var sub types.Subst
	if n := len(x.TypeArgs); n > 0 && n == len(cl.tparams) {
		// Explicit ones, contextOf<A>(), whose errors resolve reports.
		e := len(c.errs)
		sub = maps.Clone(cl.recv)
		if sub == nil {
			sub = types.Subst{}
		}
		for j, tp := range cl.tparams {
			if a := x.TypeArgs[j]; a.Type != nil {
				sub[tp] = types.Arg{T: c.resolveType(a.Type)}
			}
		}
		c.errs = c.errs[:e]
	} else {
		sub = sys.fresh(cl.tparams, cl.recv, x.P)
	}
	out := make([]contextArg, len(ctx))
	for k, cp := range ctx {
		want := sub.ApplyIn(cp.Type)
		v, t, ok := c.contextValue(x.P, func(t types.Type) bool {
			trial := sys.clone()
			trial.sub(t, want)
			return trial.ok
		})
		if !ok {
			return nil, false
		}
		out[k] = contextArg{v, t}
	}
	return out, true
}

// noContextArg reports a call at p of f, none of whose candidates found
// the context arguments.
func (c *Checker) noContextArg(p syntax.Pos, f *types.FunSym) {
	for _, cp := range f.Contexts {
		c.errorf(p, "no context argument for '%s: %s' found", cp.Name, cp.Type)
		return
	}
}

// setContexts records the context arguments of call x, of callee cl.
func (c *Checker) setContexts(x *syntax.Call, cl *callee) {
	x.Ctx = nil
	for _, a := range cl.ctx {
		c.value(a.x, noWant)
		x.Ctx = append(x.Ctx, a.x)
	}
}

// namesContextFun reports whether a top-level function of the program with
// a name has context parameters, which a callable reference can't name.
func (c *Checker) namesContextFun(name string) bool {
	contexts := func(f *types.FunSym) bool {
		d, ok := f.Decl.(*syntax.FunDecl)
		return ok && len(d.Contexts) > 0
	}
	return slices.ContainsFunc(c.pkg.Funs[name], contexts) || slices.ContainsFunc(c.userFuns[name], contexts)
}

// implicitContexts passes the contexts of a call of a value of context
// function type ft that leaves them out: block() for block: context(A)
// () -> R, in a lambda whose receiver is an A, is block(this).
func (c *Checker) implicitContexts(x *syntax.Call, ft types.Type) {
	k := ft.Contexts()
	if k == 0 || len(x.Args) != ft.A.Len()-1-k {
		return
	}
	ctx := make([]syntax.Expr, k)
	for i := range k {
		want := ft.A.Get(i).T
		v, _, ok := c.contextValue(x.P, func(t types.Type) bool { return c.subtype(t, want) })
		if !ok {
			c.errorf(x.P, "no context argument for '%s' found", want)
			return
		}
		ctx[i] = v
	}
	x.Args = append(ctx, x.Args...)
}

// Properties with context parameters.
//
// A property with context parameters has no field, and its accessors take
// them first, as a function does: a read of it is a call of its getter,
// and an assignment a call of its setter, which pass the context
// arguments. The checker records the call on the Ident or Member read or
// assigned, as its Call, which is evaluated instead. A compound assignment
// is a read and an assignment, x.p = x.p + v, with x in a temporary.

// contextAccessor returns the accessor of the property with context
// parameters that x reads, its getter, or assigns if set, its setter, and
// the member access x is or stands for; nil if x isn't one.
func contextAccessor(x syntax.Expr, set bool) (*types.FunSym, *syntax.Member) {
	var m *syntax.Member
	switch x := x.(type) {
	case *syntax.Ident:
		switch {
		case x.Member != nil:
			m = x.Member
		case x.Sym != nil && len(x.Sym.Contexts) > 0:
			return accessor(x.Sym, set), nil
		}
	case *syntax.Member:
		m = x
	}
	switch {
	case m == nil:
	case m.Prop != nil && len(m.Prop.Contexts) > 0:
		return accessor(m.Prop, set), m
	case m.Getter != nil && len(m.Getter.Contexts) > 0:
		// An extension property's, or a class path's.
		switch {
		case set && m.Setter != nil:
			return m.Setter, m
		case set:
			return m.Getter.ExtSetter, m
		}
		return m.Getter, m
	}
	return nil, nil
}

// accessor returns property p's getter, or its setter if set: an abstract
// property's, which it doesn't declare, made for the call.
func accessor(p *types.VarSym, set bool) *types.FunSym {
	ensureVar(p)
	switch {
	case set && p.Setter != nil:
		return p.Setter
	case set && p.Mutable:
		return &types.FunSym{Name: types.SetterName(p.Name), JVMName: p.JVMSetter, Owner: p.Owner, Facade: p.Facade, Abstract: p.Abstract, Module: p.Module,
			Contexts: p.Contexts, Params: []*types.VarSym{{Name: "value", Type: p.Type, Kind: types.Param, Resolved: true}}, Ret: types.UnitType, Resolved: true}
	case set:
		return nil
	case p.Getter != nil:
		return p.Getter
	}
	return &types.FunSym{Name: types.GetterName(p.Name), JVMName: p.JVMGetter, Owner: p.Owner, Facade: p.Facade, Abstract: p.Abstract, Module: p.Module,
		Contexts: p.Contexts, Ret: p.Type, Resolved: true}
}

// contextAccess returns the call of accessor f that x, a read or an
// assignment of a property with context parameters, is, unchecked, with
// its callee; m is the member access x is or stands for, or nil.
func (c *Checker) contextAccess(x syntax.Expr, f *types.FunSym, m *syntax.Member) (*syntax.Call, []callee) {
	call := &syntax.Call{}
	call.P = x.Pos()
	cl := callee{f: f, tparams: f.TypeParams}
	switch {
	case m != nil:
		rt := m.X.Type()
		if m.Safe {
			rt = rt.NonNull()
		}
		fn := &syntax.Member{X: m.X, Name: f.Name, Safe: m.Safe, Dispatch: m.Dispatch, DispatchThis: m.DispatchThis}
		fn.P, fn.T = m.P, rt
		call.Fun = fn
		switch {
		case f.Ext && f.Owner != nil:
			// A member extension's dispatch receiver: an object, an
			// implicit receiver or this.
			cl.ext, cl.dispatch = rt, m.Dispatch
			switch {
			case m.DispatchThis != nil:
				cl.recv = c.memberSubst(m.DispatchThis.Type(), f.Owner)
			case m.Dispatch != nil:
				cl.recv = c.memberSubst(types.ClassType(m.Dispatch), f.Owner)
			case c.cls != nil:
				cl.recv = c.memberSubst(c.cls.This(), f.Owner)
			}
		case f.Ext:
			cl.ext = rt
		default:
			cl.recv = c.memberSubst(rt, f.Owner)
		}
	case f.Owner != nil:
		// A member of this.
		this := &syntax.This{Class: true}
		this.P = x.Pos()
		rt := c.value(this, noWant)
		fn := &syntax.Member{X: this, Name: f.Name}
		fn.P, fn.T = x.Pos(), rt
		call.Fun = fn
		cl.recv = c.memberSubst(rt, f.Owner)
	default:
		id := &syntax.Ident{Name: f.Name}
		id.P = x.Pos()
		call.Fun = id
	}
	return call, []callee{cl}
}

// contextRead checks read x, of type t, of a property with context
// parameters as the call of its getter, and returns its type; that of
// another, t.
func (c *Checker) contextRead(x syntax.Expr, t, want types.Type) types.Type {
	if t.K == types.Invalid {
		return t
	}
	f, m := contextAccessor(x, false)
	if f == nil {
		return t
	}
	if f.Owner != nil && m == nil && (c.cls == nil || !c.cls.IsSubclassOf(f.Owner)) {
		c.unsupported(x.Pos(), "context parameters", "reading an outer class's property with context parameters is not supported")
		return t
	}
	call, cands := c.contextAccess(x, f, m)
	t = c.resolve(call, cands, want, nil)
	call.SetType(t)
	setAccess(x, call)
	return t
}

// contextWrite checks assignment s of a property with context parameters,
// as the call of its setter, or a compound one as an assignment of the
// value read and v. It reports false for the assignment of another.
func (c *Checker) contextWrite(s *syntax.Assign) bool {
	f, m := contextAccessor(s.Target, true)
	if f == nil {
		return false
	}
	if s.Op != syntax.ASSIGN {
		// x.p op= v: val r = x; r.p = r.p op v.
		target, read := func() syntax.Expr { return s.Target }, func() syntax.Expr { id := *s.Target.(*syntax.Ident); id.Call = nil; return &id }
		if m != nil {
			recv := c.temp(s.P, "receiver", m.X, m.X.Type())
			s.Stmts = append(s.Stmts, recv)
			member := func() syntax.Expr {
				r := &syntax.Member{X: tempRef(recv, m.P), Name: m.Name}
				r.P = m.P
				return r
			}
			target, read = member, member
		}
		value := &syntax.Binary{Op: arithOf(s.Op), X: read(), Y: s.Value}
		value.P = s.P
		set := &syntax.Assign{P: s.P, Op: syntax.ASSIGN, Target: target(), Value: value}
		c.assign(set)
		s.Stmts = append(s.Stmts, set)
		return true
	}
	if f.Owner != nil && m == nil && (c.cls == nil || !c.cls.IsSubclassOf(f.Owner)) {
		c.unsupported(s.P, "context parameters", "assigning an outer class's property with context parameters is not supported")
		return true
	}
	call, cands := c.contextAccess(s.Target, f, m)
	call.Args = []syntax.Expr{s.Value}
	call.SetType(c.resolve(call, cands, noWant, nil))
	setAccess(s.Target, call)
	return true
}

// setAccess records the call of an accessor on x, a read or an assignment
// of a property with context parameters.
func setAccess(x syntax.Expr, call *syntax.Call) {
	switch x := x.(type) {
	case *syntax.Ident:
		x.Call = call
	case *syntax.Member:
		x.Call = call
	}
}
