package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// exprRet is a return, from an inline lambda, of a function with an
// expression body and no return type: the type of its value and where.
type exprRet struct {
	t types.Type
	x *syntax.Jump
}

// exprBodyReturn checks return x of a function whose expression body's
// type is its return type, if it is in an inline lambda of the body: as
// in Kotlin 2.4, where it is a warning, it returns a value of the type
// the body infers, which it doesn't take part in: test2() = run { ...
// return { "1" }; { "2" } } returns a () -> String. kotlinc warns of it
// (RETURN_IN_FUNCTION_WITH_EXPRESSION_BODY_WARNING). One in the body
// itself is an error.
func (c *Checker) exprBodyReturn(x *syntax.Jump) bool {
	if c.exprRets == nil || len(c.lambdas) <= c.base {
		return false
	}
	c.warnf(x.P, "RETURN_IN_FUNCTION_WITH_EXPRESSION_BODY_WARNING", "return in function with expression body and without explicit return type; use block body '{...}' or add an explicit return type")
	t := types.UnitType
	if x.X != nil {
		t = c.value(x.X, noWant)
	}
	*c.exprRets = append(*c.exprRets, exprRet{t, x})
	return true
}

// checkExprReturns checks the returns exprBodyReturn took against the
// return type, ret, of function fn, once its body infers it. A Nothing,
// of a body that only returns, needs to be written: fun f() = run {
// return 1 }.
func (c *Checker) checkExprReturns(rets []exprRet, ret types.Type, fn string) {
	for _, r := range rets {
		switch {
		case ret.K == types.Invalid || r.t.K == types.Invalid:
		case ret.K == types.Nothing && !ret.N:
			c.errorf(r.x.P, "'return' needs an explicit return type on '%s'", fn)
		case r.x.X != nil:
			c.assignable(r.t, ret, syntax.StartPos(r.x.X))
		case ret.K != types.Unit:
			c.errorf(r.x.P, "return type mismatch: expected %s, actual Unit", ret)
		}
	}
}
