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
- **Codes and ranges.** About 55 of the checker's message formats map to
  kotlinc's diagnostic names (`internal/check/codes.go`; a test checks
  every format is still used). `kc -features` prints the code, and psi's
  syntax errors are `SYNTAX`.
- **Precision** (`bench/results/lsp-precision.tsv`). Of the 44 codes kc
  reports in the conformance tests, 32 have no false errors on tests
  kotlinc accepts. 12 are below 99% and are warnings, among them `TYPE_MISMATCH` (0.880, 81 false of 673),
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
  assignments as writes. A library declaration gives no definition
  (L6). `TestReferencesScan` checks, on every program of
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

## Gaps and open questions

- **Diagnostics `kc` lacks.** `kc` reports no warnings and runs few of
  `kotlinc`'s checkers, so the editor shows fewer problems than
  IntelliJ. The precision policy keeps false errors out, not missing
  ones in.
- **Declaration edits** check the whole module until L6. Every update
  lowers every file's signatures and declares them again, while the
  checker writes into the trees. A body edit on a 200-file module is at
  its 30 ms target, with no margin, and grows with the module (L1
  above).
- **Project dependencies** are analyzed from source with the module
  that depends on them, as one session: a module and its dependencies'
  files are checked together, and a dependency's files are checked
  again in each module that depends on it. Their own diagnostics are
  shown from their own module only.
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
- **Library navigation**: a library declaration has no definition and
  no KDoc, and workspace symbols leave library classes out, until L6
  reads library sources.
- **Navigation follows the module.** References and implementations are
  found in the modules whose sources hold the file, not in modules that
  depend on it without analyzing it, and a property's overrides aren't
  recorded, so implementations of a property find nothing.
- **Requests are served one at a time**, so a slow one (references right
  after an update, while its index builds) delays the next.
- **Hover notes non-null smart casts only**; a smart cast to a subtype
  shows the declared type.
- **KDoc and library sources** need `-sources.jar`s, which Gradle
  downloads only when asked; the import doesn't ask for them yet.
