// Package psi parses Kotlin source into a lossless syntax tree with the
// shape of the Kotlin compiler's PSI.
//
// The parser is a port of the Kotlin 2.4.21 parser (KotlinParsing,
// KotlinExpressionParsing) and of the parts of IntelliJ's PsiBuilderImpl it
// relies on, so element kinds, nesting, the placement of whitespace and
// comments, and syntax error messages match what Kotlin's own PSI gives.
// The tests compare trees with dumps from stock Kotlin (see tools/psidump).
// The one deliberate difference is the message for '*' where a type is
// expected: "star projection is not a type" instead of "Type expected".
//
// A Snapshot is immutable and safe for concurrent use. Its nodes live in one
// preorder array; a NodeRef is a small value referring to one of them, and
// the zero NodeRef is inert. Ranges are UTF-8 byte offsets; Snapshot
// converts them to UTF-16 offsets and 1-based lines and columns.
//
// Kind numbers come from kinds.txt, which is append-only, so they can be
// persisted. The one exception is a kind Kotlin renames, which is renamed in
// place and keeps its number (CONTEXT_RECEIVER_LIST became
// CONTEXT_PARAMETER_LIST in Kotlin 2.4).
package psi

//go:generate sh -c "cd .. && go run ./cmd/psi-kinds"
