package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// dropContracts removes the contract { } calls that start the bodies of
// the checkers' functions, as kotlinc compiles them to nothing: the
// effects they describe, as callsInPlace(block), aren't uses of block,
// which an inline function may only invoke for its lambda to be
// inlined. The files must import kotlin.contracts.contract, or call it
// by its full name. It reports the contracts that don't describe
// effects kotlinc can read.
func dropContracts(checkers []*Checker) {
	for _, c := range checkers {
		f := c.file
		imported := importsContract(f)
		var decls func(ds []syntax.Decl, cls *syntax.ClassDecl)
		decls = func(ds []syntax.Decl, cls *syntax.ClassDecl) {
			for _, d := range ds {
				switch d := d.(type) {
				case *syntax.FunDecl:
					o := contractOwner{name: d.Name, recv: d.Recv != nil, params: d.Params, contexts: d.Contexts}
					if cls != nil {
						o.class = cls.Name
					}
					d.Once, d.Implies = c.dropContract(d.Body, imported, o)
				case *syntax.PropDecl:
					o := contractOwner{name: d.Name, recv: d.Recv != nil, contexts: d.Contexts}
					if cls != nil {
						o.class = cls.Name
					}
					if d.Getter != nil {
						c.dropContract(d.Getter.Body, imported, o)
					}
					if d.Setter != nil {
						so := o
						so.params = d.Setter.Params
						c.dropContract(d.Setter.Body, imported, so)
					}
				case *syntax.ClassDecl:
					decls(d.Members, d)
				}
			}
		}
		decls(f.Decls, nil)
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

// isContractFun says whether fun, called with a lambda that starts a
// body, is kotlin.contracts.contract.
func isContractFun(fun syntax.Expr, imported bool) bool {
	switch f := fun.(type) {
	case *syntax.Ident:
		return imported && f.Name == "contract"
	case *syntax.Member:
		return f.Name == "contract" && qualifiedName(f.X) == "kotlin.contracts"
	}
	return false
}

// qualifiedName returns a.b.c for the expression a.b.c of identifiers,
// or "".
func qualifiedName(x syntax.Expr) string {
	switch x := x.(type) {
	case *syntax.Ident:
		return x.Name
	case *syntax.Member:
		if q := qualifiedName(x.X); q != "" && !x.Safe {
			return q + "." + x.Name
		}
	}
	return ""
}

// dropContract removes a block's first statement if it is contract { },
// and returns the names of the parameters it says the function calls
// exactly once, callsInPlace(f, InvocationKind.EXACTLY_ONCE), and its
// returns(value) implies (p != null) and (p is T) effects. imported says
// whether contract may be called by its simple name.
func (c *Checker) dropContract(b *syntax.Block, imported bool, o contractOwner) (map[string]bool, []syntax.Implies) {
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
	if !ok || len(call.Args) != 1 || !isContractFun(call.Fun, imported) {
		return nil, nil
	}
	l, ok := call.Args[0].(*syntax.Lambda)
	if !ok || l.Anon {
		c.errorf(call.P, "contract is not allowed here: its argument must be a lambda")
		return nil, nil
	}
	c.contractDescription(call, l, o)
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
			// As in metadata, a context parameter's index follows the
			// value parameters'.
			for i, p := range d.Contexts {
				if e.Param == -2 && p.Name == im.Param && p.Name != "_" {
					e.Param = len(d.Params) + i
				}
			}
		}
		if e.Param == -2 {
			continue
		}
		if im.Is != nil {
			// As in x is T, T's type arguments may be left out where
			// x's type gives them: this@bind is Either.Right.
			c.bareRef, c.bareSubject = im.Is, sym.Recv
			if e.Param >= len(sym.Params) {
				c.bareSubject = sym.Contexts[e.Param-len(sym.Params)].Type
			} else if e.Param >= 0 {
				c.bareSubject = sym.Params[e.Param].Type
			}
			t := c.resolveType(im.Is)
			c.bareRef = nil
			if t.K == types.Invalid {
				continue
			}
			e.Is = &t
			c.impossibleIs(im.Is.P, c.bareSubject, c.bareSubject, t, false)
		}
		sym.Implies = append(sym.Implies, e)
	}
}

// contractOwner is the function, or property accessor, a contract
// describes.
type contractOwner struct {
	name     string // the function's, or the property's
	class    string // the name of the class it is a member of, or ""
	recv     bool   // it is an extension
	params   []*syntax.Param
	contexts []*syntax.Param
}

// contractDescription reports the statements of the lambda of contract
// call that aren't effects that kotlinc can read, as kotlinc's
// ConeEffectExtractor does: returns(), returns(true, false or null),
// returnsNotNull() and callsInPlace(p, kind), the first two of which may
// imply a condition made of parameters, this@owner, true, false, is
// checks, comparisons with null, !, && and ||.
func (c *Checker) contractDescription(call *syntax.Call, l *syntax.Lambda, o contractOwner) {
	if len(l.Body.Stmts) == 0 {
		c.errorf(call.P, "error in contract description: the contract is empty")
		return
	}
	for _, s := range l.Body.Stmts {
		var x syntax.Expr
		switch s := s.(type) {
		case *syntax.ExprStmt:
			x = s.X
		case syntax.Expr:
			x = s
		}
		if x == nil || !o.effect(x) {
			p := l.P
			switch s := s.(type) {
			case *syntax.VarStmt:
				p = s.P
			case *syntax.Assign:
				p = s.P
			case *syntax.While:
				p = s.P
			case *syntax.For:
				p = s.P
			case *syntax.Destructure:
				p = s.P
			}
			if x != nil {
				p = x.Pos()
			}
			c.errorf(p, "error in contract description: not an effect that a contract can declare")
		}
	}
}

// effect says whether x declares an effect.
func (o contractOwner) effect(x syntax.Expr) bool {
	call, ok := x.(*syntax.Call)
	if !ok {
		return false
	}
	switch f := call.Fun.(type) {
	case *syntax.Ident:
		switch f.Name {
		case "callsInPlace":
			return (len(call.Args) == 1 || len(call.Args) == 2 && invocationKind(call.Args[1])) && o.value(call.Args[0])
		case "returnsResultOf":
			return len(call.Args) == 1 && o.value(call.Args[0])
		}
		return o.simpleEffect(x)
	case *syntax.Member:
		if len(call.Args) != 1 {
			return false
		}
		switch f.Name {
		case "implies":
			// returns() implies cond, or cond implies returns().
			return o.simpleEffect(f.X) && o.condition(call.Args[0]) || o.condition(f.X) && o.simpleEffect(call.Args[0])
		case "holdsIn":
			return o.condition(f.X) && o.value(call.Args[0])
		}
	}
	return false
}

// simpleEffect says whether x is returns(), returns(constant) or
// returnsNotNull(), which may imply a condition.
func (o contractOwner) simpleEffect(x syntax.Expr) bool {
	call, ok := x.(*syntax.Call)
	if !ok {
		return false
	}
	f, ok := call.Fun.(*syntax.Ident)
	if !ok {
		return false
	}
	switch {
	case f.Name == "returns" && len(call.Args) == 0, f.Name == "returnsNotNull" && len(call.Args) == 0:
		return true
	case f.Name == "returns" && len(call.Args) == 1:
		switch call.Args[0].(type) {
		case *syntax.BoolLit, *syntax.NullLit:
			return true
		}
	}
	return false
}

// invocationKind says whether x names an InvocationKind entry.
func invocationKind(x syntax.Expr) bool {
	var name string
	switch x := x.(type) {
	case *syntax.Ident:
		name = x.Name
	case *syntax.Member:
		if q := qualifiedName(x.X); q != "InvocationKind" && q != "kotlin.contracts.InvocationKind" {
			return false
		}
		name = x.Name
	}
	switch name {
	case "EXACTLY_ONCE", "AT_LEAST_ONCE", "AT_MOST_ONCE", "UNKNOWN":
		return true
	}
	return false
}

// condition says whether x is a condition an effect may imply.
func (o contractOwner) condition(x syntax.Expr) bool {
	switch x := x.(type) {
	case *syntax.BoolLit:
		return true
	case *syntax.Ident, *syntax.This:
		return o.value(x)
	case *syntax.Is:
		return o.value(x.X)
	case *syntax.Unary:
		return x.Op == syntax.NOT && o.condition(x.X)
	case *syntax.Binary:
		switch x.Op {
		case syntax.AND, syntax.OR:
			return o.condition(x.X) && o.condition(x.Y)
		case syntax.EQ, syntax.NE, syntax.EQ_EQ_EQ, syntax.NOT_EQ_EQ:
			_, null := x.Y.(*syntax.NullLit)
			return null && o.value(x.X)
		}
	}
	return false
}

// value says whether x is a parameter of the owner, or its receiver.
func (o contractOwner) value(x syntax.Expr) bool {
	switch x := x.(type) {
	case *syntax.Ident:
		for _, p := range o.params {
			if p.Name == x.Name {
				return true
			}
		}
		for _, p := range o.contexts {
			if p.Name == x.Name && p.Name != "_" {
				return true
			}
		}
	case *syntax.This:
		// An unlabeled this is the ContractBuilder.
		return x.Label != "" && (x.Label == o.name || x.Label == o.class && !o.recv)
	}
	return false
}
