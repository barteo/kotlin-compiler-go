# Survey of real code

How far `kc` is from compiling real Kotlin: a survey of the features
nine JVM libraries use, and builds of kotlinpoet and ktor with `kc`,
checked against `kotlinc`'s. Earlier results,
milestone by milestone, are in this file's git history.

## Running

| Script | Input | Writes to `bench/results/` |
|---|---|---|
| `bench/survey.sh` | the libraries of `bench/survey/repos.txt` | `survey.md`, `survey/` |
| `bench/kotlinpoet.sh` | kotlinpoet's Gradle build and tests with `kc`'s library | `tests-kotlinpoet.md` |

```sh
bench/survey.sh       # about 3 s after the first clones
bench/kotlinpoet.sh   # several minutes: two Gradle builds of kotlinpoet
```

The scripts clone each library at its pinned release tag into
`$TMPDIR/kc-survey`, and need the Kotlin standard library
(`KOTLIN_HOME`) and a JDK (`JAVA_HOME`).

## The survey

`bench/survey` lists, for each file, every Kotlin feature `kc` doesn't
support. A file is *blocked* by each of them.

- **The detector** reads `kc`'s tokens, so constructs such as `x is T`,
  `@Annotation` or `'c'` are recognized, not misparsed. Calls of the
  standard library's inline-only functions count only if `kc` can't
  inline them.
- **The parser** parses each file again, skipping to the next
  declaration after each error. Every error names a feature, which
  blocks the file even where the detector missed it; an error that names
  none is a *parser gap*, so a file the parser rejects is never
  unblocked.
- **The checker** runs on each library's files together, with only the
  standard library and `kotlin-reflect` on the class path, in a child
  process. It finds features that syntax doesn't show, and a panic is a
  finding, not the end of the run.

An unblocked file isn't guaranteed to compile: without the libraries'
dependencies the checker can't resolve many names. Only building a
library with its dependencies, as below, shows that.

### Results

`bench/survey.sh` surveys the library code that each project's JVM
compilation includes: for a multiplatform project, `commonMain`,
`jvmMain` and the source sets between them, such as okio's `nonJsMain`.
Tests are counted apart.

| Library | Tag | Files | Lines | Unblocked | Test files | Unblocked |
|:---|:---|---:|---:|---:|---:|---:|
| kotlinpoet | 2.4.0 | 39 | 8,696 | 39 | 37 | 37 |
| okio | 3.18.2 | 96 | 17,346 | 95 | 99 | 99 |
| kotlinx.serialization | 1.11.0 | 99 | 16,291 | 98 | 212 | 212 |
| kotlinx.coroutines | 1.11.0 | 169 | 34,248 | 169 | 456 | 455 |
| kotlinx-datetime | 0.8.0 | 69 | 13,870 | 69 | 59 | 59 |
| Exposed | 1.5.1 | 169 | 32,485 | 168 | 124 | 124 |
| Ktor (`ktor-io`, `ktor-utils`, `ktor-http`) | 3.6.0 | 236 | 23,774 | 236 | 78 | 78 |
| Arrow | 2.2.3 | 107 | 26,909 | 106 | 191 | 191 |
| MockK | 1.14.11 | 144 | 17,302 | 143 | 117 | 117 |
| **All** | | 1,128 | 190,921 | 1,123 (99%) | 1,373 | 1,372 |

The 5 library files left are blocked by one feature each:

| Feature | Files | Libraries |
|:---|---:|:---|
| member extension properties | 2 | kotlinx.serialization, Arrow |
| safe-call assignments (`a?.b = c`) | 2 | Exposed, MockK |
| a labeled expression | 1 | okio |

The test file left is blocked by a safe-call assignment. The parser
rejects no file, and the checker finds no unsupported feature the
detector misses. Each library's report, with every file's blockers, is in
[bench/results/survey/](../bench/results/survey/).

Between them the nine libraries cover a wide spread of features:
coroutines and `expect`/`actual` hierarchies, DSLs with infix functions,
operators and delegation, context parameters, reflection, sealed
hierarchies and Java interop.

The survey says which features code uses, not whether `kc` compiles
them right or rejects what `kotlinc` rejects.
[Conformance](conformance.md) measures that with `kotlinc`'s own box and
diagnostics tests. Nor does it say which library is closest to building:
ktor's first compile with its dependencies had 90 errors in 29 files the
survey counted as unblocked.

## kotlinpoet end to end

kotlinpoet's JVM library is 39 files, 8,696 lines. `bench/kotlinpoet.sh`
runs kotlinpoet's Gradle build twice: as is, and with `kc` compiling
the `:kotlinpoet` JVM library (`bench/kotlinpoet/kc.init.gradle.kts`
replaces `compileKotlinJvm`'s action). It writes
[bench/results/tests-kotlinpoet.md](../bench/results/tests-kotlinpoet.md).

**As a library.** In the second build `kotlinc` compiles kotlinpoet's
tests, with `allWarningsAsErrors`, and the `interop:kotlin-metadata`,
`interop:ksp` and `interop:ksp:test-processor` modules against `kc`'s
classes, reading their metadata and inlining their inline functions,
such as `buildCodeBlock` and `typeNameOf<T>()`; the tests use internal
declarations through the build's friend paths; and the
binary-compatibility validator checks `kc`'s API against kotlinpoet's
`kotlinpoet/api/kotlinpoet.api`. Every task succeeds, and every test has
the control build's result:

| Tests | Passed | Failed | Skipped |
|:---|---:|---:|---:|
| `:kotlinpoet` (JVM) | 966 | 0 | 40 |
| `:interop:kotlin-metadata` | 131 | 0 | 27 |
| `:interop:ksp:test-processor` | 19 | 0 | 0 |

**Binary compatibility alone.** The script then runs the control
build's JVM tests, compiled by `kotlinc` against `kotlinc`'s build of the
library, with JUnit 4 and `java -Xverify:all`: on `kotlinc`'s classes,
and then with `kc`'s in their place. This needs no metadata from `kc`,
so when the second build fails it tells the class files apart from the
metadata. On both, 957 pass and 6 are skipped. Among them are the tests
that run `javac` through compile-testing to convert Java elements and
type mirrors.

The compile times are in [Benchmark](benchmark.md#results).

## ktor end to end

`kc` compiles ktor 3.6.0's `ktor-io`, `ktor-utils` and `ktor-http` (236
files, 23,774 lines) one module at a time, each against `kc`'s builds
of the ones before, with ktor's options (`-jvm-default=enable`,
kotlinx.serialization). All three compile without errors, and their 671
classes load under `-Xverify:all`.

- **API.** `bench/apidump` finds it the same as ktor's `api/*.api` files
  and `kotlinc`'s builds. The one difference comes from ktor's
  `-language-version 2.3`: `kotlinc` 2.4 writes covariant overrides'
  bridges in interfaces too, as `kc` does (`ParametersBuilder.build()`).
  `kc` has no language versions.
- **Metadata.** `bench/metadump` finds it the same as `kotlinc`'s, but
  for the differences in [Unsupported Kotlin](unsupported.md#differences-from-kotlinc).
- **Tests.** ktor's own JVM tests (`common` and `jvm`, and `jvmAndPosix`
  for `ktor-io`: 687 tests), compiled by `kotlinc` against either build
  and run with the JUnit 5 console under `-Xverify:all`, give each test
  the same result on `kc`'s classes as on `kotlinc`'s. Two of
  `ktor-io`'s fail on both (`WriterReaderTest`'s `testReaderOnCancelled`
  and `testWriterOnCancelled`), and 9 are skipped on both.

`bench/ktor-time.sh` times the build ([Benchmark](benchmark.md#results));
the API, metadata and test comparisons aren't scripted in `bench/`.
