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
source). The parser and lexer are ports of Kotlin 2.4.21's, so element
kinds, nesting, whitespace and comment placement match the Kotlin
compiler's PSI. The tests compare trees with dumps from stock Kotlin made by
[`tools/psidump`](../tools/psidump/README.md) and committed under
`psi/testdata`, so CI needs no JVM.

- **Nodes** live in one preorder array. IDs run from 1 (the `FILE` node) to
  `NodeCount()`, a parent's ID is lower than its children's, and a node's
  subtree is exactly `[ID, SubtreeEnd)`. A `NodeRef` is a small comparable
  value; the zero `NodeRef` is invalid and every method is safe on it.
- **Kinds** are numbered by `psi/kinds.txt`, which is append-only, so kind
  numbers can be persisted. A kind Kotlin renames is renamed in place and
  keeps its number (`CONTEXT_RECEIVER_LIST` became `CONTEXT_PARAMETER_LIST`
  in Kotlin 2.4); a kind Kotlin drops (`HEADER_KEYWORD`, `IMPL_KEYWORD`)
  stays in the file and is never produced. `go generate ./psi` regenerates
  `kinds_gen.go`; CI runs `go run ./cmd/psi-kinds -check`.
- **Offsets** are UTF-8 byte offsets. A snapshot converts them to UTF-16
  offsets and to 1-based lines and columns. `psi` doesn't normalize line
  endings.
- **Snapshots** are immutable and safe for concurrent use. A `Parser` can
  be reused per worker; snapshots don't keep its buffers.
- **Reparse** (`psi.Reparse(old, from, to, text)`) gives the snapshot of an
  edited text and always equals `Parse` of it. Some edits lie strictly
  inside one declaration body, the lazily parsed `BLOCK` that IntelliJ
  calls a chameleon. For those, only that body is parsed and spliced into
  a copy of the old tree, as IntelliJ's `BlockSupportImpl` does. Two
  conditions apply: the body's braces must stay balanced, and the old and
  new body must each leave the lexer in the state it started in. An
  unmatched backtick or an unterminated raw string fails the lexer
  condition. A body inside a string template is never spliced. Every
  other edit parses the whole text. `TestReparse` compares chains of
  random edits to every `testdata` file against `Parse`, and
  `FuzzReparse` runs in CI. A keystroke in a 47 KB file takes 0.7 ms,
  against 7 ms for a whole parse.

### Performance

`BenchmarkFrontends` (`internal/lower/bench_test.go`) parses a corpus with
each of the two paths the driver's parser can take: `kc`'s own parser
(`syntax`), and `psi.Parse` followed by lowering (`psi+lower`). It also times
`psi.Parse` alone, both with a new parser per file (`psi`) and with one reused
`psi.Parser` (`psi-reused`). It uses only files both frontends accept.

```sh
go test ./internal/lower -run '^$' -bench Frontends -benchmem -count 6
KC_BENCH_CORPUS=<dir> go test ./internal/lower -run '^$' -bench Frontends -benchmem -count 6
```

Medians of 6 runs, Go 1.27.2, 4-core Intel Xeon (linux/amd64), on
`testdata` (489 of 503 `.kt` files, 0.83 MB) and on kotlinx-coroutines-core
1.11.0 (707 of 711 files, 2.64 MB; `bench/survey.sh` clones it):

| Path | testdata MB/s | testdata MB/op | testdata allocs/op | coroutines MB/s | coroutines MB/op | coroutines allocs/op |
|---|---:|---:|---:|---:|---:|---:|
| `syntax` | 13.6 | 64.7 | 195k | 33.1 | 99.9 | 300k |
| `psi` | 4.1 | 212.6 | 136k | 11.1 | 323.6 | 332k |
| `psi-reused` | 5.8 | 150.2 | 124k | 13.9 | 229.0 | 315k |
| `psi+lower` | 3.0 | 252.8 | 469k | 8.6 | 382.9 | 823k |

- **Lowering costs about what parsing does**, as planned. On coroutines it
  takes 68 ms per pass, against 80 ms for `syntax`; on testdata 70 ms,
  against 61 ms.
- **`psi.Parse` itself is about 3× slower than `kc`'s parser**, and that
  makes the whole psi path 3.8× (coroutines) to 4.5× (testdata) slower.
  The parser builds the full lossless tree, including whitespace, comments
  and every intermediate element. Most of its allocation is the marker list
  (`builder.mark`, 54% of bytes) and the tree under construction
  (`treeWriter.add`, 27%). A reused `Parser` keeps those buffers between
  files and cuts the parse to about 2.4× `syntax`. Runs of the same
  benchmark varied by up to 10%.

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
