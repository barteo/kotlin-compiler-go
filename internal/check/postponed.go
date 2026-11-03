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
// bound, as A's of fun <A : (Int) -> Unit> take(x: A).
func (s *system) funcUpper(tv *tvar) (types.Type, bool) {
	if slices.ContainsFunc(tv.lower, s.proper) || slices.ContainsFunc(tv.upper, s.proper) {
		return types.InvalidType, false
	}
	seen := map[*tvar]bool{tv: true}
	queue := []*tvar{tv}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		for _, u := range slices.Concat(v.upper, v.v.Bounds) {
			u = s.apply(u)
			if w := s.unfixedVar(u); w != nil {
				if !seen[w] {
					seen[w] = true
					queue = append(queue, w)
				}
				continue
			}
			if !u.HasVars() && u.Flex().IsFunc() {
				return u, true
			}
		}
	}
	return types.InvalidType, false
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
		if p.sys != nil {
			sink.merge(p.sys)
			body, t := p.body, p.t
			sink.fixups = append(sink.fixups, func(s *system) { body.T = s.final(t) })
		} else {
			p.body.T = bt
		}
		switch {
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
		ok := s.unfixedVar(s.apply(pl.p)) == nil
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
