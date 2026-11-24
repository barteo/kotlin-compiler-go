# detekt on kc

detekt 2.0 resolves types through the Kotlin Analysis API (AA) in standalone
mode. [`jvm/detekt`](../jvm/detekt) builds detekt-cli from the sources of
[detekt's fork](https://github.com/barteo/detekt/commits/kc-analysis-api), with a few
changes, on [kc's Analysis API](analysis-api.md) (`jvm/kc-aa`, the same
implementation the KSP backend uses) instead of FIR. kc answers detekt's
questions from its [analysis facts](psi.md).

## Results

detekt-cli 2.0.0-alpha.6 was run with `--analysis-mode full --all-rules` on a
4-core machine. Stock is the released `detekt-cli-2.0.0-alpha.6-all.jar` with
`--parallel`. "On kc" is `jvm/detekt/run.sh`, which runs the rules with
`--parallel`. Both used the same inputs, class path and `-opt-in`s.

| Target | Files | Stock, `--parallel` | On kc | Speedup | Findings that match stock | Multiplatform stock |
|:---|---:|---:|---:|---:|---:|---:|
| kotlinpoet 2.4.0 (`commonMain`, `jvmMain`) | 39 | 22.2 s | 3.7 s | 5.9× | 1597 of 1597 (100%) | 1601 of 1601 (100%) |
| kotlinx-coroutines-core 1.11.0 (`common`, `concurrent`, `jvm`) | 169 | 35.8 s | 6.4 s | 5.6× | 2748 of 2778 (98.9%) | 2760 of 2763 (99.9%) |

- Each time is the median of 5 runs, stock's and kc's round robin
  (`bench/detekt-vs-stock.sh`), after a first run of each that writes kc's
  class-data sharing archive.
- kc's findings are the same in every run, and the same serially and with
  `--parallel`. Stock's are the same in every run too.
- The ratio depends on the machine's load. On the same cloud machine under
  less load, stock took 16.1 s and 24.8 s, and kc 2.7 s and 4.7 s (6.0× and
  5.2×); under more, 32.9 s and 53.5 s, and 5.8 s and 10.5 s (5.7× and 5.1×).

A finding matches when its file, line, column and rule are the same.

detekt-cli analyzes all the source roots as one module. There, an `expect`
declaration and its `actual` one are redeclarations of each other:
- much of the bodies of `actual` classes doesn't resolve;
- calls of an `expect` function with an `actual` one are ambiguous.

So stock reports `DefaultExecutor`'s private constants, which are used, as
unused, and misses findings in code it leaves unresolved. "Multiplatform
stock" is stock with a source module per fragment, each depending on the
fragments it refines, as Gradle's multiplatform compilations have them
(`STOCK_FRAGMENTS` in `bench/detekt-vs-stock.sh`, which runs
`bench/detekt-stock-mpp`). Against it:
- On kotlinpoet, the findings are the same.
- On kotlinx-coroutines-core, kc reports no finding that it doesn't, and
  lacks 3 of its findings, for the reasons under [Differences](#differences):
  2 `UnnamedParameterUse` in common code, and an `UnreachableCode` that
  kotlinc doesn't report.

Against plain stock, kc has 4 findings more on kotlinpoet, all in `actual`
classes. On kotlinx-coroutines-core, it lacks 30 of stock's and has 12 that
stock lacks.

Most of the time stock saves goes into detekt's `ValidateClasspath` step.
Stock collects diagnostics on every file there, which is full FIR resolution.
kc reports its errors when it analyzes the sources.

## How it works

```
detekt-cli (kc's fork)        jvm/kc-aa                              Go: libkc.so
  EnvironmentAware ──► KcStandalone.open: PSI, project
  rules: analyze(e){} ──► KcSessionProvider ─► KaSession proxy   JNI   internal/kaops
                          KcAnalysis + KcLint: symbols,       ◄──►      └─ analysis/ka
                          types, calls, facts                             (check + facts)
                          real KtFile PSI ↔ kc psi node index
```

**Build.**
- `jvm/detekt/build.sh` fetches detekt's fork, branch
  [`kc-analysis-api`](https://github.com/barteo/detekt/commits/kc-analysis-api)
  of `barteo/detekt`, at the commit it pins: tag `v2.0.0-alpha.6` and one commit per change.
- It compiles the modules detekt-cli runs with kotlinc 2.4.21, one at a time,
  against `jvm/aa-api`'s AA 2.4.21 jar and `kc-aa.jar`. The modules are the
  API, core, parser, psi utilities, metrics, tooling, utils, the rule sets,
  the reports and the command line.
- No Gradle is involved. It does two things detekt's Gradle build does:
  - It writes detekt-api's `BuildConfig`.
  - It runs the Poko compiler plugin on detekt-api.
- Kotlin 2.4.21 puts two compiler APIs detekt uses behind opt-ins, so the
  build passes those `-opt-in`s.

**The changes.** They are the fork's commits on top of the tag, numbered
`0001`–`0008` here in their order. Each commit's message says why it's
needed.
- `0001` replaces detekt's `buildStandaloneAnalysisAPISession` with
  `KcStandalone.open`. It passes the module's source roots, class path,
  friend paths, JDK home, JVM target, `-opt-in`s and language settings, and
  takes the session's `KtFile`s.
- `0002` makes the rules that read a compiler warning match it by factory
  name, such as `UNNECESSARY_SAFE_CALL`, not by FIR's `KaFirDiagnostic`
  classes.
- `0003` follows `JavaClassFinderImpl`'s constructor in kotlin-compiler 2.4.21.
- `0004` lets the rules that need no types run while kc analyzes, with
  `-Dkc.overlap=true`. It then skips `ValidateClasspath`, whose diagnostics
  kc prints itself, and with `--parallel` runs those rules before the
  type-resolution rules, largest files first. It is off by default: on 4
  cores it competes with kc's check and measured no faster.
- `0005` starts kc's check in detekt-cli's `Runner`, as soon as the
  arguments are parsed (`KcStandalone.prestart`). The session takes it over
  when it gets the same module.
- `0006` makes `DetektVisitor`'s walk step over the leaves of the tree
  (tokens, white space and comments) for a visitor that overrides none of
  `visitElement`, `visitWhiteSpace`, `visitComment` and `visitPlainText`.
  Most of a tree's elements are leaves, and every rule walks every file.
  `-Ddetekt.visitLeaves=true` walks everything. It speeds up stock detekt
  too.
- `0007` makes each rule's path filters once, not for each file.
- `0008` reads each annotated element's suppressions once.

**Analysis.**
- When detekt opens the session, kc's ka operations open the module with
  facts on, on a background thread. One check of all the sources, read from
  disk, builds both the model KSP uses and package analysis's facts per
  file. Meanwhile a small pool parses the PSI, and then decodes each file's
  facts. The first `analyze` waits for the check and the round.
  - If the sources don't compile, kc prints the errors and the session has
    no facts, so detekt's type-resolution rules see no types.
  - Otherwise, each file's facts are joined with its PSI when a rule first
    asks about the file. The facts are keyed by `psi` node: types, symbols, resolved calls
    with argument mappings, smart casts, and whether an expression is used
    as one.
- Symbols and types are kc-aa's own, the ones KSP sees. Source declarations
  come from the round's PSI index. Library classes and callables come from the
  class path.
- `directDiagnostics` reports the warnings the facts tell:
  - `DEPRECATION`
  - `UNNECESSARY_SAFE_CALL`, `UNNECESSARY_NOT_NULL_ASSERTION` and
    `USELESS_ELVIS`, on values that can't be null
  - `CAST_NEVER_SUCCEEDS`
  - `UNREACHABLE_CODE`
- Names, calls and other elements get the references FIR gives them. So
  `mainReference.resolveToSymbol()` resolves through the session.

**PSI.**
- detekt's `KtFile`s are kotlin-compiler 2.4.21's PSI.
- They map to kc's nodes by preorder index. The trees have the same shape,
  and a per-file hash of the kind sequence guards the mapping, as in
  [the KSP backend](analysis-api.md).

**The Analysis API.**
- `KaSession` is kc-aa's dynamic proxy. Its members dispatch by name and
  parameter types to `KcAnalysis`.
- For a linter's session (`KcStandalone`), a member kc doesn't answer returns
  an empty value: null, false, 0 or an empty collection. Unanswered members
  of symbols and types do the same. KSP's session still fails on them.
- Under `-Dkc.trace=true`, the unanswered members are counted. Under
  `-Dkc.strict=true`, they fail.

## Running

```
jvm/detekt/build.sh
jvm/detekt/run.sh -i src --analysis-mode full -cp ... --jdk-home ... [-opt-in=...]
jvm/detekt/build.sh test
```

- `run.sh` takes detekt-cli's arguments. Compiler options, such as
  `-opt-in`, go last.
- `run.sh` runs detekt with `--parallel`, a class-data sharing archive and
  the parallel collector. The run is CPU-bound on all cores, where G1's
  barriers and concurrent work cost more than the parallel collector's
  pauses. `KC_JAVA_OPTS` naming a collector replaces it. An archive under
  8 MB, written by a run that stopped early, is deleted.
- kc's check starts as soon as detekt-cli has parsed its arguments
  (change `0005`), while detekt reads its configuration and the session
  builds its IntelliJ environment.
- Go's runtime runs in the JVM's process, so `run.sh` preloads the JDK's
  `libjsig.so`. That chains Go's signal handlers with the JVM's.
- `build.sh test` runs detekt with `--all-rules` on the fixture in
  `jvm/detekt/test/src`. It compares the findings with `test/want.txt`, which
  is also what stock detekt finds there. CI runs it.

**Changing detekt.**
1. In a clone of `barteo/detekt`, check out `kc-analysis-api`, and edit and
   commit there: amend the change's commit, or add a new one.
2. Run `DETEKT_SRC=<the clone> jvm/detekt/build.sh test`, which builds the
   clone as it is.
3. Push the branch, and set `DETEKT_COMMIT` in `jvm/detekt/build.sh` to its
   head.

## Differences

These are the known causes of the differences that remain:

- **Common code.** kc checks every body against the `actual` declarations,
  as one platform compilation. K2 resolves common code in its own fragment,
  against the `expect` ones. They differ where an `actual typealias` names
  a class with other members: `_state.compareAndSet(a, b)` in common code,
  where `_state` is an `expect class WorkaroundAtomicReference` whose
  actual is `java.util.concurrent.atomic.AtomicReference`, calls a Kotlin
  member under K2 and a Java method on kc. `UnnamedParameterUse` skips
  Java methods, so it misses 2 calls in `StateFlow.kt`.
- **Warnings.** kc reports no warnings of its own. `directDiagnostics` reports
  only what the facts tell. The Analysis API reports the right side of
  `cause?.let { throw it } ?: …` in `MainDispatchers.kt` as unreachable,
  which kotlinc 2.4.21 doesn't, so neither does kc.
- **Substitution overrides.** kc gives a call of an inherited library member
  the origin of FIR's substitution override where the supertype's type
  arguments change the member's signature, as `cache.get(key)` on an
  `object : ClassValue<Ctor>()`. A source member called that way stays the
  declaration.
- **Unanswered members.** On kotlinpoet, `-Dkc.trace` counts only these:
  - a library property's `isVal`
  - a library parameter's `hasDefaultValue`

  On kotlinx-coroutines-core, also `expectedType`, a typealias's
  `defaultType`, a collection literal's reference and one class's
  `superTypes`, 33 calls in all.
- **Subtyping.** `isSubtypeOf` uses kc's supertype lists. It compares type
  arguments only between types of the same class, by the declared variance
  of the stdlib's collections and of the sources' classes.
- **Scopes.** `scopeContext` is built from PSI. It holds the enclosing local
  declarations and class members, the file's explicit imports, and its
  package's top-level declarations. Default imports are left out.
