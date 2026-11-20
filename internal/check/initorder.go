package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// initBeforeDecl reports an init block of class d that assigns a property
// of d declared after it, as kotlinc's INITIALIZATION_BEFORE_DECLARATION:
// init { x = 1 }; var x = 0. Lambdas, local functions and classes, and
// object expressions in the block may run later, and aren't checked.
func (c *Checker) initBeforeDecl(d *syntax.ClassDecl) {
	later := map[*types.VarSym]bool{}
	for _, m := range d.Members {
		if p, ok := m.(*syntax.PropDecl); ok && p.Sym != nil {
			later[p.Sym] = true
		}
	}
	if len(later) == 0 {
		return
	}
	// prop returns the property that assignment target t, a simple name or
	// this.name, is, or nil.
	prop := func(t syntax.Expr) *types.VarSym {
		switch t := t.(type) {
		case *syntax.Ident:
			if t.Member != nil {
				return t.Member.Prop
			}
			return t.Sym
		case *syntax.Member:
			if _, ok := t.X.(*syntax.This); ok {
				return t.Prop
			}
		}
		return nil
	}
	visit := func(n any) bool {
		switch n := n.(type) {
		case *syntax.Lambda, *syntax.LocalFun, *syntax.LocalClass, *syntax.ObjectExpr:
			return false
		case *syntax.Assign:
			if n.Op != syntax.ASSIGN {
				break
			}
			if v := prop(n.Target); v != nil && later[v] {
				p := n.Target.Pos()
				if m, ok := n.Target.(*syntax.Member); ok {
					p = m.X.Pos()
				}
				c.errorf(p, "variable cannot be initialized before declaration")
			}
		}
		return true
	}
	for _, m := range d.Members {
		switch m := m.(type) {
		case *syntax.PropDecl:
			delete(later, m.Sym)
		case *syntax.InitBlock:
			syntax.Inspect(m.Body, visit)
		}
	}
}
