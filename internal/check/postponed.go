package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Postponed arguments: lambdas whose expected type is an inference
// variable.

// funcUpper returns a function type that variable tv is below through
// the variables it is below, if it has no proper bound of its own: T of
// listOf(id { a -> }) for an expected List<(String) -> Unit> is below
// listOf's E, which is below (String) -> Unit. As in Kotlin, a lambda
// passed for T takes its parameter types from it, or from a declared
// bound, as A's of fun <A : (Int) -> Unit> take(x: A). If the function
// type is another argument's, a lower bound of a variable tv is below,
// only its inputs are tv's: see openResult.
func (s *system) funcUpper(tv *tvar) (types.Type, bool) {
	if slices.ContainsFunc(tv.lower, s.proper) || slices.ContainsFunc(tv.upper, s.proper) {
		return types.InvalidType, false
	}
	type bound struct {
		t     types.Type
		lower bool
	}
	seen := map[*tvar]bool{tv: true}
	queue := []*tvar{tv}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		var bounds []bound
		for _, u := range slices.Concat(v.upper, v.v.Bounds) {
			bounds = append(bounds, bound{u, false})
		}
		if v != tv {
			// Or a function type another argument is: { this } of
			// select(id { this }, fun Int.() = this) is an Int.() -> R.
			for _, u := range v.lower {
				bounds = append(bounds, bound{u, true})
			}
		}
		for _, b := range bounds {
			u := s.apply(b.t)
			if w := s.unfixedVar(u); w != nil {
				if !seen[w] {
					seen[w] = true
					queue = append(queue, w)
				}
				continue
			}
			if !u.HasVars() && u.Flex().IsFunc() {
				if b.lower {
					u = s.openResult(u.Flex(), tv.pos)
				}
				return u, true
			}
		}
	}
	return types.InvalidType, false
}

// openResult returns function type t with a new variable for its result.
// As in Kotlin, a lambda passed for a variable whose bounds relate it to
// another argument's function type takes its receiver and parameter types
// from it, but has a result of its own: the second id's T of
// select(id { B() }, id { C() }) is a () -> C, and select's K a () -> A,
// for B and C both A, not the () -> B the second lambda would fail.
func (s *system) openResult(t types.Type, pos syntax.Pos) types.Type {
	n := len(t.A.List) - 1
	if n < 0 {
		return t
	}
	r := &types.TypeParamSym{Name: "R"}
	sub := s.fresh([]*types.TypeParamSym{r}, nil, pos)
	s.byVar[sub[r].T.Param()].synth = true
	args := slices.Clone(t.A.List)
	args[n] = types.Arg{T: sub[r].T}
	t.A = types.MakeArgs(args)
	return t
}

// openVar reports whether t is an unfixed variable without proper
// bounds.
func (s *system) openVar(t types.Type) bool {
	tv := s.unfixedVar(s.apply(t).Flex())
	return tv != nil && !s.fixable(tv)
}

// untypedParams reports whether lambda l takes its parameter types from
// its expected type: it declares a parameter without a type, or may take
// it. A callable reference or an anonymous function whose parameters
// have types has a type of its own.
func untypedParams(l *syntax.Lambda) bool {
	if l.Ref != nil {
		return false
	}
	if !l.Arrow && !l.Anon {
		return true
	}
	return slices.ContainsFunc(l.Params, func(p *syntax.LambdaParam) bool { return p.Type == nil })
}

// Delegate expressions: kotlinc infers the type arguments of the calls
// an if, a try or an elvis that is a property's delegate ends in with the
// delegate's operators, as the arguments of a synthetic call: String by
// if (c) list.getElement(0) else list.getElement(1) infers both E's.

// delegateValue checks x, a property's delegate, whose generic calls'
// variables go to s.
func (c *Checker) delegateValue(x syntax.Expr, s *system) types.Type {
	c.sink = s
	switch y := x.(type) {
	case *syntax.If, *syntax.Try:
		c.branchSink = s
	case *syntax.Binary:
		if y.Op == syntax.ELVIS {
			c.branchSink = s
		}
	}
	t := c.value(x, noWant)
	c.branchSink = nil
	return t
}

// takeBranchSink returns the system an if, a try or an elvis that is a
// delegate ends its calls' inference in, or nil, which only it takes.
func (c *Checker) takeBranchSink() *system {
	s := c.branchSink
	c.branchSink = nil
	return s
}

// sinkBranches takes the pending branches of an if into sink, as
// completeBranches completes them, and returns the type of all: that of
// the first, which the others' are below, or of the others, result.
func (c *Checker) sinkBranches(sink *system, pending []pendingBranch, result types.Type) types.Type {
	for _, p := range pending {
		bt := p.t
		if p.sunk {
			bt = c.sinkLambda(sink, p)
		} else if p.lambda {
			bt = c.branchLambda(p, result, noWant)
		} else if p.sys != nil {
			sink.merge(p.sys)
			body, t := p.body, p.t
			sink.fixups = append(sink.fixups, func(s *system) { body.T = s.final(t) })
		} else {
			p.body.T = bt
		}
		switch {
		case (result.K == types.Null || result.K == types.Nothing && result.N) && p.sunk:
			result = bt.Nullable() // if (b) { -> } else null
		case result.K == types.Nothing || result.K == types.Invalid:
			result = bt
		case result.HasVars() || bt.HasVars():
			trial := sink.clone()
			trial.sub(bt, result)
			if trial.ok {
				*sink = *trial
			}
		default:
			if j, ok := c.join(result, bt); ok {
				result = j
			}
		}
	}
	return result
}

// sinkTail has the last statement of block b, if it is a call, infer its
// variables in sink.
func (c *Checker) sinkTail(b *syntax.Block, sink *system) {
	if sink == nil || len(b.Stmts) == 0 {
		return
	}
	if es, ok := b.Stmts[len(b.Stmts)-1].(*syntax.ExprStmt); ok {
		if _, call := es.X.(*syntax.Call); call {
			c.tailBlock, c.tailSink = b, sink
			sink.fixups = append(sink.fixups, func(s *system) { b.T = s.final(b.T) })
		}
	}
}

// readyFirst moves the lambdas whose parameter types are known before
// the others, as Kotlin analyzes them first: T of codec({ x + it },
// { y }) for decode: ReadContext.() -> T? is the String the second
// returns, which the first's it then has.
func (c *Checker) readyFirst(s *system) {
	if len(s.lambdas) < 2 {
		return
	}
	var ready, rest []pendingLambda
	for _, pl := range s.lambdas {
		// An anonymous function whose parameters have types has a type
		// of its own, which the others passed for its variable then take:
		// select(id { this }, fun Int.() = this).
		// Not one that the function type its variable is below types.
		tv := s.unfixedVar(s.apply(pl.p))
		ok := tv == nil || pl.l.Anon && !untypedParams(pl.l) && s.funcShape(tv, pl.p) == pl.p
		for _, p := range c.inputs(s, pl) {
			ok = ok && s.known(p)
		}
		if ok {
			ready = append(ready, pl)
		} else {
			rest = append(rest, pl)
		}
	}
	s.lambdas = append(ready, rest...)
}

// untypedDelegate reports whether p is a property without a type whose
// delegate's type arguments its getValue's thisRef may infer: R of
// val a by MyProperty() for getValue(thisRef: R, ...).
func untypedDelegate(p *syntax.PropDecl) bool {
	return p != nil && p.Sym != nil && p.Type == nil
}

// funcShape returns the function type, with variables, that variable tv
// is below, if it has no other bounds, or p: a lambda passed for tv has
// that type, whose result it constrains, and tv is then fixed to it:
// { materialize() } of run(select({ materialize() }, ...)) for an Int
// is a () -> R, R of run's, whose Int infers materialize's T.
func (s *system) funcShape(tv *tvar, p types.Type) types.Type {
	if len(tv.lower) > 0 || len(tv.upper) != 1 {
		return p
	}
	if u := s.apply(tv.upper[0]); u.HasVars() && u.Flex().IsFunc() && s.unfixedVar(u) == nil {
		return u
	}
	return p
}

// untypedResult reports whether lambda l is an anonymous function
// without parameters whose result type is its expression body's, which
// takes the expected type: fun() = materialize(). One with parameters
// has their types to choose its call's overload with, as
// foo(1, id(fun(x: Int) = x)) does.
func untypedResult(l *syntax.Lambda) bool {
	return l.Anon && l.AnonRet == nil && !l.AnonUnit && len(l.Params) == 0 && l.AnonRecv == nil
}

// sinkLambda makes the lambda of branch p a postponed argument of sink,
// for a variable of its own, and returns the variable.
func (c *Checker) sinkLambda(sink *system, p pendingBranch) types.Type {
	l := p.body.Stmts[0].(*syntax.ExprStmt).X.(*syntax.Lambda)
	k := &types.TypeParamSym{Name: "K"}
	v := sink.fresh([]*types.TypeParamSym{k}, nil, l.P)[k].T
	sink.lambdas = append(sink.lambdas, pendingLambda{l, v, p.scope})
	body := p.body
	sink.fixups = append(sink.fixups, func(s *system) { body.T = s.final(v) })
	return v
}
