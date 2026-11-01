# kotlin-compiler-go: a Kotlin compiler in Go

`kc` is a Kotlin compiler for the JVM, written in Go. On real libraries
it compiles 10x faster or more than `kotlinc` running in a warm JVM, and uses
about a tenth of the memory. The libraries it builds pass their own test
suites ([Survey of real code](docs/survey.md)), and it passes 95% of
`kotlinc`'s box tests ([Conformance](docs/conformance.md)).

It is a prototype: it compiles most of Kotlin but not all of it, and it
checks less than `kotlinc` does ([What it doesn't do](#what-it-doesnt-do)).

## Results

`kc` against `kotlinc` 2.4.21 on an Apple M2 MacBook Air (8 cores, 16 GB),
JDK 21. `kotlinc` runs in a warm JVM, as it does in the Gradle daemon:
the median of repeated compilations in one JVM, after the JIT has warmed
up. `kc` uses all cores, as it does by default.

| Project | Files | Lines | `kotlinc` | `kc` | Speedup |
|:---|---:|---:|---:|---:|---:|
| kotlinpoet 2.4.0 | 39 | 8.7k | 1.23 s | 97 ms | **13x** |
| Ktor 3.6.0 (`ktor-io`, `ktor-utils`, `ktor-http`) | 236 | 24k | 2.32 s | 226 ms | **10x** |
| kotlinx-coroutines-core 1.11.0 | 169 | 34k | 2.50 s | 159 ms | **16x** |

- **On one core** (`kc -j 1`) `kc` is still 6.6–8.8x faster than warm
  `kotlinc`: 143 ms, 350 ms and 285 ms.
- **Cold**, with a new JVM per build as on the command line or in CI,
  `kotlinc` takes 6.9 s, 19.2 s and 10.9 s, 69–85x slower than `kc`.
- **Memory:** `kc` peaks at 85–118 MB, and cold `kotlinc`, with an 8 GB
  heap limit, at 1.0–1.1 GB.

How each number is measured, and the full tables with CPU time, are in
[Benchmark](docs/benchmark.md).

## What it doesn't do

`kc` doesn't do all the work `kotlinc` does. It leaves out:

- **Some of the language.** A few features aren't supported, and
  visibility isn't checked. [Unsupported Kotlin](docs/unsupported.md)
  lists them all.
- **Most of `kotlinc`'s checks.** `kotlinc` runs 290 checkers for
  warnings and errors after type checking. `kc` has no separate
  checkers: it reports the errors that type checking finds, which is why
  it agrees with `kotlinc` on only 79% of the diagnostics tests.
- **Anything but the JVM.** There is no Kotlin/Native, JS or Wasm, and no
  language versions.
- **Incremental compilation, a Gradle plugin and IDE support.**
- **Identical output.** The class files work the same, but differ in
  places. For example, `kc` shares the objects of inline functions where
  `kotlinc` copies them, so it writes 790 classes for
  kotlinx-coroutines-core and `kotlinc` writes 850.
  [Differences from kotlinc](docs/unsupported.md#differences-from-kotlinc)
  lists them.

The checkers don't explain the speedup. In a profile of both compilers
on one thread, `kc` is faster at each phase that both of them run:

| Work | `kotlinc` | `kc -j 1` | Ratio |
|:---|---:|---:|---:|
| Parsing | 4.7 s | 0.27 s | ~18x |
| Resolution and type checking | 8.8 s | 0.92 s | ~10x |
| Checkers (warnings and errors) | 5.4 s | none | |
| Reading the class path | 0.4 s | 0.01 s | |
| Backend, with inlining and writing | 19.1 s | 1.32 s | ~14x |
| **Total** | **38.4 s** | **2.51 s** | **15x** |

Even without its checkers, `kotlinc` would take 33 s, 13x as long as
`kc`. This profile is of 500 generated files (202k lines) in a warm JVM
([Where kotlinc's time goes](docs/benchmark.md#where-kotlincs-time-goes)).

## Why it's faster

- **No JVM.** `kc` starts in milliseconds; a cold `kotlinc` spends
  seconds starting the JVM and warming up the JIT.
- **Parallelism.** `kc` parses, checks bodies and generates class files
  on all cores; `kotlinc`'s frontend uses one.
- **Fewer passes.** `kotlinc`'s time is spread over 88 IR lowerings and
  290 checkers, none above about 2%. `kc` generates code in one pass
  over the checked AST.
- **Lazy inputs.** `kc` indexes the class path in about 5 ms while
  parsing, and decodes a class only when it's first used.

[Design](docs/design.md) describes the pipeline.

## Try it

Requires Go 1.27.2+ and a JDK. The tests and benchmarks also need
`kotlinc` and `hyperfine` (`brew install go openjdk@21 kotlin hyperfine`).

```sh
go build -o bin/kc ./cmd/kc
bin/kc -d out testdata/run/fizzBuzz.kt
java -cp out FizzBuzzKt
```

`kc` doesn't add the standard library by itself. Put `kotlin-stdlib.jar`,
and any other libraries the program uses, on the class path to compile
and run it. For example, with kotlinx.coroutines, which ships with
`kotlinc`:

```sh
L=$(brew --prefix kotlin)/libexec/lib
CP=$L/kotlin-stdlib.jar:$L/kotlinx-coroutines-core-jvm.jar
bin/kc -cp $CP -d out testdata/run/stdlibCoroutinesBasics.kt
java -cp out:$CP StdlibCoroutinesBasicsKt
```

Arguments can be files or directories. Flags:

| Flag | Meaning |
|---|---|
| `-d` | output directory (default `.`) |
| `-cp`, `-classpath` | jars and directories to compile against |
| `-jdk-home` | JDK whose classes to use (default: `$JAVA_HOME`, or the JDK of `java` on `PATH`) |
| `-no-jdk` | don't use JDK classes |
| `-module-name` | module name, as in `kotlinc` (default `main`) |
| `-friend-paths` | class path entries whose internal declarations are visible (`kotlinc`'s `-Xfriend-paths`) |
| `-parcelize`, `-serialization`, `-compose` | apply the parcelize, kotlinx.serialization and Compose plugins |
| `-jvm-default` | how interface bodies compile, as in `kotlinc` (default `no-compatibility`) |
| `-Xdebug` | keep dead variables in suspend functions for debuggers, as in `kotlinc` |
| `-j` | parallel workers (default: all CPUs) |
| `-time` | print per-phase timings |
| `-features` | print each error with its unsupported feature, tab-separated |

There is no Gradle plugin. To try `kc` in a Gradle build, a Gradle init
script can replace a module's Kotlin compile task with a call to `kc`.
[bench/kotlinpoet/kc.init.gradle.kts](bench/kotlinpoet/kc.init.gradle.kts)
does this for kotlinpoet.

### Repeat the benchmark

```sh
git clone --depth 1 --branch 1.11.0 https://github.com/Kotlin/kotlinx.coroutines.git \
  "${TMPDIR:-/tmp}/kc-survey/kotlinx-coroutines-1.11.0"
RUNS=5 WARM_RUNS=20 bench/coroutines-time.sh
```

`bench/coroutines-time.sh` builds `kc`, downloads the library's
dependencies from Maven Central, compiles it with both compilers using
its Gradle build's options, and writes the table to
[bench/results/time-coroutines.md](bench/results/time-coroutines.md).
It expects `kotlinc` and the JDK where Homebrew installs them; set
`KOTLIN_HOME` and `JAVA_HOME` otherwise. `bench/kotlinpoet-time.sh` and
`bench/ktor-time.sh` time the other two libraries
([Benchmark](docs/benchmark.md#running)).
## Documentation

- [Unsupported Kotlin](docs/unsupported.md): what `kc` doesn't support, and
  where its output differs from `kotlinc`'s
- [Design](docs/design.md): the pipeline, type checking, class file
  generation, the inliner, coroutines and compiler plugins
- [Benchmark](docs/benchmark.md): how `kc` is measured against `kotlinc`,
  and the results
- [Survey of real code](docs/survey.md): which features real libraries use,
  and libraries built with `kc` and tested end to end
- [Conformance](docs/conformance.md): `kotlinc`'s own box and diagnostics
  tests run against `kc`

## Tests

```sh
go test ./...
```

Tests use the JDK in `$JAVA_HOME`, or the JDK of `java` on `PATH`. On macOS
`/usr/bin/java` is only a launcher, so set `JAVA_HOME`, for example
`export JAVA_HOME=$(/usr/libexec/java_home -v 21)`. Without a JDK, tests fail
with unresolved references to Java classes such as `HashMap`.

Tests that need the standard library find `kotlin-stdlib.jar` in
`$KOTLIN_HOME/lib` or next to `kotlinc` on `PATH`, and are skipped without
it. Tests that compare with `kotlinc` are skipped if it isn't on `PATH`. The
kotlinx.serialization 1.9.0 and Compose runtime 1.9.2 tests use jars from
Gradle's cache, or from `KC_SERIALIZATION_CP` and `KC_COMPOSE_CP`, and are
skipped without them.

- `testdata/run/`: programs compiled with `kc` and run under
  `java -Xverify:all`. Output must match `*.out`, which
  `testdata/gen_expected.sh` generates with `kotlinc`. A directory is a
  multi-file program with `main` in `main.kt`.
- `testdata/lib/`: a library compiled by `kc` and a program compiled
  against it by `kc` and by `kotlinc`. Both must produce `X.out`.
- `testdata/errors/`: each `// ERROR <text>` comment marks a line that must
  produce a diagnostic containing that text.
- `TestCoroutineShapes`: compares `kc`'s and `kotlinc`'s coroutine classes;
  differences must be listed in `testdata/coroutine-shapes.golden`.
- Package tests in `internal/` cover the parser, Kotlin metadata, class
  file frames and methods, the inliner and class path loading.

### CI

[`.github/workflows/ci.yml`](.github/workflows/ci.yml) runs on each pull
request and push to `main`: `gofmt`, `go vet`, the tests on Linux and
macOS, and the tests under `-race` on Linux. To run the tests the way CI
does, with nothing skipped for lack of `kotlinc` or a library:

```sh
eval "$(.github/ci/deps.sh)"
.github/ci/test.sh
```

`deps.sh` downloads `kotlinc` and the library jars to `~/.cache/kc-ci` and
prints the environment the tests read. `test.sh` takes `go test` flags,
such as `-race`, and fails if a test is skipped for a reason other than
the expected ones it lists.

## License

Apache License 2.0, the same as `kotlinc`. See [LICENSE](LICENSE).
