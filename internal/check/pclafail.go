package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// pclaFailedCall follows K2's shared constraint system of a builder's
// lambda once a call has failed it. x is a call just checked, of type
// t, in a builder's lambda whose variables are unfixed; the type of the
// call is returned.
//
// A call fails the system if it is of a generic extension on an
// unresolved receiver, whose type parameter it can't infer, and takes a
// lambda or the lambda's this: Unresolved().apply { } and
// Unresolved().bar(this) for fun <V, Y> V.bar(y: Y). K2 adds such a
// call, which it postpones, to the system with its error. Another
// call on an unresolved receiver, Unresolved().bar(1), is completed on
// its own and leaves the system alone.
//
// Every call after it that K2 resolves on that system, one that takes a
// lambda or whose receiver or arguments have a type with one of the
// builder's variables, is then inapplicable: K2 reports each of its
// arguments, the type parameters it can't infer, and for an extension
// its receiver. The builder's variables stay uninferred (see
// stubInference).
func (c *Checker) pclaFailedCall(x *syntax.Call, t types.Type) types.Type {
	r := c.pclaRun()
	if r == nil {
		return t
	}
	lambda := slices.ContainsFunc(x.Args, func(a syntax.Expr) bool { _, ok := a.(*syntax.Lambda); return ok })
	m, member := x.Fun.(*syntax.Member)
	cannot := func(p syntax.Pos, v *types.TypeParamSym) {
		msg := "cannot infer type for type parameter '" + v.Name + "'. Specify it explicitly."
		if !c.hasErrorAt(p, msg) {
			c.errorf(p, "%s", msg)
		}
	}
	rigid := func(f *types.FunSym, t types.Type) types.Type {
		if !t.Has(func(u types.Type) bool { return u.K == types.TypeParam && slices.Contains(f.TypeParams, u.Param()) }) {
			return types.InvalidType
		}
		return t
	}
	if member && m.X.Type().K == types.Invalid {
		// The call that fails the system.
		this := slices.ContainsFunc(x.Args, func(a syntax.Expr) bool {
			t, ok := a.(*syntax.This)
			return ok && t.Label == ""
		})
		if !lambda && !this {
			return t
		}
		for _, cl := range c.extCalleesFor(m.Name, types.Type{K: types.Any}.Nullable(), nil) {
			f := cl.f
			if f.Recv.K != types.TypeParam {
				continue
			}
			for _, o := range c.pclaRuns {
				if o.failed.Line == 0 {
					o.failed = x.Fun.Pos()
				}
			}
			for _, v := range f.TypeParams {
				cannot(x.Fun.Pos(), v)
			}
			// And the parameters of its lambdas that are the
			// receiver's type.
			for i, a := range x.Args {
				l, ok := a.(*syntax.Lambda)
				if !ok || i >= len(f.Params) {
					continue
				}
				ft := f.Params[i].Type
				ps, _, ok := funcParts(ft)
				if !ok {
					continue
				}
				for j, q := range ps {
					switch {
					case q != f.Recv:
					case j == 0 && ft.IsExtFunc():
						c.errorf(l.P, "cannot infer type for receiver parameter. Specify it explicitly.")
					default:
						cannot(l.P, q.Param())
					}
				}
			}
			if t.K == types.Invalid && f.Ret.K == types.TypeParam {
				// As K2, the call's type is the parameter that
				// nothing inferred: no member of it resolves.
				return rigid(f, f.Ret)
			}
			return t
		}
		return t
	}
	f := x.Sym
	if r.failed.Line == 0 || f == nil || x.Fun.Pos() == r.failed || len(x.Args) == 0 {
		return t
	}
	uses := lambda || member && c.hasStub(m.X.Type())
	for _, a := range x.Args {
		uses = uses || c.hasStub(a.Type())
	}
	if !uses {
		return t
	}
	p := x.Fun.Pos()
	if f.Ext && member && lambda {
		for _, v := range f.TypeParams {
			if !f.Recv.Has(func(u types.Type) bool { return u.K == types.TypeParam && u.Param() == v }) {
				cannot(p, v)
			}
		}
		if msg := "candidate '" + f.Name + "' is inapplicable because of a receiver type mismatch"; !c.hasErrorAt(p, msg) {
			c.errorf(p, "%s", msg)
		}
		return types.InvalidType
	}
	if lambda {
		for _, v := range f.TypeParams {
			cannot(p, v)
		}
	}
	reported := false
	for i, a := range x.Args {
		pi := i
		if x.ArgMap != nil {
			pi = x.ArgMap[i]
		}
		at := a.Type()
		if at.K == types.Invalid || pi < 0 || pi >= len(f.Params) {
			continue
		}
		reported = true
		if !c.hasErrorAt(a.Pos(), "type mismatch: inferred type is ") {
			c.errorf(a.Pos(), "type mismatch: inferred type is %s but %s was expected", at, f.Params[pi].Type)
		}
	}
	if msg := "inapplicable candidate '" + f.Name + "': the type arguments of the builder it is in can't be inferred"; !reported && !c.hasErrorAt(p, msg) {
		c.errorf(p, "%s", msg)
	}
	if lambda && len(f.TypeParams) > 0 {
		return rigid(f, f.Ret)
	}
	return t
}
