package syntax

// Inspect calls f for node, a statement or an expression, and, while f
// returns true, for the statements and expressions in it, depth first. It
// sees the code as written and as the checker rewrites it: a compound
// assignment's Stmts as well as its target and value.
func Inspect(node any, f func(any) bool) {
	switch n := node.(type) {
	case nil:
		return
	case *Block:
		if n == nil || !f(n) {
			return
		}
		for _, s := range n.Stmts {
			Inspect(s, f)
		}
		return
	case Expr:
		if isNil(n) {
			return
		}
	}
	if !f(node) {
		return
	}
	switch n := node.(type) {
	case *ExprStmt:
		Inspect(n.X, f)
	case *VarStmt:
		inspectExpr(n.Init, f)
		inspectExpr(n.Delegate, f)
	case *Destructure:
		inspectExpr(n.Init, f)
		for _, s := range n.Stmts {
			Inspect(s, f)
		}
	case *Assign:
		inspectExpr(n.Target, f)
		inspectExpr(n.Value, f)
		for _, s := range n.Stmts {
			Inspect(s, f)
		}
	case *While:
		inspectExpr(n.Cond, f)
		Inspect(n.Body, f)
	case *For:
		inspectExpr(n.Range, f)
		Inspect(n.Body, f)
	case *LocalFun:
		for _, p := range n.Decl.Params {
			inspectExpr(p.Default, f)
		}
		Inspect(n.Decl.Body, f)
		inspectExpr(n.Decl.ExprBody, f)
	case *Super:
		if n.Outer != nil {
			Inspect(n.Outer, f)
		}
	case *StringLit:
		for _, p := range n.Parts {
			inspectExpr(p.X, f)
		}
	case *Unary:
		inspectExpr(n.X, f)
		for _, s := range n.Stmts {
			Inspect(s, f)
		}
	case *Binary:
		inspectExpr(n.X, f)
		inspectExpr(n.Y, f)
	case *ClassLit:
		inspectExpr(n.X, f)
	case *Checked:
		inspectExpr(n.X, f)
	case *Member:
		inspectExpr(n.X, f)
	case *Index:
		inspectExpr(n.X, f)
		for _, a := range n.Args {
			inspectExpr(a, f)
		}
	case *Call:
		inspectExpr(n.Fun, f)
		for _, a := range n.Args {
			inspectExpr(a, f)
		}
	case *Lambda:
		if n.Ref != nil {
			inspectExpr(n.Ref.X, f)
			inspectExpr(n.Ref.BoundExpr, f)
		}
		Inspect(n.Body, f)
	case *NotNull:
		inspectExpr(n.X, f)
	case *Spread:
		inspectExpr(n.X, f)
	case *If:
		inspectExpr(n.Cond, f)
		Inspect(n.Then, f)
		Inspect(n.Else, f)
	case *When:
		inspectExpr(n.Subject, f)
		for _, b := range n.Branches {
			for _, c := range b.Conds {
				inspectExpr(c.X, f)
			}
			Inspect(b.Body, f)
		}
	case *Is:
		inspectExpr(n.X, f)
	case *As:
		inspectExpr(n.X, f)
	case *Try:
		Inspect(n.Body, f)
		for _, c := range n.Catches {
			Inspect(c.Body, f)
		}
		Inspect(n.Finally, f)
	case *Jump:
		inspectExpr(n.X, f)
	}
}

func inspectExpr(x Expr, f func(any) bool) {
	if x != nil {
		Inspect(x, f)
	}
}

// isNil reports whether x is a typed nil pointer.
func isNil(x Expr) bool {
	switch n := x.(type) {
	case *Ident:
		return n == nil
	case *Lambda:
		return n == nil
	case *Call:
		return n == nil
	}
	return false
}
