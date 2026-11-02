# PSI and analysis integration plan

## Context

`barteo/kotlinc@native-detekt-no-results` is one squashed commit, c462563: "Add native detekt lint,
benchmarks and JVM PSI bridge from codex/native-psi-detekt". It holds a native Go port of detekt:
- `lint/detekt`: 214 core rules
- `lint/detekt-formatting`: 77 ktlint-based rules
- `lint/detekt-compose`: 32 Compose rules

**None of it builds.** Every lint package imports packages that were left out of the commit:

| Missing import | Used by | Goes to |
|---|---|---|
| `kgo/psi`: lossless Kotlin PSI | ~110 lint files | **kotlin-compiler-go** `psi/` |
| `kgo/analysis`: semantic facts from kc's checker | ~25 files (`lint/detekt` only) | **kotlin-compiler-go** `analysis/` |
| `kgo/internal/{detektref,detektmeta}`: reference cache, pinned detekt 1.23.8 metadata | lint tests, tools | kotlin-linters-go |
| `kgo/internal/{sourceorigin,sourcepath,signaturetext}`: small lint utilities | lint | kotlin-linters-go |
| `kgo/internal/{psicore,psiserver,psibinary}`: JNI and stdio session server | `cmd/kgo-psi*` | **dropped** |

The branch's `internal/` is an older copy of kc with no PSI hooks, so nothing can be salvaged from it.
The PSI and analysis layers have to be written fresh on **current** kotlin-compiler-go, against the
API the lint code already uses. The lint code then moves to a new repo, `kotlin-linters-go`, which
imports them.

**Decisions made**
- The public packages are `github.com/barteo/kotlin-compiler-go/psi` and `.../analysis`.
- The JVM PSI bridge is dropped:
  - `jvm/src/main` (except `ParserGuard`), `cmd/kgo-psi`, `cmd/kgo-psi-native`
  - `psicore`, `psiserver`, `psibinary`
  - `bench/{psi.sh, psi-runtime.*, psi-profile.sh, build-psi-native.sh}`
  - `docs/psi-native-abi.md`
  - the JNI-backed test classes: `CompatibilityTest`, `NativeBridgeTest`, `RuntimeBenchmark`, `NativeEngineBenchmark`
- The **pure-JVM detekt oracles** stay. They are `NativeRuleReference`, `NativeTypedRuleReference`,
  `NativeFormattingReference`, `ProductionRuleRunner` and `FindingFingerprint`, and they depend only
  on `ParserGuard`. They generate the reference data the lint tests compare against, so they move to
  the linters repo as `oracle/`.
- `analysis` builds kc's AST **directly from the PSI tree** (direct lowering) and records provenance
  as it goes. It does not re-parse the source or match nodes back by position.
- `kotlin-linters-go` starts as a fresh import, with the first commit citing c462563.

---

## Part A: kotlin-compiler-go (branch `work/busy-heisenberg-xkecpi`)

### A1. Shared scanner in `internal/syntax`

> **Status: replaced.** psi has its own lexer (`psi/lexer.go`, a port of `Kotlin.flex`). Matching
> stock Kotlin's token stream exactly (soft keywords, KDoc, template entries, `BAD_CHARACTER`,
> shebang) needs the full Kotlin lexer, which `internal/syntax` doesn't have, and changing kc's
> compile path to share it would risk the compiler for no gain to the lint API. kc's `Tokenize` is
> unchanged. The steps below were not done.

kc's lexer (`internal/syntax/lexer.go:25`, `Tokenize`) throws trivia away, keeps only line/column
positions, and counts columns as bytes in some places and runes in others.
- Add a streaming `Scan(src string, emit func(Tok))` that reports **byte offsets**, whitespace,
  `//` and `/* */` comments, KDoc and shebang, and emits no tokens of its own for those.
- Re-implement `Tokenize` on top of it, so its output stays unchanged. The existing
  `parser_test.go` and the full `go test ./...` suite prove that.
- `psi` reuses the same scanner, as `docs/psi.md` describes: "uses the compiler's token scanner with
  a separate source grammar".

### A2. `psi/` package (new, public)
**Kinds**
- Bring over `schema/psi-kinds.txt` unchanged. It is append-only, 261 names, SHA-256 `79453880…`.
- Bring over `cmd/psi-kinds`, trimmed to produce only `psi/kinds_gen.go`. That file holds
  `type Kind uint16`, `KindUnknown`, `Kind<NAME>`, `const KindCount`, `KindForName` and `String()`.
- Add a `go generate` directive and a CI check with `-check`.

**API.** This is exactly the surface the lint code uses; the call sites are listed in the exploration
notes.
- **Value types**
  - `Range{Start, End int}`: half-open UTF-8 byte offsets, JSON `start`/`end`.
  - `Diagnostic{Range, Code, Message}`.
  - `NodeID uint32`, where 0 means "none".
  - `SnapshotID uint64`: unique across the process, from an atomic counter.
  - `ParseStages{Lex, Events, Publish, Positions time.Duration}`.
- **`NodeRef`** is a comparable `{s *Snapshot; id NodeID}`. Its zero value is invalid, and **every
  method must be safe to call on it**. Compose asserts its size with `Sizeof`.
  - Core methods: `Valid`, `ID`, `NumericKind`, `Kind() string`, `Text`, `Range`, `SubtreeEnd`,
    `Parent`, `FirstChild`, `NextSibling`, `PreviousSibling`, `ChildKind(Kind)`, `Child(name)`,
    `IsLeaf`, `IsTrivia`, `ContainingFile`.
  - Typed accessors: `Declaration`, `Function`, `Import`, `Element`, `Name` (backticks stripped),
    `NameIdentifier`, `TypeReference`, `ReturnType`, `ReceiverType`, `Body`, `Parameters`, `Path`,
    `Alias`, `IsAllUnder`. The re-implementations in `lint/detekt-compose/mutable.go:52-166` are the
    behavioural reference.
- **`WalkNodes(root, fn func(NodeRef) bool)`** walks in preorder, includes the root and trivia, and
  prunes only the subtree under a node where `fn` returns false. An invalid root does nothing.
- **`*Snapshot`** methods: `RootNode` (the FILE node, ID 1), `Node(id)`, `NodeCount`, `Source`,
  `Path`, `ID`, `Diagnostics`, `NodeAt(byteOff)` (the leaf containing the offset), `UTF16Offset`,
  `Position`, `OffsetPosition`, `DeclarationNodes`.
  - Line and column numbers are 1-based. In `OffsetPosition`, columns are counted in UTF-16 units.
  - Offsets that fall inside a UTF-8 sequence or a surrogate pair are rejected.
  - ASCII sources need no conversion table. Non-ASCII sources use sparse checkpoints.
- **Parsing**
  - `Parse(path, text) *Snapshot` never fails and is safe to call concurrently.
  - `Parser` has a usable zero value and is reused per worker. It provides `Parse`,
    `ParseMeasured`, `RetainedBytes`.
  - A snapshot never holds on to the parser's buffers.
  - `psi` does **not** normalize CRLF; the lint host does that.
- **Not built**, because only the dropped bridge used them: `Edit`, `Apply`, `ParseFragment`,
  `FragmentKind`.

**Arena invariants** (lint code relies on these in `engine.go:516-544`, `facts.go:60`,
`detekt-formatting/document.go:56`, `detekt-compose/lint_index.go:80`)
- One contiguous preorder array. IDs run 1..N, and a parent's ID is always lower than its child's.
- A node's subtree is exactly `[ID, SubtreeEnd)`.
- Each node stores parent, first-child, next-sibling and subtree-end indexes.

**Lossless tree with stock Kotlin shape**
- The leaf texts, concatenated, equal `Source()`.
- A run of whitespace is one `WHITE_SPACE` leaf.
- Soft keywords become `*_KEYWORD` leaves.
- A file always has a `PACKAGE_DIRECTIVE` and an `IMPORT_LIST`, possibly empty.
- KDoc and comments attach to declarations the way Kotlin's `KotlinWhitespaceAndCommentsBinders`
  does. Keep that THIRD_PARTY_NOTICES entry in this repo.
- Recursion and tree depth are bounded. Past the limit the tree stays lossless and gets a
  diagnostic.
- One exact diagnostic is required: `syntax_error` with the message "star projection is not a
  type" (`detekt-compose/provider.go:226`).

**Files:** `psi/{kinds_gen.go, node.go, snapshot.go, arena.go, offsets.go, parser.go, grammar_*.go,
accessors.go, walk.go}` and `psi/testdata/`.

**Tree-shape goldens.** The JNI gate that compared tree shapes is gone, so add a standalone dump tool
in its place.
- `tools/psidump/` is a small Java program (single-file source launch) that parses with stock
  `kotlin-compiler-embeddable` 2.0.21 and writes `{kind, start, end}` in preorder.
- Run it offline on `testdata/**/*.kt` plus targeted grammar fixtures. The fixture dumps are
  committed as `psi/testdata/trees/*.tree`; the corpus dumps are committed only as SHA-256 sums,
  `psi/testdata/corpus.sum`, to keep the repository small.
- `psi` tests compare against those goldens, so CI needs no JVM.

### A3. Direct lowering from PSI to the kc AST (`internal/lower`, new)

> **Status: done.** `internal/lower` lowers psi trees; `internal/lower/lower_test.go` checks it
> against `syntax.ParseAll` on every `testdata/**/*.kt` with `syntax.Diff`, which compares positions
> too on ASCII files. Files that stock Kotlin 2.0.21's grammar rejects but kc accepts are skipped
> there and fail under `KC_FRONTEND=psi`:
> - context parameters, `context(x: T)`, which 2.0.21 doesn't have
> - `@A (x)` in an expression, where stock Kotlin reads `(x)` as the annotation's arguments and kc
>   only takes arguments directly after the name
>
> `driver.Options.Frontend` (or `$KC_FRONTEND`) picks the parser, and CI runs the whole suite with
> each.

kc's parser already desugars some constructs while parsing:
- callable references become `Lambda{Ref}`
- delegated properties get a `$delegate` field and generated accessors
- modifiers become booleans
- string-template parts are parsed from source text
- annotations go into `File.Annotated`

The lowerer has to produce the **same** AST.
- Move those desugarings out of `internal/syntax/parser.go` into shared constructor functions in
  `internal/syntax/build.go`, and call them from both the token parser and the lowerer.
- `lower.File(snap *psi.Snapshot) (*syntax.File, *Provenance, []*syntax.Error)` returns the AST, a
  provenance table, and errors.
  - The provenance table is `map[any]psi.NodeID`, from AST node pointer to PSI node. Nodes the
    checker synthesizes later have no entry.
  - `syntax.Pos` is computed from PSI byte offsets, so lowered positions use byte columns, which is
    what `lint/detekt/engine.go:736` expects.
  - Constructs kc doesn't support produce `Error.Feature` diagnostics, which surface as
    `unsupported:<feature>`.
- **Differential gate**
  - Add `syntax.Equal(a, b)`, a structural AST comparison that ignores positions.
  - A test asserts that `syntax.Parse` and `psi.Parse` followed by `lower.File` produce equal ASTs
    for every `testdata/**/*.kt`.
  - Add a `driver.Options.Frontend` field (`"syntax"` by default, or `"psi"`) and a CI job with
    `KC_FRONTEND=psi` that runs the whole `go test ./...` suite (run, lib, errors, compose,
    serialization, klib) through the lowering path.

### A4. Checker hooks and cancellation (`internal/check`, `internal/driver`)
- Add an optional `check.Recorder` interface on `Checker`. It is nil on the compile path, so
  compilation pays nothing. Its callbacks report facts the AST doesn't keep:
  - `When` exhaustiveness: category and missing cases
  - unreachable code
  - smart-cast or flow non-null
  - definitely-non-null
  - impossible casts
  - `UsedAsExpression`
  - return and receiver targets
  - delegated calls
  - deprecation
- The call sites are `check/expr.go:64` (`SetType`) and the call, `when`, flow and cast code in
  `call.go`, `stmt.go` and `flow.go`.
- Add `check.CheckContext(ctx, files, lib, jobs, rec)`. It checks `ctx` between passes and per file
  in `checkBodies` (`check.go:2181`). `CheckWith` stays as a wrapper around it.
- Split library setup out of `driver.Compile` (`driver.go:120-153`: `FindJDK`, `classpath.Open`,
  `NewLibrary`, `Prefetch`) into a reusable `driver.OpenLibrary(opts)`.

### A5. `analysis/` package (new, public)
**API**
- `Config{JDKHome, ModuleName, ClassPath, NoJDK, Jobs, ExplicitAPIMode}`.
- `ErrUnsupported`, a sentinel error. `.kts` scripts return it.
- `AnalyzeArena(ctx, Config, []*psi.Snapshot) (*Result, error)`, with `Analyze` as an alias.
- `Result` methods: `Fact(ElementID) (Fact, bool)`, `Symbol(uint32) (Symbol, bool)`,
  `Diagnostics()`, `Data()`. **All of them return deep copies.**
- `Diagnostic{Path, Line, Column, Code, Message}`. Columns are 1-based bytes, and `Code` is
  `source_error` or `unsupported:<feature>`.
- The value types `ElementID`, `Data`, `Fact`, `Type`, `Symbol`, `Call` and `When` use exactly the
  field sets in the exploration notes. They come from `lint/detekt/function_pattern.go:115`,
  `rules_typed_contracts.go:153-260`, `rules_semantic_batch.go:185-410` and
  `rules_final_helpers.go:15`.

**Pipeline**
1. Open the library.
2. Lower every snapshot in parallel.
3. Run `check.CheckContext` with a recorder.
4. Walk the AST using provenance.
5. Export facts.

**Where facts come from**

| Fact | Source |
|---|---|
| `Type` | `Expr.Type()`, converted from `types.Type` to the JVM-style `analysis.Type` |
| `Symbol` | `Ident.Sym`, `Member.Prop`/`Getter`, `*Decl.Sym`, interned into `uint32` IDs |
| `Call` | `Call.Sym`/`Ctor`/`ArgMap`/`Operator`/`Invoke`/`SAM`, plus annotations and flags from `FunSym` metadata |
| `When`, flow, unreachable and cast flags | the recorder |

- `Data.Capabilities` lists the fact families that are implemented. Lint already fails with
  `unsupported_analysis` when a required fact is missing, and it must never publish partial results.

### A6. Repo hygiene
- README gets a short "PSI and analysis API" section, and `docs/psi.md` is rewritten from the
  branch's version without the JVM/JNI parts.
- `docs/design.md` gets a note on the PSI frontend.
- CI gains three things:
  - `go run ./cmd/psi-kinds -check`
  - `go test -race ./psi ./analysis`
  - `FuzzParse`: leaf text concatenates to the source, the arena invariants hold, and nothing panics
- Tag a version once A2 lands, and again after A5, so the linters repo can depend on it.

---

## Part B: kotlin-linters-go (new repo)

> **Status: imported.** [barteo/kotlin-linters-go](https://github.com/barteo/kotlin-linters-go)
> holds the import (one commit citing c462563) and depends on this branch by pseudo-version. Its
> `docs/status.md` lists what passes: the syntax-only rules almost all do, and most remaining
> failures are B3.2 fact families `analysis` does not provide yet. Not done: the version tags in
> A6, which wait for this branch to merge, and the speed gate, whose harness still needs the
> dropped bridge.

**Prerequisite:** the user creates the empty GitHub repo `barteo/kotlin-linters-go`. This session
cannot create repositories. It is then added to the session.

### B1. Import and layout
Module path: `github.com/barteo/kotlin-linters-go`.

```
detekt/  detekt-compose/  detekt-formatting/        ← from lint/* (LICENSE.* files kept alongside)
internal/{detektref,detektmeta,sourceorigin,sourcepath,signaturetext}
cmd/{kgo-detekt,kgo-detekt-bench,detekt-reference,detekt-inventory}
bench/{detekt,full-detekt,compare-detekt,native-detekt.sh,native-detekt.py,native-opt.sh,pgo-detekt.py,build-rule-fixtures.sh,results/(detekt-only)}
oracle/   ← trimmed jvm/: Gradle build, dependencies.lock, src/custom, the 5 oracle runners + ParserGuard
docs/{native-detekt.md,native-detekt-rules.md,native-detekt-optimization.md}
THIRD_PARTY_NOTICES.md (detekt, ktlint, compose-rules, unbescape, yaml entries)
```

- Rewrite imports with one mechanical pass:

  | Old import | New import |
  |---|---|
  | `kgo/psi` | `github.com/barteo/kotlin-compiler-go/psi` |
  | `kgo/analysis` | `github.com/barteo/kotlin-compiler-go/analysis` |
  | `kgo/lint/X` | `github.com/barteo/kotlin-linters-go/X` |
  | `kgo/internal/X` | `.../kotlin-linters-go/internal/X` |

- Fix repo-relative paths: `../../bench/...` in `full_suite_test.go:29` and `fixture_classpath_test.go:41`, `jvm/` changes to `oracle/`, and the Python scripts' `go run ./cmd/...` calls.
- Drop `bench/full-detekt/baseline/instrument.py`'s edits to kc internals that don't exist.
- `go.mod` requires `gopkg.in/yaml.v3` and `kotlin-compiler-go`. For local development, use
  `go.work` with a sibling checkout; CI checks out both repos.

### B2. Re-create the missing lint-side packages
The signatures were inferred from call sites.
- **`sourcepath`**: `Absolute(ctx, path)`, `WithAbsolute(ctx, path, abs)`, and `Resolver` (zero
  value usable, caches the working directory).
- **`signaturetext.Normalize`**: newlines and runs of two or more whitespace characters become one
  space. Deduplicate it with `detekt/suppression.go:155` `signatureWhitespace`.
- **`sourceorigin`**: maps normalized offsets back to original offsets.
  - `Normalized(original, compose bool) View`, `Constant(origin, n) View`
  - `Builder{Append, Repeat, View}`
  - `View{Len, At, Slice, Copy}`
- **`detektmeta`** holds the embedded `contracts.json`, `rules.json`, `runtime-rules.json`,
  `native-status.json`, `default.yml`, `corpora.json` and LICENSE/NOTICE. Generate it with
  `go run ./cmd/detekt-inventory -source detekt-1.23.8`, a checksum-verified release.
- **`detektref`**: `Root`, `ReadFile(ctx, logicalPath)`, `Entries`, `CoreLedgerNames`,
  `New(root).Resolve`, `CachedArtifact`.
  - The cache lives in `oracle/build/detekt-reference`, or `$KGO_DETEKT_REFERENCE_CACHE`.
  - Writes take a lock, and files are published atomically.
  - The manifest `internal/detektref/manifest.json` is missing, so the SHA pins have to be rebuilt.
    Generate every reference with the oracle once, review the output, and commit a new manifest.
    This is trust on first use, and the commit message records that.

### B3. Bring up the linters in stages
1. **Syntax-only:** formatting, Compose, and the ~145 core rules without semantic callbacks need
   only A2. Get `go test ./detekt-formatting/... ./detekt-compose/...` and the syntax parity tests
   green first. Most of the remaining failures there will be in the psi grammar.
2. **Semantic:** the 69 rules with `semantic: true` in `rules_{calls,semantic,final,resolution}_batch.go`,
   plus the typed branches and the `ignoreAnnotated` resolution, need A5.
   - Work through them by fact family, using the pending ledgers (`*-pending.json`) and the fixed
     rejection counts such as `rules_calls_batch_test.go:31` expecting 29.
   - When kc gains support for a construct, update those counts deliberately.
3. Bring back `kgo-detekt --backend=jvm` only as an explicit "not supported" error. Delete the JNI
   references in docs.

---

## Verification

**kotlin-compiler-go**
- `go vet ./... && go test -race ./...` passes, with existing results unchanged. The new `Scan` must
  not change `Tokenize` output.
- `go run ./cmd/psi-kinds -check`.
- psi unit tests:
  - lossless round trip on every `testdata/**/*.kt`
  - arena invariants
  - zero-value `NodeRef` safety
  - UTF-16 and position conversions on supplementary characters and CRLF
  - tree shapes match `psi/testdata/trees` goldens
  - `FuzzParse` for 60 s in CI
- Lowering: the `syntax.Equal` differential test on every `testdata/**/*.kt`, and
  `KC_FRONTEND=psi go test ./...` passing as completely as the default frontend does.
- analysis: table tests checking facts for representative snippets (types, symbols, calls, `when`
  exhaustiveness, smart casts), cancellation, `.kts` returning `ErrUnsupported`, and deep-copy
  isolation, as in the branch's `semantic_memo_test.go`.
- Performance check: `psi.Parse` plus lowering throughput on the kotlinx-coroutines corpus,
  compared with `syntax.Parse`. Lowering should cost about the same as parsing.

**kotlin-linters-go**
- `go build ./... && go test ./...` against the local kc checkout via `go.work`.
- `go run ./cmd/detekt-reference` regenerates all references through `oracle/`, which needs JDK 21,
  and checks them against the new manifest.
- End to end:

  ```bash
  go build -o bin/kgo-detekt ./cmd/kgo-detekt
  ```

  ```bash
  bin/kgo-detekt --input detekt/testdata --all-rules --formatting --compose --report sarif
  ```

  With no classpath, the run either produces findings or exits with an explicit
  `unsupported_analysis` error. It never publishes partial results.
- `bench/native-detekt.sh` is the speed gate at 10× or better, run once the semantic rules are
  green.
