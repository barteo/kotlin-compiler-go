package check

import "github.com/barteo/kotlin-compiler-go/internal/types"

// javaSuspend makes a Java instance method whose last parameter is a
// kotlin.coroutines.Continuation<R> a suspend function returning R, as
// Kotlin sees it: f(x, Continuation<R>) is suspend fun f(x): R. A raw or
// star-projected continuation returns Any?.
func javaSuspend(f *types.FunSym) {
	if f.Static || f.Name == "<init>" || len(f.Params) == 0 || f.Params[len(f.Params)-1].Vararg {
		return
	}
	p := f.Params[len(f.Params)-1].Type
	if p.K != types.Class || p.C == nil || p.C.Name != "kotlin/coroutines/Continuation" {
		return
	}
	ret := types.AnyType.Nullable()
	if p.A.Len() == 1 && p.A.Get(0).V != types.Star && p.A.Get(0).T.K != types.Invalid {
		ret = p.A.Get(0).T
	}
	f.Params = f.Params[:len(f.Params)-1]
	f.Ret, f.Suspend = ret, true
}
