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
	// Annotation reports the class an annotation of file f resolves to.
	Annotation(f *syntax.File, a *syntax.Annotation, cls *types.ClassSym)
	// Elvis reports whether the non-null type of an elvis expression's
	// left side is a subtype of its right side's type.
	Elvis(x *syntax.Binary, nonNullLeftSubtypeRight bool)
}
