package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// varargNothing reports call x of a function whose vararg parameter's
// element type is one of its type parameters, not reified, that the
// call infers or is given as Nothing, or as Nothing? if the parameter
// has a bound other than Any and Any?: as kotlinc's
// FirVarargWithNonTrivialUpperBoundInferredToNothingChecker, whose
// ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING says the array may
// fail at run time. It is reported at the type argument, if written, and
// else at the call's name. kc leaves out constructor calls.
func (c *Checker) varargNothing(x *syntax.Call) {
	f := x.Sym
	if f == nil || x.Ctor != nil || x.Invoke || x.Infix || isArrayFactory(f.Builtin) || len(f.TypeParams) == 0 || x.Vararg.K != types.Class || x.Vararg.A == nil || x.Vararg.A.Len() != 1 {
		return
	}
	for _, p := range f.Params {
		if !p.Vararg || p.Type.K != types.Class || p.Type.A == nil || p.Type.A.Len() != 1 {
			continue
		}
		et := p.Type.A.List[0].T
		if et.K != types.TypeParam || et.N {
			continue
		}
		tp := et.Param()
		i := -1
		for j, q := range f.TypeParams {
			if q == tp {
				i = j
			}
		}
		if i < 0 || tp.Reified {
			continue
		}
		at := x.Vararg.A.List[0].T
		switch {
		case at.K == types.Nothing && !at.N:
		case at.K == types.Nothing || at.K == types.Null:
			bounded := false
			for _, b := range tp.Bounds {
				if b.K != types.Any {
					bounded = true
				}
			}
			if !bounded {
				continue
			}
			at = types.NullType
		default:
			continue
		}
		pos := syntax.StartPos(x)
		if m, ok := x.Fun.(*syntax.Member); ok {
			pos = m.P // x.f(), reported at f
		}
		if i < len(x.TypeArgs) && x.TypeArgs[i] != nil {
			pos = x.TypeArgs[i].P
		}
		c.warnf(pos, "ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING",
			"'%s' is inferred as the vararg parameter type, which might lead to exceptions at runtime. Consider specifying the type argument explicitly.", at)
	}
}
