package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// refOverloadHint returns the function type of the only overload that a
// reference ::f, which lambda l ends with, can name for l's result type r
// in system s, as in Kotlin, which resolves the reference with the
// constraints on r: foo(Int): Unit of myLet { ::foo } ?: unitFun() for
// myLet(x: () -> (T) -> R): R?, where R must be Unit, though foo(String):
// String is in scope too. It only looks at a reference without a
// receiver to overloads that aren't generic, extensions, suspend or
// vararg functions.
func (c *Checker) refOverloadHint(s *system, l *syntax.Lambda, r types.Type) (types.Type, bool) {
	stmts := l.Body.Stmts
	es, _ := stmts[len(stmts)-1].(*syntax.ExprStmt)
	ref, _ := es.X.(*syntax.Lambda)
	if ref == nil || ref.Ref == nil {
		return types.InvalidType, false
	}
	x := ref.Ref
	if x.X != nil || x.Type != nil || x.Bound != nil || c.isVar(x.Name) || c.lookupClass(x.Name) != nil || c.cls != nil && c.hasCallable(c.cls, x.Name) {
		return types.InvalidType, false
	}
	fs, _ := c.scope.findFuns(x.Name)
	if len(fs) == 0 {
		fs = c.topFuns(x.Name)
	}
	if len(fs) < 2 {
		return types.InvalidType, false
	}
	var fit types.Type
	n := 0
	for _, f := range fs {
		ensureFun(f)
		if f.Ext || f.Suspend || len(f.TypeParams) > 0 || f.Ret.K == types.Invalid {
			return types.InvalidType, false
		}
		var ps []types.Type
		for _, q := range f.Params {
			if q.Vararg {
				return types.InvalidType, false
			}
			ps = append(ps, q.Type)
		}
		ft := c.u.ext.funcs.Type(ps, f.Ret)
		trial := s.clone()
		if trial.sub(ft, r); trial.ok && len(trial.errs) == len(s.errs) {
			fit = ft
			n++
		}
	}
	return fit, n == 1
}
