package check

import (
	"maps"

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
					d.Once, d.InPlace, d.AtLeastOnce, d.Implies, d.Calls, d.ContractConds = c.dropContract(d.Body, imported, o)
					c.misplacedContracts(d.Body, d.ExprBody, imported)
				case *syntax.PropDecl:
					o := contractOwner{name: d.Name, recv: d.Recv != nil, contexts: d.Contexts}
					if cls != nil {
						o.class = cls.Name
					}
					if d.Getter != nil {
						c.dropContract(d.Getter.Body, imported, o)
						c.misplacedContracts(d.Getter.Body, d.Getter.ExprBody, imported)
					}
					if d.Setter != nil {
						so := o
						so.params = d.Setter.Params
						c.dropContract(d.Setter.Body, imported, so)
						c.misplacedContracts(d.Setter.Body, d.Setter.ExprBody, imported)
					}
				case *syntax.ClassDecl:
					decls(d.Members, d)
				}
			}
		}
		decls(f.Decls, nil)
	}
}

// misplacedContracts reports the contract { } calls left in a function's
// body, or its expression body, once dropContract took the one that may
// start it (CONTRACT_NOT_ALLOWED): one that isn't the body's first
// statement, an expression body, and any in a local function or a member
// of a local class or object.
func (c *Checker) misplacedContracts(body *syntax.Block, exprBody syntax.Expr, imported bool) {
	if call, ok := exprBody.(*syntax.Call); ok && c.contractCall(call, imported) {
		c.errorf(call.Fun.Pos(), "CONTRACT_NOT_ALLOWED", "contracts are only allowed in function body blocks")
		exprBody = nil
		for _, a := range call.Args {
			c.contractsIn(a, imported, false)
		}
	}
	c.contractsIn(body, imported, false)
	c.contractsIn(exprBody, imported, false)
}

// contractsIn reports the contract { } calls in code n, in a local
// declaration if local.
func (c *Checker) contractsIn(n any, imported, local bool) {
	members := func(d *syntax.ClassDecl) {
		for _, m := range d.Members {
			switch m := m.(type) {
			case *syntax.FunDecl:
				c.contractsIn(m.Body, imported, true)
				c.contractsIn(m.ExprBody, imported, true)
			case *syntax.PropDecl:
				for _, acc := range []*syntax.FunDecl{m.Getter, m.Setter} {
					if acc != nil {
						c.contractsIn(acc.Body, imported, true)
						c.contractsIn(acc.ExprBody, imported, true)
					}
				}
			}
		}
	}
	syntax.Inspect(n, func(x any) bool {
		switch x := x.(type) {
		case *syntax.LocalFun:
			c.contractsIn(x.Decl.Body, imported, true)
			c.contractsIn(x.Decl.ExprBody, imported, true)
			return false
		case *syntax.LocalClass:
			members(x.Decl)
			return false
		case *syntax.ObjectExpr:
			members(x.Decl)
		case *syntax.Call:
			if c.contractCall(x, imported) {
				if local {
					c.errorf(x.Fun.Pos(), "CONTRACT_NOT_ALLOWED", "contracts are not allowed for local functions")
				} else {
					c.errorf(x.Fun.Pos(), "CONTRACT_NOT_ALLOWED", "contract should be the first statement")
				}
			}
		}
		return true
	})
}

// contractCall reports whether call is contract { }.
func (c *Checker) contractCall(call *syntax.Call, imported bool) bool {
	if len(call.Args) != 1 || !isContractFun(call.Fun, imported) {
		return false
	}
	l, ok := call.Args[0].(*syntax.Lambda)
	return ok && !l.Anon
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

// contractBuilders are the functions of kotlin.contracts' ContractBuilder
// and its effects that a contract's lambda calls.
var contractBuilders = map[string]bool{"returns": true, "returnsNotNull": true, "callsInPlace": true, "holdsIn": true, "implies": true}

// contractBuilderCalls records the calls of contractBuilders in contract
// lambda l, taken out of body b, for optInErrors.
func (c *Checker) contractBuilderCalls(b *syntax.Block, l *syntax.Lambda) {
	syntax.Inspect(l.Body, func(n any) bool {
		x, ok := n.(*syntax.Call)
		if !ok {
			return true
		}
		switch f := x.Fun.(type) {
		case *syntax.Ident:
			if contractBuilders[f.Name] && f.Name != "implies" {
				c.addContractCall(b, f.P)
			}
		case *syntax.Member:
			if x.Infix && f.Name == "implies" {
				c.addContractCall(b, f.P)
			}
		}
		return true
	})
}

func (c *Checker) addContractCall(b *syntax.Block, p syntax.Pos) {
	if c.contractCalls == nil {
		c.contractCalls = map[*syntax.Block][]syntax.Pos{}
	}
	c.contractCalls[b] = append(c.contractCalls[b], p)
}

// dropContract removes a block's first statement if it is contract { },
// and returns the names of the parameters it says the function calls
// exactly once, callsInPlace(f, InvocationKind.EXACTLY_ONCE), and in
// place, callsInPlace(f, kind) of any kind, and its returns(value) implies (p != null) and (p is T) effects. imported says
// whether contract may be called by its simple name.
func (c *Checker) dropContract(b *syntax.Block, imported bool, o contractOwner) (once, inPlace, atLeast map[string]bool, implies []syntax.Implies, calls []syntax.CallsEffect, conds []syntax.Expr) {
	if b == nil || len(b.Stmts) == 0 {
		return nil, nil, nil, nil, nil, nil
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
		return nil, nil, nil, nil, nil, nil
	}
	l, ok := call.Args[0].(*syntax.Lambda)
	if !ok || l.Anon {
		c.errorf(call.P, "CONTRACT_NOT_ALLOWED", "contract is not allowed here: its argument must be a lambda")
		return nil, nil, nil, nil, nil, nil
	}
	c.contractDescription(call, l, o)
	if c.file != nil && c.file.Annotated != nil {
		syntax.Inspect(l.Body, func(n any) bool {
			for _, a := range c.file.Annotated[n] {
				c.errorf(a.P, "ANNOTATION_IN_CONTRACT_ERROR", "annotations are prohibited inside contracts")
			}
			// As kotlinc, they suppress what they name all the same.
			if as, ok := c.file.Annotated[n]; ok {
				c.contractSuppress = annotationsRange(c.contractSuppress, as, nodePos(n), nodeEnd(n))
			}
			return true
		})
		for _, n := range []any{b.Stmts[0], x} {
			if as, ok := c.file.Annotated[n]; ok {
				c.contractSuppress = annotationsRange(c.contractSuppress, as, nodePos(n), nodeEnd(n))
			}
		}
	}
	b.Stmts = b.Stmts[1:]
	if c.contractAt == nil {
		c.contractAt = map[*syntax.Block]syntax.Pos{}
	}
	c.contractAt[b] = call.Fun.Pos()
	c.contractBuilderCalls(b, l)
	syntax.Inspect(l.Body, func(n any) bool {
		c, ok := n.(*syntax.Call)
		if !ok {
			return true
		}
		if m, ok := c.Fun.(*syntax.Member); ok && m.Name == "implies" && len(c.Args) == 1 {
			switch {
			case o.simpleEffect(m.X) && o.condition(c.Args[0]):
				conds = append(conds, c.Args[0])
			case o.condition(m.X) && o.simpleEffect(c.Args[0]):
				conds = append(conds, m.X)
			case o.simpleEffect(m.X) && builderNullCheck(c.Args[0]):
				// this != null: not a condition, but checked as one.
				conds = append(conds, c.Args[0])
			}
		}
		if ims, ok := contractImplies(c); ok {
			implies = append(implies, ims...)
			return false
		}
		if len(c.Args) != 1 && len(c.Args) != 2 {
			return true
		}
		fn, ok1 := c.Fun.(*syntax.Ident)
		p, ok2 := c.Args[0].(*syntax.Ident)
		kind, ok3 := &syntax.Member{Name: "UNKNOWN"}, true
		if len(c.Args) == 2 {
			kind, ok3 = c.Args[1].(*syntax.Member)
		}
		if ok1 && ok2 && ok3 && fn.Name == "callsInPlace" {
			calls = append(calls, syntax.CallsEffect{P: syntax.StartPos(c), Param: p.Name, Kind: kind.Name})
			if inPlace == nil {
				inPlace = map[string]bool{}
			}
			inPlace[p.Name] = true
			if kind.Name == "EXACTLY_ONCE" {
				if once == nil {
					once = map[string]bool{}
				}
				once[p.Name] = true
			}
			if kind.Name == "AT_LEAST_ONCE" {
				if atLeast == nil {
					atLeast = map[string]bool{}
				}
				atLeast[p.Name] = true
			}
		}
		return true
	})
	return once, inPlace, atLeast, implies, calls, conds
}

// contractImplies reads returns(value), for true, false or null,
// implies (p != null), (p is T) or
// (p), for a Boolean p, or a conjunction of them, and returnsNotNull()
// implies them, (x is Int && y is
// String), each of which holds when the function returns the value.
func contractImplies(c *syntax.Call) ([]syntax.Implies, bool) {
	var im syntax.Implies
	m, ok := c.Fun.(*syntax.Member)
	if !ok || m.Name != "implies" || len(c.Args) != 1 {
		return nil, false
	}
	ret, ok := m.X.(*syntax.Call)
	if !ok || len(ret.Args) > 1 {
		return nil, false
	}
	id, ok := ret.Fun.(*syntax.Ident)
	if !ok || id.Name != "returns" && id.Name != "returnsNotNull" {
		return nil, false
	}
	im.Returns = -1
	if id.Name == "returnsNotNull" {
		if len(ret.Args) != 0 {
			return nil, false
		}
		im.Returns = 2
	} else if len(ret.Args) == 1 {
		switch v := ret.Args[0].(type) {
		case *syntax.BoolLit:
			im.Returns = 0
			if v.Val {
				im.Returns = 1
			}
		case *syntax.NullLit:
			im.Returns = 3
		default:
			return nil, false
		}
	}
	var out []syntax.Implies
	var conj func(x syntax.Expr) bool
	conj = func(x syntax.Expr) bool {
		if b, ok := x.(*syntax.Binary); ok && b.Op == syntax.AND {
			return conj(b.X) && conj(b.Y)
		}
		one, ok := impliedCondition(im, x)
		if ok {
			out = append(out, one)
		}
		return ok
	}
	if !conj(c.Args[0]) {
		return nil, false
	}
	return out, true
}

// impliedCondition reads one condition of a contract's implies, im with
// its Returns set: p != null, p == null, p is T or p.
func impliedCondition(im syntax.Implies, cond syntax.Expr) (syntax.Implies, bool) {
	var p syntax.Expr
	switch x := cond.(type) {
	case *syntax.Binary:
		_, null := x.Y.(*syntax.NullLit)
		switch {
		case !null:
			return im, false
		case x.Op == syntax.EQ || x.Op == syntax.EQ_EQ_EQ:
			im.Null = true // p == null
		case x.Op != syntax.NE && x.Op != syntax.NOT_EQ_EQ:
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
	case *syntax.Unary:
		if b, ok := x.X.(*syntax.Binary); ok && x.Op == syntax.NOT {
			// !(p == null) is p != null.
			if _, null := b.Y.(*syntax.NullLit); !null || b.Op != syntax.EQ && b.Op != syntax.EQ_EQ_EQ {
				return im, false
			}
			p = b.X
			break
		}
		id, ok := x.X.(*syntax.Ident)
		if x.Op != syntax.NOT || !ok {
			return im, false
		}
		p, im.Holds, im.Negated = id, true, true // implies (!p)
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
		e := types.Effect{Returns: im.Returns, Param: -2, Holds: im.Holds, Negated: im.Negated, Null: im.Null}
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
	if len(d.ContractConds) > 0 {
		env := map[string]types.Type{}
		for i, p := range d.Params {
			if i < len(sym.Params) {
				env[p.Name] = sym.Params[i].Type
			}
		}
		for _, cond := range d.ContractConds {
			c.contractCondWarnings(cond, env, sym)
		}
	}
}

// contractCondWarnings reports the checks in contract condition x, made
// of parameters, null comparisons, is checks, !, && and ||, that the
// parameters' types, env, decide, as kotlinc checks the condition as
// an expression: SENSELESS_COMPARISON and USELESS_IS_CHECK. x && y sees
// what x being true tells of the parameters, and x || y what x being
// false does. It returns what x being true, and being false, tells.
func (c *Checker) contractCondWarnings(x syntax.Expr, env map[string]types.Type, sym *types.FunSym) (onTrue, onFalse map[string]types.Type) {
	with := func(env, more map[string]types.Type) map[string]types.Type {
		if len(more) == 0 {
			return env
		}
		m := make(map[string]types.Type, len(env)+len(more))
		maps.Copy(m, env)
		maps.Copy(m, more)
		return m
	}
	switch x := x.(type) {
	case *syntax.Unary:
		if x.Op == syntax.NOT {
			t, f := c.contractCondWarnings(x.X, env, sym)
			return f, t
		}
	case *syntax.Binary:
		switch x.Op {
		case syntax.AND:
			lt, _ := c.contractCondWarnings(x.X, env, sym)
			rt, _ := c.contractCondWarnings(x.Y, with(env, lt), sym)
			return with(lt, rt), nil
		case syntax.OR:
			_, lf := c.contractCondWarnings(x.X, env, sym)
			_, rf := c.contractCondWarnings(x.Y, with(env, lf), sym)
			return nil, with(lf, rf)
		case syntax.EQ, syntax.NE, syntax.EQ_EQ_EQ, syntax.NOT_EQ_EQ:
			if builderNullCheck(x) {
				// The builder, never null.
				c.warnf(x.X.Pos(), "SENSELESS_COMPARISON", "condition is always '%v'", x.Op == syntax.NE || x.Op == syntax.NOT_EQ_EQ)
				return nil, nil
			}
			id, ok := x.X.(*syntax.Ident)
			if _, null := x.Y.(*syntax.NullLit); !ok || !null {
				return nil, nil
			}
			t, ok := env[id.Name]
			if !ok {
				return nil, nil
			}
			eq := x.Op == syntax.EQ || x.Op == syntax.EQ_EQ_EQ
			if definitelyNonNull(t) {
				c.warnf(x.X.Pos(), "SENSELESS_COMPARISON", "condition is always '%v'", !eq)
				return nil, nil
			}
			nonNull := map[string]types.Type{id.Name: t.NonNull()}
			if eq {
				return nil, nonNull
			}
			return nonNull, nil
		}
	case *syntax.Is:
		id, ok := x.X.(*syntax.Ident)
		if !ok || x.Ref == nil {
			return nil, nil
		}
		t, ok := env[id.Name]
		if !ok || !certainCastType(t) || t.K == types.Nothing || t.K == types.Null {
			return nil, nil
		}
		c.bareRef, c.bareSubject = x.Ref, t
		target := c.resolveTypeQuiet(x.Ref)
		c.bareRef = nil
		if !certainCastType(target) {
			return nil, nil
		}
		if c.subtype(t, target) {
			c.warnf(syntax.StartPos(x), "USELESS_IS_CHECK", "check for instance is always '%v'", !x.Not)
			return nil, nil
		}
		if !c.subtype(target, t) {
			return nil, nil
		}
		narrowed := map[string]types.Type{id.Name: target}
		if x.Not {
			return nil, narrowed
		}
		return narrowed, nil
	}
	return nil, nil
}

// builderNullCheck reports whether x compares an unlabeled this with
// null in a contract: this is the ContractBuilder there, which kotlinc
// reports the comparison of as senseless.
func builderNullCheck(x syntax.Expr) bool {
	b, ok := x.(*syntax.Binary)
	if !ok {
		return false
	}
	switch b.Op {
	case syntax.EQ, syntax.NE, syntax.EQ_EQ_EQ, syntax.NOT_EQ_EQ:
	default:
		return false
	}
	th, ok := b.X.(*syntax.This)
	_, null := b.Y.(*syntax.NullLit)
	return ok && null && th.Label == ""
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
		c.errorf(call.P, "ERROR_IN_CONTRACT_DESCRIPTION", "error in contract description: the contract is empty")
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
		if p, ok := o.contextCallsInPlace(x); ok {
			// As kotlinc, at the contract call.
			c.errorf(call.P, "ERROR_IN_CONTRACT_DESCRIPTION", "error in contract description: callsInPlace contract cannot be applied to context parameter '%s' because context arguments can never be lambdas", p)
			continue
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
			c.errorf(p, "ERROR_IN_CONTRACT_DESCRIPTION", "error in contract description: not an effect that a contract can declare")
		}
	}
}

// contextCallsInPlace returns the name of the context parameter x is a
// callsInPlace of, if it is one.
func (o contractOwner) contextCallsInPlace(x syntax.Expr) (string, bool) {
	call, ok := x.(*syntax.Call)
	if !ok || len(call.Args) == 0 {
		return "", false
	}
	if f, ok := call.Fun.(*syntax.Ident); !ok || f.Name != "callsInPlace" {
		return "", false
	}
	id, ok := call.Args[0].(*syntax.Ident)
	if !ok {
		return "", false
	}
	for _, p := range o.contexts {
		if p.Name == id.Name {
			return id.Name, true
		}
	}
	return "", false
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

// resolveTypeQuiet resolves type ref as resolveType does, but reports
// nothing: it returns the invalid type where resolveType reports an
// error.
func (c *Checker) resolveTypeQuiet(ref *syntax.TypeRef) types.Type {
	n := len(c.errs)
	t := c.resolveType(ref)
	if len(c.errs) > n {
		c.errs = c.errs[:n]
		return types.InvalidType
	}
	return t
}
