package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// contractArgs returns is, the type that function f's contract says its
// parameter param (its receiver, if negative), of type at, is of, with
// the type arguments f's type parameters in it have where the parameter's
// declared type has them, as the call infers them: Terminal<T> of
// AsyncStatus<T>.isPending()'s returns(false) implies (this is Terminal)
// is a Terminal<String> for a Pending<String>, an AsyncStatus<String>.
func (c *Checker) contractArgs(f *types.FunSym, param int, at, is types.Type) types.Type {
	var decl types.Type
	switch {
	case param < 0:
		decl = f.Recv
	case param < len(f.Params):
		decl = f.Params[param].Type
	default:
		return is
	}
	decl = decl.NonNull()
	if decl.K != types.Class || decl.A == nil || len(f.TypeParams) == 0 {
		return is
	}
	sup, ok := c.asSuper(at.Flex().NonNull(), decl.C)
	if !ok || sup.A == nil {
		return is
	}
	sub := types.Subst{}
	for i, a := range decl.A.List {
		if a.V == types.Star || a.T.K != types.TypeParam || a.T.N || !slices.Contains(f.TypeParams, a.T.Param()) {
			continue
		}
		if b := sup.A.Get(i); b.V != types.Star && b.V != types.In && !b.T.HasVars() {
			sub[a.T.Param()] = types.Arg{T: b.T}
		}
	}
	if len(sub) == 0 {
		return is
	}
	return sub.Apply(is)
}
