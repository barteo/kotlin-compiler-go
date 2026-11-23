package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports, with -Wextra, the code after a statement that always
// jumps, as kotlinc's extra UnreachableCodeChecker does: UNREACHABLE_CODE
// at each statement that can't run, and at one that jumps before it
// completes, as val s = todo(), whose declaration never happens. It
// follows how kotlinc marks the code, which reports the condition and
// branch statements of an if or a when that can't run, but only the
// start of a loop, a try, a lambda or a local declaration, and the start
// of each property delegate in it. A class whose superclass constructor
// call jumps, class C : B(todo()), has its initializers reported.

// noteJump records that statement jump of block b always jumps, or, if it
// is -1, that none does: b's check may be repeated.
func (c *Checker) noteJump(b *syntax.Block, jump int) {
	if jump < 0 {
		delete(c.blockJumps, b)
		return
	}
	if c.blockJumps == nil {
		c.blockJumps = map[*syntax.Block]int{}
	}
	c.blockJumps[b] = jump
}

// unreachableWarnings reports the unreachable code of block b, which
// isn't in code reported already.
func (c *Checker) unreachableWarnings(b *syntax.Block) {
	if c.deadBlocks[b] {
		return
	}
	i := c.jumpingStmt(b)
	if i < 0 {
		return
	}
	c.partlyDead(b.Stmts[i])
	for _, s := range b.Stmts[i+1:] {
		c.deadStmt(s)
	}
}

// jumpingStmt returns the index of the statement of block b that always
// jumps in kotlinc's graph, by kc's judgment (stmtJumps) or not: x::f of
// an x that jumps, an object expression whose superclass's constructor
// call jumps. A loop is one only if kc found it never ends. It returns -1
// if there is none.
func (c *Checker) jumpingStmt(b *syntax.Block) int {
	n, j := len(b.Stmts), -1
	if k, ok := c.blockJumps[b]; ok {
		n, j = k+1, k
	}
	return slices.IndexFunc(b.Stmts[:n], func(s syntax.Stmt) bool {
		_, loop := s.(*syntax.While)
		return (!loop || j >= 0 && s == b.Stmts[j]) && c.stmtCFGJumps(s)
	})
}

// deadDoWhileCond reports the condition of do-while loop s if its body
// always jumps, but not by a continue of s, which goes to the condition.
func (c *Checker) deadDoWhileCond(s *syntax.While) {
	if c.deadBlocks[s.Body] || c.jumpingStmt(s.Body) < 0 {
		return
	}
	cont := false
	syntax.Inspect(s.Body, func(n any) bool {
		if j, ok := n.(*syntax.Jump); ok && j.Kind == syntax.CONTINUE && j.Loop == s {
			cont = true
		}
		return !cont
	})
	if !cont {
		c.unreachable(syntax.StartPos(s.Cond))
		c.markDead(s.Cond)
	}
}

// partlyDead reports the part of statement s, which always jumps, that
// can't run.
func (c *Checker) partlyDead(s syntax.Stmt) {
	report := false
	switch s := s.(type) {
	case *syntax.ExprStmt:
		switch x := s.X.(type) {
		case *syntax.If:
			if c.cfgJumps(x.Cond, false) {
				c.deadBlock(x.Then)
				c.deadBlock(x.Else)
			}
			return
		case *syntax.When:
			if c.cfgJumps(x.Subject, false) {
				c.deadBranches(x)
			}
			return
		case *syntax.Jump:
			// return todo(), but not return null!!, whose null!! takes
			// the function's return type, if it isn't Unit.
			report = c.cfgJumps(x.X, true)
		default:
			report = c.operandJumps(s.X)
		}
	case *syntax.While:
		report = !s.Do && c.cfgJumps(s.Cond, false) // not one that never ends
	default:
		report = c.stmtCFGJumps(s)
	}
	if report {
		p := syntax.StmtPos(s)
		if es, ok := s.(*syntax.ExprStmt); ok {
			if l, ok := es.X.(*syntax.Lambda); ok && l.Ref != nil && l.Ref.X != nil {
				p = syntax.StartPos(l.Ref.X) // x::f, which starts at x
			}
		}
		c.unreachable(p)
	}
}

// stmtCFGJumps reports whether statement s, which kc found always jumps,
// does so in kotlinc's control flow graph too (see cfgJumps). A loop kc
// found never ends is trusted.
func (c *Checker) stmtCFGJumps(s syntax.Stmt) bool {
	switch s := s.(type) {
	case *syntax.ExprStmt:
		return c.cfgJumps(s.X, false)
	case *syntax.VarStmt:
		return s.Delegate == nil && c.cfgJumps(s.Init, declaredNotNothing(s))
	case *syntax.Destructure:
		return c.cfgJumps(s.Init, false)
	case *syntax.Assign:
		return c.assignJumps(s)
	case *syntax.For:
		return c.cfgJumps(s.Range, false)
	case *syntax.While:
		return true
	}
	return false
}

// assignJumps reports whether assignment s always jumps before it
// assigns. Its value has the target's type as expected type.
func (c *Checker) assignJumps(s *syntax.Assign) bool {
	switch t := s.Target.(type) {
	case *syntax.Member:
		if c.cfgJumps(t.X, false) {
			return true
		}
	case *syntax.Index:
		if c.cfgJumps(t.X, false) || !hasSafeCall(t.X) && c.anyCFGJumps(t.Args) {
			return true
		}
	}
	return c.cfgJumps(s.Value, true)
}

// cfgJumps reports whether evaluating e always jumps as kotlinc's control
// flow graph has it, which jumps differs from: there, x as Nothing doesn't
// jump, nor does x!! where an expected type, as of val y: Int = null!!,
// gives it that type instead of Nothing. expected is set when e has one.
func (c *Checker) cfgJumps(e syntax.Expr, expected bool) bool {
	if e == nil {
		return false
	}
	switch e := e.(type) {
	case *syntax.As:
		return c.cfgJumps(e.X, false)
	case *syntax.NotNull:
		if expected {
			return c.cfgJumps(e.X, false)
		}
	case *syntax.If:
		// Its branches don't get its expected type.
		return c.cfgJumps(e.Cond, false) ||
			e.Else != nil && c.blockCFGJumps(e.Then) && c.blockCFGJumps(e.Else)
	case *syntax.When:
		if c.cfgJumps(e.Subject, false) {
			return true
		}
		exhaustive := c.coveredWhens[e]
		for _, br := range e.Branches {
			exhaustive = exhaustive || br.Else && br.Guard == nil
			if !c.blockCFGJumps(br.Body) {
				return false
			}
		}
		return exhaustive
	case *syntax.Try:
		if c.blockCFGJumps(e.Finally) {
			return true
		}
		if !c.blockCFGJumps(e.Body) {
			return false
		}
		for _, ct := range e.Catches {
			if !c.blockCFGJumps(ct.Body) {
				return false
			}
		}
		return true
	case *syntax.Checked:
		return c.cfgJumps(e.X, expected)
	case *syntax.Lambda:
		return e.Ref != nil && c.operandJumps(e) // x::f, not a lambda
	case *syntax.Jump, *syntax.Call, *syntax.Member, *syntax.Index, *syntax.Binary, *syntax.Unary, *syntax.Is,
		*syntax.StringLit, *syntax.Ident, *syntax.This, *syntax.ObjectExpr:
		// A variable of type Nothing too: reading it jumps.
	default:
		return false // not judged: a lambda, an object expression, ...
	}
	if t := e.Type(); t.K == types.Nothing && !t.N {
		return true
	}
	return c.operandJumps(e)
}

// declaredNotNothing reports whether variable s is declared with a type
// other than Nothing, which its initializer has as expected type.
func declaredNotNothing(s *syntax.VarStmt) bool {
	return s.Type != nil && (s.Sym == nil || s.Sym.Type.K != types.Nothing)
}

// blockCFGJumps reports whether block b, an if's or a when's branch or a
// try's block or catch, always jumps (see cfgJumps).
func (c *Checker) blockCFGJumps(b *syntax.Block) bool {
	return b != nil && slices.ContainsFunc(b.Stmts, c.stmtCFGJumps)
}

func (c *Checker) anyCFGJumps(es []syntax.Expr) bool {
	for _, e := range es {
		if c.cfgJumps(e, false) {
			return true
		}
	}
	return false
}

// operandJumps reports whether evaluating e always jumps before e itself
// is: an expression it always evaluates first jumps, as todo() in
// todo().foo() or bar(todo()), as kotlinc's control flow graph has it
// (see cfgJumps). That a lambda called in place jumps, as in
// run { return }, doesn't count: the call is where it jumps from.
func (c *Checker) operandJumps(e syntax.Expr) bool {
	switch e := e.(type) {
	case *syntax.Call:
		if e.Invoke {
			if c.cfgJumps(e.Fun, false) {
				return true
			}
		} else if m, ok := e.Fun.(*syntax.Member); ok {
			if c.cfgJumps(m.X, false) {
				return true
			}
			if hasSafeCall(m) {
				return false
			}
		}
		for _, a := range e.Args {
			if _, ok := a.(*syntax.Lambda); !ok && c.cfgJumps(a, false) {
				return true
			}
		}
	case *syntax.Member:
		return c.cfgJumps(e.X, false)
	case *syntax.Index:
		return c.cfgJumps(e.X, false) || !hasSafeCall(e.X) && c.anyCFGJumps(e.Args)
	case *syntax.Binary:
		return c.cfgJumps(e.X, false) || e.Op != syntax.AND && e.Op != syntax.OR && e.Op != syntax.ELVIS && c.cfgJumps(e.Y, false)
	case *syntax.Unary:
		return c.cfgJumps(e.X, false)
	case *syntax.As:
		return c.cfgJumps(e.X, false)
	case *syntax.Is:
		return c.cfgJumps(e.X, false)
	case *syntax.NotNull:
		return c.cfgJumps(e.X, false)
	case *syntax.StringLit:
		for _, p := range e.Parts {
			if c.cfgJumps(p.X, false) {
				return true
			}
		}
	case *syntax.Checked:
		return c.operandJumps(e.X)
	case *syntax.Lambda:
		// (break)::toString: the receiver is evaluated first.
		if r := e.Ref; r != nil {
			return r.X != nil && c.cfgJumps(r.X, false) || r.BoundExpr != nil && c.cfgJumps(r.BoundExpr, false)
		}
	case *syntax.ObjectExpr:
		// object : Foo(todo()) {}: the superclass's constructor call.
		return e.Decl != nil && c.superCallJumps(e.Decl) != nil
	}
	return false
}

// superCallJumps returns the call of class d's superclass's constructor
// in its header, B(x), if an argument of it always jumps, or nil.
func (c *Checker) superCallJumps(d *syntax.ClassDecl) *syntax.Call {
	for _, st := range d.Supers {
		if st.Call != nil && c.operandJumps(st.Call) {
			return st.Call
		}
	}
	return nil
}

// deadStmt reports statement s, which can't run.
func (c *Checker) deadStmt(s syntax.Stmt) {
	if es, ok := s.(*syntax.ExprStmt); ok {
		switch x := es.X.(type) {
		case *syntax.If:
			c.unreachable(syntax.StartPos(x.Cond))
			c.markDead(x.Cond)
			c.deadBlock(x.Then)
			c.deadBlock(x.Else)
			return
		case *syntax.When:
			if x.Subject != nil {
				c.unreachable(syntax.StartPos(x.Subject))
				c.markDead(x.Subject)
			}
			c.deadBranches(x)
			return
		}
	}
	c.markDead(s)
	switch s := s.(type) {
	case *syntax.LocalClass:
		c.unreachable(s.Decl.P)
	default:
		c.unreachable(syntax.StmtPos(s))
	}
	c.deadDelegates(s)
}

// deadDelegates reports each property delegate in statement s, which
// can't run, as kotlinc does, at its start: val x by lazy { }, in a
// lambda, a local class or function or an object expression too.
func (c *Checker) deadDelegates(s syntax.Stmt) {
	seen := map[*syntax.ClassDecl]bool{}
	var members func(d *syntax.ClassDecl)
	members = func(d *syntax.ClassDecl) {
		if d == nil || seen[d] {
			return
		}
		seen[d] = true
		for _, m := range d.Members {
			switch m := m.(type) {
			case *syntax.PropDecl:
				if m.DelegateOf != nil && m.Init != nil {
					c.unreachable(syntax.StartPos(m.Init))
				}
			case *syntax.ClassDecl:
				members(m)
			}
		}
	}
	// walkCode enters the local classes and object expressions in the
	// code it walks; syntax.Inspect doesn't.
	see := func(n any) bool {
		switch n := n.(type) {
		case *syntax.VarStmt:
			if n.Delegate != nil {
				c.unreachable(syntax.StartPos(n.Delegate))
			}
		case *syntax.LocalClass:
			members(n.Decl)
		case *syntax.ObjectExpr:
			members(n.Decl)
		}
		return true
	}
	syntax.Inspect(s, func(n any) bool {
		see(n)
		var d *syntax.ClassDecl
		switch n := n.(type) {
		case *syntax.LocalClass:
			d = n.Decl
		case *syntax.ObjectExpr:
			d = n.Decl
		}
		if d != nil {
			walkCode([]syntax.Decl{d}, see)
		}
		return true
	})
}

// deadBranches reports the branches of when x, which can't run.
func (c *Checker) deadBranches(x *syntax.When) {
	for _, br := range x.Branches {
		for _, cond := range br.Conds {
			if cond.X != nil {
				c.unreachable(syntax.StartPos(cond.X))
				c.markDead(cond.X)
			} else {
				c.unreachable(cond.P)
			}
		}
		if br.Guard != nil {
			c.markDead(br.Guard)
		}
		c.deadBlock(br.Body)
	}
}

// deadBlock reports the statements of block b, which can't run.
func (c *Checker) deadBlock(b *syntax.Block) {
	if b == nil {
		return
	}
	c.markBlockDead(b)
	for _, s := range b.Stmts {
		c.deadStmt(s)
	}
}

// markDead marks the blocks in n as unreachable code, reported already.
func (c *Checker) markDead(n any) {
	syntax.Inspect(n, func(n any) bool {
		switch n := n.(type) {
		case *syntax.Block:
			c.markBlockDead(n)
		case *syntax.ClassDecl:
			if c.deadClasses == nil {
				c.deadClasses = map[*syntax.ClassDecl]bool{}
			}
			c.deadClasses[n] = true
		}
		return true
	})
}

func (c *Checker) markBlockDead(b *syntax.Block) {
	if c.deadBlocks == nil {
		c.deadBlocks = map[*syntax.Block]bool{}
	}
	c.deadBlocks[b] = true
}

func (c *Checker) unreachable(p syntax.Pos) {
	if p.Line > 0 {
		c.warnf(p, "UNREACHABLE_CODE", "unreachable code")
	}
}

// classDeadCode reports the code of class d that can't run, and of the
// classes nested in it, once: the initializers, init blocks and
// secondary constructors' bodies that its initialization runs after a
// property's initializer or an init block that always jumps, as with
// kotlinc, which judges a class's initialization as one graph.
func (c *Checker) classDeadCode(d *syntax.ClassDecl, done map[*syntax.ClassDecl]bool) {
	if d == nil || done[d] || d.Generated || d.Synthetic || c.deadClasses[d] {
		return
	}
	done[d] = true
	dead := false
	if call := c.superCallJumps(d); call != nil && !d.Anonymous {
		// class C : B(todo()): the call, then the initializers. An object
		// expression's is reported with the code it is in.
		c.unreachable(syntax.StartPos(call))
		dead = true
	}
	for _, m := range d.Members {
		switch m := m.(type) {
		case *syntax.ClassDecl:
			c.classDeadCode(m, done)
		case *syntax.PropDecl:
			switch {
			case m.Init == nil || m.Generated:
			case dead:
				c.deadInit(m.Init)
			case m.DelegateOf != nil:
				dead = c.cfgJumps(m.Init, false)
			default:
				dead = c.cfgJumps(m.Init, m.Type != nil && (m.Sym == nil || m.Sym.Type.K != types.Nothing))
			}
		case *syntax.InitBlock:
			if dead {
				c.deadBlock(m.Body)
			} else {
				dead = c.blockCFGJumps(m.Body)
			}
		}
	}
	if !dead {
		return
	}
	for _, m := range d.Members {
		if s, ok := m.(*syntax.SecondaryCtor); ok && s.Serial == nil {
			// Its body runs after the class's initializers.
			c.unreachable(s.P)
			c.deadBlock(s.Body)
		}
	}
}

// deadInit reports the initializer or delegate e of a property, which
// can't run, where kotlinc does: at its start and, of a lambda or a call
// with a trailing one, as lazy { }, at the lambda's closing brace; of an
// object expression, at the closing braces of its properties' lambdas,
// and a delegate call's start.
func (c *Checker) deadInit(e syntax.Expr) {
	c.unreachable(syntax.StartPos(e))
	if o, ok := e.(*syntax.ObjectExpr); ok && o.Decl != nil {
		for _, m := range o.Decl.Members {
			if pd, ok := m.(*syntax.PropDecl); ok && pd.Init != nil {
				if l := trailingLambda(pd.Init); l != nil {
					if _, call := pd.Init.(*syntax.Call); call {
						c.unreachable(syntax.StartPos(pd.Init))
					}
					c.unreachable(l.Body.End)
				}
			}
		}
	} else if l := trailingLambda(e); l != nil {
		c.unreachable(l.Body.End)
	}
	c.markDead(e)
}

// trailingLambda returns e if it is a lambda or an anonymous function,
// or the lambda that call e takes last, or nil.
func trailingLambda(e syntax.Expr) *syntax.Lambda {
	switch e := e.(type) {
	case *syntax.Lambda:
		if e.Body != nil {
			return e
		}
	case *syntax.Call:
		if len(e.Args) > 0 {
			if l, ok := e.Args[len(e.Args)-1].(*syntax.Lambda); ok && l.Body != nil {
				return l
			}
		}
	}
	return nil
}
