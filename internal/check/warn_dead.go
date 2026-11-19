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
// start of a loop, a try, a lambda or a local declaration.

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
	i, ok := c.blockJumps[b]
	if !ok || c.deadBlocks[b] || !c.stmtCFGJumps(b.Stmts[i]) {
		return
	}
	c.partlyDead(b.Stmts[i])
	for _, s := range b.Stmts[i+1:] {
		c.deadStmt(s)
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
		c.unreachable(syntax.StmtPos(s))
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
	case *syntax.Jump, *syntax.Call, *syntax.Member, *syntax.Index, *syntax.Binary, *syntax.Unary, *syntax.Is,
		*syntax.StringLit:
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
	}
	return false
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
		if b, ok := n.(*syntax.Block); ok {
			c.markBlockDead(b)
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
