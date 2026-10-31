package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Smart casts of var locals.
//
// As in Kotlin, a local var can be smart cast while it isn't assigned: a
// check narrows it, as it does a val, and an assignment narrows it to the
// type of the value assigned. Lambdas, local functions and object
// expressions that assign a var may run at any time after they are
// created, so:
//   - in one of them, a var declared outside it is stable unless code
//     outside it assigns the var after it is created: later in the
//     enclosing code, or in a loop around it, or in other nested code;
//   - in the code that declares the var, a smart cast holds where every
//     value nested code has assigned to it so far has the narrowed type:
//     after { x = "a" } a String? x != null is a String, as kotlinc has
//     it, and after { x = null } it isn't.
//
// Narrowing follows the code's flow:
//   - after an if or a when, a var that a branch assigns has the join of
//     its types at the ends of the branches that don't jump;
//   - in a loop, the vars it assigns lose their narrowing at its start,
//     and after it;
//   - after a try, so do the vars it assigns, and in its catch and
//     finally blocks the vars its body assigns;
//   - x!! narrows x to its non-null type for the code after it.
//
// The members of stable values are stable too, when they are final val
// properties of the program with a field: a.b narrows after a.b != null.

// varSite is where a local var is declared: statement i of block b.
type varSite struct {
	b *syntax.Block
	i int
}

// varFrame is a local var read in the frame of a lambda, a local
// function or an object expression that it is declared outside of.
type varFrame struct {
	v     *types.VarSym
	frame *syntax.Lambda
}

// isVar reports whether v is a local var, whose smart casts follow the
// flow, or a local val declared without a value, which the flow assigns:
// a branch that assigns it and checks it for null narrows it after the
// if, when the other branches jump.
func isVar(v *types.VarSym) bool {
	return (v.Mutable || v.Deferred) && v.Kind == types.Local && v.Delegate == nil
}

// varStable reports whether a smart cast of local var v to t holds where
// the code is (see above). An invalid t asks whether a check can narrow
// v: in the code that declares v, the read decides.
func (c *Checker) varStable(v *types.VarSym, t types.Type) bool {
	if !v.Mutable {
		return true // a val, which nested code can't assign
	}
	found, depth := c.scope.find(v.Name)
	if found != v {
		return false
	}
	if depth == c.scope.depth {
		if t.K == types.Invalid {
			return true
		}
		for _, at := range c.nestedAssigns[v] {
			if !c.subtype(at, t) {
				return false
			}
		}
		return true
	}
	if depth >= len(c.lambdas) {
		return false
	}
	key := varFrame{v, c.lambdas[depth]}
	if assigned, ok := c.concurrent[key]; ok {
		return !assigned
	}
	site, ok := c.varSites[v]
	assigned := !ok || assignedOutside(site.b.Stmts[site.i+1:], v.Name, key.frame)
	c.concurrent[key] = assigned
	return !assigned
}

// assignedOutside reports whether stmts assign a local named name where
// it may run while frame, a lambda, a local function's or an object
// expression's, does: after frame's start, in a loop around it, or in
// other nested code.
func assignedOutside(stmts []syntax.Stmt, name string, frame *syntax.Lambda) bool {
	found := false
	for _, s := range stmts {
		found = found || assignedOutside1(s, name, frame)
	}
	return found
}

func assignedOutside1(node any, name string, frame *syntax.Lambda) bool {
	found := false
	inspect(node, func(n any) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *syntax.Lambda:
			found = n != frame && assignedNames(n)[name]
			return false
		case *syntax.LocalFun:
			found = n.Frame != frame && assignedNames(n)[name]
			return false
		case *syntax.ObjectExpr:
			found = n.Decl.P != frame.P && assignedNames(n)[name]
			return false
		case *syntax.While:
			found = hasFrame(n, frame) && assignedNames(n)[name]
		case *syntax.For:
			found = hasFrame(n, frame) && assignedNames(n)[name]
		}
		if a, ok := assignedName(n); ok && a == name {
			var p syntax.Pos
			switch n := n.(type) {
			case *syntax.Assign:
				p = n.P
			case *syntax.Unary:
				p = n.P
			}
			found = p.Line > frame.P.Line || p.Line == frame.P.Line && p.Col > frame.P.Col
		}
		return !found
	})
	return found
}

// hasFrame reports whether node has the lambda, the local function or
// the object expression whose frame is frame.
func hasFrame(node any, frame *syntax.Lambda) bool {
	found := false
	inspect(node, func(n any) bool {
		switch n := n.(type) {
		case *syntax.Lambda:
			found = found || n == frame
		case *syntax.LocalFun:
			found = found || n.Frame == frame
		case *syntax.ObjectExpr:
			found = found || n.Decl.P == frame.P
		}
		return !found
	})
	return found
}

// assignedNested records that code nested in the code declaring local
// var v assigns it a value of type t.
func (c *Checker) assignedNested(v *types.VarSym, t types.Type) {
	if _, depth := c.scope.find(v.Name); depth < c.scope.depth {
		c.nestedAssigns[v] = append(c.nestedAssigns[v], t)
	}
}

// assignedNames returns the names of the locals that node assigns, also
// in code nested in it.
func assignedNames(node any) map[string]bool {
	names := map[string]bool{}
	inspect(node, func(n any) bool {
		if name, ok := assignedName(n); ok {
			names[name] = true
		}
		return true
	})
	return names
}

// assignedName returns the name of the local that node assigns, if it is
// an assignment or an increment of a name.
func assignedName(node any) (string, bool) {
	switch n := node.(type) {
	case *syntax.Assign:
		if id, ok := n.Target.(*syntax.Ident); ok {
			return id.Name, true
		}
	case *syntax.Unary:
		if id, ok := n.X.(*syntax.Ident); ok && (n.Op == syntax.INC || n.Op == syntax.DEC) {
			return id.Name, true
		}
	}
	return "", false
}

// assigns returns the names that node assigns, as assignedNames, but
// memoized for the blocks and statements nested in each other that
// watchedVars asks about, so that each node is walked once.
func (c *Checker) assigns(node any) []string {
	if names, ok := c.assignsMemo[node]; ok {
		return names
	}
	var names []string
	add := func(name string) {
		for _, n := range names {
			if n == name {
				return
			}
		}
		names = append(names, name)
	}
	syntax.Inspect(node, func(n any) bool {
		if n == node {
			return true
		}
		switch n.(type) {
		case *syntax.Block, *syntax.If, *syntax.When, *syntax.While, *syntax.For, *syntax.Try, *syntax.Lambda:
			for _, name := range c.assigns(n) {
				add(name)
			}
			return false
		case *syntax.ObjectExpr:
			for name := range assignedNames(n) {
				add(name)
			}
			return false
		}
		if name, ok := assignedName(n); ok {
			add(name)
		}
		return true
	})
	c.assignsMemo[node] = names
	return names
}

// inspect is syntax.Inspect that also visits the members of object
// expressions.
func inspect(node any, f func(any) bool) {
	syntax.Inspect(node, func(n any) bool {
		if !f(n) {
			return false
		}
		if o, ok := n.(*syntax.ObjectExpr); ok {
			for _, m := range o.Decl.Members {
				switch m := m.(type) {
				case *syntax.FunDecl:
					inspect(m.Body, f)
					if m.ExprBody != nil {
						inspect(m.ExprBody, f)
					}
				case *syntax.PropDecl:
					if m.Init != nil {
						inspect(m.Init, f)
					}
					if m.Getter != nil {
						inspect(m.Getter.Body, f)
						if m.Getter.ExprBody != nil {
							inspect(m.Getter.ExprBody, f)
						}
					}
				case *syntax.InitBlock:
					inspect(m.Body, f)
				}
			}
		}
		return true
	})
}

// watchedVars returns the stable local vars in scope that the nodes
// assign, whose narrowing a branch or a loop changes.
func (c *Checker) watchedVars(nodes ...any) []*types.VarSym {
	if !c.hasVars() {
		return nil // most bodies: nothing to walk
	}
	var out []*types.VarSym
	for _, n := range nodes {
		if b, ok := n.(*syntax.Block); n == nil || ok && b == nil {
			continue
		}
		for _, name := range c.assigns(n) {
			if v, _ := c.scope.find(name); v != nil && isVar(v) && narrowable(v.Type) && !slices.Contains(out, v) {
				out = append(out, v)
			}
		}
	}
	return out
}

// hasVars reports whether a local var whose type can be narrowed is in
// scope.
func (c *Checker) hasVars() bool {
	for sc := c.scope; sc != nil; sc = sc.parent {
		for _, v := range sc.vars {
			if isVar(v) && narrowable(v.Type) {
				return true
			}
		}
	}
	return false
}

// narrowable reports whether a check or an assignment can narrow a var of
// type t: unless it is a non-null value of a built-in type or a final
// class, whose subtypes are itself.
func narrowable(t types.Type) bool {
	switch {
	case t.N || t.P || t.K == types.Invalid:
		return true
	case t.IsPrimitive() || t.K == types.String || t.K == types.Unit:
		return false
	case t.K == types.Class:
		t.C.Ensure()
		return !t.C.Final || t.C.Array && t.A.Len() > 0
	}
	return true
}

// reset drops the narrowing of vars for the rest of the current scope.
func (c *Checker) reset(vars []*types.VarSym) {
	for _, v := range vars {
		c.scope.narrows = append(c.scope.narrows, narrow{sym: v, t: v.Type})
	}
}

// branchExit records the types of watched vars where a branch ends.
type branchExit struct {
	vars   []*types.VarSym
	types  []types.Type
	jumped bool // the branch doesn't end normally: its types don't count
}

// watchExit makes the next block checked record the types of vars at its
// end in the returned exit.
func (c *Checker) watchExit(vars []*types.VarSym) *branchExit {
	if len(vars) == 0 {
		return nil
	}
	e := &branchExit{vars: vars}
	c.exitWatch = e
	return e
}

// record records the types of e's vars in the current scope, at the end
// of a block whose type is t.
func (e *branchExit) record(c *Checker, t types.Type) {
	if t.K == types.Nothing {
		e.jumped = true
		return
	}
	e.types = make([]types.Type, len(e.vars))
	for i, v := range e.vars {
		e.types[i] = c.scope.narrowed(v)
	}
}

// startExit returns an exit with the types of vars in the current scope:
// the path past an if without else, or a when without a matching branch.
func (c *Checker) startExit(vars []*types.VarSym, ns []narrow) *branchExit {
	c.push()
	c.scope.narrows = ns
	e := &branchExit{vars: vars}
	e.record(c, types.UnitType)
	c.pop()
	return e
}

// merge narrows each var to the join of its types at the exits that
// don't jump, for the rest of the current scope.
func (c *Checker) merge(vars []*types.VarSym, exits []*branchExit) {
	for i, v := range vars {
		var t types.Type
		any := false
		for _, e := range exits {
			if e == nil || e.jumped || e.types == nil {
				continue
			}
			if !any {
				t, any = e.types[i], true
				continue
			}
			j, ok := c.join(t, e.types[i])
			if !ok || !c.subtype(j, v.Type) {
				j = v.Type
			}
			t = j
		}
		if !any {
			continue // every branch jumps: the code after is unreachable
		}
		c.scope.narrows = append(c.scope.narrows, narrow{sym: v, t: t})
	}
}

// assignedVar narrows local var v to the type vt of the value assigned to
// it, for the rest of the current scope.
func (c *Checker) assignedVar(v *types.VarSym, vt types.Type) {
	t := v.Type
	if vt.K != types.Invalid && vt.K != types.Null && vt.K != types.Nothing && c.subtype(vt, v.Type) {
		t = vt
	}
	c.scope.narrows = append(c.scope.narrows, narrow{sym: v, t: t})
}

// narrowKeyOf returns the narrowing of a stable value x, with its current
// type: a stable variable, this, or a final val property of the program
// read through either.
func (c *Checker) narrowKeyOf(x syntax.Expr) (narrow, bool) {
	if m, ok := x.(*syntax.Member); ok {
		if m.Safe {
			return narrow{}, false
		}
		return c.memberKey(m, m.T)
	}
	if id, ok := x.(*syntax.Ident); ok && id.Member != nil && !id.Member.Safe {
		// A property of an implicit receiver, read as this.p.
		return c.memberKey(id.Member, id.Member.T)
	}
	sym := c.narrowSym(x)
	if sym == nil {
		return narrow{}, false
	}
	return narrow{sym: sym, t: c.scope.narrowed(sym)}, true
}

// memberKey returns the narrowing of a.p, where p is a stable property
// of a stable value a, and the type of a.p is t.
func (c *Checker) memberKey(m *syntax.Member, t types.Type) (narrow, bool) {
	if m.Prop == nil || !c.stableMember(m.Prop) {
		return narrow{}, false
	}
	if this, ok := m.X.(*syntax.This); ok && this.Recv == nil && this.Label == "" {
		// this.p is p.
		return narrow{sym: m.Prop, t: c.scope.narrowedMember(nil, m.Prop, t)}, true
	}
	recv := c.narrowSym(m.X)
	if recv == nil {
		return narrow{}, false
	}
	return narrow{recv: recv, sym: m.Prop, t: c.scope.narrowedMember(recv, m.Prop, t)}, true
}

// stableMember reports whether reading property p through a stable value
// always gives the same value: a final val of the program with a field,
// or an override of one in a final class other than an enum class, whose
// entries may override it.
func (c *Checker) stableMember(p *types.VarSym) bool {
	return p.Kind == types.Prop && !p.Mutable && p.Getter == nil && p.HasField() && p.Owner != nil && !p.Owner.External &&
		(p.Final || p.Owner.Final && p.Owner.Entries == nil)
}
