package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// javaVariances checks the arguments of a call x of cl's function, whose
// parameter types are param(i), with javaVariance, and their names.
func (c *Checker) javaVariances(x *syntax.Call, cl *callee, param func(i int) types.Type) {
	if cl.f.Owner == nil || !isJavaClass(cl.f.Owner) {
		return
	}
	c.javaNamedArgs(x, cl.f)
	for i, a := range x.Args {
		if _, ok := a.(*syntax.Lambda); !ok {
			c.javaVariance(cl.f, cl.param(i), a, param(i))
		}
	}
}

// javaVariance checks an argument a of a call of a Java function f, for
// a parameter p whose type, with the call's type arguments, is want. As
// in K2 (FirJavaGenericVarianceViolationTypeChecker), a Java collection
// parameter is a platform type that takes a read-only collection, List<T>
// for java.util.List<T>, but a mutable one only of its exact element
// type: a MutableList<String> passed for a List<Object> would let the Java
// code add any object to it (JAVA_TYPE_MISMATCH).
func (c *Checker) javaVariance(f *types.FunSym, p *types.VarSym, a syntax.Expr, want types.Type) {
	if f == nil || f.Owner == nil || !isJavaClass(f.Owner) || p.Vararg {
		return
	}
	// A Java collection type with type arguments: raw ones take anything.
	if !want.P || want.K != types.Class || want.A == nil || want.C.ReadOnly == nil {
		return
	}
	t := a.Type()
	if t.K != types.Class && !t.IsParam() {
		return
	}
	// A value of another class than the mutable collection's, as a
	// read-only List, is what the platform type takes.
	if _, ok := c.asSuper(t.NonNull(), want.C); !ok {
		return
	}
	lower := want // the platform type's lower bound, nullable
	lower.P, lower.N = false, true
	if !c.subtype(dropOut(t, true), lower) {
		c.errorf(a.Pos(), "Java type mismatch: expected '%s' but found '%s'. Use explicit cast.", want, t)
	}
}

// dropOut returns t with its out projections in covariant positions as
// invariant arguments, and its star projections there as Any?: K2 compares
// a collection's type with the parameter's without them, since what the
// check is about is the element type, not whether it can be written.
func dropOut(t types.Type, covariant bool) types.Type {
	if t.K != types.Class || t.A == nil {
		return t
	}
	args := make([]types.Arg, t.A.Len())
	copy(args, t.A.List)
	for i, a := range args {
		switch a.V {
		case types.Out:
			if covariant {
				args[i] = types.Arg{T: a.T}
			}
		case types.Star:
			if covariant {
				args[i] = types.Arg{T: types.AnyType.Nullable()}
			}
		case types.In:
			args[i].T = dropOut(a.T, !covariant)
		}
	}
	t.A = types.MakeArgs(args)
	return t
}
