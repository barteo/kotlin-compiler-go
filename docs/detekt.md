# detekt on kc

detekt 2.0 resolves types through the Kotlin Analysis API (AA) in standalone
mode. [`jvm/kc-detekt`](../jvm/kc-detekt) answers that API from `kc`'s
[analysis facts](psi.md) instead of FIR. detekt-cli runs unchanged: only the
jar of the AA's standalone mode is swapped on its class path.

## Results

detekt-cli 2.0.0-alpha.6 was run with `--analysis-mode full --all-rules`, with
the same inputs and class path, on a 4-core machine. Each kc run took about
one second inside `kc`.

| Target | Files | Stock | On kc | Speedup | Findings that match |
|:---|---:|---:|---:|---:|---:|
| kotlinpoet 2.4.0 (`commonMain`, `jvmMain`) | 39 | 26.5–30 s | 9.1–9.3 s | 2.9–3.3× | 1583 of 1604 (98.7%) |
| kotlinx-coroutines-core 1.11.0 (`common`, `concurrent`, `jvm`) | 169 | 45.6–60 s | 15.9–16.5 s | 2.9–3.7× | 2804 of 2834 (98.9%) |

With `--parallel` on kotlinx-coroutines-core, stock took 49 s and kc took
13.4 s. kc's findings with `--parallel` are identical to its serial run.

A finding matches when its file, line, column and rule are the same.

- On kotlinpoet, stock has 21 findings that kc lacks, and kc has 8 that stock
  lacks.
- On kotlinx-coroutines-core, the counts are 30 and 55. There, stock's
  single-module session rejects `expect` and `actual` declarations, which
  causes 730 compiler errors in stock, including 224 overload resolution
  ambiguities. Some of the differences are therefore stock's.

**Where the time goes.**

| Phase | Stock | On kc |
|:---|---:|---:|
| `ValidateClasspath`: stock collects diagnostics on every file, which is full FIR resolution | 15–40 s | 0.05 s, because kc reports its errors when it analyzes |
| `Analyzer`: running the rules | 9.0 s (kotlinpoet), 17.5 s (coroutines) | 6.7 s, 13 s |

Light mode's `Analyzer`, with no types, takes 2.4 s on kotlinpoet and 7.1 s on
coroutines. That is the floor. Most of the rest is the rules' own code
running on a cold JVM: JIT compilation and deoptimization.

## How it works

```
detekt-cli (unchanged)        jvm/kc-detekt                         Go: libkc.so
  EnvironmentFacade ──► StandaloneAnalysisAPISessionBuilder (shim)
                        Kt*ModuleBuilder (shim): records roots
  rules: analyze(e){} ──► KcSessionProvider ─► KaSession proxy   JNI   internal/lintops
                          KcAnalysis: calls, symbols, types   ◄──►      └─ analysis.Analyze
                          real KtFile PSI ↔ kc psi node index            (psi.Parse, check)
```

**Analysis.**
- detekt builds its session with `buildStandaloneAnalysisAPISession`. That
  function is inline, so detekt's bytecode calls the builder classes directly.
- `kc-detekt` provides those classes. They record the source roots, the
  binary roots, the JDK home and the `-opt-in`s of the source module.
- On the first `analyze`, the libkc op `OpLintAnalyze` parses and checks all
  the sources at once.
  - If the sources don't compile, kc reports the errors and returns no facts,
    so detekt's type-resolution rules see no types.
  - Otherwise, each file's facts come back keyed by `psi` node: types,
    symbols, resolved calls with argument mappings, smart casts, and whether
    an expression is used as one.

**PSI.**
- detekt's `KtFile`s are kotlin-compiler 2.4.10's PSI.
- They map to `kc`'s nodes by preorder index. The trees have the same shape,
  and a per-file hash of the kind sequence guards the mapping, as in
  [the KSP backend](analysis-api.md).

**The Analysis API.**
- `KaSession` is a dynamic proxy. Its members dispatch by name and parameter
  types to `KcAnalysis`.
- A member that `kc-detekt` doesn't implement returns an empty value: null,
  false, 0 or an empty collection. Unimplemented members of symbols and
  types do the same.
- Under `-Dkc.trace=true`, the members that weren't answered are counted.
  Under `-Dkc.strict=true`, they fail.
- The AA classes come from detekt's own `detekt-kotlin-analysis-api` jar
  (AA 2.4.10). AA 2.4.21 lacks a member detekt calls,
  `resolveSymbol(KtNameReferenceExpression)`.

## Running

```
DETEKT_HOME=.../detekt-cli jvm/kc-detekt/build.sh
DETEKT_HOME=.../detekt-cli jvm/kc-detekt/run.sh -i src --analysis-mode full -cp ... --jdk-home ...
```

- `DETEKT_HOME` is a detekt-cli distribution, for example from
  `./gradlew :detekt-cli:installDist`.
- `run.sh` takes detekt-cli's arguments.
- Go's runtime runs in the JVM's process, so `run.sh` preloads the JDK's
  `libjsig.so`. That chains Go's signal handlers with the JVM's.

## Differences

These are the known causes of the differences that remain:

- **`String` members.** `kc` resolves `s.substring(a, b)` to the Java member
  `java.lang.String.substring`. kotlinc resolves it to the stdlib extension
  `kotlin.text.substring`, because Kotlin's `String` doesn't expose Java's
  members. `UnnamedParameterUse` skips Java callees, so it misses these calls
  on kc: 14 of the 21 kotlinpoet findings that kc lacks.
- **Unanswered members.** These count as unanswered under `-Dkc.trace`:
  - `allOverriddenSymbols`, which detekt uses only against configured names.
  - `findTopLevelCallables`.
  - `expectedType`.
  - A library property's `isVal`, a library class's `classKind` and
    `modality`.
  - A parameter's `generatedPrimaryConstructorProperty`.
  - `annotationApplicableTargets`.
- **Subtyping.** `isSubtypeOf` uses kc's supertype lists. It compares type
  arguments only between types of the same class, by the declared variance
  of the stdlib's collections and of the sources' classes.
- **Scopes.** `scopeContext` is built from PSI. It holds the enclosing local
  declarations and class members, the file's explicit imports, and its
  package's top-level declarations. Default imports are left out.
