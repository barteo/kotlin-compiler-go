# A Kotlin language server on kc

This is the plan for `kc-lsp`, a language server for Kotlin written in Go
on top of `kc`. It aims at what JetBrains'
[kotlin-lsp](https://github.com/Kotlin/kotlin-lsp) offers for JVM projects:

- diagnostics with quick fixes, completion, hover, definition, references,
  rename, signature help and call hierarchy;
- semantic highlighting, inlay hints, folding, formatting, organize
  imports and workspace symbols.

kotlin-lsp is an alpha, runs IntelliJ in a JVM and supports Gradle and
Maven JVM projects, Android experimentally (no KMP). The milestones
below were the order of work; L0 to L8 are built, and the table after
L7 compares the two servers.

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
The plan works within that; L6 narrows what an update checks, not what
it lowers.

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
- **Partial facts.** A file's facts are exported but for the code its
  errors invalidate: the statements from a broken one to the end of its
  body, a broken expression after `=`, or a broken declaration (L2).
  `Result.Complete()` reports whether there was no error anywhere, which
  is the contract `AnalyzeArena` keeps.
- Facts are kept per file and carried over for files an update doesn't
  check again, as `ka` keeps model files.

## Project model

- **`kc-lsp.json`** is the native format, and the tests'. Each module has a
  name, a kind (`main`, `test`), Kotlin and Java source roots, a class
  path, friend modules or paths, module dependencies, a JDK home and
  compiler options (`-opt-in`, `compose`, `serialization`, `parcelize`,
  `-jvm-default`, explicit API). Bazel and hand-made setups write it
  themselves.
- **Gradle.** `kc-lsp` runs `./gradlew -q --init-script
  kc-lsp.init.gradle.kts kcLspModel`, with the script embedded in the
  binary. It generalizes `bench/kotlinpoet/classpath.init.gradle.kts`:
  for each Kotlin JVM compilation of each project it writes the sources,
  `compileDependencyFiles`, friend paths, module name and compiler
  options as JSON. The model is cached in `.kc-lsp/model.json`, keyed by
  the hashes of the build files (`*.gradle*`, `gradle.properties`,
  `libs.versions.toml`), and imported again when they change. Import is
  the only time a JVM runs.
- **Maven** (from L7). Without a `kc-lsp.json` or a Gradle build, a
  `pom.xml` is read for its modules, source directories, and the
  `kotlin-maven-plugin`'s executions, compiler plugins and arguments,
  with the parent's `pluginManagement` and properties. The class paths
  come from `mvn dependency:build-classpath`, once per scope. The model
  is cached in `.kc-lsp/maven.json`, keyed by the poms' hash.
- **Source sets.** A test source set is its own module, which depends on
  main (Gradle's associated compilations).
- **Several modules.** A project dependency, a class path entry that is
  another module's output, is that module's sources, analyzed with the
  module's in one session, so stale or missing class output doesn't
  matter. From L6 it is checked once, as a library.
- **Out of scope:** `.kts` (build scripts included), KMP, Android (AGP),
  and code intelligence for Java. Java sources in a module are read for
  their declarations, as `kc` reads them (`internal/javasrc`).

## Diagnostics

`kc`'s errors are shown as errors only where they are reliable:

- **Syntax errors** come from `psi`, with ranges and kotlinc's messages.
- **Ranges.** A checker error's range is the psi token at its position,
  or the whole string literal at an opening quote.
- **Codes.** `syntax.Error` has a `Code`, named after kotlinc's
  factories (`UNRESOLVED_REFERENCE`, `TYPE_MISMATCH`, ...), from a table
  of the checker's message formats (`internal/check/codes.go`).
- **Precision.** `bench/conformance -precision` measures each code's
  false errors on the diagnostics tests `kotlinc` accepts. A code is an
  error if its precision is at least 99%, otherwise a warning, and
  messages without a code are warnings. The setting `diagnostics`
  (`initializationOptions`) is `default`, `coded` (only coded messages)
  or `strict` (everything an error). The table is committed as
  `bench/results/lsp-precision.tsv`, and a test keeps
  `internal/ide`'s list of demoted codes equal to it.
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
| L6 | Daily use | modules checked against each other's sources, library navigation (`-sources.jar` from Gradle's cache, else a read-only stub from metadata), declaration edits checked per file, file watching and re-import | declaration edit < 150 ms on octi-server; rename and references across modules; definition into the standard library opens its source |
| L7 | Parity with kotlin-lsp | refactorings and intentions, type hierarchy, moving files, the library in workspace symbols and call hierarchy, completion templates, run lens, KDoc Enter, inspections with fixes, Maven import, downloading sources | each refactoring's and fix's result checks with `kc` throughout `testdata/run`; completion doesn't regress; the parity table below |

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
| **Probe**: a name at the caret, `check.ProbeName`, like IntelliJ's dummy identifier; the checker reports the scope there to an optional `ScopeRecorder` and stops checking that file | L4 | `internal/check` (`recorder.go`, `scope.go`, new `probe.go`) |
| `ScopeView`: locals in order, implicit receivers (`state.recvs`, `ctxs`, `cls`) with smart-cast types, the expected type, the receiver of `x.`, `x?.` and `x::`; members with extensions, a package's top-level names (`kpkg.names`), imports | L4 | `internal/check` |
| `Options.OnlyDecl`: check one declaration, inferring others lazily as `Only` does (not needed: completion is under its target checking the file) | L4 | `check.go` |
| All candidates of a call, with their mappings | L4 | `internal/check` |
| Index of library classes and top-level names | L3, L4 | `internal/classpath`, `internal/kmeta` |
| `syntax.Error.Data`, fix data per code (`analysis.Diagnostic.Data`) | L5 | `internal/syntax/token.go`, `internal/check` |
| Codes for not implemented members, `NO_ELSE_IN_WHEN`, `RETURN_TYPE_MISMATCH_ON_OVERRIDE` | L5 | `internal/check/codes.go`, `class.go`, `expr.go` |
| `MANY_IMPL_MEMBER_NOT_IMPLEMENTED` with the member's stub | L5 | `internal/check/delegation.go` |
| A checked module's symbols as a `check.Library` layer for other modules | L6 | `internal/check`, `internal/driver` |
| Written signatures resolved lazily per file, and checker results kept beside the trees ([step 24](analysis-api.md#step-24-what-a-one-file-edit-costs-and-what-incremental-checking-needs)) (not done: L6 narrows what is checked instead) | L6 | `internal/check` |
| Lookups recorded only for the files checked (`Options.Only`); `analysis`'s recorder keeps them per file | L6 | `internal/check/lookups.go`, `check.go`, `analysis/declared.go` |
| `Library.Stub`: a class, or a package's functions and properties of a name, rendered as Kotlin source | L6 | `internal/check/stub.go` |

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
  check. `FuzzEdits`'s seeds run with the tests; fuzzing needs the
  standard library, which the fuzz job doesn't install, so it runs by
  hand (L2).

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

Syntax diagnostics after a keystroke take 0.7 ms on a 47 KB file
(`BenchmarkKeystroke`: apply an edit, reparse, list diagnostics;
`jvm/kc-aa/.../KcRound.kt`, 4-core Xeon). That is well under the 10 ms
target. When an edit stays inside one declaration body, `psi.Reparse`
parses only that body (see [psi](psi.md)). A whole reparse of the same
file takes 7 ms with pooled parsers and 10.5 ms without. Splicing still
copies the node array, so the cost grows with file size at about
65 MB/s. The VS Code and Neovim smoke runs are not written down yet.

### L1: project model and semantic session

The server learns a workspace's modules and shows `kc`'s errors in them:

- **`internal/kasession`** has `ka`'s refcounted library cache and its
  signature hash, now shared with `analysis`. `ka`'s snapshot reuse
  (`reuse.go`) stays in `ka`: `analysis` reuses facts, not models, and
  needs none of it.
- **`analysis.Session`** keeps a library open across updates. Its facts
  are per file: a file with an error has diagnostics and no facts, and
  the others keep theirs. A body-only edit (equal signature hash) checks
  only the changed files (`check.Options.Only`) and carries the other
  files' facts and diagnostics over. The other files are lowered without
  their block bodies (`lower.Signatures`), but for contracts and inline
  functions' bodies, which the checker reads in files it doesn't check.
  Carried-over facts and symbols keep their numbers and are shared with
  the last Result. Only those that point into a changed file are copied,
  with their elements remapped to the new parse. The changed file's facts
  name the kept symbols by declaration, kind and name. A library symbol
  is matched by its checker pointer, which holds for one update; after
  that it is added again. A Result may therefore list symbols no fact
  names, until they number more than twice the last count plus 256 and
  are compacted. `TestSessionIncrementalRun` edits a body in every file
  of `testdata/run`: all 574 edits are incremental, and each result
  equals a whole update's. `TestSessionIncrementalMany` covers
  compaction, and `TestSessionIncrementalContracts` covers another
  file's contract and inline function.
- **`internal/project`** loads `kc-lsp.json`, else imports Gradle, else
  takes the folder as one module. The import is an embedded Groovy init
  script with task `kcLspModel`. It reports each JVM compilation's
  sources, class path, friend paths, outputs, module name, opt-ins, JVM
  default, explicit API, JDK and compiler plugins (from the plugin class
  path's jar names). The model is cached in `.kc-lsp/model.json` under a
  hash of the build scripts, properties and version catalogs. A class
  path entry that is another compilation's output becomes a module
  dependency, whose sources are analyzed with the module's. A two-project
  build (a library, and an app with tests on JUnit) imports in 43 s cold,
  most of it downloads, and in 1.9 s with a warm daemon. Its four
  modules check with no errors. The imports of kotlinpoet, Ktor and
  octi-server are not run yet: running their builds needs a machine where
  third-party build code may run.
- **Codes and ranges.** About 60 of the checker's message formats map to
  kotlinc's diagnostic names (`internal/check/codes.go`; a test checks
  every format is still used). `kc -features` prints the code, and psi's
  syntax errors are `SYNTAX`.
- **Fix data.** `syntax.Error.Data`, passed through as
  `analysis.Diagnostic.Data`, is what a quick fix needs, with fully
  qualified names: a coded error's message arguments; for the
  not-implemented codes and `MANY_IMPL_MEMBER_NOT_IMPLEMENTED` of a
  function, the member's override stub
  (`fun f(a: kotlin.Int): p.R`, `var name: kotlin.String`); for
  `NO_ELSE_IN_WHEN`, the missing cases (`p.Color.RED`, `is p.Circle`,
  `p.Empty`, `false`, `null`); for `RETURN_TYPE_MISMATCH_ON_OVERRIDE`, the
  overridden member's return type.
- **Precision** (`bench/results/lsp-precision.tsv`). Of the 49 codes kc
  reports in the conformance tests, 35 have no false errors on tests
  kotlinc accepts. 14 are below 99% and are warnings, among them `TYPE_MISMATCH` (0.880, 81 false of 673),
  `UNRESOLVED_REFERENCE` (0.927) and `UNSAFE_CALL` (0.970). Uncoded
  messages are at 0.939. The conformance tests are edge cases, so these
  are lower bounds for ordinary code.
- **`internal/lsp`.**
  - Each workspace folder loads in the background after `initialized`.
    Each module has a session and an update goroutine.
  - An open, edit, close or watched-file change kicks the modules whose
    sources hold the file (`Project.Dependents`). An update runs 50 ms
    after the last kick, on the open documents' snapshots and the other
    files' from disk (cached by size and time). A newer kick cancels it,
    unless the module has never finished an update.
  - A file shows the diagnostics its own module computed for that
    version. An edit moves the last ones by the edit, and drops those it
    touches, until the next update. Closed files show theirs too, and a
    file in no module shows syntax errors only.
  - A change to `kc-lsp.json` or a Gradle file reloads the project. File
    watchers are registered dynamically when the client supports it;
    otherwise `didSave` reloads.
  - Four new golden scripts run real workspaces: `semantic*.txtar`, with
    the `wait`, `notify` and `write` commands.

`internal/lsp/bench_test.go` measures through the server over pipes, on
a generated 200-file module (or `KC_LSP_BENCH`), on a 4-core Xeon:

| Measure | Target | L1 |
|---|---:|---:|
| Cold open to first semantic diagnostics (library, whole check) | < 1 s | 0.39–0.43 s |
| Body edit to semantic diagnostics, without the 50 ms debounce | p50 < 30 ms, p95 < 100 ms | p50 28–30 ms, p95 38–46 ms |

The first measurement of the body edit was p50 110–126 ms. Four changes
brought it to target:

- **Sharing the carried-over facts** instead of copying them, with
  symbol numbers stable across updates. This took about 32 ms.
- **Lowering the unchanged files without their bodies.** This took
  about 19 ms.
- **Per-file facts in a Result**, so an update shares whole files
  rather than rebuilding a module-wide map. The builder's later passes
  also skip the unchanged files. Together these took about 17 ms.
- **Updating on every core** (`Config.Jobs`). This took about 8 ms, and
  cold open went from 0.57 s to about 0.4 s.

Per edit, the rest is:

- signature lowering of the other files, about 5 ms on four cores;
- the checker declaring and resolving every file's signatures, about
  15 ms, sequential;
- the changed file's check and facts;
- garbage collection, about a quarter of the CPU.

The p95 doubles, to about 86 ms, when the process also holds the
libraries of earlier sessions, because GC has a larger heap to mark.
Lowered trees that the checker doesn't write into, so that unchanged
files are neither lowered nor declared again, are the L6 work on
declaration edits.

### L2: error recovery

A half-typed statement now costs the facts of its own code only:

- **Lowering** (`lower.Recover`, and `lower.Signatures`) lowers a
  statement of a closed block that fails as `syntax.ErrorExpr`, and an
  expression after the `=` of a property, function or accessor that
  fails the same way, keeping the declaration. A psi error inside such a
  statement or expression is checked there, not for the whole
  declaration. A block that lost its `}` (an unclosed string or comment
  took it) still fails its declaration, since psi may have taken the
  code after it too. Lowering reports each error as before, so `kc` and
  `ka` fail as they did; `lower.File` doesn't recover. An error found
  past the statement, as an expression missing at the end of the file,
  is put at the statement's last token. The string template's saved
  lowering state is restored by `defer`, now that lowering goes on after
  a panic.
- **The checker** gives `ErrorExpr` the invalid type, with no error, and
  counts it as a jump, so a body cut short reports no missing return.
  `CheckOptions` polls the context per statement and per declaration of
  `inferAll`, and a panic of a cancelled check is dropped.
  `TestCancelInBody` cancels inside a body.
- **`analysis`** maps each error to the code it invalidates
  (`errorSpans`): in a statement of a body, that statement, the ones
  after it, which it may narrow or declare for, and the block; at a
  brace, the block; after `=`, the expression; in a class header, the
  header but its members; elsewhere in a declaration, the declaration;
  in the package, the imports or past the declarations, the file. Facts
  of those nodes are dropped in the builder's last pass; the others are
  kept, though they may name symbols declared in dropped code. A
  lowering error that drops a declaration others may name leaves no
  incremental state, or makes an update a whole one: the files that
  name it would otherwise keep stale results.
- **`TestSessionTyping`** types every function body of `testdata/run`,
  member or top level, in one session per program: the body is cut to
  each prefix, with its `}` kept, as an editor auto-closes it. At each
  prefix the facts outside the body, and those of the statements that
  end before the caret, must equal the original ones (offsets after the
  body shifted). Prefixes where psi's block loses its `}` are skipped.
  By default the test replays every 14th program (every 70th with
  `-short`), cut at each statement and in the middle of each: 23
  programs, 120 bodies, 1,563 cuts, 109 skipped, in 15 s.
  `KC_TYPING=all` replays every program cut after every token: 323
  programs, 1,095 bodies, 76,826 cuts, 18,193 skipped, none failed, in
  9 minutes. An earlier run failed 43 cuts, all in the two kinds of
  body below.
  Two kinds of body are left out, as their code is part of what callers
  see: a statement with a contract changes its callers' smart casts, and
  an inline function's body decides how callers may pass lambdas and
  numbers the anonymous classes inlined into them. A function inside an
  object expression is part of that expression, whose error drops it
  all.
- **`FuzzEdits`** edits one file of a two-file module at random and
  compares the incremental result with a whole update. Its seed
  `run {` found the stale-results bug above. Five minutes, about
  190,000 runs, found nothing more.
- **`semantic_recovery.txtar`** half-types a statement after one with a
  type error: both errors show.

Recovery costs the benchmarks nothing measurable: a body edit is p50
27 ms, p95 51 ms, and cold open 0.45 s.

### L3: navigation

The server keeps each module's last update as an `ide.View`: its result
and the snapshots it checked, with lazy indexes of the module's source
classes and of each declaration's uses, built from per-file facts
(`analysis.Result.FileFacts`). A request reads the views of the modules
whose sources hold its file, the owner's first. A view may be of older
texts than the open documents: an offset maps to it, and a result back,
through the span the edits since changed (`ide.Mapper`, which the
diagnostics' shifting now uses too). A position in that span finds
nothing until the next update.

- **`definition`, `typeDefinition`, `references`, `documentHighlight`.**
  A use is a reference whose symbol or call is declared in the sources,
  a type naming a source class or type parameter, or an operator whose
  call is. A declaration is named across views by its file and range.
  References gather the uses in every view of the file's modules;
  highlights mark assignment targets, `++`/`--` and compound
  assignments as writes. A library declaration's definition is in its
  sources or a stub (L6). `TestReferencesScan` checks, on every program of
  `testdata/run`, that the references of each declaration in the
  sources equal an independent scan of the module's facts, that the
  definition of each use is its declaration, and that hover describes
  both: 351 programs, 12,873 declarations, 20,582 references, none
  failed.
- **`hover`** shows a source declaration's header as written, without
  annotations and comments and up to its body, initializer or
  accessors, adding the type inference gives where none is written;
  then its KDoc as Markdown (`[name]` links as code, tag sections); then
  its class or package. A library function shows its call's signature,
  a library property its accessors' type, and a variable that a smart
  cast makes non-null says so. An operator on primitives, which has no
  call, shows nothing.
- **`implementation`** finds the source classes that have a class or
  interface among their supertypes, and the functions that override a
  function, through `Symbol.Overrides`, which `analysis` now records:
  the source declarations a function overrides, transitively.
- **`workspace/symbol`** searches the outlines of every module's files,
  open ones by their current text: a subsequence of the name, ignoring
  case, ranking exact names, prefixes and camel humps first, at most 256.
- **Semantic tokens** add the names that references and types use to
  the lexical tokens, classified by the declaration as its own name is
  (a `val` is read-only, an enum entry an enum member), or a library
  symbol by its kind. Enum entries now have their declarations: the
  checker's entry fields point at them.
- **`inlayHint`** shows the inferred type of a variable or lambda
  parameter with none written, unless its initializer is a literal or
  constructor call, and the parameter name before a literal positional
  argument, as IntelliJ does by default, once per vararg.

`BenchmarkNavigation`, on the generated 200-file module, times requests
on a callee: hover p50 0.03 ms, p95 0.11 ms; definition p50 0.03 ms,
p95 0.06 ms; references p50 0.04 ms, p95 0.12 ms. The first request
after an update builds the index it needs: hover and definition the
class map, 3.4 and 2.0 ms; references the index of uses, 73 to 90 ms.
Building that index in the background after each update cost body
edits 11 ms at p50 and 35 ms at p95, as it competed with the next
update, so it is built on demand. Body edits are p50 32 ms, p95 46 ms,
and cold open 0.43 s.

### L4: completion

Completion checks the document as it is now, with a probe at the caret,
as IntelliJ does with its dummy identifier. Nothing is mapped from the
last update's view, which fails inside the span just typed.

1. `ide.CompletionAt` uses psi only. It finds the name at the caret (the
   prefix is the part before it) and the context:
   - nothing in comments, strings or after a digit;
   - an `import` or `package` directive, with its qualifier;
   - a named argument's name;
   - whether `(` or `<` follows, so that no parentheses are inserted;
   - the keywords that fit: statement, expression, class-body and
     top-level sets, none after a dot, in a type or where a declaration
     is being named.

   It returns the document with `check.ProbeName` in place of the name,
   and a `)` for each `(` the line leaves open.
2. `analysis.Session.Complete` lowers the probe's file with recovery and
   the others by their signatures. It checks only that file
   (`check.Options.Only`) against the session's library, and never
   touches the session's state, so the next update stays incremental.
   The checker, on reaching the probe, builds a `check.ScopeView` and
   stops:
   - as an identifier: locals innermost first, with smart casts;
     implicit receivers' members; extensions that fit them; top-level
     functions, properties and classifiers from the package, imports,
     star imports and default imports; root packages;
   - after `x.`, `x?.`, `super.` or `x::`: the receiver's members and the
     extensions that fit it, by the resolution rules' own scopes and
     `extFits` for bounded type parameters; a class's statics,
     companion and nested classes; a package's members;
   - in a type or an annotation: classifiers, type parameters and
     packages.

   With the view come the expected type, the calls the probe is an
   argument of, with every candidate and its substitution, and, when the
   prefix has a letter, the class path's and other packages'
   declarations it may import. The library index
   (`check.Library.Importables`) reads the class path's packages and
   Kotlin metadata once, about 40 ms for the standard library and JDK.
   It leaves out facades and synthetic classes, and classes whose class
   file isn't public (read from the header on first use).
3. `ide.Complete` ranks the items:
   1. prefix matches (as typed, then in any case), then camel humps,
      then other subsequences;
   2. the expected type, taken from the checker or else the parameter
      every candidate has for the argument;
   3. then by scope: named arguments in parameter order, locals,
      innermost receiver's members, other receivers', keywords typed in
      part, extensions, top-level names, other keywords, imports;
   4. then non-deprecated first, sources before libraries, shorter names
      and labels.

   A function inserts `name()` or `name(`, or with snippets `name($0)`,
   or `name { $0 }` for a sole lambda parameter. An import adds an edit
   after the last import, or after `package`. The list is capped at 200
   and marked incomplete. `completionItem/resolve` adds the declaration's
   header and KDoc.
4. `signatureHelp` (`ide.SignatureAt`) finds the argument list around the
   caret in psi, counts commas, and replaces the argument's expression
   with the probe. The candidates come fewest parameters first; the
   active one is the first with a parameter for the argument, by
   position or name. Each candidate carries its KDoc.

Goldens: 150 completion carets in `complete_{scope,typing,receivers,
args,types,keywords,import,class,misc}.txtar`, and 14 signature carets.
`BenchmarkCompletion`, on the generated 200-file module, completes in a
function body after `items.`, at a statement with `su` (which lists
importables), and in a type after `val x: Ma`:

| Case | first | p50 | p95 |
|---|---:|---:|---:|
| after a dot | 82 ms | 31 ms | 54 ms |
| statement | 105 ms | 39 ms | 69 ms |
| type | 31 ms | 33 ms | 63 ms |

The first request reads the library index. Each request lowers the
module's other files' signatures again, as an update does; in a CPU
profile that lowering costs about as much as checking the probe's file.

### L5: editing

1. **Rename** (`ide.Rename`, `textDocument/rename` and `prepareRename`)
   edits the declaration's name and its uses in every view that analyzed
   its file. It also edits the names that stand for the declaration
   without a fact:
   - the override family: what the declaration overrides, and what
     overrides that, in the sources;
   - a parameter's name in named arguments;
   - the last name of import directives that import it;
   - KDoc links (`[name]`, `[Owner.name]`) and `@param` tags in the
     files edited.

   A use written with another name (an import alias or an operator)
   keeps it. Library declarations, `it`, `this` and `componentN` are
   refused. A keyword name is backticked where that is legal, and refused
   where it isn't. The server waits for the module's pending check, and
   it fails with "analysis is out of date" rather than send a partial
   edit when an edit falls in text typed since. `TestRenameScan`
   renames every top-level, member and local name of `testdata/run` to
   a fresh one and requires the program to check as before: 3,066
   renamed, 107 refused, and 45 known failures, listed in
   `internal/ide/testdata/rename-budget.txt`. That list may only shrink.
2. **Call hierarchy** (`ide.PrepareCallHierarchy`, `IncomingCalls`,
   `OutgoingCalls`) covers functions, constructors, properties and
   classes. A class stands for its constructors and initializers.
   Incoming calls come from the references of the declaration and its
   override family that are calls (including operator, delegated and
   accessor calls). They are grouped by the enclosing named declaration
   (`psi.NodeRef.EnclosingDeclaration`, which skips lambdas and
   accessors). Outgoing calls are the source declarations called from
   the body outside nested named declarations.
3. **Code actions.** The fixes come from the server's own diagnostics,
   as last published for the document's current text, not the client's
   copy. Those carry the fix data, and a diagnostic whose text was
   edited since has no fix. `ide.QuickFixes` builds each fix from psi
   around the diagnostic plus its `Data`. Types are shortened where the
   file sees them (default imports, its package, its imports), and an
   import is added otherwise.

   | Code | Fixes |
   |---|---|
   | `UNRESOLVED_REFERENCE` | Import `pkg.Name`, one per candidate from the class path and the module's sources; classes only in a type |
   | `UNRESOLVED_IMPORT`, `UNUSED_IMPORT` | Remove the import |
   | `UNUSED_VARIABLE` | Remove the variable; an initializer that calls something stays as a statement |
   | `UNSAFE_CALL` | Replace with `?.` (preferred), or `!!.` |
   | `VAL_REASSIGNMENT` | Change `val` to `var` at the declaration, in another file too; found from the syntax, since a function with an error keeps no facts |
   | `ABSTRACT_MEMBER_NOT_IMPLEMENTED` (and the class and enum entry variants) | Implement members: every missing member's stub in one edit, `TODO("Not yet implemented")` bodies |
   | `MANY_IMPL_MEMBER_NOT_IMPLEMENTED` | Override the member |
   | `NO_ELSE_IN_WHEN` | Add the remaining branches (preferred), or an `else` branch |
   | `TYPE_MISMATCH` | Change the type of the property or variable, or the function's return type, to the actual one; add `!!` when the actual type is the expected one made nullable |
   | `VIRTUAL_MEMBER_HIDDEN` | Add `override` |
   | `NOTHING_TO_OVERRIDE` | Remove `override` |
   | `MUST_BE_INITIALIZED_OR_BE_ABSTRACT` | Add an initializer (`null`, `0`, `false`, `""`, ... or `TODO()`), or `lateinit` |
   | `RETURN_TYPE_MISMATCH_ON_OVERRIDE` | Change the return type to the overridden one |

   `TestQuickFixes` applies each fix to a program and requires the result
   to have no diagnostics and to compile with `driver.Compile` (29
   cases).
4. **Organize imports** (`source.organizeImports`) removes unused and
   duplicate imports, then sorts the rest in IntelliJ's default Kotlin
   layout: other imports, `java.*`, `javax.*`, `kotlin.*`, then alias
   imports, each in lexicographic order. An import is used if a
   reference, type or call in the file resolves through it, or a KDoc
   link names it. A file with diagnostics, or a name an operator
   convention may call, counts as used. Completion and the import fix
   insert at the sorted position. `TestOrganizeImportsScan` adds an
   unused import to every program of `testdata/run` and requires that
   exactly it is removed and the program still checks (332 files
   organized).
5. **Hints.** Unused imports, and locals whose names aren't written
   again in their block, are published as hint diagnostics tagged
   unnecessary (`UNUSED_IMPORT`, `UNUSED_VARIABLE`). `kc` doesn't report
   them.
6. **Formatting** (`ide.Format`, `FormatRange`, `OnType`) changes only
   whitespace, close to IntelliJ's Kotlin official style:
   - **Line breaks are kept.** Runs of blank lines are capped at 2 in
     a file or class body and at 1 elsewhere, with none after `{` or
     before `}` of a block, lambda or `when`. Trailing whitespace goes,
     and the file ends with one newline.
   - **Indentation.** A line is indented one unit from the line where
     its innermost enclosing node starts, if that is an earlier line. A
     braced body indents from its owner's first line (a class, function,
     `if` or call), and closing brackets, `else`, `catch`, `finally` and
     modifiers on their own line align with their owner. The unit is
     `tabSize` spaces, or a tab.
   - **Spaces:** around binary operators, `=` and `->` (none around `..`
     and `..<`); after commas; `a: T`, but ` : ` for supertypes and type
     constraints; after `if`, `when`, `for`, `while` and `catch`; none
     inside `()` and `[]`, before a call's `(`, around `.`, `?.` and
     `::`, or after a prefix operator; one inside a one-line `{ x }` and
     before `{`.

   It returns nothing for a file with syntax errors, and it checks that
   the reparsed result has the same tokens. A range formats the
   whitespace before the tokens that start in it. Typing `}` reindents
   the block from its owner's line, and a newline indents the new line.
   `TestFormatCorpus`: the 604 corpus files without syntax errors keep
   their tokens, and formatting is idempotent. On kotlinpoet's 125 files
   (2-space indent), 648 of 40,137 lines differ from the source. All of
   them are line wrapping and continuation that ktfmt nests and this
   formatter doesn't.
7. **The budget.** `TestFormatBudget` compares the formatted corpus with
   a reference. That is IntelliJ's output, by the sha256 per file in
   `internal/ide/testdata/format.sum` written by `tools/fmtoracle`,
   or, until the oracle is run, the corpus as written. The files that
   differ must be those in `format-budget.txt`, now 20 of 604, and the
   list may only shrink.

Benchmarks, on the generated 200-file module (`BenchmarkNavigation`;
the first request after an update builds the index of uses):

| Request | first | p50 | p95 |
|---|---:|---:|---:|
| references | 65 ms | 0.07 ms | 0.19 ms |
| rename | 62 ms | 1.4 ms | 1.9 ms |
| formatting | 0.2 ms | 0.07 ms | 0.17 ms |

`BenchmarkFormat` formats the largest corpus file (12 KB, 399 lines) in
3.9 ms, parsing included. Completion did not regress (p95 54–61 ms).
The 300 ms target is for kotlinx-coroutines-core, which is not measured
yet.

### L6: daily use

1. **Declaration edits check the files they affect.** Each file's
   checker records the names it looks up in packages and classes
   (`check.LookupRecorder`), and `analysis` keeps them per file with the
   names each file declares and the hash of the top-level declaration
   that declares each. An update that changes, adds or removes a
   declaration touches the names declared differently before and after.
   It checks the edited files, the files that looked up a touched name,
   the files that declare one (overloads and conflicts), and the files
   that look up a supertype's name of a touched class (sealed
   hierarchies). The others keep their facts, as for a body edit. If
   more than half of the module is dirty, the whole module is checked.
   Lookups are recorded only for the files checked, so a body edit pays
   nothing for them. `TestSessionIncrementalDeclarations` and
   `TestSessionIncrementalDeclarationScan` (a parameter with a default
   added to the first function of each file of `testdata/run`) require
   every incremental result to equal a whole check.
2. **A dependency module's files are declared, not checked**, in the
   modules that depend on it (`analysis.UpdateOptions.Declared`). They
   are lowered with signatures only, are never in `Only`, and have no
   facts or diagnostics there: their own module checks them. A change in
   which files are declared checks the module whole. Navigation from a
   dependency's declaration (references, rename, implementations,
   incoming calls) widens to the modules of the declaration's file, and
   a class of a dependency is found by its syntax when no symbol of the
   dependent's names it. `TestSessionDeclared` and `modules.txtar`
   cover the two-module case.
3. **Library sources** (`internal/libsrc`). A jar's sources are a
   sibling `-sources.jar`, or the one in Gradle's
   `files-2.1/<group>/<artifact>/<version>/*/` cache, and the JDK's are
   `$JAVA_HOME/lib/src.zip`. An archive is indexed once, by the package
   and top-level names each `.kt` file's header declares and by each
   `.java` file's class, and the index is cached by the archive's size
   and time. A target (`ide.LibraryTarget`: a class, function,
   constructor or property with the call's parameters and receiver) is
   located by splitting its name into package and classes, longest
   package first, then walking members. Overloads are ranked by receiver,
   parameter count, varargs and parameter types' simple names, and an
   `actual` beats an `expect`. The standard library's sources, laid out
   by source set rather than package, resolve `listOf`, `println`,
   `String.trim`, `Pair`, `MutableList.add` and `String.length`
   (`TestStdlibSources`).
4. **Library navigation in the server.** Definition and type definition
   of a library target, and outgoing calls to library callees, go to the
   declaration in a file extracted from the archive to a cache directory
   (`initializationOptions.libraryCache`, else the user cache dir's
   `kc-lsp/sources`), read-only. Every client opens it as a file, with
   the syntax features of a file outside the modules. That replaces the
   planned `kc-lib:` URIs and `kc/libSource` request, which every client
   would need a content provider for. Hover adds the declaration's KDoc.
   `initializationOptions.librarySources: false` turns the sources off.
5. **Stubs from metadata.** When no sources have the target, or sources
   are off, `check.Library.Stub` renders the top-level class (members,
   constructors, companion and Java statics, enum entries, supertypes)
   or the package's functions and properties of the name, as Kotlin
   without bodies. The stub is extracted to `stubs-<hash>/` in the same
   cache and the target is located in it as in sources
   (`libsrc.LocateIn`). A getter's call written as its property,
   `p.first`, goes to the property. `library_stubs.txtar` covers it.
6. **Watching and re-import.** With the client's file watchers, created
   and deleted files re-list the modules' sources as before. Without
   them, opening, saving or closing a Kotlin file that a module's roots
   hold but its list doesn't re-lists the module. A scan every 5 s lists
   the sources again, kicks the modules whose list changed, and drops the
   disk snapshots of files no module lists. `workspace/executeCommand
   kc.reimport` reloads the projects with Gradle's model refreshed, and
   a project that fails to load keeps the modules loaded before
   (`unwatched.txtar`).

Benchmarks, on the generated 200-file module (`-benchtime 30x`):

| Edit to semantic diagnostics | before L6 p50 | p50 | p95 |
|---|---:|---:|---:|
| body edit | 35 ms | 28 ms | 47 ms |
| body edit in a dependency module (`BenchmarkModuleBodyEdit`) | 33 ms | 27 ms | 40 ms |
| declaration edit (`BenchmarkDeclarationEdit`) | 340 ms | 31 ms | 46 ms |

The declaration edit is under the 150 ms target on the generated
module. octi-server isn't measured. Completion did not regress (p95
56–57 ms), nor did navigation.

### L7: parity with kotlin-lsp

The reference is JetBrains' kotlin-lsp, which runs IntelliJ's Kotlin
plugin headless. fwcd's server is deprecated, and kc-lsp already did
everything it does. L7 adds what kotlin-lsp has and kc-lsp lacked, one
commit per step:

1. **Refactorings and intentions** as `refactor.*` code actions:
   - extract variable and inline variable;
   - specify and remove an explicit type;
   - add names to call arguments;
   - convert a function between an expression body and a block body.

   Each is conservative: one that might change what the code means, as
   moving a smart cast or a conditional evaluation, isn't offered.
   `TestRefactorScan` applies them throughout `testdata/run`, with one
   known failure budgeted.
2. **Type hierarchy.** Supertypes resolve through the module's facts.
   Subtypes come from every module's view. A library class, in sources
   or a stub, resolves its supertypes by its file's syntax.
3. **Moving files and packages.** `workspace/willRenameFiles` returns
   the edits that keep the sources compiling:
   - the moved file's `package` directive;
   - other files' imports and fully qualified uses of its declarations;
   - imports for the files that used them unqualified;
   - in the moved file, imports of what it used from its old package.

   `workspace/didCreateFiles` gives a new empty file its directory's
   `package`.
4. **The library in more places:**
   - `workspace/symbol` appends the library's matches, located lazily
     through `workspaceSymbol/resolve`;
   - stubs render the builtin classes' metadata, so `s.length` works
     without sources;
   - call hierarchy prepares on a library declaration, and its incoming
     calls are the sources' uses of the same overload.
5. **Completion templates**, for clients that take snippets, after the
   semantic items:
   - postfix templates after `expr.`, chosen by type and position: `if`,
     `else`, `not`, `for`, `nn`, `null`, `val`, `var`, `return`, `when`,
     `let`, `also`, `print`, `par`;
   - live templates where a statement or declaration starts: `sout`,
     `iter`, `fori`, `ifn`, `inn`, `todo`, `main`, `fun0`–`fun2`,
     `exfun`, `singleton`, `enum`, `data` and others.
6. **Run lens and commands:**
   - **Run** and **Debug** lenses over each `main`, top level or
     `@JvmStatic`. `kc.run` and `kc.debug` answer the `kc` command that
     builds the module into `build/kc-run`, and the `java` command that
     runs the main class (with a JDWP agent on port 5005 for Debug).
     `kc/mainClass` gives the same for a document.
   - `kc.clearCaches` deletes the extracted sources and stubs and
     reloads.
   - `kc.restart` reopens the sessions.
   - Enter in a KDoc continues its `*`. After `/**` alone, it closes the
     comment with `@param` and `@return` tags.
   - Hover on a type parameter shows its bounds, the `where` clause's
     included.
7. **Inspections**: warnings with a quick fix each, for code that
   compiles but is redundant:
   - `UNNECESSARY_SAFE_CALL`;
   - `UNNECESSARY_NOT_NULL_ASSERTION`;
   - `USELESS_CAST`;
   - `CAN_BE_VAL`;
   - `UNUSED_PARAMETER`, for private and local functions. Its fix also
     removes the call sites' arguments.
   - `UNUSED_PRIVATE_MEMBER`.

   They are judged only on a file without diagnostics, whose facts are
   complete. They skip what they can't tell: platform and generic types,
   smart-cast receivers, casts that may infer a call's type arguments,
   overloads, and names a string literal holds (reflection).
   `TestInspectionsScan` applies every fix in `testdata/run`, and each
   program must still check. It finds 11 warnings and applies 9 fixes,
   none of them budgeted; the two unused parameters' functions are
   overloaded, so they get no fix. Two are on
   `nullSafetyCodegen.kt`'s deliberately unsound reads of uninitialized
   non-null properties, where `kotlinc` warns too.
8. **Maven import**, as in [Project model](#project-model).
   `initializationOptions.downloadSources` has the import fetch the
   dependencies' source jars into the build tool's cache, where
   navigation already looks. Gradle's init script runs an artifact
   resolution query, and Maven runs `dependency:sources`.

The table compares kc-lsp with what kotlin-lsp documents in its README
and `RELEASES.md`, up to v263.6379.0. It doesn't cover what kotlin-lsp
does without documenting it. IntelliJ's own breadth, hundreds of
inspections and quick fixes, is a longer-term reference, not kotlin-lsp's
stated scope; kotlin-lsp says it carries "most of the Kotlin intentions
from the IntelliJ Kotlin plugin".

| Feature | kotlin-lsp | kc-lsp |
|---|---|---|
| Diagnostics | the compiler's errors and warnings, and IntelliJ's default inspections | partial: errors only where the code's precision is at least 99%, few compiler warnings (`USELESS_ELVIS`), and 25 inspection codes, silent on a file with any diagnostic |
| Quick fixes | IntelliJ's, formatted when applied | partial: about 50, one for each inspection included; no fix-all, no suppress action |
| Intentions | "most of the Kotlin intentions from the IntelliJ Kotlin plugin" | partial: specify and remove type, argument names, body style, and L8's 13 rewrites |
| Refactorings | rename, extract variable, inline local variable, move files and packages | done: rename (no conflict check), extract and inline variable, move files, packages and folders |
| Completion, signature help | done | done; ranking not measured against IntelliJ's |
| Postfix and live templates | live templates | done: IntelliJ's common postfix and live templates, not user-defined ones |
| Hover, KDoc | done | done; library KDoc needs sources |
| Definition, type definition, implementation | done | done; implementations of a property find nothing |
| References, rename | done | done |
| Document highlight, selection range, type hierarchy | not documented | done |
| Call hierarchy | done | done, library declarations included |
| Workspace symbols | done, library included | done, library included |
| Document symbols, folding, semantic tokens | done, with `semanticTokens/range` for large files | done; whole-file semantic tokens only |
| Inlay hints | done, with `jetbrains.kotlin.hints.*` settings | done; no settings |
| Formatting, range, on type | done, `.editorconfig` read; closing brackets and quotes inserted as you type | done; never wraps or joins lines; no `.editorconfig`, no bracket insertion |
| KDoc Enter | done | done |
| Organize imports | done | done |
| File templates | IntelliJ-style, with variables | partial: a new file's `package` |
| Run lens | run and debug `main` | done; builds with `kc`, not with Gradle or Maven |
| Debugger (DAP) | experimental: attach, launch from `launch.json`, breakpoints, stepping, threads, frames, variables, evaluation, thread dumps | gap: Debug gives a JDWP command line, not a debug adapter |
| Gradle import, JVM | done | done |
| Maven import | done, `settings.xml` credentials included | done |
| Android import | experimental | gap |
| KMP import | not yet ("coming in the future releases") | gap |
| Several projects in one workspace | done (`intellij.projects`) | done: workspace folders and `kc-lsp.json` modules |
| Build-file changes | reload: always, ask first or never | always re-imported |
| Library sources | downloaded on sync by default | done; downloading needs `downloadSources` |
| Decompiled library classes | not documented | partial: stubs from metadata |
| Java documents | served in mixed projects, with references between Kotlin and Java | gap: Java sources are read for their declarations only |
| Settings | changed at run time; a JDK chosen for symbol resolution | at initialization only (`initializationOptions`); the JDK is found, not chosen |
| Reimport, clear caches, restart | done | done |

Benchmarks, on the generated 200-file module (`-benchtime 30x`):

| Request | p50 | p95 |
|---|---:|---:|
| completion after `.` | 26 ms | 46 ms |
| completion at a statement, with templates | 36 ms | 60 ms |
| `workspace/symbol HashMap`, mostly library (`BenchmarkWorkspaceSymbol`) | 8 ms | 13 ms |
| `workspace/symbol Item`, sources | 10 ms | 17 ms |

Completion stays under its 100 ms target with templates.

### L8: more inspections, fixes and intentions

L8 widens L7's three partial rows of the parity table with IntelliJ's
commonest Kotlin inspections, quick fixes and intentions. There are no
protocol changes: all of them ride on published diagnostics and
`codeAction`.

1. **Style inspections**, by syntax alone (`inspect_style.go`). Each is
   a `Hint` marked unnecessary, so it is grayed out, with a fix that
   deletes or rewrites it:
   - `REDUNDANT_SEMICOLON`;
   - `REDUNDANT_UNIT_RETURN_TYPE`, not on an override;
   - `REDUNDANT_VISIBILITY_MODIFIER`: `public`, unless the module's
     explicit API mode is set, and `private` in a private class;
   - `REDUNDANT_MODALITY_MODIFIER`: `final` in a final class, and `open`
     or `abstract` in an interface;
   - `REMOVE_EMPTY_CLASS_BODY`, `REMOVE_EMPTY_PRIMARY_CONSTRUCTOR`,
     `REDUNDANT_CONSTRUCTOR_KEYWORD`;
   - `REMOVE_EMPTY_PARENTHESES_FROM_LAMBDA_CALL`;
   - `REMOVE_CURLY_BRACES_FROM_TEMPLATE`.

   Each skips what deleting would break or change: a `;` before a
   lambda on the next line, or ending an empty loop body; an object
   literal's body; an enum's entry list.
2. **Idiom inspections**, by the facts' types (`inspect_idioms.go`):
   - `USELESS_ELVIS`, a warning as in `kotlinc`;
   - `REDUNDANT_STRING_TEMPLATE`, a hint;
   - `REPLACE_GET_OR_SET`, `REPLACE_SIZE_ZERO_CHECK_WITH_IS_EMPTY`,
     `REPLACE_SIZE_CHECK_WITH_IS_NOT_EMPTY`, `REPLACE_NEGATED_IS_EMPTY`,
     `REDUNDANT_IF`, as information;
   - `REMOVE_REDUNDANT_QUALIFIER_NAME`, a hint, for a qualified type or
     call that the file's imports and default imports already name.

   A Java `get` or `set` counts as an operator only on a collection or
   map. Negating a comparison flips it, except on floating-point types,
   where `NaN` makes `!(a < b)` differ from `a >= b`.
3. **Quick fixes** for codes that had none:
   - the unresolved type message gets imports, and **create class** or
     **create interface**;
   - `UNRESOLVED_REFERENCE` gets **create function**, top level or a
     member of the enclosing class or of the receiver's class (which
     can be in another file), **create class** for a constructor call,
     and **create local variable** or **create property**. Parameter
     types come from the arguments; the return type comes from the
     expected type, or is `Unit` at a statement;
   - `NO_RETURN_IN_FUNCTION_WITH_BLOCK_BODY`: add `return`;
   - `UNINITIALIZED_VARIABLE`: initialize with the type's default value;
   - `FINAL_SUPERTYPE` and `OVERRIDING_FINAL_MEMBER`: make the source
     declaration `open`;
   - `REIFIED_TYPE_PARAMETER_NO_INLINE`: add `inline`;
     `TYPE_PARAMETER_AS_REIFIED`: make the callee's type parameter
     `reified`, adding `inline` if needed;
   - `JVM_STATIC_NOT_IN_OBJECT_OR_COMPANION`, `WRONG_ANNOTATION_TARGET`,
     `REPEATED_ANNOTATION`: remove the annotation.
4. **Rewrite intentions**, `refactor.rewrite` actions
   (`refactor_rewrite.go`):
   - replace `if` with `when`, and a `when` without a subject with
     `if`;
   - add braces to an `if`, `else`, `for`, `while` or `do` body, and
     remove them from a body of one statement;
   - invert an `if` with an `else`;
   - convert a String concatenation to a template, and back;
   - replace `it` with an explicit parameter, and back;
   - join a local's declaration and the assignment after it;
   - put arguments or parameters on separate lines, or on one line.

   They aren't offered where the result would mean something else:
   - a dangling `else`;
   - a declaration or lambda left as a branch;
   - an `if` expression without `else`;
   - a lambda branch, which loses its arrow and turns into a block;
   - concatenation in a `const` initializer;
   - a nested lambda's own `it`;
   - lists holding comments.

   `codeAction` carries no formatting options, so the rewrites, and the
   fixes that insert members or bodies, indent by the file's own unit:
   a tab, or its smallest indent of spaces.
5. **Settings.** `initializationOptions.inspections` maps a code to
   `off`, `hint`, `information` or `warning`, for example
   `{"REPLACE_GET_OR_SET": "off"}`. An unknown code or level is logged
   when the client is initialized.

Tests:
- `TestQuickFixes` and `TestRewrites` check each fix's and intention's
  result: it must check without diagnostics and compile with `kc`.
- `TestInspectionsScan` applies every inspection's fix throughout
  `testdata/run`, and each program must still check. It finds
  75 warnings and applies 73 fixes, none failing; 42 of the warnings are
  `REPLACE_GET_OR_SET`. Each fix must also leave fewer warnings of its
  code in the file. The one exception is a safe call, where fixing
  `s?.` in `s?.length?.toString()` rightly makes the next `?.`
  unnecessary.
- `TestRefactorScan` now also samples the rewrites' constructs. Its
  roughly 13,700 applications have one known failure, budgeted since L7.
  It found three older bugs, now fixed:
  - argument names on a call the checker rewrote, as a safe call;
  - argument names on a callee whose parameter names repeat;
  - a block body for an annotated expression body, where
    `return @A x` reads as a label.

Benchmarks: `Hints`, the unused-code checks and all the inspections,
takes 0.11 ms p50 and 1.6 ms p95 per file of `testdata/run`, and 4.7 ms
on the slowest. That is within the 5 ms budget for each publish.
`BenchmarkBodyEdit` and `BenchmarkCompletion` before and after L8 stay
within their run-to-run noise on the same machine.

## Gaps and open questions

- **Diagnostics `kc` lacks.** `kc` reports no warnings beyond the
  inspections of L7 and L8, and runs few of `kotlinc`'s checkers, so the editor shows
  fewer problems than IntelliJ. The inspections are silent on a file
  with any diagnostic. The precision policy keeps false errors out, not missing
  ones in.
- **Every update declares every file again.** L6 narrows what is
  checked, not what is lowered and declared, since the checker writes
  into the trees. Lookups are a superset of what a file depends on, so
  a declaration edit may check more files than it needs, and a name
  many files look up (a common class's member) can make the module
  check whole.
- **Project dependencies** are declared in each dependent module under
  the dependent's module name, so a dependency's `internal`
  declarations stay visible there.
- **Ranges** are one token, or a string literal, at the error's position.
  kotlinc's ranges are often a whole call or expression.
- **Precision** is measured on the conformance tests, not on real
  projects. A code that never fires on a valid test counts as precise,
  even when it fires rarely.
- **Gradle import** of the projects named in L1's acceptance criterion
  (kotlinpoet, Ktor, octi-server) is not yet run.
- **Recovery is per statement.** A broken statement drops the facts of
  every statement after it in its body, though most don't depend on it,
  and a broken signature drops its declaration. A block whose `}` an
  unclosed string or comment took drops the whole declaration, or the
  whole file when the error leaves no closing brace.
- **Whole-file reparse** still happens for edits outside a declaration
  body: signatures, imports, and bodies inside string templates.
- **Gradle import** depends on the Kotlin Gradle plugin's API, read
  dynamically from Groovy; each plugin version it supports needs a
  fixture.
- **Library navigation** stops at the library's file: inside it there
  are only syntax features, with no semantic navigation. The JDK's
  sources need `src.zip`, which some JDK installs leave out. Stubs leave
  out default values, annotations and KDoc, and use simple type names.
  A library declaration's outgoing calls are empty.
- **Navigation follows the module.** References and implementations are
  found in the modules whose sources hold the file and, from L6, those
  of the declaration's file. A property's overrides aren't recorded, so
  implementations of a property find nothing.
- **Requests are served one at a time**, so a slow one (references right
  after an update, while its index builds) delays the next.
- **Hover notes non-null smart casts only**; a smart cast to a subtype
  shows the declared type.
- **KDoc and library sources** need `-sources.jar`s. They are
  downloaded only with `initializationOptions.downloadSources`.
  Otherwise only those already in Gradle's cache or beside a jar are
  found. The Gradle query is read dynamically, as the rest of the init
  script is, and isn't run against a real build in the tests.
- **Maven import** runs no lifecycle phase. Maven 3 may then fail to
  resolve a reactor module that was never installed or compiled, and
  the import fails with Maven's message. Profiles, `build-helper` source
  roots and the Kotlin plugin's `jvmTarget` aren't read. Compiler
  plugins other than serialization (`spring`, `all-open`, `no-arg`) are
  warned about and dropped, as for Gradle.
- **Out of L7:**
  - a debug adapter, which kotlin-lsp has, experimental;
  - Android import, experimental in kotlin-lsp, and KMP import, which
    kotlin-lsp lacks too;
  - `.java` documents;
  - IntelliJ's breadth of intentions and inspections, much of which
    kotlin-lsp carries;
  - user-defined live templates.

  The run lens builds with `kc`, so a build that needs Gradle tasks
  (resources, code generation) runs differently there.
- **Completion** is measured on the generated module, not octi-server.
  Each request lowers every file's signatures again (L6 keeps them).
  Rendered signatures leave out type parameters' bounds and default
  values. A member of a nullable receiver after `.` is offered without
  rewriting it to `?.`. Keyword sets are approximations of where the
  grammar allows each one. Library declarations have no KDoc in
  completion, and Java parameters have no names. Importables come from the class path
  and the module's sources, not from modules that don't depend on each
  other.
- **Rename** doesn't check conflicts with other declarations (IntelliJ
  warns), doesn't rename a property's Java-style accessors, and keeps
  aliased and operator uses.
- **Quick fixes.** `MANY_IMPL_MEMBER_NOT_IMPLEMENTED` is reported only for delegation.
  Stubs call `TODO()`, not `super`. A file with syntax errors has no
  fixes beyond its diagnostics' psi, and a function with an error has
  no facts for fixes that need them.
- **Formatter.** It never wraps or joins lines, and doesn't nest
  continuations as ktfmt does. It doesn't keep the convention of a
  constructor parameter list at column 0. The IntelliJ oracle
  (`tools/fmtoracle`) is not run yet, so the budget's reference is the
  corpus as written, not IntelliJ's output.
