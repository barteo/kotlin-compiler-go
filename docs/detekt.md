# detekt on kc

detekt 2.0 resolves types through the Kotlin Analysis API (AA) in standalone
mode. [`jvm/detekt`](../jvm/detekt) builds detekt-cli from detekt's sources,
with a few patches, on [kc's Analysis API](analysis-api.md) (`jvm/kc-aa`, the
same implementation the KSP backend uses) instead of FIR. kc answers detekt's
questions from its [analysis facts](psi.md).

## Results

detekt-cli 2.0.0-alpha.6 was run with `--analysis-mode full --all-rules` on a
4-core machine. Stock is the released `detekt-cli-2.0.0-alpha.6-all.jar`. "On
kc" is `jvm/detekt/run.sh`, which runs the rules with `--parallel`. Both runs
used the same inputs, class path and `-opt-in`s.

| Target | Files | Stock | Stock, `--parallel` | On kc | Speedup | Findings that match |
|:---|---:|---:|---:|---:|---:|---:|
| kotlinpoet 2.4.0 (`commonMain`, `jvmMain`) | 39 | 26.0 s | 18.5 s | 4.9 s | 5.3×, 3.8× | 1581 of 1597 (99.0%) |
| kotlinx-coroutines-core 1.11.0 (`common`, `concurrent`, `jvm`) | 169 | 37.2 s | 29.5 s | 8.9 s | 4.2×, 3.3× | 2730 of 2778 (98.3%) |

- "On kc" is the median of 5 runs after the first, which writes the
  class-data sharing archive.
- kc's findings with `--parallel` are identical to its serial run's. Serially,
  kc takes 7.3 s on kotlinpoet.
- [Making detekt on kc faster](detekt-performance.md) has the details.

A finding matches when its file, line, column and rule are the same.

- On kotlinpoet, stock has 16 findings that kc lacks, and kc has 5 that stock
  lacks.
- On kotlinx-coroutines-core, the counts are 48 and 57. There, stock's
  single-module session rejects `expect` and `actual` declarations. So stock
  reports, for example, `DefaultExecutor`'s private constants as unused, though
  they are used. Some of the differences are therefore stock's.

Most of the time stock saves goes into detekt's `ValidateClasspath` step.
Stock collects diagnostics on every file there, which is full FIR resolution.
kc reports its errors when it analyzes the sources.

## How it works

```
detekt-cli (patched)          jvm/kc-aa                              Go: libkc.so
  EnvironmentAware ──► KcStandalone.open: PSI, project
  rules: analyze(e){} ──► KcSessionProvider ─► KaSession proxy   JNI   internal/kaops
                          KcAnalysis + KcLint: symbols,       ◄──►      └─ analysis/ka
                          types, calls, facts                             (check + facts)
                          real KtFile PSI ↔ kc psi node index
```

**Build.**
- `jvm/detekt/build.sh` fetches detekt at tag `v2.0.0-alpha.6` and applies
  `jvm/detekt/patches` with `git am`.
- It compiles the modules detekt-cli runs with kotlinc 2.4.21, one at a time,
  against `jvm/aa-api`'s AA 2.4.21 jar and `kc-aa.jar`. The modules are the
  API, core, parser, psi utilities, metrics, tooling, utils, the rule sets,
  the reports and the command line.
- No Gradle is involved. It does two things detekt's Gradle build does:
  - It writes detekt-api's `BuildConfig`.
  - It runs the Poko compiler plugin on detekt-api.
- Kotlin 2.4.21 puts two compiler APIs detekt uses behind opt-ins, so the
  build passes those `-opt-in`s.

**The patches.** Each patch's message says why it's needed.
- `0001` replaces detekt's `buildStandaloneAnalysisAPISession` with
  `KcStandalone.open`. It passes the module's source roots, class path,
  friend paths, JDK home, JVM target, `-opt-in`s and language settings, and
  takes the session's `KtFile`s.
- `0002` makes the rules that read a compiler warning match it by factory
  name, such as `UNNECESSARY_SAFE_CALL`, not by FIR's `KaFirDiagnostic`
  classes.
- `0003` follows `JavaClassFinderImpl`'s constructor in kotlin-compiler 2.4.21.

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
- `run.sh` runs detekt with `--parallel` and a class-data sharing archive,
  and kc analyzes in the background as soon as detekt builds the session.
  [Making detekt on kc faster](detekt-performance.md) measures these and
  the other speedups.
- Go's runtime runs in the JVM's process, so `run.sh` preloads the JDK's
  `libjsig.so`. That chains Go's signal handlers with the JVM's.
- `build.sh test` runs detekt with `--all-rules` on the fixture in
  `jvm/detekt/test/src`. It compares the findings with `test/want.txt`, which
  is also what stock detekt finds there. CI runs it.

**Changing detekt.**
1. Edit the checkout in `jvm/detekt/build/detekt`.
2. Commit there: amend the patch's commit, or add a new one.
3. Run `jvm/detekt/build.sh patches`, which rewrites `patches/`.
4. Run `jvm/detekt/build.sh`.

## Differences

These are the known causes of the differences that remain:

- **`String` members.** `kc` resolves `s.substring(a, b)` to the Java member
  `java.lang.String.substring`. kotlinc resolves it to the stdlib extension
  `kotlin.text.substring`, because Kotlin's `String` doesn't expose Java's
  members. `UnnamedParameterUse` skips Java callees, so it misses these calls
  on kc: 14 of the 16 kotlinpoet findings that kc lacks.
- **Warnings.** kc reports no warnings of its own. `directDiagnostics` reports
  only what the facts tell, and some deprecations and unreachable code are
  missing from them.
- **Unanswered members.** On kotlinpoet, `-Dkc.trace` counts only these:
  - a library property's `isVal`
  - a library parameter's `hasDefaultValue`
- **Subtyping.** `isSubtypeOf` uses kc's supertype lists. It compares type
  arguments only between types of the same class, by the declared variance
  of the stdlib's collections and of the sources' classes.
- **Scopes.** `scopeContext` is built from PSI. It holds the enclosing local
  declarations and class members, the file's explicit imports, and its
  package's top-level declarations. Default imports are left out.
