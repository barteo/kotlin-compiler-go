package check

import "github.com/barteo/kotlin-compiler-go/internal/types"

// localClassParams has local class sym see the type parameters around it,
// as an object expression's class does: they are its last type
// parameters, which a type of it substitutes as its own, so that the
// C<F> that foo<CharSequence, Number>() returns for a class C<G> in fun
// <E, F> foo() has a CharSequence e: E.
func (c *Checker) localClassParams(sym *types.ClassSym) {
	if tps := c.tparams; len(tps) > 0 {
		sym.TypeParams = append(sym.TypeParams[:len(sym.TypeParams):len(sym.TypeParams)], tps...)
		sym.OuterParams = len(tps)
	}
}

// localOwnParams returns the type parameters of class cls that a reference
// to it gives arguments for, and the arguments of the others: those of a
// local class's code around it, which are themselves where it is in
// scope.
func localOwnParams(cls *types.ClassSym) ([]*types.TypeParamSym, []types.Arg) {
	if cls == nil {
		return nil, nil
	}
	if !cls.Local || cls.Inner || cls.OuterParams == 0 {
		return cls.TypeParams, nil
	}
	own := len(cls.TypeParams) - cls.OuterParams
	hidden := make([]types.Arg, 0, cls.OuterParams)
	for _, p := range cls.TypeParams[own:] {
		hidden = append(hidden, types.Arg{T: types.ParamType(p)})
	}
	return cls.TypeParams[:own], hidden
}
