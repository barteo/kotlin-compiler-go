# A Kotlin language server on kc

This is the plan for `kc-lsp`, a language server for Kotlin written in Go
on top of `kc`. It aims at what JetBrains'
[kotlin-lsp](https://github.com/Kotlin/kotlin-lsp) offers for JVM projects:

- diagnostics with quick fixes, completion, hover, definition, references,
  rename, signature help and call hierarchy;
- semantic highlighting, inlay hints, folding, formatting, organize
  imports and workspace symbols.

kotlin-lsp is an alpha, runs IntelliJ in a JVM and supports Gradle and
Maven JVM projects (no KMP). Nothing here is built yet; the milestones
below are the order of work.

## Why

- `kc` checks octi-server (60 files, 7.7k lines) in about 45 ms once its
  class path is read, and a body-only edit in about 25 ms
  ([analysis-api.md, step 25](analysis-api.md#step-25-body-only-edits-check-only-the-edited-files)).
  That is fast enough to check on every pause in typing, with no JVM and
  no warm-up.
- `psi` already parses with the Kotlin compiler's tree shape and error
  messages, never fails, and converts offsets to UTF-16, which is what LSP
  positions count.
- `analysis` already exports what an editor shows: expression types,
  resolved symbols with their declarations, calls with argument mappings,
  implicit receivers, overrides ([psi.md](psi.md#analysis)).

The targets are in [Benchmarks](#benchmarks): cold open to first
diagnostics under 1 s on a 200-file module, semantic diagnostics after a
body edit under 30 ms (median) on octi-server, completion under 100 ms.

## What exists, and what doesn't

Checked against the code on `ee444da`:

| Piece | State | Gap for an editor |
|---|---|---|
| `psi` | lossless, never fails, UTF-16 offsets (`Snapshot.Position`) | no (line, UTF-16 column) → byte offset; no incremental reparse (a file reparses whole, which is cheap) |
| `analysis` | body facts keyed by psi node | no facts at all if any file has an error (`analysis/build.go:39-41`, `:64`); opens a new library per call; not incremental |
| `analysis/ka` | error tolerant, library cache, body-only edits check only edited files | signatures only (`check.Options{Signatures: true}`, `analysis/ka/ka.go:610`): no expression facts, no body diagnostics |
| `internal/lower` | keeps a class's other members around a broken one (M2) | drops a whole function or property for any error element in it (`syntaxError`, `lower.go:265`) or any unsupported construct |
| `internal/check` | goes on after errors (`types.InvalidType` is compatible with everything) | no "names visible here": `scope` (`scope.go:26`), `state` and `implicitRecv` (`lambda.go:2406`) live only while a body is checked, and lookup is by name; cancellation only between passes and per top-level declaration |
| Diagnostics | `syntax.Error{Pos, Msg, Feature}` | start position only, byte column, no code, no severity, no warnings; 92% agreement per test file, 68% per expected error |
| Project model | flat `-cp`, `-friend-paths` | nothing reads Gradle; `bench/kotlinpoet/*.init.gradle.kts` reflects on the Kotlin plugin for one module |

The checker writes its results into the syntax trees (types, symbols and
structural rewrites, about 170 places in `internal/check`; codegen reads
them back), so a tree can't be checked twice and every update lowers
every file again
([step 24](analysis-api.md#step-24-what-a-one-file-edit-costs-and-what-incremental-checking-needs)).
The plan works within that until L6.

## Design

```
editor ─stdio JSON-RPC─▶ cmd/kc-lsp ─▶ internal/lsp            protocol, server, handlers
                                        ├─ internal/lsp/jsonrpc framing, cancellation (stdlib only)
                                        ├─ internal/project     kc-lsp.json, Gradle import
                                        └─ ide                  documents, snapshots, features
                                             ├─ psi                       syntax tier: every change
                                             └─ analysis.Session (new)    semantic tier: debounced
                                                  ├─ internal/kasession   library cache, reuse (from ka)
                                                  └─ internal/check       + probe, partial results
```

- **Protocol.** JSON-RPC 2.0 with Content-Length framing and LSP 3.17's
  types, written by hand as features need them (about 60), so `go.mod`
  keeps no dependencies. `$/cancelRequest` cancels the request's
  `context`. Positions are UTF-16 unless the client accepts
  `positionEncoding: utf-8`.
- **`ide`** holds the open documents over the files on disk, an immutable
  `Snapshot` (psi snapshots and each module's last semantic result) and
  the features as functions of a snapshot (`Hover`, `Definition`,
  `References`, `Complete`, `Rename`, ...) that return Go structs, not LSP
  types. `internal/lsp` translates. `ide` stays internal until it settles.
- **Two tiers.**
  - *Syntax*: each change reparses the file with `psi.Parse` (under 5 ms)
    for syntax diagnostics, outline, folding, selection ranges and
    lexical highlighting.
  - *Semantic*: `analysis.Session.Update` runs about 50 ms after the last
    change and is cancelled by a newer one. Requests are answered from
    the newest finished snapshot; positions in an older one are mapped
    through the edits since.
- **Concurrency.** One update goroutine per module; requests read
  immutable snapshots without locks.

### The semantic session

`analysis` gets an incremental API:

```go
s := analysis.Open(cfg)
r, err := s.Update(ctx, sources) // sources carry editor texts, as ka.Source
```

- `ka`'s library cache and body-only reuse (`libcache.go`, `reuse.go`,
  `signature.go`) move to `internal/kasession`, shared by `ka`
  (signatures only, for KSP) and `analysis` (bodies).
- **Partial facts.** A file's facts are exported for each declaration in
  which lowering and checking reported no error. `Result.Complete()`
  reports whether there was no error anywhere, which is the contract
  `AnalyzeArena` keeps.
- Facts are kept per file and carried over for files an update doesn't
  check again, as `ka` keeps model files.

## Project model

- **`kc-lsp.json`** is the native format, and the tests'. Each module has a
  name, a kind (`main`, `test`), Kotlin and Java source roots, a class
  path, friend modules or paths, module dependencies, a JDK home and
  compiler options (`-opt-in`, `compose`, `serialization`, `parcelize`,
  `-jvm-default`, explicit API). Maven, Bazel and hand-made setups write
  it themselves.
- **Gradle.** `kc-lsp` runs `./gradlew -q --init-script
  kc-lsp.init.gradle.kts kcLspModel`, with the script embedded in the
  binary. It generalizes `bench/kotlinpoet/classpath.init.gradle.kts`:
  for each Kotlin JVM compilation of each project it writes the sources,
  `compileDependencyFiles`, friend paths, module name and compiler
  options as JSON. The model is cached in `.kc-lsp/model.json`, keyed by
  the hashes of the build files (`*.gradle*`, `gradle.properties`,
  `libs.versions.toml`), and imported again when they change. Import is
  the only time a JVM runs.
- **Source sets.** A test source set is its own module with main as a
  friend, as `-Xfriend-paths` does.
- **Several modules.** Until L6 a project dependency is its Gradle class
  output, with a notice when that is missing or older than its sources.
  From L6 it is the other module's sources, checked as a library.
- **Out of scope:** `.kts` (build scripts included), KMP, Android (AGP),
  and code intelligence for Java. Java sources in a module are read for
  their declarations, as `kc` reads them (`internal/javasrc`).

## Diagnostics

`kc`'s errors are shown as errors only where they are reliable:

- **Syntax errors** come from `psi`, with ranges and kotlinc's messages.
- **Ranges.** A checker error's position maps to the smallest psi node
  starting there, through `lower.Provenance`.
- **Codes.** `syntax.Error` gets a `Code`, named after kotlinc's
  factories (`UNRESOLVED_REFERENCE`, `ARGUMENT_TYPE_MISMATCH`, ...),
  starting with the 40 most common messages.
- **Precision.** `bench/conformance` gets a mode that measures each code's
  false errors on the diagnostics tests `kotlinc` accepts. A code is an
  error if its precision is at least 99%, otherwise a warning, and
  messages without a code are hidden. The setting `kc.diagnostics.level`
  shows them all. The table is committed as `bench/results/lsp-precision.tsv`
  and works as a ratchet, as `diagnostics.tsv` does.
- **Unsupported features** (`unsupported:<feature>`) are never errors in
  the editor: they are information ("`kc` doesn't support this; `kotlinc`
  may accept it"), and the declaration has no facts.

## Milestones

| | Milestone | Features | Acceptance |
|---|---|---|---|
| L0 | Protocol, syntax tier | `initialize`, incremental document sync, syntax diagnostics, `documentSymbol`, `foldingRange`, `selectionRange`, lexical `semanticTokens` | golden scripts pass; smoke runs in VS Code and Neovim written down; syntax diagnostics p95 < 10 ms on coroutines' largest file |
| L1 | Project model, semantic session | `kc-lsp.json`, Gradle import, `analysis.Session`, semantic diagnostics with ranges, codes and the precision policy | session facts equal `AnalyzeArena`'s on `testdata/run`; incremental results equal a whole check (as `TestIncremental`); the Gradle import of kotlinpoet, Ktor and octi-server checks with no errors |
| L2 | Error recovery | statement- and expression-level recovery, partial facts, cancellation inside bodies and `inferAll` | typing replay: at every prefix of every function body in `testdata/run`, the other declarations and the statements before the caret keep their facts; `FuzzEdits` finds no panic |
| L3 | Navigation | `hover` (type, signature, KDoc), `definition`, `typeDefinition`, `references`, `documentHighlight`, `implementation` (`Inheritors`, `Call.Targets`), `workspace/symbol`, semantic `semanticTokens`, `inlayHint` (inferred types, parameter names from `Call`'s mapping) | golden scripts; references equal a scan of the module's facts; hover and definition p95 < 20 ms |
| L4 | Completion | `completion` (locals, receivers, members, extensions, keywords, named arguments, imports from the class index), `completionItem/resolve`, `signatureHelp` | golden lists at about 150 carets; p95 < 100 ms on octi-server |
| L5 | Editing | `rename`, `codeAction` (organize imports and about 15 fixes keyed by code: add import, `?.` or `!!`, make `var`, implement members, add `when` branches, change type, remove unused, ...), `callHierarchy`, `formatting` | each fix's result compiles with `kc`; the formatter is idempotent on the corpus, with a committed diff budget against IntelliJ's formatter |
| L6 | Daily use | modules checked against each other's sources, library navigation (`-sources.jar` from Gradle's cache, else a read-only `kc-lib:` stub from metadata), declaration edits checked per file, file watching and re-import | declaration edit < 150 ms on octi-server; rename and references across modules; definition into the standard library opens its source |

L0–L4 make a usable server, about 4–6 engineer-months. That is more than an
earlier 3–5 estimate, which assumed `ka` checks bodies. L5–L6 add about
6–10. The formatter is the largest single item of L5.

### Checker and lowering changes

| Change | For | Where |
|---|---|---|
| (line, UTF-16 column) → byte offset | L0 | `psi/snapshot.go` |
| Session with body facts, partial facts, library cache and reuse shared with `ka` | L1 | `analysis`, `internal/kasession` |
| Codes on the most common checker errors | L1 | `internal/check` (`errorf` sites) |
| A broken statement or expression lowers to `syntax.ErrorExpr` (type `Invalid`) and its member is kept; the compiler still fails, since lowering reports the error | L2 | `internal/lower`, `internal/syntax/ast.go`, `internal/check` |
| No checker panic on recovered trees | L2 | `internal/check` |
| Cancellation inside `inferAll` (`check.go:2403`) and every few statements of a body | L2 | `internal/check` |
| **Probe**: lowering puts a `syntax.Probe` at the caret, like IntelliJ's dummy identifier; the checker reports the scope there to an optional `ScopeRecorder` and stops checking that declaration | L4 | `internal/check` (`recorder.go`, `scope.go`, new `probe.go`) |
| `ScopeView`: locals in order, implicit receivers (`state.recvs`, `ctxs`, `cls`) with smart-cast types, the expected type, the receiver of `x.`, `x?.` and `x::`; members with extensions, a package's top-level names (`kpkg.names`), imports | L4 | `internal/check` |
| `Options.OnlyDecl`: check one declaration, inferring others lazily as `Only` does | L4 | `check.go` |
| All candidates of a call, with their mappings | L4 | `internal/check` |
| Index of library classes and top-level names | L3, L4 | `internal/classpath`, `internal/kmeta` |
| A checked module's symbols as a `check.Library` layer for other modules | L6 | `internal/check`, `internal/driver` |
| Written signatures resolved lazily per file, and checker results kept beside the trees ([step 24](analysis-api.md#step-24-what-a-one-file-edit-costs-and-what-incremental-checking-needs)) | L6 | `internal/check` |

## Tests

No test needs a JVM, as with `psi`'s goldens:

- **Golden scripts.** `internal/lsp/testdata/*.txtar` hold a
  `kc-lsp.json`, sources with `/*|name*/` caret markers, and a script of
  edits and requests. Responses are committed as normalized JSON and
  rewritten with `-update`, as in `psi/golden_test.go`. The server runs
  in process over pipes. Fixtures that need the standard library find it
  as other tests do (`KOTLIN_HOME`), and CI's skip guard fails if it's
  missing.
- **An oracle outside CI.** `tools/lspdump` runs kotlin-lsp, at a version
  pinned by SHA-256, on the same fixtures and commits its hover texts,
  definition targets, references and completion sets. CI never runs it,
  as it never runs `tools/psidump`. Accepted differences are committed as
  `.diff` files, as `jvm/kc-aa/test/*/stock.diff` are.
- **Differential.** Session against `AnalyzeArena`; incremental against
  whole checks; partial facts against whole facts on declarations
  without errors.
- **Replay and fuzzing.** Each `testdata/**/*.kt` typed in a token at a
  time: no panic, valid protocol, and results that end equal to a whole
  check. `FuzzEdits` runs 60 s in `pr-checks.yml`, next to `FuzzParse`.

The tests run in `pr-checks.yml`'s existing shards.

## Benchmarks

`internal/lsp/bench_test.go`, on the corpora of `KA_BENCH`
(octi-server, kotlinpoet, Ktor, kotlinx-coroutines-core):

| Measure | Target |
|---|---:|
| Cold open to first diagnostics, 200-file module, Gradle model cached | < 1 s |
| Change to syntax diagnostics | p95 < 10 ms |
| Body edit to semantic diagnostics, octi-server | p50 < 30 ms, p95 < 100 ms |
| Declaration edit to semantic diagnostics, octi-server | < 300 ms; < 150 ms from L6 |
| Completion | p95 < 100 ms |
| Hover, definition | p95 < 20 ms |
| References, rename, kotlinx-coroutines-core | < 300 ms |
| Resident memory, kotlinx-coroutines-core workspace | < 400 MB |

kotlin-lsp is timed by hand on the same projects for reference, with the
method written down as in [Benchmark](benchmark.md).

## Steps

### L0: protocol and syntax tier

`cmd/kc-lsp` serves LSP 3.17 over stdio, with the standard library only:

- **`internal/lsp/jsonrpc`** handles Content-Length framing, requests in
  order, and `$/cancelRequest` as a cancelled `context`. A panic in a
  handler is returned as an internal error.
- **`internal/lsp/protocol`** holds the types L0 uses, written by hand.
- **`internal/ide`** answers the editor's questions with plain Go values.
  It has `Document` (one immutable version, its psi snapshot, and offset ↔
  position mapping in UTF-16 or UTF-8), `SyntaxDiagnostics`, `Symbols`,
  `Folds`, `SelectionRanges` and `LexicalTokens`. The inverse position
  mapping lives in `Document`, not in psi.
- **`internal/lsp`** covers the lifecycle (`initialize`, `shutdown`,
  `exit`), incremental `didChange`, versioned `publishDiagnostics`,
  `documentSymbol` (hierarchical, or flat for older clients),
  `foldingRange`, `selectionRange` and `semanticTokens/full`. It offers
  `positionEncoding: utf-8` when the client accepts it.

`internal/lsp/testdata/*.txtar` holds 10 golden scripts. They run the
server in process over pipes. `go test ./internal/lsp -update` rewrites
them.

Syntax diagnostics after a keystroke (`BenchmarkKeystroke`: apply an edit,
reparse, list diagnostics) take 7 ms on a 47 KB file
(`jvm/kc-aa/.../KcRound.kt`, 4-core Xeon). That is with psi parsers reused
from a pool; without the pool it takes 10.5 ms. The whole file is
reparsed, at psi's 6–7 MB/s. A file over about 60 KB misses the 10 ms
target. The fix is to reparse only the edited top-level declaration, which
is not done yet. The VS Code and Neovim smoke runs are not written down yet.

## Gaps and open questions

- **Diagnostics `kc` lacks.** `kc` reports no warnings and runs few of
  `kotlinc`'s checkers, so the editor shows fewer problems than
  IntelliJ. The precision policy keeps false errors out, not missing
  ones in.
- **Declaration edits** check the whole module until L6, and every update
  lowers every file while the checker writes into the trees.
- **Project dependencies** read stale class output until L6.
- **Whole-file reparse.** Every keystroke reparses the whole file; files
  over about 60 KB miss the 10 ms syntax target.
- **Gradle import** depends on the Kotlin Gradle plugin's API, read by
  reflection; each plugin version it supports needs a fixture.
- **KDoc and library sources** need `-sources.jar`s, which Gradle
  downloads only when asked; the import asks for them.
