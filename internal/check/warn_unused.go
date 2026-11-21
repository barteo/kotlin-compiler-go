package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports kotlinc's UNUSED_EXPRESSION, of a statement whose
// value isn't used and whose evaluation does nothing, and
// UNUSED_LAMBDA_EXPRESSION, of a lambda written as such a statement.
//
// As kotlinc, it judges only the statements of blocks: every statement of
// a function's body, an init block, a loop's body or a finally block, and
// all but the last of a lambda's body or of a branch of an if, when or
// try whose value is used. A branch of one that is itself a statement
// whose value isn't used is a block whose statements are all unused, and
// so is a when without else that ends a lambda but isn't exhaustive. A
// lambda's last statement is unused too if the lambda's body is checked
// against an expected Unit result, coerced to Unit.

// unusedWarnings tracks, during the walk of the file's code, which
// blocks' last statement is the block's value, and which if, when and try
// expressions are statements whose value isn't used.
type unusedWarnings struct {
	usedBlocks map[*syntax.Block]bool
	unused     map[syntax.Expr]bool
	// defaults are the parameters' default values, whose code kotlinc
	// doesn't judge, and skip the blocks in them.
	defaults map[syntax.Expr]bool
	skip     map[*syntax.Block]bool
}

// newUnusedWarnings returns the state of the walk of decls' code.
func newUnusedWarnings(decls []syntax.Decl) *unusedWarnings {
	u := &unusedWarnings{usedBlocks: map[*syntax.Block]bool{}, unused: map[syntax.Expr]bool{},
		defaults: map[syntax.Expr]bool{}, skip: map[*syntax.Block]bool{}}
	params := func(ps []*syntax.Param) {
		for _, p := range ps {
			if p.Default != nil {
				u.defaults[p.Default] = true
			}
		}
	}
	var class func(d *syntax.ClassDecl)
	decl := func(d syntax.Decl) {
		switch d := d.(type) {
		case *syntax.FunDecl:
			params(d.Params)
		case *syntax.SecondaryCtor:
			params(d.Params)
		case *syntax.ClassDecl:
			class(d)
		}
	}
	class = func(d *syntax.ClassDecl) {
		if d == nil {
			return
		}
		for _, p := range d.CtorParams {
			if p.Default != nil {
				u.defaults[p.Default] = true
			}
		}
		for _, e := range d.Entries {
			class(e.Body)
		}
		for _, m := range d.Members {
			decl(m)
		}
	}
	for _, d := range decls {
		decl(d)
	}
	walkCode(decls, func(n any) bool {
		switch n := n.(type) {
		case *syntax.LocalFun:
			params(n.Decl.Params)
		case *syntax.ObjectExpr:
			class(n.Decl)
		case *syntax.LocalClass:
			class(n.Decl)
		}
		return true
	})
	return u
}

// unusedNode notes what n, a node the walk reaches before the blocks in
// it, says of them, and judges n's statements if it is a block.
func (c *Checker) unusedNode(u *unusedWarnings, n any) {
	if x, ok := n.(syntax.Expr); ok && u.defaults[x] {
		walkNode(x, func(m any) bool {
			if b, ok := m.(*syntax.Block); ok {
				u.skip[b] = true
			}
			return true
		})
	}
	switch n := n.(type) {
	case *syntax.Lambda:
		if n.Body != nil && (!n.Anon || n.AnonExpr) && !n.UnitCoerced {
			u.usedBlocks[n.Body] = true
			if k := len(n.Body.Stmts); k > 0 && !n.Anon {
				if es, ok := n.Body.Stmts[k-1].(*syntax.ExprStmt); ok {
					if w, ok := es.X.(*syntax.When); ok && !w.Soft && !whenHasElse(w) {
						u.unused[w] = true
					}
				}
			}
		}
	case *syntax.If:
		if !u.unused[n] {
			u.usedBlocks[n.Then] = true
			if n.Else != nil {
				u.usedBlocks[n.Else] = true
			}
		}
	case *syntax.When:
		if !u.unused[n] {
			for _, br := range n.Branches {
				u.usedBlocks[br.Body] = true
			}
		}
	case *syntax.Try:
		if !u.unused[n] {
			u.usedBlocks[n.Body] = true
			for _, ct := range n.Catches {
				u.usedBlocks[ct.Body] = true
			}
		}
	case *syntax.Block:
		if n == nil || u.skip[n] {
			return
		}
		for i, s := range n.Stmts {
			es, ok := s.(*syntax.ExprStmt)
			if !ok || es.X == nil || i == len(n.Stmts)-1 && u.usedBlocks[n] {
				continue
			}
			switch x := es.X.(type) {
			case *syntax.If, *syntax.When, *syntax.Try:
				u.unused[x] = true
				continue
			case *syntax.Lambda:
				if x.Ref == nil && !x.Anon {
					c.warnf(x.Pos(), "UNUSED_LAMBDA_EXPRESSION", "lambda expression is never invoked; to create a scoped block, use 'run { ... }'")
					continue
				}
			case *syntax.Ident:
				// kotlinc doesn't warn of Unit as a branch without braces,
				// if (c) Unit else Unit.
				if x.Type().K == types.Unit && n.End == (syntax.Pos{}) && len(n.Stmts) == 1 {
					continue
				}
			}
			if pureExpr(es.X) {
				p := syntax.StartPos(es.X)
				if cl, ok := es.X.(*syntax.ClassLit); ok {
					// String::class, at its left side.
					if cl.X != nil {
						p = syntax.StartPos(cl.X)
					} else if cl.Ref != nil && cl.Ref.P.Line > 0 {
						p = cl.Ref.P
					}
				}
				c.warnf(p, "UNUSED_EXPRESSION", "expression is unused")
			}
		}
	}
}

// pureExpr reports whether evaluating x does nothing, as kotlinc judges it
// for UNUSED_EXPRESSION: a literal, a string template of such parts, a
// local variable or parameter, this, an object, or a class literal or
// callable reference of one or of a type. A property, even a constant,
// isn't.
func pureExpr(x syntax.Expr) bool {
	switch x := x.(type) {
	case *syntax.IntLit:
		// The parser folds -1L into a literal, which kotlinc calls
		// unaryMinus on.
		return !x.Long || x.Unsigned || x.Val >= 0
	case *syntax.DoubleLit, *syntax.FloatLit, *syntax.CharLit, *syntax.BoolLit, *syntax.NullLit:
		return true
	case *syntax.Unary:
		// -1 and +1 are Int literals to kotlinc; -1L and -1.0 are calls.
		l, ok := x.X.(*syntax.IntLit)
		return ok && (x.Op == syntax.MINUS || x.Op == syntax.PLUS) && x.Call == nil && !l.Long && !l.Unsigned
	case *syntax.StringLit:
		for _, p := range x.Parts {
			if p.X != nil && !pureExpr(p.X) {
				return false
			}
		}
		return true
	case *syntax.Ident:
		if x.Member != nil || x.Call != nil || x.Getter != nil {
			return false
		}
		if x.Object != nil || x.Sym == nil && x.Unit {
			return true
		}
		v := x.Sym
		return v != nil && (v.Kind == types.Local || v.Kind == types.Param) && v.Delegate == nil
	case *syntax.This:
		return !x.Implicit && !x.Class
	case *syntax.ClassLit:
		return !x.Bound || x.X != nil && pureExpr(x.X)
	case *syntax.Lambda:
		if r := x.Ref; r != nil {
			return r.X == nil || r.Bound == nil || pureExpr(r.X)
		}
		return x.Anon // an anonymous function's definition
	}
	return false
}

// whenHasElse reports whether when expression w has an else branch.
func whenHasElse(w *syntax.When) bool {
	return slices.ContainsFunc(w.Branches, func(br *syntax.WhenBranch) bool { return br.Else })
}
