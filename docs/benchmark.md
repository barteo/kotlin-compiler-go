# Benchmark

`kc` against `kotlinc` 2.4.21, on a generated project and three real
libraries. Earlier runs are in this file's git history.

## Running

| Script | Input | Writes to `bench/results/` |
|---|---|---|
| `bench/run.sh` | generated project | `summary-<sizes>.md` |
| `bench/kotlinpoet-time.sh` | kotlinpoet 2.4.0, JVM | `time-kotlinpoet.md` |
| `bench/coroutines-time.sh` | kotlinx-coroutines-core 1.11.0, JVM | `time-coroutines.md` |
| `bench/ktor-time.sh` | Ktor 3.6.0: `ktor-io`, `ktor-utils`, `ktor-http` | `time-ktor.md` |
| `bench/profile.sh [N]` | generated project, profiling `kotlinc` | `phases-N.md` |
| `bench/runtime.sh` | `bench/runtime/Bench.kt`, run after compiling | `runtime.md` |
| `bench/ring.sh` | JetBrains' ring micro-benchmarks, run under JMH | `ring.md` |

```sh
bench/run.sh                                                  # 200 files, about 1 minute
COOLDOWN=30 SIZES="100 1000" RUNS=1 WARM_RUNS=4 bench/run.sh  # the results below
COOLDOWN=10 RUNS=5 WARM_RUNS=20 bench/kotlinpoet-time.sh      # also coroutines-time.sh, ktor-time.sh
ITERS=10 BACKEND_THREADS=1 bench/profile.sh 500               # about 12 minutes
bench/runtime.sh                                              # speed of the generated code, about 30 s
bench/ring.sh                                                 # the same, JetBrains' benchmarks, about 15 minutes
```

Each script times four commands with hyperfine:

- `kc -j 1` and `kc -j <all CPUs>`;
- cold `kotlinc`, a new JVM per run (per module for Ktor);
- warm `kotlinc`: `bench/warm/WarmKotlinc.java` compiles repeatedly in
  one JVM, as the Gradle daemon would, and reports the median of the
  later runs.

The settings are `SIZES` (files per generated project), `RUNS` (cold
`kotlinc` runs), `WARM_RUNS` (runs in the warm JVM), `HEAP` (`kotlinc`'s
heap, 8 GB by default; 4 GB is too small at 1,000 files) and `COOLDOWN`
(seconds idle before every compilation, outside the measured time, for
laptops that throttle).

`bench/gen` writes N files of about 400 lines in 10 packages, using most
of the supported language: cross-file and JDK calls, lambdas, the
standard library and its inline functions, class hierarchies, sealed,
data and enum classes, arrays, extensions and operators, and
`sequence { }` coroutines. Both compilers' programs must print the same
checksum.

The library scripts use the clones that `bench/survey.sh` makes, and
pass the builds' options (module name, opt-ins, `-jvm-default`).
`kotlinc` compiles the source sets as multiplatform fragments, as Gradle
does. Gradle itself, incremental compilation and tests are left out.

## Results

Apple M2 MacBook Air (4 performance and 4 efficiency cores, 16 GB),
JDK 21, at `a38d4ce` on 30 October 2026.

Wall time:

| Input | Files | Lines | `kc -j 1` | `kc -j 8` | `kotlinc` cold | `kotlinc` warm | Warm `kotlinc` / `kc -j 1` |
|---|---:|---:|---:|---:|---:|---:|---:|
| Generated | 100 | 40k | 0.61 s | 0.27 s | 17.7 s | 7.9 s | 13x |
| Generated | 1,000 | 404k | 5.54 s | 2.49 s | 78.5 s | 107.6 s | 19x |
| kotlinpoet | 39 | 8.7k | 143 ms | 97 ms | 6.9 s | 1.23 s | 8.6x |
| kotlinx-coroutines-core | 169 | 34k | 285 ms | 159 ms | 10.9 s | 2.50 s | 8.8x |
| Ktor, 3 modules | 236 | 24k | 350 ms | 226 ms | 19.2 s | 2.32 s | 6.6x |

Peak memory (resident set size):

| Input | Files | Lines | `kc -j 1` | `kc -j 8` | `kotlinc` cold | Cold `kotlinc` / `kc -j 1` |
|---|---:|---:|---:|---:|---:|---:|
| Generated | 100 | 40k | 163 MB | 219 MB | 1.71 GB | 10x |
| Generated | 1,000 | 404k | 1.24 GB | 1.64 GB | 4.52 GB | 3.7x |
| kotlinpoet | 39 | 8.7k | 76 MB | 85 MB | 988 MB | 13x |
| kotlinx-coroutines-core | 169 | 34k | 107 MB | 118 MB | 1.15 GB | 11x |
| Ktor, 3 modules | 236 | 24k | 74 MB | 85 MB | 1.04 GB | 14x |

- `kotlinc` runs with an 8 GB heap limit (`HEAP`). Warm `kotlinc`'s
  memory isn't measured.
- Warm `kotlinc` doesn't settle at 1,000 files on this 16 GB machine:
  its four runs took 90–108 s, slower than cold.
- `kotlinc`'s times vary by 10–20% between sessions although it doesn't
  change. To measure a change to `kc`, time both builds in one session.
- Full tables, with CPU time, are in [bench/results](../bench/results/).
  Whether `kc`'s library builds are correct (API, `-Xverify:all`, the
  libraries' own tests) is checked in [Survey of real code](survey.md).

## Speed of the generated code

The other scripts time compiling. `bench/runtime.sh` times the programs
instead: it compiles `bench/runtime/Bench.kt` with both compilers and runs
each build in `FORKS` JVMs (5 by default, alternating between the
builds), each running every workload for `WARMUP` untimed and `ROUNDS`
timed rounds (10 and 20). A workload's time is the median over the JVMs
of each JVM's median round, and both builds must compute the same
checksums. The workloads are loops over ranges, the standard library's
inline functions over an `IntArray`, collection pipelines, data-class and
string map keys, string building, interface calls and a `when` over a
sealed type, a `when` over an enum, function values with a captured
`var`, and `sequence { }`.

On a 4-CPU Linux x86_64 machine with JDK 21
([runtime.md](../bench/results/runtime.md)), `kc`'s code runs at about
the speed of `kotlinc`'s on most workloads (0.7–1.03x its time) but takes
4.9x as long on the inline functions over an `IntArray`: `kc`'s inlined
lambdas take and return boxed values
([Differences from kotlinc](unsupported.md#differences-from-kotlinc)),
which the JIT doesn't remove. The geometric mean is 1.12x.

`bench/ring.sh` runs a suite that isn't ours: the "ring" micro-benchmarks
of Kotlin's own performance suite (`kotlin-native/performance/ring` in
JetBrains/kotlin), 200 benchmarks of calls, lambdas, loops, collections,
strings, `when` and small programs (DeltaBlue, Richards). They are
written for Kotlin/Native and kotlinx-benchmark, whose annotations on the
JVM are JMH's. The script leaves out the 7 files that need
Kotlin/Native, compiles the other 45 with both compilers, opens the
`@State` classes as kotlinx-benchmark's all-open plugin would, generates
JMH's harness from each build's class files, and runs both under JMH
(`FILTER` picks benchmarks by regexp; `FORKS`, `WARMUP`, `ITERS` and
`TIME` set the JMH runs).

On the same machine ([ring.md](../bench/results/ring.md)), with one fork
of 5 warmup and 5 measured iterations of 200 ms, `kc`'s code is within
10% of `kotlinc`'s on 149 of the 200, more than 10% slower on 39 and more
than 10% faster on 12; the geometric mean is 1.24x. The large ratios come
from a few things `kc` writes differently:

- Values pushed before an inlined call (a receiver, earlier arguments)
  stay on the operand stack while the inlined body runs. HotSpot can't
  compile a loop on the fly (OSR) while the stack isn't empty ("OSR
  starts with non-empty stack" in `-XX:+PrintCompilation`), so a method
  that runs one long inlined loop, like `bh.consume(bench(arr) { ... })`,
  stays interpreted. `kotlinc` spills the stack to locals before inlining
  (`GenericArrayView` 36–161x, `Iterator` 19–21x).
- `for (i in array.indices)` builds an `IntRange` and calls the boxing
  `IntIterator.next()`, where `kotlinc` writes a counted loop
  (`ForLoops.intArrayIndicesLoop` 8.9x, `uIntArrayIndicesLoop` 6.5x).
- Inlined lambdas box their arguments (`IntArray.countFilteredSome`
  6.6x, `ClassList.countWithLambda` 12x, `Euler`).
- A `when` over an enum calls `equals` rather than switching on the
  ordinal (`Switch.testEnumsSwitch` 5.8x). On the other hand, `kc`'s
  `when` over `Int` constants is 2–4x faster than `kotlinc`'s here
  (`testDenseIntSwitch`, `testConstSwitch`).

## Where kotlinc's time goes

A profile (`bench/profile.sh`, in
[phases-500.md](../bench/results/phases-500.md)): 500 generated
files of 202k lines, warm, on an 8-CPU machine, both compilers on one
thread.

| Work | `kotlinc` | `kc -j 1` | Ratio |
|:---|---:|---:|---:|
| Parsing | 4.7 s | 0.27 s | ~18x |
| Resolution and type checking | 8.8 s | 0.92 s | ~10x |
| Checkers (warnings and errors) | 5.4 s | none | |
| Reading the class path | 0.4 s | 0.01 s | |
| Backend, with inlining and writing | 19.1 s | 1.32 s | ~14x |
| **Total** | **38.4 s** | **2.51 s** | **15x** |

- There is no hotspot: no single lowering or checker takes more than
  about 2%. The time is spread over 88 IR lowerings and 290 checkers,
  each walking the tree.
- `kotlinc`'s frontend runs on one core. With `-Xbackend-threads=0` it
  took 22.4 s, against 40.7 s for the profiled runs; part of that
  difference is the profiling itself. `kc` checks and generates code
  in parallel.
- Of `kc`'s backend, 0.56 s is writing 10,001 class files.

## What the numbers don't show

They are an upper bound, not a forecast for a Go port of `kotlinc`:

- `kc` compiles a subset of Kotlin ([Unsupported Kotlin](unsupported.md))
  with simpler rules than K2's, and has no separate checkers
  ([Design](design.md)).
- Its inliner and backend are a single simple pass, without an SMAP.
- Real builds run `kotlinc` in the Gradle daemon, incrementally.
