package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// A typealias constructor call, Test(1, "") for typealias Test<A, B> =
// Hr<A, B, Foo<A>, Bar<B>>, infers the alias's type arguments, and the
// class's that the alias gives in terms of them are inferred with them,
// as K2 does: C is Foo<A>. aliasUnderscores makes such an argument a _
// whose Type is the alias's argument in terms of the class's own type
// parameters, Foo<A> for C, and underscoreSub makes the variable for it
// equal to that.

// holeDep returns arg, a type argument of class cls in the expansion of a
// typealias, with the placeholders of holes replaced by the class's type
// parameters they are the arguments of, if each is one, as A for Foo<A>
// in Hr<A, B, Foo<A>, Bar<B>>.
func holeDep(arg types.Type, expansion types.Type, cls *types.ClassSym, holes map[*types.TypeParamSym]bool) (types.Type, bool) {
	sub := types.Subst{}
	for k, a := range expansion.A.List {
		if k < len(cls.TypeParams) && a.V == types.Inv && a.T.IsParam() && !a.T.N && holes[a.T.Param()] {
			if _, ok := sub[a.T.Param()]; !ok {
				sub[a.T.Param()] = types.Arg{T: types.ParamType(cls.TypeParams[k])}
			}
		}
	}
	ok := !arg.Has(func(u types.Type) bool {
		if !u.IsParam() || !holes[u.Param()] {
			return false
		}
		_, found := sub[u.Param()]
		return !found
	})
	if !ok {
		return types.InvalidType, false
	}
	return sub.ApplyExact(arg), true
}

// relateUnderscores makes the variable of each _ among a call's type
// arguments that holeDep gave a type equal to that type, with the call's
// substitution sub.
func relateUnderscores(sys *system, tparams []*types.TypeParamSym, typeArgs []*syntax.TypeArg, sub types.Subst) {
	for j, ta := range typeArgs {
		if !ta.Underscore || ta.Type == nil || ta.Type.Known == nil || j >= len(tparams) {
			continue
		}
		v, dep := sub[tparams[j]].T, sub.Apply(*ta.Type.Known)
		sys.sub(v, dep)
		sys.sub(dep, v)
	}
}
