# Conformance with kotlinc's own tests

```sh
bench/kotlin-tests.sh box            # 7,480 box tests, about 20 s after the first clone
bench/kotlin-tests.sh diagnostics    # 10,827 diagnostics tests, about 45 s
```

The [survey](survey.md) counts the features real code uses, but can't say
whether `kc` gets their semantics right, or rejects what `kotlinc`
rejects. The Kotlin repository's own tests can:

- **Box tests** (`compiler/testData/codegen/box`): small programs whose
  `fun box(): String` returns `"OK"`. Each is compiled by `kc` and run.
- **Diagnostics tests** (`compiler/testData/diagnostics`): sources whose
  markup, `<!NAME!>code<!>`, marks the diagnostics `kotlinc` reports.
  Each is compiled by `kc` without its markup, and whether `kc` rejects
  it is compared with `kotlinc`.

The script makes a sparse checkout of JetBrains/kotlin at the tag of the
installed Kotlin (`v2.4.21`) in `$TMPDIR`, with the test data and the
files that declare the diagnostics, and runs `bench/conformance`. It
writes [bench/results/box.md](../bench/results/box.md) and
[bench/results/diagnostics.md](../bench/results/diagnostics.md), each
with a `.tsv` file beside it: one line per test, with its path, category,
key and detail. Flags after the mode go to the harness:

- `-filter when/` runs the tests whose path starts with it.
- `-list FILE` runs the tests whose paths the file lists, one a line
  (the first field of a TSV line; `#` starts a comment).
- `-errors FILE` (diagnostics) writes `kc`'s errors on lines where the
  markup expects no diagnostic to a TSV file: the test, its category, the
  file and line, and the message. It leaves out errors of a kind the
  test's `DIAGNOSTICS` directive switches off, where `kc`'s message names
  the kind (`check.IsNamed`).
- `-marks FILE` (diagnostics) writes each diagnostic the markup expects
  to a TSV file: the test, its category, the file and line, the
  diagnostic and its severity, and `hit` if `kc` reported on its line an
  error, for an error, or the same warning, for a warning, or `miss`,
  followed for a missed error or warning by its cause.
- `-warnings FILE` (diagnostics) writes `kc`'s warnings on lines where
  the markup doesn't expect them, other than those a test's `DIAGNOSTICS`
  directive switches off, to a TSV file: the test, its category, the
  file and line, the warning and its message. Warnings in the shared
  helper files, which the markup doesn't cover, aren't counted, nor
  `ERROR_SUPPRESSION` in a test with `DONT_WARN_ON_ERROR_SUPPRESSION`.
  `UNUSED_EXPRESSION` and `UNUSED_LAMBDA_EXPRESSION` count as switched
  off unless the test asks for them, with `WITH_EXTRA_CHECKERS`,
  `DIAGNOSTICS: +NAME` or a mark of the warning: kotlinc reports them by
  default, but the markup of the other tests never expects them.
- `-baseline bench/results/box.tsv -o "" -tsv ""` compares a run with the
  committed one, printing the tests that stopped passing and those that
  started, and fails if any stopped. The baseline is read before the run,
  so a run can also write its results over it.
- `-missed FILE` (diagnostics) writes each expected error `kc` doesn't
  report on its line to a TSV file: the test, the file and line, the
  diagnostic and why it was missed (see below). `bench/kotlin-tests.sh
  diagnostics` writes it to
  [bench/results/diagnostics-missed.tsv](../bench/results/diagnostics-missed.tsv),
  the backlog of missed errors.
- `-missed-baseline bench/results/diagnostics-missed.tsv` (diagnostics,
  with `-baseline`) compares a run's missed errors with that file's, and
  fails if `kc` misses an error it reported on its line before. Tests
  that weren't type-checked in either run, such as unsupported ones,
  are left out.
- `-missed-warnings FILE` and `-missed-warnings-baseline FILE`
  (diagnostics) do the same for the expected warnings `kc` doesn't report
  on their lines, by their names.
  `bench/kotlin-tests.sh diagnostics` writes them to
  [bench/results/diagnostics-missed-warnings.tsv](../bench/results/diagnostics-missed-warnings.tsv),
  the backlog of missed warnings. A warning's cause is *syntax-error*,
  *module-not-compiled* or *missing-check*.
- `-kotlinc kotlinc -sample 100` runs a random sample with `kotlinc`
  instead of `kc` (see [Checking the harness](#checking-the-harness)).
- `-serve=false` starts a `kc` process for each module, instead of
  long-lived ones (see [Long-lived kc](#long-lived-kc)).

### Long-lived kc

Most of compiling a test is work every test repeats: indexing the JDK's
28,900 classes, and reading the standard library's default imports and
the classes the test uses. Compiling an empty file takes about 62 ms of
CPU, and the median test 66 ms. So the harness compiles on `-j`
processes of `kc -serve`, each of which keeps the class path and the
library it reads it with (`driver.Libraries`) for its next compiles,
and leaves each test's own classes to that test's check, as
`analysis/ka` does for KSP. Only class paths of the JDK and jars are
kept, keyed by their size and modification time and by the settings the
library reads classes with; a later module of a test, compiled against
the output directories of the modules it depends on, gets a class path
of its own.

On 4 CPUs this takes each suite from about 3 minutes to 10 to 15
seconds; each server peaks at about 225 MB.

| Suite (4 CPUs) | A process a module | Long-lived |
|:---|---:|---:|
| box | 2 min 53 s | 14 s |
| diagnostics | 2 min 57 s | 9 s |

Those times are from before the diagnostics tests with Java sources
ran. A compile with Java sources gets a class path of its own, since
the sources are added to it, so those 1,221 tests take most of the
diagnostics suite's 45 s now.

A server compiles one test at a time, so a panic, which ends it with
its trace as it would end `kc`, is charged to the test that caused it,
and a new server takes the next. A server also exits after any failure
that isn't a diagnostic, such as code generation that panicked and
recovered, so no later test sees what it left behind. Both modes put
every test in the same category and write the same reports. Their
class files differ only where `kc` differs from run to run anyway, in
the order of some members, and in one test where `kc`'s result depends
on whether `lazy` was used before (`regressions/objectInsideDelegation.kt`: a
signature `kotlinc` doesn't write at all), which a warm library looks
like. `-serve=false` rules out such order effects.

## Box tests

The harness compiles a test's modules (`// MODULE: main(lib)(friend)`,
`// FILE: a.kt`, where the code before a module's first `// FILE:` is
that file's start) in order, each against the modules it names as
dependencies and friends, as the test infrastructure does: only those,
not theirs, so a library's own dependencies are missing from the class
path unless the module names them too. Friends' output directories are
also its `-friend-paths`. It compiles with `kc -features`, which names the unsupported feature behind an error.
It then runs `box()` in a fresh class loader of a long-lived JVM
(`bench/conformance/BoxRunner.java`), with a 10 s timeout. As the test
infrastructure does, it puts the JetBrains annotations
(`org.jetbrains.annotations`), the standard library and kotlin-test on
the class path (and kotlin-reflect for `WITH_REFLECT`), adds the helper
sources for `WITH_COROUTINES`, `CHECK_TYPE` and `INFERENCE_HELPERS`,
turns `OPTIONAL_JVM_INLINE_ANNOTATION` into `@JvmInline` and
a module's `JVM_DEFAULT_MODE` and `JVM_TARGET` (or the test's, before
its first module) into `-jvm-default` and `-jvm-target`, removes
diagnostic markup, and looks for `box()` in the class `kotlinc` puts it
in. It skips tests for other backends, tests `kotlinc`'s JVM backend
ignores, and tests that need `LANGUAGE` features, compiler arguments,
Java sources, or modules with `dependsOn`, and the one test that
compiles only against a JDK before 21 (`extensionFunctions/kt475`:
there `array.last = "5"` assigns the synthetic `val` of JDK 21's
`ArrayList.getLast()`, which `kotlinc` rejects too). A `LANGUAGE` that
only sets features as Kotlin 2.4 has them by default, or as `kc`
follows them (`+AllowAccessToProtectedFieldFromSuperCompanion`,
`+AllowReifiedTypeInCatchClause`,
`+AllowReturnInExpressionBodyWithExplicitType`,
`+AvoidWrongOptimizationOfTypeOperatorsOnValueClasses`,
`+BreakContinueInInlineLambdas`, `+ContextParameters`,
`+DataFlowBasedExhaustiveness`, `+ExplicitBackingFields`,
`-ForbidParenthesizedLhsInAssignments`,
`+JvmNullOutSpilledCoroutineLocals`, `+MultiDollarInterpolation`,
`+NestedTypeAliases`, `+NullableNothingInReifiedPosition`,
`+PCLAEnhancementsIn21`, `+PrioritizedEnumEntries`,
`+ProperIeee754Comparisons`,
`+RefinedVarargConversionRulesForCallableReferences`,
`+ResolveTopLevelLambdasAsSyntheticCallArgument`, `+WhenGuards`,
and the features new in Kotlin 2.4, which `kotlin24Features` in
`bench/conformance/testfile.go` lists) doesn't count.

| Category | What happened |
|:---|:---|
| pass | `box()` returned `"OK"` |
| parse/unsupported | `kc` doesn't support a feature (`syntax.Error.Feature`), or can't parse the code |
| check error | `kc`'s checker rejected valid code |
| crash | `kc` panicked, ran out of stack, or timed out |
| codegen error | `kc` checked the code but couldn't generate it |
| VerifyError | the JVM rejected a class (`VerifyError`, `ClassFormatError`) |
| linkage error | another `LinkageError`, or no `box()` where `kotlinc` puts it |
| exception | `box()` threw |
| wrong result | `box()` returned something else |
| timeout | `box()` ran for more than 10 s |

The last five are silent miscompilation: `kc` accepted the program and
wrote classes that don't do what `kotlinc`'s do.

## Diagnostics tests

The JVM tests are those of `tests/`, `testsWithStdLib/`,
`testsWithJvmBackend/` and `testsWithAnyBackend/`, without the variant
files (`.ll.kt`, `.latestLV.kt`, ...). A test is **invalid** if it marks
an error or its `RUN_PIPELINE_TILL` stops it before the backend
(`FRONTEND`, `FIR2IR`); one that runs to the backend can still mark the
JVM backend's errors, such as `CONFLICTING_JVM_DECLARATIONS`. Severities
come from the declarations of the diagnostics (`FirErrors`,
`FirJvmErrors`, `FirSyntaxErrors`, `JvmBackendErrors`,
`CommonBackendErrors`), and a deprecation's from its name in the markup
(`NAME_ERROR` or `NAME_WARNING`).

| Category | `kotlinc` | `kc` |
|:---|:---|:---|
| rejects invalid | rejects | rejects |
| accepts valid | compiles | compiles |
| **accepts invalid** | rejects | compiles |
| **rejects valid** | compiles | rejects |
| parse/unsupported | either | uses a feature it doesn't support, or can't parse the code |

Accepting invalid code is a correctness bug, not a missing feature. The
report ranks those tests by the errors they expect, with *only error*
counting the tests one check alone would fix. Since `kc` reports many
errors per file, it also checks each expected error for one of `kc`'s on
its line, and lists `kc`'s errors on lines that expect none. Warnings
don't change a test's category: the report counts, by warning, those the
tests expect that `kc` reports on their lines and those it reports on
lines that don't expect them, which `-Werror` would turn into rejected
valid code, and lists the warnings `kc` doesn't report.

Four things differ from the test infrastructure:

- **The standard library.** Without `WITH_STDLIB`, the infrastructure
  gives `kotlinc` an unpublished minimal library. Both compilers get the
  full one instead, so a test can resolve what it expects not to. The
  most common such call, `x.hashCode()` or `x.toString()` on a nullable
  `x` that the markup expects to be an `UNSAFE_CALL` (or that it
  can't be smart cast, a `SMARTCAST_IMPOSSIBLE`), would resolve to
  the full library's `Any?` extension: the harness calls an extension of
  `Any` that it declares at the end of the file instead, which both
  compilers then reject unless `x` is smart cast to non-null. It does
  the same for a bound reference, `x::hashCode`, that the markup expects
  to be an `UNSAFE_CALLABLE_REFERENCE`.
- **The `CHECK_TYPE` helpers** declare names (`_`) that `kotlinc` reports
  outside the infrastructure. A test that adds them stops in the
  frontend even when valid, so only its marks count.
- **Directory options.** Directories whose test classes give `kotlinc`
  options are compiled with them where `kc` has them (explicit API mode in
  `testsWithExplicitApi/`, explicit return types in
  `testsWithExplicitReturnTypes/`, as for the `EXPLICIT_API_MODE`
  directive), and skipped where it doesn't (the return value checker in
  `crv/`, header mode, strict equality), as are tests
  with `LANGUAGE` features (but those on by default in Kotlin 2.4),
  multiplatform modules or `IGNORE_FIR_DIAGNOSTICS`. So are `IDE_MODE`
  tests that mark the error form of a diagnostic (such as
  `IMPLICIT_PROPERTY_TYPE_MAKES_BEHAVIOR_ORDER_DEPENDANT_ERROR`), which
  `kotlinc` reports only in the IDE.
- **Switched-off backend errors.** A test that switches off
  `CONFLICTING_JVM_DECLARATIONS`, `CONFLICTING_INHERITED_JVM_DECLARATIONS`
  or `ACCIDENTAL_OVERRIDE` with the `DIAGNOSTICS` directive still runs
  to the backend, which `kotlinc` outside the infrastructure stops on.
  `kc` reports these errors in its checker, and the harness drops them
  (by their messages, which begin as the backend's do) from such tests.

`kotlinc`'s extra and experimental checkers report only with `-Wextra`
and `-Xuse-fir-experimental-checkers`. The infrastructure runs the extra
ones for tests with `WITH_EXTRA_CHECKERS` and those of
`controlFlowAnalysis/deadCode/` and `extraCheckers/`, and the experimental ones for tests with
`WITH_EXPERIMENTAL_CHECKERS`; the harness passes `kc` the flags for the
same tests.

Tests with Java sources run: `kc` gets a module's `.java` files with its
`.kt` files and reads their declarations, as `kotlinc` does. Like
`kotlinc`'s Java parser, `kc` reads past the syntax errors that IntelliJ's
Java PSI recovers from, leaving out the member, as some tests' malformed
Java (`public statis void function()`, a parameter of type `?`) needs;
`javac` reports them in a build. The Java
sources of a module that later modules depend on are then compiled with
`javac` against its Kotlin classes, as the test infrastructure does, and
those of the last module are only read.

Some *accepts invalid* tests are out of `kc`'s reach, and stay so.
`kotlinc` compiles them under the harness too (see the self-check
below), except for the FIR2IR tests:

- **A class path without the standard library**: the two
  `unsignedLiteralsWithoutArtifactOnClasspath` tests, and
  `nullableTypes/nullableArgumentForIn`, whose `Int? in IntRange` the full
  library's `contains` extension resolves (see above). The same goes for
  `j+k/collectionOverrides/containsAll` and the library's `containsAll`
  extension, and for `typealias/starProjection`, whose index of a
  `Map<*, *>` the library's `get` extension resolves.
- **A directive `kc` doesn't model**: `PROGRESSIVE_MODE`
  (`dataClassInternalConstructorUsageWillBecomeInaccessible`).
- **Java source roots**: `j+k/differentFilename` and
  `problems/multipleJavaClassesInOneFile`. They expect a Java file's
  classes other than the one it is named after to be unresolved. That
  is `kotlinc`'s view of a source root directory. Given the `.java`
  files themselves, as Gradle and `kc` take them, `kotlinc` resolves
  them too.
- **An error of the test pipeline only**:
  `testsWithStdLib/typealias/upperBoundViolated` and
  `upperBoundViolated2` expect, besides the deprecation warning of a
  typealias constructor call whose expansion breaks a bound, an
  `OTHER_ERROR_WITH_REASON` ("Unknown error: Inapplicable"). Neither
  `kotlinc` 2.4.21's command line nor its analysis API (the tests'
  `.ll.diag.txt`) reports it: both give only the warning, as `kc` does.
- **The FIR2IR tests that mark no error**:
  `nullableTypes/nullAssertOnTypeWithNullableUpperBound` and
  `safeCallOnTypeWithNullableUpperBound` are valid to the frontend, and
  `kotlinc`'s JVM backend crashes on them, which is why they stop at
  `FIR2IR`.
- **A dependency's dependency off the class path**:
  `j+k/missingTypeOfParameterInFakeOverride_1` marks no error, and
  `kotlinc` stops it at the frontend only because its backend can't
  generate the fake override whose parameter type is missing. The
  harness puts only the modules a module names on its class path, not
  their dependencies, as the test does, and the frontend accepts it.
  (Its sibling `_2`, where the missing type keeps a Java method from
  being implemented, `kc` rejects.)
- **A JDK class declared again in a library**:
  `jvm/classpath/classFromJdkInLibrary` declares `java.util.Date` in a
  Java library, which `javac` compiles with `--patch-module`, and
  expects its new method to be unresolved. Given the library's class
  files on the class path, `kotlinc` resolves it too, as `kc` does.

The other one is `kc`'s own: `regressions/kt334` expects
`java.lang.Comparable` to be a type of its own, which `kc` takes as
`kotlin.Comparable` (see [unsupported](unsupported.md)).

## Checking the harness

A failure must be `kc`'s, not the harness's, so `-kotlinc` runs tests
through the same steps with `kotlinc`. Every box test must pass, and
every diagnostics test must come out *rejects invalid* or *accepts
valid*. A sample of 309 box tests all pass, and a sample of 300
diagnostics tests all agree; earlier samples found the rules above.
Repeated runs put every test in the same category, though some details
change from run to run, such as which missing member a class is reported
for, or the order of two clashing methods.

## Results

Kotlin 2.4.21's tests on `kc`:

| Box tests | Tests | Share of run |
|:---|---:|---:|
| run (of 7,480) | 6,575 | |
| pass | 6,575 | 100% |
| check error, parse/unsupported, crash, codegen error, timeout | 0 | |
| VerifyError, linkage error, exception, wrong result | 0 | |

| Diagnostics tests | Tests | Share of run |
|:---|---:|---:|
| run (of 10,827) | 8,613 | |
| rejects invalid | 4,448 | 51.6% |
| accepts valid | 4,150 | 48.2% |
| accepts invalid | 15 | 0.2% |
| rejects valid | 0 | |
| parse/unsupported | 0 | |
| crash | 0 | |

`kc` agrees with `kotlinc` on 99% of the diagnostics tests, and reports
21,739 of the 22,025 expected errors (98.7%) on their lines, and 3,521 of
the 3,667 expected warnings (96.0%), with 18 warnings on lines that expect
none. Tests with Java sources are 1,221 of the run, and 5 of the 15
that *accept invalid* code. The reports break the failures down; the
largest groups are:

- **Silent miscompilation**: none. No box test that `kc` compiles
  gives a wrong result, throws, or fails to verify or link.
- **Accepts invalid**, by the tests that expect each error: argument
  type mismatches (`ARGUMENT_TYPE_MISMATCH` 3) and unresolved names
  (`UNRESOLVED_REFERENCE` 3); no other error is expected by more than
  two. All but one are out of reach (see above).
- **Rejects valid**: none, and no box test is rejected.
- **Crashes**: none, and no harness errors.

The committed `.tsv` files serve as a ratchet: a change must make
`-baseline` report no regressions, and the tests it fixes measure it.
CI's *Conformance* job (`.github/workflows/pr-checks.yml`) runs both
suites against them on every pull request and push to `main`, and fails
if a test that passes there stopped passing, or, for the diagnostics
tests, if an expected error or warning `kc` reported on its line is now
missed. The job's summary has each suite's counts. A test that started
passing, or an error or warning newly reported, is a warning: running `bench/kotlin-tests.sh
box` or `diagnostics` writes the new baselines to commit with the change.

The diagnostics report sorts the missed errors by cause (*Missed errors
by cause*), the first of these that applies:

- *accepted*: `kc` compiled the test.
- *syntax-error*: `kc`'s parser rejected the code. `kc` then checks
  it as `psi`, which has `kotlinc`'s grammar, parses it, leaving out
  only the statements, or declarations outside function bodies, that
  have a syntax error or that it can't lower, so the errors in those
  are missed.
- *module-not-compiled*: the error is in a module after one that
  failed, which the harness doesn't compile.
- *nearby*: `kc` reported an error within three lines of it, on a line
  the markup expects none on: most likely this error, at another
  position.
- *missing-check*: none of those.
