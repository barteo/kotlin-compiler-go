# PSI and analysis

Two public packages give tools such as linters a JVM-free view of Kotlin
code: [`psi`](../psi/doc.go) parses it, and [`analysis`](../analysis/analysis.go)
runs `kc`'s checker on what `psi` parsed and reports what it resolved.

```
.kt text ─▶ psi.Parse ─▶ *psi.Snapshot ─▶ internal/lower ─▶ kc AST + provenance
                                                   ─▶ check (with a Recorder) ─▶ analysis facts
```

## psi

`psi.Parse(path, text)` never fails: syntax errors are diagnostics on the
snapshot, and the tree stays lossless (its leaves' texts concatenate to the
source). The parser and lexer are ports of Kotlin 2.0.21's, so element
kinds, nesting, whitespace and comment placement match the Kotlin
compiler's PSI. The tests compare trees with dumps from stock Kotlin made by
[`tools/psidump`](../tools/psidump/README.md) and committed under
`psi/testdata`, so CI needs no JVM.

- **Nodes** live in one preorder array. IDs run from 1 (the `FILE` node) to
  `NodeCount()`, a parent's ID is lower than its children's, and a node's
  subtree is exactly `[ID, SubtreeEnd)`. A `NodeRef` is a small comparable
  value; the zero `NodeRef` is invalid and every method is safe on it.
- **Kinds** are numbered by `psi/kinds.txt`, which is append-only, so kind
  numbers can be persisted. `go generate ./psi` regenerates
  `kinds_gen.go`; CI runs `go run ./cmd/psi-kinds -check`.
- **Offsets** are UTF-8 byte offsets. A snapshot converts them to UTF-16
  offsets and to 1-based lines and columns. `psi` doesn't normalize line
  endings.
- **Snapshots** are immutable and safe for concurrent use. A `Parser` can
  be reused per worker; snapshots don't keep its buffers.

## analysis

`analysis.AnalyzeArena(ctx, cfg, snapshots)` analyzes the snapshots as one
module:

1. Each snapshot is lowered to `kc`'s AST by `internal/lower`, directly
   from the tree, with a provenance table from AST node to psi node. The
   lowering produces the same AST as `kc`'s own parser; a differential test
   compares them on every `testdata/**/*.kt`, and CI runs the whole test
   suite with `KC_FRONTEND=psi`.
2. The library (JDK, class path) is opened as `kc` opens it, and the files
   are checked by `check.CheckContext`, which honors cancellation. A
   `check.Recorder` reports what the AST doesn't keep: when coverage,
   whether a statement's value is used, unreachable statements.
3. The checked AST is walked back to psi nodes, and facts are exported
   keyed by `ElementID{Snapshot, Node}`.

Code that `kc` can't compile is reported as diagnostics (`source_error`,
or `unsupported:<feature>` for constructs `kc` doesn't support) and no
facts at all: callers must not act on partial results. Scripts (`.kts`)
return `ErrUnsupported`.

`Data.Capabilities` lists the fact families that are computed; a fact field
of a family outside it is always zero, and a caller that needs it should
refuse to run rather than read zeros. `Result`'s methods return deep copies.

### Where facts come from

| Fact | Source |
|---|---|
| `Type` | the checked expression's or declaration's `types.Type`, converted to Kotlin's names (`kotlin.Any`, `kotlin.Function1`, ...) |
| `Symbol` | the symbol an identifier, member, call or declaration resolved to, interned per result |
| `Call` | the resolved function, argument mapping, receivers, type arguments and inline flags |
| `When` | the recorder; `Exhaustive` and `Missing` ignore the `else` branch |
| `FlowNonNull`, `DefinitelyNonNull` | a nullable declaration read as non-null after a smart cast |
| `UsedAsExpression`, `Unreachable` | the recorder |
| `ReturnTarget`, `ReceiverTarget` | the jump's lambda or enclosing function; `this`'s receiver |

Facts recorded on an expression are copied to the wrapper nodes Kotlin's
PSI puts around it (`DOT_QUALIFIED_EXPRESSION`, `PARENTHESIZED`, labels,
annotations), and a type reference's type is set on both its `USER_TYPE`
and its `TYPE_REFERENCE`.
