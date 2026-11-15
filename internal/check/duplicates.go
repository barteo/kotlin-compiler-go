package check

import "github.com/barteo/kotlin-compiler-go/internal/types"

// sameDeclaration reports whether f and g, top-level functions of the
// class path, are one function of two modules: of the same package,
// name and signature, as a module's in the output of two modules that
// both declare it. As in K2 (its ConeEquivalentCallConflictResolver),
// a call then has one candidate, not two ambiguous ones. Functions of
// one package that only erase to one descriptor, as Long's and ULong's
// countTrailingZeroBits, aren't one.
func sameDeclaration(f, g *types.FunSym) bool {
	if !equivalentFun(f, g) || f.JVMName != g.JVMName || len(f.Params) != len(g.Params) || len(f.TypeParams) != len(g.TypeParams) {
		return false
	}
	if f.Ext && f.Recv.String() != g.Recv.String() || f.Ret.String() != g.Ret.String() {
		return false
	}
	for i, p := range f.Params {
		if p.Type.String() != g.Params[i].Type.String() {
			return false
		}
	}
	return true
}
