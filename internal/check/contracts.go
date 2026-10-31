package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// dropContracts removes the contract { } calls that start the bodies of
// the files' functions, as kotlinc compiles them to nothing: the effects
// they describe, as callsInPlace(block), aren't uses of block, which an
// inline function may only invoke for its lambda to be inlined. The
// files must import kotlin.contracts.contract.
func dropContracts(files []*syntax.File) {
	for _, f := range files {
		if !importsContract(f) {
			continue
		}
		var decls func(ds []syntax.Decl)
		decls = func(ds []syntax.Decl) {
			for _, d := range ds {
				switch d := d.(type) {
				case *syntax.FunDecl:
					d.Once, d.Implies = dropContract(d.Body)
				case *syntax.PropDecl:
					if d.Getter != nil {
						dropContract(d.Getter.Body)
					}
					if d.Setter != nil {
						dropContract(d.Setter.Body)
					}
				case *syntax.ClassDecl:
					decls(d.Members)
				}
			}
		}
		decls(f.Decls)
	}
}

func importsContract(f *syntax.File) bool {
	for _, imp := range f.Imports {
		if imp.Path == "kotlin.contracts.contract" && (imp.Alias == "" || imp.Alias == "contract") || imp.Star && imp.Path == "kotlin.contracts" {
			return true
		}
	}
	return false
}

// dropContract removes a block's first statement if it is contract { },
// and returns the names of the parameters it says the function calls
// exactly once, callsInPlace(f, InvocationKind.EXACTLY_ONCE), and its
// returns(value) implies (p != null) and (p is T) effects.
func dropContract(b *syntax.Block) (map[string]bool, []syntax.Implies) {
	if b == nil || len(b.Stmts) == 0 {
		return nil, nil
	}
	var x syntax.Expr
	switch s := b.Stmts[0].(type) {
	case *syntax.ExprStmt:
		x = s.X
	case syntax.Expr:
		x = s
	}
	call, ok := x.(*syntax.Call)
	if !ok || len(call.Args) != 1 {
		return nil, nil
	}
	if id, ok := call.Fun.(*syntax.Ident); !ok || id.Name != "contract" {
		return nil, nil
	}
	l, ok := call.Args[0].(*syntax.Lambda)
	if !ok {
		return nil, nil
	}
	b.Stmts = b.Stmts[1:]
	var once map[string]bool
	var implies []syntax.Implies
	syntax.Inspect(l.Body, func(n any) bool {
		c, ok := n.(*syntax.Call)
		if !ok {
			return true
		}
		if im, ok := contractImplies(c); ok {
			implies = append(implies, im)
			return false
		}
		if len(c.Args) != 2 {
			return true
		}
		fn, ok1 := c.Fun.(*syntax.Ident)
		p, ok2 := c.Args[0].(*syntax.Ident)
		kind, ok3 := c.Args[1].(*syntax.Member)
		if ok1 && ok2 && ok3 && fn.Name == "callsInPlace" && kind.Name == "EXACTLY_ONCE" {
			if once == nil {
				once = map[string]bool{}
			}
			once[p.Name] = true
		}
		return true
	})
	return once, implies
}

// contractImplies reads returns(value) implies (p != null), (p is T) or
// (p), for a Boolean p.
func contractImplies(c *syntax.Call) (syntax.Implies, bool) {
	var im syntax.Implies
	m, ok := c.Fun.(*syntax.Member)
	if !ok || m.Name != "implies" || len(c.Args) != 1 {
		return im, false
	}
	ret, ok := m.X.(*syntax.Call)
	if !ok || len(ret.Args) > 1 {
		return im, false
	}
	if id, ok := ret.Fun.(*syntax.Ident); !ok || id.Name != "returns" {
		return im, false
	}
	im.Returns = -1
	if len(ret.Args) == 1 {
		v, ok := ret.Args[0].(*syntax.BoolLit)
		if !ok {
			return im, false
		}
		im.Returns = 0
		if v.Val {
			im.Returns = 1
		}
	}
	var p syntax.Expr
	switch x := c.Args[0].(type) {
	case *syntax.Binary:
		if _, null := x.Y.(*syntax.NullLit); !null || x.Op != syntax.NE {
			return im, false
		}
		p = x.X
	case *syntax.Is:
		if x.Not {
			return im, false
		}
		p, im.Is = x.X, x.Ref
	case *syntax.Ident:
		p, im.Holds = x, true
	default:
		return im, false
	}
	switch p := p.(type) {
	case *syntax.Ident:
		im.Param = p.Name
	case *syntax.This:
		// this, or this@f.
		im.Param = "this"
	default:
		return im, false
	}
	return im, true
}

// contractEffects sets the effects of function d's contract on its
// symbol, once its parameters are declared.
func (c *Checker) contractEffects(d *syntax.FunDecl, sym *types.FunSym) {
	for _, im := range d.Implies {
		e := types.Effect{Returns: im.Returns, Param: -2, Holds: im.Holds}
		if im.Param == "this" {
			if d.Recv != nil {
				e.Param = -1
			}
		} else {
			for i, p := range d.Params {
				if p.Name == im.Param {
					e.Param = i
				}
			}
		}
		if e.Param == -2 {
			continue
		}
		if im.Is != nil {
			t := c.resolveType(im.Is)
			if t.K == types.Invalid {
				continue
			}
			e.Is = &t
		}
		sym.Implies = append(sym.Implies, e)
	}
}
