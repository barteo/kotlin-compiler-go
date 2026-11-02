package check

import "github.com/barteo/kotlin-compiler-go/internal/syntax"

// Recorder receives facts the checker finds about the program that the
// AST it annotates doesn't keep, for tools that analyze code rather than
// compile it. Checking calls it from several goroutines, one per file at
// a time, so it must be safe for concurrent use. A node may be reported
// more than once when the checker checks code again, as lambdas while
// overloads are chosen: the last report counts.
//
// Compilation passes no Recorder and pays nothing for it.
type Recorder interface {
	// When reports whether a when covers every case of its subject, with
	// an else branch or without, and whether its value is used.
	When(x *syntax.When, exhaustive, valueUsed bool)
	// Stmt reports whether the value of a statement of a block is used:
	// the last of a block whose value is.
	Stmt(s syntax.Stmt, valueUsed bool)
	// Unreachable reports a statement after one that always jumps.
	Unreachable(s syntax.Stmt)
}
