package check

import (
	"slices"

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
						_, d.Getter.Implies = c.dropContract(d.Getter.Body, imported, o)
					}
					if d.Setter != nil {
						so := o
						so.params = d.Setter.Params
						_, d.Setter.Implies = c.dropContract(d.Setter.Body, imported, so)
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

// contractImplies reads an effect with a condition: returns(value)
// implies cond, cond implies returnsNotNull(), or cond holdsIn lambda.
func contractImplies(c *syntax.Call) (syntax.Implies, bool) {
	var im syntax.Implies
	m, ok := c.Fun.(*syntax.Member)
	if !ok || len(c.Args) != 1 {
		return im, false
	}
	switch m.Name {
	case "holdsIn":
		l, ok := c.Args[0].(*syntax.Ident)
		if !ok {
			return im, false
		}
		return syntax.Implies{Kind: types.HoldsIn, Cond: m.X, Lambda: l.Name}, true
	case "implies":
	default:
		return im, false
	}
	if r, ok := c.Args[0].(*syntax.Call); ok && len(r.Args) == 0 {
		if id, ok := r.Fun.(*syntax.Ident); ok && id.Name == "returnsNotNull" {
			return syntax.Implies{Kind: types.ImpliesNotNull, Cond: m.X}, true
		}
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
	im.Cond = c.Args[0]
	return im, true
}

// contractEffects sets the effects of function d's contract on its
// symbol, once its parameters are declared.
func (c *Checker) contractEffects(d *syntax.FunDecl, sym *types.FunSym) {
	for _, im := range d.Implies {
		cond := c.contractCond(im.Cond, d, sym)
		if cond == nil {
			continue
		}
		e := types.Effect{Kind: im.Kind, Returns: im.Returns, Param: -2}
		if im.Kind == types.HoldsIn {
			e.Lambda = -1
			for i, p := range d.Params {
				if p.Name == im.Lambda {
					e.Lambda = i
				}
			}
			if e.Lambda < 0 {
				continue
			}
		}
		switch cond.Op {
		case types.CondNotNull, types.CondIs, types.CondHolds:
			e.Param, e.Is, e.Holds = cond.Param, cond.Is, cond.Op == types.CondHolds
		default:
			e.Cond = cond
		}
		sym.Implies = append(sym.Implies, e)
	}
}

// accessorContracts sets the effects of the contracts of property d's
// accessors on their symbols.
func (c *Checker) accessorContracts(d *syntax.PropDecl) {
	for _, a := range []*syntax.FunDecl{d.Getter, d.Setter} {
		if a != nil && a.Sym != nil && len(a.Implies) > 0 {
			c.contractEffects(a, a.Sym)
		}
	}
}

// contractParam returns the index of the parameter of d that x names, as
// in Effect, or -2.
func contractParam(x syntax.Expr, d *syntax.FunDecl, sym *types.FunSym) int {
	switch x := x.(type) {
	case *syntax.This:
		// this, or this@f: the extension receiver.
		if d.Recv != nil || sym.Ext {
			return -1
		}
	case *syntax.Ident:
		for i, p := range d.Params {
			if p.Name == x.Name {
				return i
			}
		}
		// As in metadata, a context parameter's index follows the
		// value parameters'.
		for i, p := range d.Contexts {
			if p.Name == x.Name && p.Name != "_" {
				return len(d.Params) + i
			}
		}
	}
	return -2
}

// contractCond converts a contract's condition, or returns nil.
func (c *Checker) contractCond(x syntax.Expr, d *syntax.FunDecl, sym *types.FunSym) *types.Cond {
	switch x := x.(type) {
	case *syntax.BoolLit:
		if x.Val {
			return &types.Cond{Op: types.CondTrue}
		}
		return &types.Cond{Op: types.CondFalse}
	case *syntax.Ident, *syntax.This:
		if p := contractParam(x, d, sym); p != -2 {
			return &types.Cond{Op: types.CondHolds, Param: p}
		}
	case *syntax.Unary:
		if x.Op == syntax.NOT {
			if y := c.contractCond(x.X, d, sym); y != nil {
				return &types.Cond{Op: types.CondNot, X: y}
			}
		}
	case *syntax.Binary:
		switch x.Op {
		case syntax.AND, syntax.OR:
			l, r := c.contractCond(x.X, d, sym), c.contractCond(x.Y, d, sym)
			if l == nil || r == nil {
				return nil
			}
			op := types.CondAnd
			if x.Op == syntax.OR {
				op = types.CondOr
			}
			return &types.Cond{Op: op, X: l, Y: r}
		case syntax.EQ, syntax.NE, syntax.EQ_EQ_EQ, syntax.NOT_EQ_EQ:
			if _, null := x.Y.(*syntax.NullLit); !null {
				return nil
			}
			p := contractParam(x.X, d, sym)
			if p == -2 {
				return nil
			}
			cond := &types.Cond{Op: types.CondNotNull, Param: p}
			if x.Op == syntax.EQ || x.Op == syntax.EQ_EQ_EQ {
				cond = &types.Cond{Op: types.CondNot, X: cond}
			}
			return cond
		}
	case *syntax.Is:
		p := contractParam(x.X, d, sym)
		if p == -2 {
			return nil
		}
		// As in x is T, T's type arguments may be left out where
		// x's type gives them: this@bind is Either.Right.
		c.bareRef, c.bareSubject = x.Ref, sym.Recv
		if p >= len(sym.Params) {
			c.bareSubject = sym.Contexts[p-len(sym.Params)].Type
		} else if p >= 0 {
			c.bareSubject = sym.Params[p].Type
		}
		t := c.resolveType(x.Ref)
		c.bareRef = nil
		if t.K == types.Invalid {
			return nil
		}
		c.impossibleIs(x.Ref.P, c.bareSubject, c.bareSubject, t, false)
		cond := &types.Cond{Op: types.CondIs, Param: p, Is: &t}
		if x.Not {
			cond = &types.Cond{Op: types.CondNot, X: cond}
		}
		return cond
	}
	return nil
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
		} else if !o.returnsNotNullCond(x) {
			c.errorf(call.Fun.Pos(), "error in contract description: arbitrary expressions are not supported in this contract, only 'null' and 'is' checks are")
		}
	}
}

// returnsNotNullCond says whether x, an effect, is not cond implies
// returnsNotNull() or its condition is one kotlinc takes there: a null or
// is check of a value, or its negation.
func (o contractOwner) returnsNotNullCond(x syntax.Expr) bool {
	call, ok := x.(*syntax.Call)
	if !ok {
		return true
	}
	m, ok := call.Fun.(*syntax.Member)
	if !ok || m.Name != "implies" || len(call.Args) != 1 {
		return true
	}
	e, ok := call.Args[0].(*syntax.Call)
	if !ok || o.simpleEffect(m.X) {
		return true
	}
	if f, ok := e.Fun.(*syntax.Ident); !ok || f.Name != "returnsNotNull" {
		return true
	}
	cond := m.X
	for {
		u, ok := cond.(*syntax.Unary)
		if !ok || u.Op != syntax.NOT {
			break
		}
		cond = u.X
	}
	switch cond := cond.(type) {
	case *syntax.Is:
		return true
	case *syntax.Binary:
		_, null := cond.Y.(*syntax.NullLit)
		return null
	}
	return false
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

// impliesOwnType says whether f's contract implies an argument is of a
// type with f's type parameters, whose calls' type arguments it needs.
func impliesOwnType(f *types.FunSym) bool {
	for _, e := range f.Implies {
		if e.Cond != nil {
			return true // its type checks may
		}
		if e.Is != nil && e.Is.Has(func(u types.Type) bool { return u.K == types.TypeParam && slices.Contains(f.TypeParams, u.Param()) }) {
			return true
		}
	}
	return false
}

// accessorNarrows returns the smart casts that the contract of a
// property's setter implies after statement s assigns it, x.p = v.
func (c *Checker) accessorNarrows(s syntax.Stmt) []narrow {
	var m *syntax.Member
	var f *types.FunSym
	var value syntax.Expr
	switch s := s.(type) {
	case *syntax.Assign:
		m, _ = s.Target.(*syntax.Member)
		if m != nil && s.Op == syntax.ASSIGN {
			f = m.Setter
			if f == nil && m.Prop != nil {
				f = m.Prop.Setter
			}
			value = s.Value
		}
	}
	if f == nil || len(f.Implies) == 0 {
		return nil
	}
	site := contractSite{f: f, arg: func(p int) syntax.Expr {
		switch {
		case p == -1 && !m.Safe:
			return m.X
		case p == 0:
			return value
		}
		return nil
	}}
	var out []narrow
	for _, e := range f.Implies {
		if e.Kind == types.ReturnsImplies && e.Returns == -1 {
			t, _ := c.condNarrows(site, e.Condition())
			out = append(out, t...)
		}
	}
	return out
}

// returnsNarrows returns the smart casts that the returns() implies
// effects of the contracts of the calls and property reads in x imply
// once it is evaluated: those that run whatever x gives, not those after
// && or || or ?:, or in lambdas.
func (c *Checker) returnsNarrows(x syntax.Expr) []narrow {
	var out []narrow
	var walk func(x syntax.Expr)
	walk = func(x syntax.Expr) {
		switch x := x.(type) {
		case *syntax.Call:
			if m, ok := x.Fun.(*syntax.Member); ok && !m.Safe {
				walk(m.X)
			}
			for _, a := range x.Args {
				if _, ok := a.(*syntax.Lambda); !ok {
					walk(a)
				}
			}
			if !safeCallOf(x) {
				out = append(out, c.contractNarrows(x, -1)...)
			}
		case *syntax.Member:
			if x.Safe {
				return
			}
			walk(x.X)
			out = append(out, c.getterNarrows(x, -1)...)
		case *syntax.NotNull:
			walk(x.X)
		case *syntax.Binary:
			walk(x.X)
			if x.Op != syntax.AND && x.Op != syntax.OR && x.Op != syntax.ELVIS {
				walk(x.Y)
			}
		case *syntax.Unary:
			walk(x.X)
		case *syntax.Is:
			walk(x.X)
		case *syntax.As:
			walk(x.X)
		}
	}
	if x != nil {
		walk(x)
	}
	return out
}

// getterNarrows returns the smart casts that the returns(v) implies
// effects of the contract of m's getter imply, for v as in types.Effect.
func (c *Checker) getterNarrows(m *syntax.Member, returns int) []narrow {
	f := m.Getter
	if f == nil && m.Prop != nil {
		f = m.Prop.Getter
	}
	if f == nil || len(f.Implies) == 0 || m.Safe {
		return nil
	}
	site := contractSite{f: f, arg: func(p int) syntax.Expr {
		if p == -1 {
			return m.X
		}
		return nil
	}}
	var out []narrow
	for _, e := range f.Implies {
		if e.Kind == types.ReturnsImplies && e.Returns == returns {
			t, _ := c.condNarrows(site, e.Condition())
			out = append(out, t...)
		}
	}
	return out
}
