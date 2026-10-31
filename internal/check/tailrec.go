package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// tailCalls marks the calls of tailrec function d to itself that are in
// tail position, which codegen turns into a jump back to the start of its
// body, as kotlinc does: the value of a return, of the expression body,
// or of the last statement of the body, through if and when branches.
// Calls in lambdas, local functions, object expressions and try blocks
// aren't, and stay calls: kotlinc warns about them, kc has no warnings.
func (c *Checker) tailCalls(d *syntax.FunDecl) {
	sym := d.Sym
	if sym.Suspend {
		return
	}
	var tail func(e syntax.Expr)
	tailBlock := func(b *syntax.Block) {
		if b == nil || len(b.Stmts) == 0 {
			return
		}
		if s, ok := b.Stmts[len(b.Stmts)-1].(*syntax.ExprStmt); ok {
			tail(s.X)
		}
	}
	tail = func(e syntax.Expr) {
		switch e := e.(type) {
		case *syntax.Call:
			if e.Sym == sym && selfCall(e, sym) {
				e.Tail = true
			}
		case *syntax.If:
			tailBlock(e.Then)
			tailBlock(e.Else)
		case *syntax.When:
			for _, br := range e.Branches {
				tailBlock(br.Body)
			}
		case *syntax.Jump:
			if e.Kind == syntax.RETURN && e.Label == "" {
				tail(e.X)
			}
		}
	}
	walk := func(n any) bool {
		switch n := n.(type) {
		case *syntax.Lambda, *syntax.LocalFun, *syntax.ObjectExpr, *syntax.Try:
			return false
		case *syntax.Jump:
			tail(n)
		}
		return true
	}
	if d.ExprBody != nil {
		syntax.Inspect(d.ExprBody, walk)
		tail(d.ExprBody)
	} else if d.Body != nil {
		syntax.Inspect(d.Body, walk)
		tailBlock(d.Body)
	}
}

// selfCall reports whether call of f has f's own receivers: this, or for
// a top-level extension function any receiver, which the jump passes as
// the new one.
func selfCall(call *syntax.Call, f *types.FunSym) bool {
	switch fn := call.Fun.(type) {
	case *syntax.Ident:
		return true
	case *syntax.Member:
		if _, ok := fn.X.(*syntax.This); ok && !fn.Safe {
			return true
		}
		return f.Ext && f.Owner == nil && !fn.Safe && fn.Dispatch == nil
	}
	return false
}
