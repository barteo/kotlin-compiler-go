package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// lambdaBranchWant returns the expected type of branch b of an if or a
// when whose expected type is want: if b is a lambda, the function type
// of an earlier branch, prev, which gives its parameters: if (c) consumer
// else { node -> put(node) }, and when { f != null -> f else -> {{ this
// + "K" }} } for a String.() -> String f. One without them has its own
// type, which the branches' type is above: if (c) { { 1 } } else
// { { 1f } }.
func lambdaBranchWant(b *syntax.Block, prev, want types.Type, valueUsed bool) types.Type {
	if l := blockLambda(b); l != nil && valueUsed && want.K == types.Invalid && prev.Flex().IsFunc() && !prev.HasVars() && (len(l.Params) > 0 || !noInputs(prev.Flex())) {
		return prev
	}
	return want
}

// firstFuncBranch returns prev, the function type of an earlier branch of
// a when, or branch type t if there is none (see lambdaBranchWant).
func firstFuncBranch(prev, t types.Type) types.Type {
	if prev.K == types.Invalid && t.Flex().IsFunc() && !t.HasVars() {
		return t
	}
	return prev
}
