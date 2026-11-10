package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Recorder receives facts the checker finds about the program that the
// AST it annotates doesn't keep, for tools that analyze code rather than
// compile it. Checking calls it from several goroutines, one per file at
// a time, so it must be safe for concurrent use. A node may be reported
// more than once when the checker checks code again, as lambdas while
// overloads are chosen: the last report counts.
//
// Compilation passes no Recorder and pays nothing for it.
type Recorder interface {
	// When reports whether the branches of a when with a subject cover
	// every case of it, its else branch aside, and whether its value is
	// used.
	When(x *syntax.When, covered, valueUsed bool)
	// Stmt reports whether the value of a statement of a block is used:
	// the last of a block whose value is.
	Stmt(s syntax.Stmt, valueUsed bool)
	// Unreachable reports a statement after one that always jumps.
	Unreachable(s syntax.Stmt)
	// Annotation reports the class an annotation of file f resolves to,
	// a nested one in another's arguments, ReplaceWith("x") in
	// @Deprecated, included: that one's position is its call's.
	Annotation(f *syntax.File, a *syntax.Annotation, cls *types.ClassSym)
	// Elvis reports whether the non-null type of an elvis expression's
	// left side is a subtype of its right side's type.
	Elvis(x *syntax.Binary, nonNullLeftSubtypeRight bool)
	// TypeRef reports the type a type reference resolves to, nested ones,
	// as type arguments and function types' parts, included. Unresolved
	// references are not reported.
	TypeRef(ref *syntax.TypeRef, t types.Type)
	// StmtType reports the type of an if, when or try used as a
	// statement, which the checker doesn't give its node.
	StmtType(x syntax.Expr, t types.Type)
}

// An OverrideRecorder is a Recorder that, once checking is done, is
// given a function that returns the functions of the supertypes of f's
// owner that f overrides, directly: as the checker resolves overrides.
type OverrideRecorder interface {
	Overridden(func(f *types.FunSym) []*types.FunSym)
}

// An AliasRecorder is a Recorder that is also told the typealias a type
// reference or a constructor call names: Alias in x: Alias and Alias(1).
type AliasRecorder interface {
	// Alias reports that x, a *syntax.TypeRef or a *syntax.Call, names a.
	Alias(x any, a *types.AliasSym)
}

// recordAlias reports to an AliasRecorder that x names a.
func (c *Checker) recordAlias(x any, a *types.AliasSym) {
	if r, ok := c.u.rec.(AliasRecorder); ok {
		r.Alias(x, a)
	}
}
