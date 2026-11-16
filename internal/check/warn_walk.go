package check

import "github.com/barteo/kotlin-compiler-go/internal/syntax"

// walkCode calls f, as syntax.Inspect does, for the statements and
// expressions of the code of declarations decls: bodies, initializers,
// default values, supertypes' constructor calls and delegates, and those
// of local classes and object expressions, which syntax.Inspect doesn't
// enter. Declarations that a compiler plugin or kc generates are skipped.
func walkCode(decls []syntax.Decl, f func(any) bool) {
	var visit func(n any) bool
	var class func(d *syntax.ClassDecl)
	inspect := func(n any) { syntax.Inspect(n, visit) }
	params := func(ps []*syntax.Param) {
		for _, p := range ps {
			if p.Default != nil {
				inspect(p.Default)
			}
		}
	}
	fun := func(d *syntax.FunDecl) {
		if d == nil || d.Generated {
			return
		}
		params(d.Params)
		if d.Body != nil {
			inspect(d.Body)
		}
		if d.ExprBody != nil {
			inspect(d.ExprBody)
		}
	}
	decl := func(d syntax.Decl) {
		switch d := d.(type) {
		case *syntax.FunDecl:
			fun(d)
		case *syntax.PropDecl:
			if d.Generated {
				return
			}
			if d.Init != nil {
				inspect(d.Init)
			}
			fun(d.Getter)
			fun(d.Setter)
		case *syntax.InitBlock:
			inspect(d.Body)
		case *syntax.SecondaryCtor:
			if d.Serial != nil {
				return
			}
			params(d.Params)
			if d.Delegation != nil {
				inspect(d.Delegation)
			}
			if d.Body != nil {
				inspect(d.Body)
			}
		case *syntax.ClassDecl:
			class(d)
		}
	}
	class = func(d *syntax.ClassDecl) {
		if d == nil || d.Generated || d.Synthetic {
			return
		}
		for _, p := range d.CtorParams {
			if p.Default != nil {
				inspect(p.Default)
			}
		}
		for _, st := range d.Supers {
			if st.Call != nil {
				inspect(st.Call)
			}
			if st.Delegate != nil {
				inspect(st.Delegate)
			}
		}
		for _, e := range d.Entries {
			if e.Call != nil {
				for _, a := range e.Call.Args {
					inspect(a)
				}
			}
			class(e.Body)
		}
		for _, m := range d.Members {
			decl(m)
		}
	}
	visit = func(n any) bool {
		if !f(n) {
			return false
		}
		switch n := n.(type) {
		case *syntax.ObjectExpr:
			class(n.Decl)
		case *syntax.LocalClass:
			class(n.Decl)
		}
		return true
	}
	for _, d := range decls {
		decl(d)
	}
}

// nodePos returns the position of a statement or an expression, or the
// zero Pos.
func nodePos(n any) syntax.Pos {
	switch n := n.(type) {
	case *syntax.Block:
		return n.P
	case syntax.Expr:
		return n.Pos()
	case syntax.Stmt:
		return syntax.StmtPos(n)
	}
	return syntax.Pos{}
}

// nodeEnd returns a position at or after the end of node n: the end of
// the last line of the code in it.
func nodeEnd(n any) syntax.Pos {
	last := nodePos(n)
	see := func(m any) bool {
		p := nodePos(m)
		if b, ok := m.(*syntax.Block); ok && b.End.Line > p.Line {
			p = b.End
		}
		if p.Line > last.Line {
			last = p
		}
		// syntax.Inspect doesn't enter an object expression's members.
		if o, ok := m.(*syntax.ObjectExpr); ok && o.Decl != nil && o.Decl.End.Line > last.Line {
			last = o.Decl.End
		}
		return true
	}
	switch n := n.(type) {
	case *syntax.LocalFun:
		if n.Decl.End.Line > last.Line {
			last = n.Decl.End
		}
		walkCode([]syntax.Decl{n.Decl}, see)
	case *syntax.LocalClass:
		if n.Decl.End.Line > last.Line {
			last = n.Decl.End
		}
		walkCode([]syntax.Decl{n.Decl}, see)
	default:
		syntax.Inspect(n, see)
	}
	return syntax.Pos{Line: last.Line, Col: 1 << 30}
}
