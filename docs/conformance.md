# Conformance with kotlinc's own tests

```sh
bench/kotlin-tests.sh box            # 7,480 box tests, about a minute after the first clone
bench/kotlin-tests.sh diagnostics    # 10,827 diagnostics tests, about a minute
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
- `-baseline bench/results/box.tsv -o "" -tsv ""` compares a run with the
  committed one, printing the tests that stopped passing and those that
  started.
- `-kotlinc kotlinc -sample 100` runs a random sample with `kotlinc`
  instead of `kc` (see [Checking the harness](#checking-the-harness)).

## Box tests

The harness compiles a test's modules (`// MODULE: main(lib)`,
`// FILE: a.kt`) in order, each against the ones before it, with
`kc -features`, which names the unsupported feature behind an error.
It then runs `box()` in a fresh class loader of a long-lived JVM
(`bench/conformance/BoxRunner.java`), with a 10 s timeout. As the test
infrastructure does, it puts the standard library and kotlin-test on the
class path (and kotlin-reflect for `WITH_REFLECT`), adds the helper
sources for `WITH_COROUTINES`, `CHECK_TYPE` and `INFERENCE_HELPERS`,
turns `OPTIONAL_JVM_INLINE_ANNOTATION` into `@JvmInline` and
`JVM_DEFAULT_MODE: disable` into `-jvm-default=disable`, removes
diagnostic markup, and looks for `box()` in the class `kotlinc` puts it
in. It skips tests for other backends, tests `kotlinc`'s JVM backend
ignores, and tests that need `LANGUAGE` features, compiler arguments,
Java sources, or modules with friends or `dependsOn`. A `LANGUAGE` that
only turns on `+ContextParameters`, which Kotlin 2.4 has on by default,
doesn't count.

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
its line, and lists `kc`'s errors on lines that expect none and the
warnings the tests expect (`kc` reports none).

Three things differ from the test infrastructure:

- **The standard library.** Without `WITH_STDLIB`, the infrastructure
  gives `kotlinc` an unpublished minimal library. Both compilers get the
  full one instead, so a test can resolve what it expects not to, such
  as `Any?.hashCode()`.
- **The `CHECK_TYPE` helpers** declare names (`_`) that `kotlinc` reports
  outside the infrastructure. A test that adds them stops in the
  frontend even when valid, so only its marks count.
- **Directory options.** Directories whose test classes give `kotlinc`
  options (the return value checker in `crv/`, explicit API mode, header
  mode, strict equality, `-jvm-default` modes) are skipped, as are tests
  with `LANGUAGE` features (but `+ContextParameters`), Java sources,
  multiplatform modules or `IGNORE_FIR_DIAGNOSTICS`.

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
| run (of 7,480) | 6,483 | |
| pass | 6,270 | 96.7% |
| check error | 142 | 2.2% |
| codegen error | 7 | 0.1% |
| VerifyError and linkage error | 29 | 0.4% |
| exception and wrong result | 35 | 0.5% |
| parse/unsupported, crash, timeout | 0 | |

| Diagnostics tests | Tests | Share of run |
|:---|---:|---:|
| run (of 10,827) | 7,200 | |
| rejects invalid | 3,047 | 42.3% |
| accepts valid | 2,807 | 39.0% |
| accepts invalid | 771 | 10.7% |
| rejects valid | 495 | 6.9% |
| parse/unsupported | 79 | 1.1% |
| crash | 1 | 0.0% |

`kc` agrees with `kotlinc` on 81% of the diagnostics tests, and reports
10,478 of the 18,906 expected errors (55%) on their lines. The reports
break the failures down; the largest groups are:

- **Silent miscompilation** (64 box tests): `NullPointerException`s
  (13), `VerifyError`s for bad local variable types (7), and duplicate
  bridge methods in `Map` implementations (3).
- **Accepts invalid**: annotation targets and arguments
  (`WRONG_ANNOTATION_TARGET` 47, `ANNOTATION_ARGUMENT_MUST_BE_CONST` 34),
  inline-only parameters (`USAGE_IS_NOT_INLINABLE` 27), opt-in (26),
  JVM signature clashes (`ACCIDENTAL_OVERRIDE`,
  `CONFLICTING_JVM_DECLARATIONS`, 24 each), and DSL markers on types
  (`DSL_SCOPE_VIOLATION` 15).
- **Rejects valid**, by `kc`'s first error: type mismatches (92 + 4
  box), inference failures (80 + 13), unresolved names and imports (73 +
  11), and calls on nullable receivers (48 + 9).

The committed `.tsv` files serve as a ratchet: a change must make
`-baseline` report no regressions, and the tests it fixes measure it.
