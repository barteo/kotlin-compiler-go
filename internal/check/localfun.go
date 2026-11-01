package check

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// localCall is a call of a local function: the lambdas and local
// functions between the function's declaration and the call must capture
// what the function captures, since the call passes it along.
type localCall struct {
	frame *syntax.Lambda   // the callee's
	chain []*syntax.Lambda // the frames the call is in, below the declaration
}

// localFun checks a local function's declaration. Its body is checked
// like a lambda's, in a frame of its own on the lambda stack, so that the
// locals and the this it uses from the enclosing code are recorded as its
// captures. return returns from it, and break and continue can't leave
// it.
func (c *Checker) localFun(s *syntax.LocalFun) {
	d := s.Decl
	sym := &types.FunSym{Name: d.Name, Decl: s, Operator: d.Operator, Infix: d.Infix}
	sym.Suspend = d.Suspend && c.coroutines(d.P)
	d.Sym = sym
	switch {
	case d.Open, d.Abstract, d.Override, d.Final:
		c.errorf(d.P, "modifiers open, abstract, override and final are not applicable to local functions")
	case d.Inline:
		c.errorf(d.P, "local inline functions are not yet supported")
	case c.u.compose && c.composeAnno(d.Annotations, "", "Composable"):
		c.unsupported(d.P, "compose", "local composable functions are not supported")
	}
	if c.localCalls == nil {
		c.localCalls = new([]localCall)
	}
	saved := c.state
	sym.TypeParams = c.declareTypeParams(d.TypeParams, sym, false)
	c.tparams = append(slices.Clone(sym.TypeParams), saved.tparams...)
	c.resolveBounds(d.TypeParams)
	if d.Recv != nil {
		c.declareRecv(d) // a local extension function
	}
	c.declareContexts(d, sym)
	names := map[string]bool{}
	for _, p := range d.Params {
		if names[p.Name] {
			c.errorf(p.P, "conflicting declarations: %s", p.Name)
		}
		names[p.Name] = true
		p.Sym = c.paramSym(p, c.resolveType(p.Type), p)
		sym.Params = append(sym.Params, p.Sym)
	}
	c.checkVarargs(d.Params)
	switch {
	case d.Ret != nil:
		sym.Ret, sym.Resolved = c.resolveType(d.Ret), true
	case d.ExprBody == nil:
		sym.Ret, sym.Resolved = types.UnitType, true
	default:
		sym.Ret = types.InvalidType // inferred from the body, below
	}
	// Declared before its body, so that it can call itself.
	if other := saved.scope.declareFun(sym); other != nil {
		od := other.Decl.(*syntax.LocalFun).Decl
		if !c.done[od] {
			c.done[od] = true // reported once
			c.errorf(od.P, "conflicting overloads: %s", d.Name)
		}
		c.errorf(d.P, "conflicting overloads: %s", d.Name)
	}

	frame := &syntax.Lambda{}
	frame.P = d.P
	s.Frame = frame
	c.lambdas = append(c.lambdas[:len(c.lambdas):len(c.lambdas)], frame)
	c.lrets = append(c.lrets[:len(c.lrets):len(c.lrets)], &lambdaResult{})
	c.base = len(c.lambdas)
	c.scope = &scope{parent: saved.scope, depth: len(c.lambdas)}
	switch {
	case saved.fnName != "":
		c.fnName = saved.fnName + "$" + d.Name
	case saved.fn != nil && saved.fn.Name != "" && !strings.HasPrefix(saved.fn.Name, "<"):
		c.fnName = saved.fn.Name + "$" + d.Name
	default:
		c.fnName = d.Name
	}
	c.fn, c.retType, c.retAllowed = sym, sym.Ret, sym.Resolved
	c.declName = ""
	declareNamedContexts(c.scope, d)
	c.pushContexts(sym.Contexts) // the receiver is nearer
	if d.RecvSym != nil {
		c.recvs = append(c.recvs[:len(c.recvs):len(c.recvs)], implicitRecv{sym: d.RecvSym, label: d.Name, depth: len(c.lambdas)})
	}
	if sym.Suspend {
		d.Continuation = c.anonName()
	}
	c.loopsOutside = c.loopsOutside || len(c.loops) > 0
	c.loops, c.ctorArgs, c.allowRange = nil, false, false
	// A default value can use the parameters before it, and what the
	// enclosing code can.
	for _, p := range d.Params {
		if p.Default != nil {
			c.assignable(c.value(p.Default, p.Sym.Type), p.Sym.Type, p.Default.Pos())
		}
		c.scope.declare(p.Sym)
	}
	switch {
	case d.Body != nil:
		t := c.block(d.Body, noWant, false)
		if sym.Ret.K != types.Unit && sym.Ret.K != types.Invalid && t.K != types.Nothing {
			c.errorf(d.Body.P, "missing return statement in function '%s'", d.Name)
		}
	case sym.Resolved:
		c.assignable(c.value(d.ExprBody, sym.Ret), sym.Ret, d.ExprBody.Pos())
	default:
		sym.InProgress = true
		t := c.expr(d.ExprBody, noWant)
		if t.K == types.Range {
			c.errorf(d.ExprBody.Pos(), "ranges are only supported in 'for' loops and 'in' checks")
			t = types.InvalidType
		}
		sym.Ret, sym.Resolved, sym.InProgress = t, true, false
	}
	c.state = saved
	c.propagateCaptures()
}

// callLocal checks a call of one of the local functions fs, declared in
// a scope at the given depth.
func (c *Checker) callLocal(x *syntax.Call, fs []*types.FunSym, depth int, want types.Type, sink *system) types.Type {
	for _, f := range fs {
		if !f.Resolved {
			c.errorf(x.P, "type checking has run into a recursive problem: '%s' needs an explicit return type", f.Name)
			c.args(x, nil)
			return types.InvalidType
		}
	}
	t := c.resolve(x, methodCallees(fs, nil), want, sink)
	if x.Sym != nil {
		if lf, ok := x.Sym.Decl.(*syntax.LocalFun); ok && c.localCalls != nil {
			chain := slices.Clone(c.lambdas[min(depth, len(c.lambdas)):])
			c.addLocalCall(localCall{lf.Frame, chain})
		}
	}
	return t
}

// propagateCaptures makes the frames that local function calls are in
// capture what the called functions capture, until nothing changes: a
// function's captures can grow after a call of it, when it calls itself
// from a lambda or calls another local function that does.
func (c *Checker) propagateCaptures() {
	if c.localCalls == nil {
		return
	}
	for changed := true; changed; {
		changed = false
		for _, lc := range *c.localCalls {
			for _, l := range lc.chain {
				if l.Inline || l == lc.frame {
					continue
				}
				for _, v := range lc.frame.Captures {
					if !slices.Contains(l.Captures, v) {
						c.addCapture(l, v)
						changed = true
					}
				}
				if lc.frame.This && !l.This {
					c.setThis(l)
					changed = true
				}
			}
		}
	}
}

// localExtCall records a call of local extension function lf, as
// callLocal records the others': the frames between its declaration and
// the call capture what it captures.
func (c *Checker) localExtCall(lf *syntax.LocalFun) {
	depth := -1
	for sc := c.scope; sc != nil && depth < 0; sc = sc.parent {
		for _, f := range sc.funs {
			if f == lf.Decl.Sym {
				depth = sc.depth
			}
		}
	}
	if depth < 0 {
		return // in its own body, before its scope has it
	}
	chain := slices.Clone(c.lambdas[min(depth, len(c.lambdas)):])
	c.addLocalCall(localCall{lf.Frame, chain})
}

// localExts returns the local extension functions named name in scope,
// innermost first.
func (c *Checker) localExts(name string) []*types.FunSym {
	var out []*types.FunSym
	for sc := c.scope; sc != nil; sc = sc.parent {
		for _, f := range sc.funs {
			if f.Ext && f.Name == name {
				out = append(out, f)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}
