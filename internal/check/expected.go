package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Expected types that operators give a generic call's result when the
// call alone can't infer it.
//
// K2 completes a call that is the operand of as with the cast's type as
// its expected type, if the call's return type is one of its own type
// parameters: foo() as String infers foo's T as String. kc checks the
// operand without an expected type first, and only if that leaves a
// variable it can't infer checks it again expecting the cast's type, so
// code that infers on its own is checked as before.

// castOperand checks the operand of x as T or x as? T.
func (c *Checker) castOperand(x *syntax.As) types.Type {
	n := len(c.errs)
	t := c.value(x.X, noWant)
	if !c.cannotInferSince(n) || !retryableCall(x.X) {
		return t
	}
	m := len(c.errs)
	want := c.resolveType(x.Ref)
	c.errs = c.errs[:m]
	if want.K == types.Invalid || want.K == types.Unit {
		return t
	}
	if m, ok := x.X.(*syntax.Call).Fun.(*syntax.Member); x.Safe || ok && m.Safe {
		want = want.Nullable() // x?.foo() as Int is foo's T Int, or null
	}
	c.errs = c.errs[:n]
	return c.value(x.X, want)
}

// cannotInferSince reports whether one of the errors reported since the
// first n is that a type variable could not be inferred.
func (c *Checker) cannotInferSince(n int) bool {
	for _, e := range c.errs[n:] {
		if strings.HasPrefix(e.Msg, "cannot infer type for type parameter") {
			return true
		}
	}
	return false
}

// retryableCall reports whether e is a call that can be checked again: a
// call, a qualified or safe call, without lambda arguments.
func retryableCall(e syntax.Expr) bool {
	call, ok := e.(*syntax.Call)
	if !ok || len(call.TypeArgs) > 0 || hasLambda(call.Args) {
		return false
	}
	switch f := call.Fun.(type) {
	case *syntax.Ident:
		return true
	case *syntax.Member:
		return simpleExpr(f.X) || retryableCall(f.X)
	}
	return false
}

// elvisRight checks y, the right side of x ?: y, expecting want: the
// expected type of the elvis, or if it has none (hint), the left side's
// type xt without null. As K2's elvis(x: K?, y: K): K, which infers K from
// both sides, a want of the left side's is only a hint: a simple y that
// doesn't type-check with it, as nullable<Int>() ?: nullable(), whose T?
// can't be an Int, is checked again expecting the want's nullable, and
// then nothing. A generic call that can't infer its variables from an
// expected type of the elvis takes them from the left side's type, as
// myOut() in oNullable ?: myOut() expecting an Out<*>.
func (c *Checker) elvisRight(y syntax.Expr, want, xt types.Type, hint bool) types.Type {
	n := len(c.errs)
	if !hint {
		// The elvis's expected type.
		yt := c.value(y, want)
		if c.cannotInferSince(n) && retryableCall(y) && xt.K != types.Invalid && xt.NonNull() != want {
			c.errs = c.errs[:n]
			yt = c.value(y, xt.NonNull())
		}
		return yt
	}
	yt := c.value(y, want)
	if len(c.errs) == n || !simpleExpr(y) {
		return yt
	}
	if want.K != types.Invalid && !types.MayBeNull(want) {
		c.errs = c.errs[:n]
		if yt = c.value(y, want.Nullable()); len(c.errs) == n {
			return yt
		}
	}
	c.errs = c.errs[:n]
	return c.value(y, noWant)
}

// branchTrial completes the pending branch t of an if or when, of system
// s, with the type of the others, target: the system if t is within it,
// or, as K2 takes the branches' common supertype, within one of target's
// generic supertypes, as R.Err(rest)'s V is the String of R<I, String>
// in when (r) { is R.Err -> R.Err(r.rest); is R.Val -> R.Val("", r.rest) }.
// It returns nil if t is within none.
func (c *Checker) branchTrial(s *system, t, target types.Type) *system {
	trial := s.clone()
	if trial.sub(t, target); trial.ok {
		return trial
	}
	at := s.apply(t)
	if at.K != types.Class || !at.HasVars() || target.K != types.Class {
		return nil
	}
	seen := map[*types.ClassSym]bool{}
	queue := []types.Type{target.NonNull()}
	for len(queue) > 0 && len(seen) < 32 {
		st := queue[0]
		queue = queue[1:]
		if st.K != types.Class || seen[st.C] {
			continue
		}
		seen[st.C] = true
		if st != target.NonNull() && len(st.C.TypeParams) > 0 {
			if target.N {
				st = st.Nullable()
			}
			trial := s.clone()
			if trial.sub(t, st); trial.ok {
				return trial
			}
		}
		queue = append(queue, c.directSupers(st)...)
	}
	return nil
}
