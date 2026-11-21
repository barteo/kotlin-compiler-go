# kotlin-compiler-go: a Kotlin compiler in Go

`kc` is a Kotlin compiler for the JVM, written in Go. On real libraries
it compiles 10x faster or more than `kotlinc` running in a warm JVM, and uses
about a tenth of the memory. The libraries it builds pass their own test
suites ([Survey of real code](docs/survey.md)). It passes all 6,575
of `kotlinc`'s box tests that it runs, and rejects what `kotlinc` rejects
in 99% of its diagnostics tests ([Conformance](docs/conformance.md)).

It is a prototype: it compiles most of Kotlin but not all of it, and it
checks less than `kotlinc` does ([What it doesn't do](#what-it-doesnt-do)).

## Results

`kc` against `kotlinc` 2.4.21 on a Linux x86_64 cloud machine (4 CPUs,
15 GB), JDK 21. `kotlinc` runs in a warm JVM, as it does in the Gradle daemon:
the median of repeated compilations in one JVM, after the JIT has warmed
up. `kc` uses all cores, as it does by default.

| Project | Files | Lines | `kotlinc` | `kc` | Speedup |
|:---|---:|---:|---:|---:|---:|
| kotlinpoet 2.4.0 | 39 | 8.7k | 3.44 s | 211 ms | **16x** |
| Ktor 3.6.0 (`ktor-io`, `ktor-utils`, `ktor-http`) | 236 | 24k | 6.96 s | 698 ms | **10x** |
| kotlinx-coroutines-core 1.11.0 | 169 | 34k | 6.88 s | 643 ms | **11x** |

- **On one core** (`kc -j 1`) `kc` is still 8.2–11x faster than warm
  `kotlinc`: 300 ms, 849 ms and 796 ms.
- **Cold**, with a new JVM per build as on the command line or in CI,
  `kotlinc` takes 17.8 s, 45.7 s and 28.5 s, 44–84x slower than `kc`.
- **Memory:** `kc` peaks at 94–140 MB, and cold `kotlinc`, with an 8 GB
  heap limit, at 0.94–1.04 GB.

How each number is measured, and the full tables with CPU time, are in
[Benchmark](docs/benchmark.md).

## What it doesn't do

`kc` doesn't do all the work `kotlinc` does. It leaves out:

- **Some of the language.** A few features aren't supported.
  [Unsupported Kotlin](docs/unsupported.md) lists them all.
- **Most of `kotlinc`'s checks.** `kotlinc` runs 289 checkers for
  warnings and errors after type checking. `kc` reports every syntax
  error, the errors that type checking finds, and only a few of those
  checkers' errors (opt-in and deprecation, for example), which is why it
  agrees with `kotlinc` on only 99% of the diagnostics tests. Of the
  warnings, it reports about ninety kinds, such as deprecations, opt-ins,
  useless and unchecked casts, unreachable code and unnecessary safe
  calls: 84% of those the diagnostics tests expect.
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
| Parsing | 4.0 s | 0.65 s | ~6x |
| Resolution and type checking | 26.8 s | 3.06 s | ~9x |
| Checkers (warnings and errors) | 17.9 s | none | |
| Reading the class path | 1.4 s | 0.03 s | |
| Backend, with inlining and writing | 34.7 s | 3.39 s | ~10x |
| **Total** | **84.8 s** | **7.13 s** | **12x** |

Even without its checkers, `kotlinc` would take 67 s, 9x as long as
`kc`. This profile is of 500 generated files (202k lines) in a warm JVM
([Where kotlinc's time goes](docs/benchmark.md#where-kotlincs-time-goes)).

## Why it's faster

- **No JVM.** `kc` starts in milliseconds; a cold `kotlinc` spends
  seconds starting the JVM and warming up the JIT.
- **Parallelism.** `kc` parses, checks bodies and generates class files
  on all cores; `kotlinc`'s frontend uses one.
- **Fewer passes.** `kotlinc`'s time is spread over 93 IR lowerings and
  330 checkers, none above about 2%. `kc` generates code in one pass
  over the checked AST.
- **Lazy inputs.** `kc` indexes the class path in about 30 ms while
  parsing, and decodes a class only when it's first used.

[Design](docs/design.md) describes the pipeline.

## Try it

Requires Go 1.27.2+ and a JDK. The tests and benchmarks also need
`kotlinc` and `hyperfine`. On macOS: `brew install go openjdk@21 kotlin
hyperfine`. On Linux, get Go from [go.dev/dl](https://go.dev/dl/) and the
rest from your package manager (for example `apt install openjdk-21-jdk
hyperfine`, plus `kotlinc` from the
[Kotlin releases](https://github.com/JetBrains/kotlin/releases)).

```sh
git clone https://github.com/barteo/kotlin-compiler-go && cd kotlin-compiler-go
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
| `-jdk-home` | JDK whose classes to use, after the class path's (default: `$JAVA_HOME`, or the JDK of `java` on `PATH`) |
| `-no-jdk` | don't use JDK classes |
| `-module-name` | module name, as in `kotlinc` (default `main`) |
| `-friend-paths` | class path entries whose internal declarations are visible (`kotlinc`'s `-Xfriend-paths`) |
| `-opt-in` | opt in to a requirement marker by qualified name, as `kotlinc`'s `-opt-in`; may be repeated |
| `-parcelize`, `-serialization`, `-compose`, `-metro` | apply the parcelize, kotlinx.serialization, Compose and Metro plugins |
| `-P` | a Metro option, as `kotlinc`'s `-P plugin:dev.zacsweers.metro.compiler:key=value`; may be repeated |
| `-jvm-default` | how interface bodies compile, as in `kotlinc` (default `no-compatibility`) |
| `-jvm-target` | the JVM version the class files are for, as in `kotlinc`: `1.8` (the default), or `9` to `25`; a library's inline functions built for a higher one are errors |
| `-Xexplicit-api` | explicit API mode, as in `kotlinc`: `strict` (errors) or `warning` where a public API declaration doesn't say its visibility or its inferred type; `-XXexplicit-return-types` requires only the types |
| `-Xmulti-platform` | compile a multiplatform module's sources (`expect` and `actual` declarations, errors without it), as in `kotlinc` |
| `-Xdebug` | keep dead variables in suspend functions for debuggers, as in `kotlinc` |
| `-j` | parallel workers (default: all CPUs) |
| `-time` | print per-phase timings |
| `-features` | print each diagnostic with its severity, name and unsupported feature, tab-separated |
| `-Wextra` | also report the warnings of `kotlinc`'s extra checkers (`UNREACHABLE_CODE`), as in `kotlinc` |
| `-Xuse-fir-experimental-checkers` | also report the warnings of `kotlinc`'s experimental checkers (`REDUNDANT_INTERPOLATION_PREFIX`), as in `kotlinc` |
| `-Werror` | fail if there are warnings, as in `kotlinc` |
| `-nowarn` | don't print warnings (and don't fail with `-Werror`), as in `kotlinc` |

`kc -serve` compiles again and again for a tool that runs many small
compiles: each line of stdin is one compile's arguments as a JSON array,
answered on stdout with its exit code and output. It keeps the JDK's
index and the classes it read from jars for the compiles after, which
is most of a small compile's work. The conformance tests use it
([Conformance](docs/conformance.md#long-lived-kc)).

### Use it in a Gradle build

There is no Gradle plugin. [tools/gradle/use-kc.init.gradle.kts](tools/gradle/use-kc.init.gradle.kts) is an init script that makes every Kotlin/JVM compile task of a build run `kc` instead of `kotlinc`: `compileKotlin`, Android's `compile<Variant>Kotlin`, and a multiplatform module's `compileAndroidMain` and `compileKotlinJvm`. `kc` gets the task's own sources, Java sources, class path, friend paths, module name, `-jvm-default` mode, JVM target, explicit API mode, `-Xmulti-platform` for a multiplatform module and Compose, kotlinx.serialization, Parcelize and Metro plugins, with Metro's options. A module compiles against its class path and then the JDK of its Java toolchain, so an Android module's `android.jar` stands in for the JDK's classes. Dexing, R8, javac and packaging then use `kc`'s classes. It is meant for fast local builds; CI and release builds should keep `kotlinc`.

```sh
KC=$PWD/bin/kc ./gradlew assembleDebug --init-script /path/to/kotlin-compiler-go/tools/gradle/use-kc.init.gradle.kts
```

If `kc` fails on a module, `kotlinc` compiles that module and the build log says so. A module that uses a compiler plugin `kc` doesn't apply, such as one that generates code the program needs, is also left to `kotlinc`, with a warning that names the plugin. `kc` compiles whole modules, so the tasks are not incremental. A new `kc` binary recompiles every module, and a build without the init script recompiles with `kotlinc`.

| Variable | Meaning |
|---|---|
| `KC` | the `kc` binary (default: `kc` on `PATH`) |
| `KC_STRICT` | `1`: a `kc` failure or an unsupported compiler plugin fails the build instead of falling back to `kotlinc` |
| `KC_SKIP` | comma-separated task paths to leave to `kotlinc`, such as `:app:compileDebugKotlin` |
| `KC_VARIANTS` | comma-separated Android build types or variants, such as `debug` or `freeDebug`, to compile with `kc`; every other variant keeps `kotlinc` |
| `KC_KSP` | the directory of kc's KSP backend, which also runs the KSP tasks on kc (below) |
| `KC_IGNORE_PLUGINS` | comma-separated compiler plugins, as `artifact` or `group:artifact`, that `kc` may leave out because the program works without them (default: `compose-stability-compiler`) |
| `KC_JDK_HOME` | the JDK to compile against, in place of the task's Java toolchain |

`KC_VARIANTS` keeps `kc` out of the variants it doesn't list, so the init script can stay on for every build of a project while a release build is compiled by `kotlinc` alone:

```sh
KC_VARIANTS=debug KC=$PWD/bin/kc ./gradlew assembleDebug assembleRelease --init-script /path/to/kotlin-compiler-go/tools/gradle/use-kc.init.gradle.kts
```

An entry is a build type, which covers each variant of that type (`debug` covers `freeDebug` and `paidDebug`), or one variant's name. A listed variant's unit and instrumented tests go with it. The compile task of any other variant is left as it is: it gets neither the `kc` step nor the binary as an input, so it stays up to date after a `kotlinc` build and never reuses classes `kc` wrote. A module without variants, such as a Kotlin/JVM library, is part of every variant that depends on it, so `kc` compiles it only in a build that runs tasks of listed variants and of no other, as `assembleDebug` does; in the build above, and in one that runs no variant's tasks at all, `kotlinc` compiles it. When that module was last compiled by `kc`, Gradle runs its compile task again with `kotlinc`. A task of an Android module whose variant the script cannot tell keeps `kotlinc`. An empty `KC_VARIANTS` lists nothing, so `kotlinc` compiles everything.

With `KC_KSP`, the KSP tasks (`kspKotlin`, `ksp<Variant>Kotlin`) of the modules and variants `kc` compiles run KSP on [kc's KSP backend](docs/analysis-api.md), in KSP's own Gradle plugin. `jvm/kc-aa/build.sh` writes the directory:

```sh
jvm/kc-aa/build.sh
KC_KSP=$PWD/jvm/kc-aa/build/kc-ksp KC_VARIANTS=debug KC=$PWD/bin/kc ./gradlew assembleDebug --init-script /path/to/kotlin-compiler-go/tools/gradle/use-kc.init.gradle.kts
```

Such a task keeps its processors, options and output directories. Its KSP class path becomes the directory's loader and KSP API jars and, from its own, KSP's configuration classes, the Kotlin standard library and kotlinx.coroutines. Those are all a processor sees of KSP, as with stock KSP: the libraries kc's backend runs on, kotlin-compiler's bundled Guava among them, are in a class loader of their own, so a processor uses the Guava or kotlinx.serialization it brings. `KC_VARIANTS` and `KC_SKIP` choose KSP tasks as they choose compile tasks, so the KSP task of a variant that isn't listed stays stock KSP's and never loads kc. If the run on kc fails, stock KSP processes the module in the same task and the build log says so; with `KC_STRICT=1` the task fails. An error a processor reports, or an exception it throws, fails the run on kc too, and stock KSP still runs, because a wrong answer from kc can be what the processor complains about. Such errors are reported once: the run on kc holds them back, so when stock KSP fails as well the build log has stock KSP's messages only, and when stock KSP passes, the task passes and one line of the log says that the processors reported errors on kc alone, a bug in kc worth reporting, followed by those errors. With `KC_STRICT=1` the errors on kc are printed as they are reported. A build that changes which of the two runs a task runs the task again.

KSP's Gradle plugin loads the processors' classes anew for every task of every build, so each run starts on code the JVM has not compiled yet. With `KC_KSP_KEEP=1`, KSP on kc keeps them loaded for as long as the processors' jars stay the same, and the tasks and builds of a daemon share them. A processor then keeps its static state from one run to the next.

Between builds, KSP on kc keeps the parsed files, the models and the class paths of the modules whose KSP tasks ran last, so that the build after an edit parses and checks only what changed. In a large project that is gigabytes of the daemon's memory, for a second or less of such a build. With `KC_KSP_FORGET=1` a run keeps nothing for the next. Either way, the memory a run's checks leave free goes back to the system two seconds after the last KSP task of a build ends.

The daemon loads the directory's `libkc.so` once and keeps it, so every build in a daemon has to name the same directory and the same file: after building kc again, restart the daemon. It is checked with the KSP Gradle plugin 2.3.12 and 2.3.13 on Linux; `jvm/kc-aa/gradle.sh` runs Moshi's, Dagger's and Room's processors this way and compares their output with stock KSP's.

[bench/kotlinpoet/kc.init.gradle.kts](bench/kotlinpoet/kc.init.gradle.kts) is a narrower version that the kotlinpoet benchmark uses.

### Repeat the benchmark

```sh
git clone --depth 1 --branch 1.11.0 https://github.com/Kotlin/kotlinx.coroutines.git \
  "${TMPDIR:-/tmp}/kc-survey/kotlinx-coroutines-1.11.0"
COOLDOWN=0 RUNS=3 WARM_RUNS=10 bench/coroutines-time.sh
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
- [PSI and analysis](docs/psi.md): the public parser and analysis API for
  linters and other tools

## PSI API

Package [`psi`](psi/doc.go) is a public, lossless Kotlin parser whose trees
have the shape of the Kotlin compiler's PSI: the same element kinds, nesting,
whitespace and comment placement, and syntax error messages. Linters and
other tools use it without a JVM:

```go
s := psi.Parse("A.kt", text)
for _, d := range s.DeclarationNodes() {
    fmt.Println(d.Kind(), d.Name())
}
```

Its tests compare every tree with stock Kotlin's, from dumps made by
[`tools/psidump`](tools/psidump/README.md).

Package [`analysis`](analysis/analysis.go) runs `kc`'s checker on psi
snapshots and reports what it resolved, keyed by psi node: expression
types, symbols, calls, `when` coverage, smart casts, unreachable code and
more:

```go
r, err := analysis.AnalyzeArena(ctx, analysis.Config{ClassPath: cp}, snapshots)
if err != nil { ... }
if d := r.Diagnostics(); len(d) > 0 { ... } // errors; facts of what kc resolved, if any
f, ok := r.Fact(analysis.ElementID{Snapshot: s.ID(), Node: n.ID()})
```

See [docs/psi.md](docs/psi.md) for how both work.

## Tests

```sh
go test ./...
```

Tests use the JDK in `$JAVA_HOME`, or the JDK of `java` on `PATH`. On macOS
`/usr/bin/java` is only a launcher, so set `JAVA_HOME`, for example
`export JAVA_HOME=$(/usr/libexec/java_home -v 21)`. Without a JDK, tests fail
with unresolved references to Java classes such as `HashMap`.

Tests that need the standard library find `kotlin-stdlib.jar` in `$KOTLIN_HOME/lib` or next to `kotlinc` on `PATH`, and are skipped without it. Tests that compare with `kotlinc` are skipped if it isn't on `PATH`. The kotlinx.serialization 1.9.0, Compose runtime 1.9.2 and Metro 1.6.0 tests use jars from Gradle's cache, or from `KC_SERIALIZATION_CP`, `KC_COMPOSE_CP`, and `KC_METRO_CP` (Metro's runtime) with `KC_METRO_PLUGIN` (its compiler plugin), and are skipped without them.

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

## License

Apache License 2.0, the same as `kotlinc`. See [LICENSE](LICENSE). Package
`psi` translates parts of Kotlin's and IntelliJ's parser; see
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
