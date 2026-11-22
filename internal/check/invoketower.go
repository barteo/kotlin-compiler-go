package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// hideUnfitLocals handles f(args) where the innermost local f can't be
// invoked with the arguments but an outer one can, or, with no local
// that can, a top-level property f: as in K2, which tries each level of
// scope in turn, the call invokes that one. It hides the locals before it
// for the call's check, and returns the function that shows them again,
// or nil if the innermost local is the one invoked.
func (c *Checker) hideUnfitLocals(name string, x *syntax.Call) func() {
	if argsMention(x.Args, name) {
		return nil // hidden, f(f) would read the outer f
	}
	var locals []*types.VarSym
	for sc := c.scope; sc != nil; sc = sc.parent {
		for i := len(sc.vars) - 1; i >= 0; i-- {
			if v := sc.vars[i]; v.Name == name {
				locals = append(locals, v)
			}
		}
	}
	if len(locals) == 0 || len(locals) == 1 && c.topVar(name) == nil {
		return nil // nothing else to invoke
	}
	hide := func(vs []*types.VarSym) func() {
		for _, v := range vs {
			v.Name = "<hidden " + name + ">"
		}
		return func() {
			for _, v := range vs {
				v.Name = name
			}
		}
	}
	for i, v := range locals {
		if v.Kind == types.Global {
			break
		}
		if c.invokeTakes(c.scope.narrowed(v), x) {
			if i == 0 {
				return nil
			}
			return hide(locals[:i])
		}
	}
	if fs, _ := c.scope.findFuns(name); len(fs) > 0 || len(c.topFuns(name)) > 0 {
		return nil // functions of the name come before a top-level property
	}
	if g := c.topVar(name); g != nil && !g.InProgress {
		ensureVar(g)
		if c.invokeTakes(g.Type, x) {
			return hide(locals)
		}
	}
	return nil
}

// invokeTakes reports whether a value of type t can be invoked with call
// x's arguments, by their types where they are known.
func (c *Checker) invokeTakes(t types.Type, x *syntax.Call) bool {
	t = t.Flex()
	if t.K == types.Invalid {
		return false
	}
	if !t.IsFunc() {
		return c.hasFun(t, "invoke", x)
	}
	if !c.invokeFitsType(t, x) {
		return false
	}
	n := t.A.Len() - 1
	params := t.A.List[:n]
	if len(params) != len(x.Args) {
		return true // implicit receivers or context arguments: no further
	}
	for i, at := range c.argTypes(x) {
		if at.K == types.Invalid || at.K == 0 || params[i].T.HasParams() || params[i].T.HasVars() {
			continue
		}
		if !c.subtype(at, params[i].T) {
			return false
		}
	}
	return true
}

// argsMention reports whether any of xs uses the name.
func argsMention(xs []syntax.Expr, name string) bool {
	found := false
	for _, x := range xs {
		syntax.Inspect(x, func(n any) bool {
			if id, ok := n.(*syntax.Ident); ok && id.Name == name {
				found = true
			}
			return !found
		})
	}
	return found
}

// safeChainInvoke checks the invoke of the end of a chain of safe calls,
// which continues it: a?.b[i](v) and a?.f()(v) are a?.b[i]?.invoke(v)
// and a?.f()?.invoke(v), if the value invoked isn't nullable, as index
// has it for a?.b[i]. It reports false for other calls.
func (c *Checker) safeChainInvoke(x *syntax.Call, want types.Type, sink *system) (types.Type, bool) {
	switch x.Fun.(type) {
	case *syntax.Call, *syntax.Index:
	default:
		return types.InvalidType, false
	}
	if !safeChain(x.Fun) {
		return types.InvalidType, false
	}
	m := &syntax.Member{X: x.Fun, Name: "invoke", Safe: true}
	m.P = x.Fun.Pos()
	fun := x.Fun
	x.Fun, x.Operator = m, true
	t := c.call(x, want, sink)
	if rt := chainType(fun); rt.K != types.Invalid && types.MayBeNull(rt) && !rt.P {
		c.errorf(m.P, "expression of type %s cannot be invoked as a function", rt)
		return types.InvalidType, true
	}
	return t, true
}
