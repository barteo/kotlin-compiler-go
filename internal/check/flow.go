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
		case *syntax.Block:
			if n == nil || n == node {
				break
			}
			// A var name in the block shadows name after it.
			for _, st := range n.Stmts {
				if found = assignedOutside1(st, name, frame); found || slices.Contains(declaredNames(st), name) {
					break
				}
			}
			return false
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
		if b, ok := n.(*syntax.Block); ok && n != node && b != nil {
			for name := range assignedNames(b) {
				names[name] = true
			}
			return false
		}
		if b, ok := n.(*syntax.Block); ok && b != nil {
			// A block's var x shadows the outer x after it.
			shadowed := map[string]bool{}
			for _, st := range b.Stmts {
				for name := range assignedNames(st) {
					if !shadowed[name] {
						names[name] = true
					}
				}
				for _, name := range declaredNames(st) {
					shadowed[name] = true
				}
			}
			return false
		}
		if name, ok := assignedName(n); ok {
			names[name] = true
		}
		return true
	})
	return names
}

// declaredNames returns the names of the locals that statement st
// declares.
func declaredNames(st syntax.Stmt) []string {
	switch st := st.(type) {
	case *syntax.VarStmt:
		return []string{st.Name}
	case *syntax.Destructure:
		return st.Names
	}
	return nil
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
	if b, ok := node.(*syntax.Block); ok && b != nil {
		// A block's own var x shadows the x outside it: its assignments
		// after the declaration aren't of the outer x.
		var shadowed []string
		for _, st := range b.Stmts {
			own := c.assigns(st)
			if name, ok := assignedName(st); ok {
				own = append([]string{name}, own...)
			}
			for _, name := range own {
				if !slices.Contains(shadowed, name) {
					add(name)
				}
			}
			shadowed = append(shadowed, declaredNames(st)...)
		}
		c.assignsMemo[node] = names
		return names
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

// loopVars returns the vars of watchedVars(nodes...) whose narrowing a
// loop over nodes drops, and a function to call after the loop. It keeps
// those smart cast to a non-null number or Char that the nodes only
// increment and decrement, or, if declared as its nullable type, also
// assign with op=, whose results, which assign checks, keep the type.
func (c *Checker) loopVars(nodes ...any) ([]*types.VarSym, func()) {
	vars := c.watchedVars(nodes...)
	out := vars[:0:0]
	var kept []*types.VarSym
	var old []types.Type
	var had []bool
	for _, v := range vars {
		if t := c.scope.narrowed(v); t == v.Type || t.N || t.P || !t.IsNumeric() && t.K != types.Char || !onlyIncDec(v.Name, nodes, v.Type.NonNull() == t) {
			out = append(out, v)
		} else {
			if c.loopKept == nil {
				c.loopKept = map[*types.VarSym]types.Type{}
			}
			kept = append(kept, v)
			prev, ok := c.loopKept[v]
			old, had = append(old, prev), append(had, ok)
			c.loopKept[v] = t
		}
	}
	return out, func() {
		for i, v := range kept {
			if had[i] {
				c.loopKept[v] = old[i]
			} else {
				delete(c.loopKept, v)
			}
		}
	}
}

// onlyIncDec reports whether every assignment of the local name in nodes
// is a ++, a -- or, if opAssign, an op=, outside lambdas, local functions
// and classes, and object expressions.
func onlyIncDec(name string, nodes []any, opAssign bool) bool {
	ok := true
	for _, node := range nodes {
		if b, isBlock := node.(*syntax.Block); node == nil || isBlock && b == nil {
			continue
		}
		inspect(node, func(n any) bool {
			switch n := n.(type) {
			case *syntax.Lambda, *syntax.ObjectExpr, *syntax.LocalFun, *syntax.LocalClass:
				if assignedNames(n)[name] {
					ok = false
				}
				return false
			case *syntax.Assign:
				if id, isID := n.Target.(*syntax.Ident); isID && id.Name == name && (n.Op == syntax.ASSIGN || !opAssign) {
					ok = false
				}
			}
			return ok
		})
	}
	return ok
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

// flowKey is a value that smart casts narrow: a variable, or a property
// of a stable value recv.
type flowKey struct{ recv, sym *types.VarSym }

// flowState is the smart casts where the code is: the narrowed type of
// each value that the scopes around it narrow.
type flowState map[flowKey]types.Type

// flowState returns the smart casts in the current scope.
func (c *Checker) flowState() flowState {
	st := flowState{}
	for sc := c.scope; sc != nil; sc = sc.parent {
		for i := len(sc.narrows) - 1; i >= 0; i-- {
			n := sc.narrows[i]
			k := flowKey{n.recv, n.sym}
			if n.onlyNot {
				continue // no smart cast
			}
			if _, ok := st[k]; !ok {
				st[k] = n.t
			}
		}
	}
	return st
}

// joinFlows returns the smart casts that hold where the code paths that
// end in states meet, as the narrowings to add to the current scope, the
// one before the paths split, or one whose smart casts are those of
// every path: a value narrowed on every path has the join of its types,
// and a var that one assigns has its declared type if they don't join.
func (c *Checker) joinFlows(states []flowState) []narrow {
	if len(states) == 0 {
		return nil
	}
	var out []narrow
	for k, t := range states[0] {
		if k.sym == nil {
			continue
		}
		v := k.recv == nil && isVar(k.sym)
		for _, s := range states[1:] {
			u, ok := s[k]
			if !ok {
				if !v {
					t = types.InvalidType
					break
				}
				u = k.sym.Type
			}
			if t == u {
				continue
			}
			j, ok := c.join(t, u)
			if !ok {
				j = types.InvalidType
			}
			t = j
			if t.K == types.Invalid && !v {
				break
			}
		}
		cur := c.scope.narrowedMember(k.recv, k.sym, types.InvalidType)
		switch {
		case v && (t.K == types.Invalid || !c.subtype(t, k.sym.Type)):
			t = k.sym.Type
		case t.K == types.Invalid:
			continue
		case !v && cur.K != types.Invalid && !c.subtype(t, cur):
			continue // only narrower than it is before the paths
		}
		if t != cur {
			out = append(out, narrow{recv: k.recv, sym: k.sym, t: t})
		}
	}
	// The map's order: sort for the same narrowing each time.
	slices.SortFunc(out, func(a, b narrow) int {
		return cmpNarrow(a, b)
	})
	return out
}

// cmpNarrow orders narrowings of different values by name.
func cmpNarrow(a, b narrow) int {
	name := func(n narrow) string {
		if n.recv != nil {
			return n.recv.Name + "." + n.sym.Name
		}
		return n.sym.Name
	}
	an, bn := name(a), name(b)
	switch {
	case an < bn:
		return -1
	case an > bn:
		return 1
	}
	return 0
}

// branchExit records the smart casts where a branch ends.
type branchExit struct {
	state  flowState
	jumped bool // the branch doesn't end normally: its smart casts don't count
}

// watchExit makes the next block checked record its smart casts at its
// end in the returned exit.
func (c *Checker) watchExit(vars []*types.VarSym) *branchExit {
	e := &branchExit{}
	c.exitWatch = e
	return e
}

// record records the smart casts in the current scope, at the end of a
// block whose type is t.
func (e *branchExit) record(c *Checker, t types.Type) {
	if t.K == types.Nothing {
		e.jumped = true
		return
	}
	e.state = c.flowState()
}

// startExit returns an exit with the smart casts of the current scope
// and ns: the path past an if without else, or a when without a
// matching branch.
func (c *Checker) startExit(vars []*types.VarSym, ns []narrow) *branchExit {
	c.push()
	c.scope.narrows = ns
	e := &branchExit{}
	e.record(c, types.UnitType)
	c.pop()
	return e
}

// merge adds the smart casts that hold after the exits that don't jump,
// where they meet, for the rest of the current scope.
func (c *Checker) merge(vars []*types.VarSym, exits []*branchExit) {
	var states []flowState
	for _, e := range exits {
		if e == nil || e.jumped || e.state == nil {
			continue
		}
		states = append(states, e.state)
	}
	if len(states) == 0 {
		return // every branch jumps: the code after is unreachable
	}
	c.scope.narrows = append(c.scope.narrows, c.joinFlows(states)...)
}

// loopFlow has the smart casts where a loop's breaks and continues jump.
type loopFlow struct {
	breaks, continues []flowState
	// lost is set when a jump's smart casts are unknown: it is in a
	// lambda.
	lost bool
}

// finallyCtx is a finally block around the code being checked: a break
// or a continue out of it runs it, which may assign vars.
type finallyCtx struct {
	loops int // len(c.loops) where the try is
	vars  []*types.VarSym
}

// loopJump records the smart casts where a break or a continue jumps
// out of the loop at index i of c.loops.
func (c *Checker) loopJump(i int, brk bool) {
	loop := c.loops[i]
	if len(c.lambdas) != loop.lambdas {
		loop.flow.lost = true
		return
	}
	st := c.flowState()
	for _, f := range c.finallies {
		if f.loops > i {
			for _, v := range f.vars {
				st[flowKey{sym: v}] = v.Type
			}
		}
	}
	if brk {
		loop.flow.breaks = append(loop.flow.breaks, st)
	} else {
		loop.flow.continues = append(loop.flow.continues, st)
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

// mayNotRun undoes the smart casts that code that may not run, as the
// right side of ?:, added to the scope's narrows from index n on: casts
// are dropped, unless other, those that hold where it doesn't run, has
// them too, as x ?: x!!, and the values it assigns lose theirs.
func (c *Checker) mayNotRun(n int, other []narrow) {
	ns := c.scope.narrows
	if n >= len(ns) {
		return
	}
	added := slices.Clone(ns[n:])
	c.scope.narrows = ns[:n]
	for _, k := range added {
		if k.onlyNot {
			continue
		}
		k.not = nil // values it isn't may not hold either
		prev := c.scope.narrowedMember(k.recv, k.sym, k.sym.Type)
		if k.t == prev || c.subtype(k.t, prev) {
			// Both ways, the wider of the two.
			if o, ok := findNarrow(other, k); ok && (c.subtype(k.t, o) || c.subtype(o, k.t)) {
				if c.subtype(k.t, o) {
					k.t = o
				}
				c.scope.narrows = append(c.scope.narrows, k)
			}
			continue
		}
		k.t = k.sym.Type
		c.scope.narrows = append(c.scope.narrows, k)
	}
}

// superArgNarrows returns the smart casts of the x!! and the casts in the
// arguments of an object expression's superclass constructor call, which
// run where it is: after object : A(o!!) {}, o isn't null.
func (c *Checker) superArgNarrows(x *syntax.ObjectExpr) []narrow {
	var out []narrow
	for _, st := range x.Decl.Supers {
		if st.Call == nil {
			continue
		}
		for _, a := range st.Call.Args {
			if _, ok := a.(*syntax.Lambda); ok {
				continue
			}
			out = append(out, c.ranNarrows(a)...)
		}
	}
	// And of those in its initializers, which also run where it is.
	for _, m := range x.Decl.Members {
		switch m := m.(type) {
		case *syntax.PropDecl:
			if m.Init != nil {
				out = append(out, c.ranNarrows(m.Init)...)
			}
		case *syntax.InitBlock:
			for _, st := range m.Body.Stmts {
				if es, ok := st.(*syntax.ExprStmt); ok {
					out = append(out, c.ranNarrows(es.X)...)
				}
			}
		}
	}
	return out
}

// ranNarrows returns the smart casts of the x!! and the casts that
// evaluating x always runs.
func (c *Checker) ranNarrows(x syntax.Expr) []narrow {
	out := c.asserted(x)
	alwaysCasts(x, func(as *syntax.As) {
		if k, ok := c.castNarrow(as); ok {
			out = append(out, k)
		}
	})
	return out
}

// chainKeyOf is narrowKeyOf that also takes a?.b, as a.b, which a?.b
// != null smart casts.
func (c *Checker) chainKeyOf(x syntax.Expr) (narrow, bool) {
	if m, ok := x.(*syntax.Member); ok && m.Safe {
		return c.memberKey(m, m.Type())
	}
	return c.narrowKeyOf(x)
}

// notNullOf returns the smart cast of stable value x to its type without
// null, which holds where it isn't null.
func (c *Checker) notNullOf(x syntax.Expr) []narrow {
	if k, ok := c.narrowKeyOf(x); ok && k.t.K != types.Invalid {
		k.t = k.t.NonNull()
		return []narrow{k}
	}
	return nil
}

// assignedVal narrows a val property that an initializer block or a
// constructor assigns, x = v, to v's type, as assignedVar a local var.
func (c *Checker) assignedVal(target syntax.Expr, vt types.Type) {
	if vt.K == types.Invalid || vt.K == types.Null || vt.K == types.Nothing {
		return
	}
	k, ok := c.narrowKeyOf(target)
	if !ok || k.sym.Kind != types.Prop || k.sym.Mutable || !c.subtype(vt, k.sym.Type) {
		return
	}
	k.t = vt
	c.scope.narrows = append(c.scope.narrows, k)
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
	if m.Prop == nil || !c.stableOn(m.Prop, m.X.Type()) {
		return narrow{}, false
	}
	if this, ok := m.X.(*syntax.This); ok && this.Recv == nil {
		// this.p is p, as this@C.p in class C.
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
	return c.stableOn(p, types.InvalidType)
}

// stableOn is stableMember for p read through a value of type recv: an
// open val is stable too if recv's class is final and inherits it, as
// x.p for an x: B, where B doesn't override A's open val p.
func (c *Checker) stableOn(p *types.VarSym, recv types.Type) bool {
	if p.Kind != types.Prop || p.Mutable || p.Getter != nil || !p.HasField() || p.Owner == nil || p.Owner.External {
		return false
	}
	if p.Final || p.Owner.Final && p.Owner.Entries == nil {
		return true
	}
	r := recv.NonNull()
	if r.K != types.Class || r.C == nil {
		return false
	}
	r.C.Ensure()
	return r.C.Final && r.C.Entries == nil && !r.C.Interface && r.C.IsSubclassOf(p.Owner) && c.memberProp(r.C, p.Name) == p
}

// thisType returns the type of this where the code is, or InvalidType.
func (c *Checker) thisType() types.Type {
	if c.this == nil {
		return types.InvalidType
	}
	return c.this.Type
}

// isNull reports whether x is null: the literal, or a value of type
// Nothing?, whose only value it is.
func isNull(x syntax.Expr) bool {
	if _, ok := x.(*syntax.NullLit); ok {
		return true
	}
	t := x.Type()
	return t.K == types.Null || t.K == types.Nothing && t.N
}

// boolEquality returns the smart casts of c == true and the like: those
// of c, or of !c, when one side is a Boolean literal and the other a
// condition.
func (c *Checker) boolEquality(x *syntax.Binary) (whenTrue, whenFalse []narrow, ok bool) {
	lit, cond := x.X, x.Y
	l, isLit := lit.(*syntax.BoolLit)
	if !isLit {
		lit, cond = x.Y, x.X
		if l, isLit = lit.(*syntax.BoolLit); !isLit {
			return nil, nil, false
		}
	}
	if cond.Type().Flex() != types.BoolType {
		return nil, nil, false
	}
	whenTrue, whenFalse = c.narrowing(cond)
	if l.Val != (x.Op == syntax.EQ || x.Op == syntax.EQ_EQ_EQ) {
		whenTrue, whenFalse = whenFalse, whenTrue
	}
	return whenTrue, whenFalse, true
}

// loopEntry returns the smart casts before a loop that assigns vars, for
// loopHead, or nil if there are no vars.
func (c *Checker) loopEntry(vars []*types.VarSym) flowState {
	if len(vars) == 0 {
		return nil
	}
	return c.flowState()
}

// loopHead returns the smart casts of the vars a loop assigns where its
// condition cond is checked, for the path out of it when cond is false:
// as with kotlinc, which checks the loop once, its body sees them with
// their declared types, but the paths to its condition, from before the
// loop (pre) and from the end of its body (end) and its continues, join
// as the branches of an if do. After
//
//	if (x != null) while (b()) { x = x.swap() }
//
// x is an Int if each swap() gives one. A var the condition assigns
// keeps its declared type.
func (c *Checker) loopHead(vars []*types.VarSym, cond syntax.Expr, pre flowState, end *branchExit, loop *loopCtx) []narrow {
	if pre == nil || loop.flow.lost {
		return nil
	}
	states := []flowState{pre}
	if !end.jumped && end.state != nil {
		states = append(states, end.state)
	}
	states = append(states, loop.flow.continues...)
	var inCond []string
	if cond != nil {
		inCond = c.assigns(cond)
	}
	var out []narrow
	for _, n := range c.joinFlows(states) {
		if n.recv == nil && slices.Contains(vars, n.sym) && !slices.Contains(inCond, n.sym.Name) {
			out = append(out, n)
		}
	}
	return out
}

// eitherNarrows returns the smart casts that hold when either of two
// conditions' smart casts do: those of a || b being true, a's when it is
// true (first) or b's when a is false and b true (ctx, the smart casts of
// a being false, and second). A value narrowed on both paths has the join
// of its types, as after a is B || a is C, where a has their common
// supertype. a && b being false is the same with the sides' negations.
func (c *Checker) eitherNarrows(first, ctx, second []narrow) []narrow {
	var out []narrow
	for _, n := range first {
		if n.onlyNot {
			other := findNot(second, n)
			if other == nil {
				other = findNot(ctx, n)
			}
			if not := bothCover(n.not, other); not != nil {
				out = append(out, narrow{recv: n.recv, sym: n.sym, t: n.t, not: not, onlyNot: true})
			}
			continue
		}
		other := second
		u, ok := findNarrow(second, n)
		if !ok {
			if u, ok = findNarrow(ctx, n); !ok {
				continue
			}
			other = ctx
		}
		// The values it isn't on both paths: C after a is A || a is B,
		// for a sealed class of A, B and C.
		not := bothCover(n.not, findNot(other, n))
		t, typed := n.t, true
		if u != t {
			j, ok := c.join(t, u)
			// x is A || x is B doesn't smart cast x to their common
			// supertype where it can't be seen: a private interface of
			// another file.
			if !ok || j.K == types.Invalid || j.K == types.Class && j.C != nil && !c.classVisible(j.C) {
				typed = false
			}
			t = j
		}
		cur := c.scope.narrowedMember(n.recv, n.sym, n.sym.Type)
		if n.recv != nil {
			cur = c.scope.narrowedMember(n.recv, n.sym, types.InvalidType)
		}
		if !typed || t == cur || cur.K != types.Invalid && !c.subtype(t, cur) {
			if not == nil || cur.K == types.Invalid {
				continue
			}
			out = append(out, narrow{recv: n.recv, sym: n.sym, t: cur, not: not, onlyNot: true})
			continue
		}
		out = append(out, narrow{recv: n.recv, sym: n.sym, t: t, not: not})
	}
	return out
}

// findNot returns the values that the last smart cast in ns of the value
// n narrows says it isn't, or nil.
func findNot(ns []narrow, n narrow) *coverage {
	for i := len(ns) - 1; i >= 0; i-- {
		if ns[i].sym == n.sym && ns[i].recv == n.recv {
			return ns[i].not
		}
	}
	return nil
}

// bothCover returns what both a and b cover, or nil if that is nothing.
func bothCover(a, b *coverage) *coverage {
	if a == nil || b == nil {
		return nil
	}
	out := &coverage{null: a.null && b.null, isTrue: a.isTrue && b.isTrue, isFalse: a.isFalse && b.isFalse}
	for e := range a.entries {
		if b.entries[e] {
			out.entry(e)
		}
	}
	for _, k := range a.classes {
		if slices.Contains(b.classes, k) {
			out.classes = append(out.classes, k)
		}
	}
	if out.empty() {
		return nil
	}
	return out
}

// findNarrow returns the last smart cast in ns of the value n narrows.
func findNarrow(ns []narrow, n narrow) (types.Type, bool) {
	for i := len(ns) - 1; i >= 0; i-- {
		if ns[i].sym == n.sym && ns[i].recv == n.recv && !ns[i].onlyNot {
			return ns[i].t, true
		}
	}
	return types.Type{}, false
}

// excluding returns ns with the values in out added to those that the
// value k keys is known not to be: on its last smart cast in ns, or on one
// of its own of its type here, k.t.
func (c *Checker) excluding(ns []narrow, k narrow, out *coverage) []narrow {
	not := &coverage{}
	if prior := c.scope.excluded(k.recv, k.sym); prior != nil {
		not.merge(prior)
	}
	not.merge(out)
	ns = slices.Clone(ns)
	for i := len(ns) - 1; i >= 0; i-- {
		if ns[i].sym == k.sym && ns[i].recv == k.recv {
			if ns[i].not != nil {
				not.merge(ns[i].not)
			}
			ns[i].not = not
			return ns
		}
	}
	k.not, k.onlyNot = not, true
	return append(ns, k)
}

// equalExclusions adds to the smart casts of a == b (or ===, != and !==)
// what it tells about the values a stable value on one side isn't, when
// the other is a constant a when can compare it with (true, an enum entry,
// an object): where they differ, the constant, and where a Boolean equals
// true or false, the other one and null. A when over the value needn't
// cover those, as after if (e == E.A) return.
func (c *Checker) equalExclusions(x *syntax.Binary, whenTrue, whenFalse []narrow) ([]narrow, []narrow) {
	for _, side := range [2][2]syntax.Expr{{x.X, x.Y}, {x.Y, x.X}} {
		var lit coverage
		lit.add(side[1])
		lit.null = false // x != null is a smart cast already
		if lit.empty() {
			continue
		}
		k, ok := c.narrowKeyOf(side[0])
		if !ok {
			continue
		}
		equal, differ := &whenTrue, &whenFalse
		if x.Op == syntax.NE || x.Op == syntax.NOT_EQ_EQ {
			equal, differ = differ, equal
		}
		*differ = c.excluding(*differ, k, &lit)
		*equal = c.excluding(*equal, k, otherValues(k.t, &lit))
		break
	}
	return whenTrue, whenFalse
}

// otherValues returns the values of type t, Boolean, an enum or a sealed
// class, that a value equal to the constant lit covers isn't: null, the
// other Boolean, the other entries, or the subclasses its class isn't.
func otherValues(t types.Type, lit *coverage) *coverage {
	out := &coverage{null: true}
	t = t.NonNull()
	switch {
	case lit.isTrue != lit.isFalse:
		out.isTrue, out.isFalse = lit.isFalse, lit.isTrue
	case t.K != types.Class:
	case len(lit.entries) == 1:
		t.C.Ensure()
		for _, e := range t.C.Entries {
			if !lit.entries[e] {
				out.entry(e)
			}
		}
	case len(lit.classes) == 1:
		otherSubclasses(t.C, lit.classes[0], out)
	}
	return out
}

// otherSubclasses adds to out the subclasses of sealed class cls that an
// instance of k isn't.
func otherSubclasses(cls, k *types.ClassSym, out *coverage) {
	if !cls.Sealed || cls.IsSubclassOf(k) {
		return
	}
	for _, s := range cls.Subclasses {
		switch {
		case s.IsSubclassOf(k):
		case k.IsSubclassOf(s):
			otherSubclasses(s, k, out)
		default:
			out.classes = append(out.classes, s)
		}
	}
}

// notNullCast returns the smart cast of (a as T)!! or (a as? T)!!, x
// being the cast: a is a T and not null for the code after it. Of
// a?.b!! and a?.f()!!, it returns those of a and a.b not being null.
func (c *Checker) notNullCast(x syntax.Expr) []narrow {
	as, ok := x.(*syntax.As)
	if !ok {
		out := c.safeReceivers(x)
		if m, ok := x.(*syntax.Member); ok && m.Safe {
			if k, ok := c.memberKey(m, m.Type()); ok && types.MayBeNull(k.t) {
				k.t = k.t.NonNull()
				out = append(out, k)
			}
		}
		return out
	}
	k, ok := c.castNarrow(as)
	if !ok {
		if k, ok = c.narrowKeyOf(as.X); !ok {
			return nil
		}
	}
	k.t = k.t.NonNull()
	return []narrow{k}
}

// safeArgCasts returns the smart casts of the casts and the x!! in the
// arguments of a safe call x, a?.f(b as T), which run if it isn't null:
// after a?.f(b as T) != null, b is a T.
func (c *Checker) safeArgCasts(x syntax.Expr) []narrow {
	call, ok := x.(*syntax.Call)
	if !ok {
		return nil
	}
	var out []narrow
	for _, a := range call.Args {
		if _, ok := a.(*syntax.Lambda); ok {
			continue
		}
		alwaysCasts(a, func(as *syntax.As) {
			if k, ok := c.castNarrow(as); ok {
				out = append(out, k)
			}
		})
		out = append(out, c.asserted(a)...)
	}
	return out
}

// tryAssigned returns the smart casts of vars, which blocks of a try
// assign, in its catch and finally blocks, which may run after any of
// those assignments: as with kotlinc, a var has the join of its type
// before the try and the types of the values assigned to it, String
// after s = "" before and in the try. It is the declared type when the
// blocks assign it other than with =, or in nested code.
func (c *Checker) tryAssigned(vars []*types.VarSym, blocks ...*syntax.Block) []narrow {
	var out []narrow
	for _, v := range vars {
		t := c.scope.narrowed(v)
		ok := true
		for _, b := range blocks {
			if b == nil || !ok {
				continue
			}
			inspect(b, func(n any) bool {
				if !ok {
					return false
				}
				switch n := n.(type) {
				case *syntax.Lambda, *syntax.LocalFun, *syntax.ObjectExpr, *syntax.LocalClass:
					if assignedNames(n)[v.Name] {
						ok = false
					}
					return false
				case *syntax.Unary:
					if name, is := assignedName(n); is && name == v.Name {
						ok = false
					}
				case *syntax.Assign:
					id, is := n.Target.(*syntax.Ident)
					if !is || id.Name != v.Name || id.Sym != nil && id.Sym != v {
						break
					}
					vt := n.Value.Type()
					if n.Op != syntax.ASSIGN || vt.K == types.Invalid || vt.K == types.Nothing || !c.subtype(vt, v.Type) {
						ok = false
						break
					}
					if vt.K == types.Null {
						vt = v.Type // assignedVar's
					}
					j, joined := c.join(t, vt)
					if !joined || j.K == types.Invalid {
						ok = false
						break
					}
					t = j
				}
				return ok
			})
		}
		if ok && t != v.Type && c.subtype(t, v.Type) {
			out = append(out, narrow{sym: v, t: t})
		}
	}
	return out
}

// onlyVars returns the smart casts in ns of the vars in vars.
func onlyVars(ns []narrow, vars []*types.VarSym) []narrow {
	var out []narrow
	for _, n := range ns {
		if n.recv == nil && slices.Contains(vars, n.sym) {
			out = append(out, n)
		}
	}
	return out
}

// elvisNarrowing returns the smart casts of x ?: false being true, and of
// x ?: true being false: x is then the value, so it isn't null, nor are
// its safe calls' receivers, and its own condition's smart casts hold.
func (c *Checker) elvisNarrowing(x *syntax.Binary) (whenTrue, whenFalse []narrow) {
	b, ok := x.Y.(*syntax.BoolLit)
	if !ok {
		return nil, nil
	}
	t, f := c.narrowing(x.X)
	n := c.equalNonNull(x.X)
	if b.Val {
		return nil, append(f[:len(f):len(f)], n...)
	}
	return append(t[:len(t):len(t)], n...), nil
}
