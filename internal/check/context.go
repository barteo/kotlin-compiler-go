package check

import (
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
	sym.Contexts = nil
	seen := map[string]bool{}
	for _, p := range d.Contexts {
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
		sym.Contexts = append(sym.Contexts, p.Sym)
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

// contextArgs chooses the context arguments of a call at p of callee cl,
// the nearest implicit value for each of its context parameters, of a
// type that fits the parameter's with some type arguments. It reports
// false if one has none.
func (c *Checker) contextArgs(cl *callee, p syntax.Pos) ([]contextArg, bool) {
	ctx := cl.f.Contexts
	if len(ctx) == 0 {
		return nil, true
	}
	sys := newSystem(c)
	sub := sys.fresh(cl.tparams, cl.recv, p)
	out := make([]contextArg, len(ctx))
	for k, cp := range ctx {
		want := sub.ApplyIn(cp.Type)
		x, t, ok := c.contextValue(p, func(t types.Type) bool {
			trial := sys.clone()
			trial.sub(t, want)
			return trial.ok
		})
		if !ok {
			return nil, false
		}
		out[k] = contextArg{x, t}
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
