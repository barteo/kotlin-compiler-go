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
			c.errorf(p, "CANNOT_INFER_PARAMETER_TYPE", "%s", msg)
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
						c.errorf(l.P, "CANNOT_INFER_RECEIVER_PARAMETER_TYPE", "cannot infer type for receiver parameter. Specify it explicitly.")
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
			c.errorf(p, "UNRESOLVED_REFERENCE_WRONG_RECEIVER", "%s", msg)
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
			c.errorf(a.Pos(), "ARGUMENT_TYPE_MISMATCH", "type mismatch: inferred type is %s but %s was expected", at, f.Params[pi].Type)
		}
	}
	if msg := "inapplicable candidate '" + f.Name + "': the type arguments of the builder it is in can't be inferred"; !reported && !c.hasErrorAt(p, msg) {
		c.errorf(p, "INAPPLICABLE_CANDIDATE", "%s", msg)
	}
	if lambda && len(f.TypeParams) > 0 {
		return rigid(f, f.Ret)
	}
	return t
}

// uninferredValue reports whether t, the type of init, a local
// variable's initializer, has a type argument that the call init
// couldn't infer: the checks of init reported the errors after the
// first n, one of them that a type parameter of the call's function
// can't be inferred, where the call is, and the call takes a lambda, as
// a builder's, or such a variable (see uninferredArg). A type with
// another error, as of an argument, isn't one kotlinc reports the uses
// of.
func (c *Checker) uninferredValue(init syntax.Expr, t types.Type, n int) bool {
	x, ok := init.(*syntax.Call)
	invalid := func(u types.Type) bool { return u.K == types.Invalid }
	if !ok || x.Sym == nil || t.K == types.Invalid || !t.Has(invalid) {
		return false
	}
	if c.uninferredCall == x {
		return true
	}
	if !slices.ContainsFunc(x.Args, func(a syntax.Expr) bool { _, ok := a.(*syntax.Lambda); return ok }) {
		return false
	}
	return slices.ContainsFunc(c.errs[n:], func(e *syntax.Error) bool {
		return e.Pos.Line == x.Fun.Pos().Line && slices.ContainsFunc(x.Sym.TypeParams, func(v *types.TypeParamSym) bool {
			return e.Msg == "cannot infer type for type parameter '"+v.Name+"'. Specify it explicitly."
		})
	})
}

// uninferredArg reports generic call x if an argument for a parameter
// with one of its type parameters is a local variable whose type has a
// type argument that nothing inferred, buildee of val buildee = build { }:
// as kotlinc, whose type for it has an error no type equals, the call
// can't infer the type parameter either, id(buildee) and
// id<Buildee<String>>(buildee). The error is at the type argument, if
// it is written.
func (c *Checker) uninferredArg(x *syntax.Call) {
	f := x.Sym
	if f == nil || len(f.TypeParams) == 0 {
		return
	}
	for i, a := range x.Args {
		id, ok := a.(*syntax.Ident)
		if !ok || id.Sym == nil || !c.uninferred[id.Sym] {
			continue
		}
		pi := i
		if x.ArgMap != nil {
			pi = x.ArgMap[i]
		}
		if pi < 0 || pi >= len(f.Params) {
			continue
		}
		for j, v := range f.TypeParams {
			if !f.Params[pi].Type.Has(func(u types.Type) bool { return u.K == types.TypeParam && u.Param() == v }) {
				continue
			}
			p := x.Fun.Pos()
			if j < len(x.TypeArgs) && !x.TypeArgs[j].Underscore {
				p = x.TypeArgs[j].P
			}
			msg := "cannot infer type for type parameter '" + v.Name + "'. Specify it explicitly."
			if !c.hasErrorAt(p, msg) {
				c.errorf(p, "CANNOT_INFER_PARAMETER_TYPE", "%s", msg)
			}
			c.uninferredCall = x
			return
		}
	}
}
