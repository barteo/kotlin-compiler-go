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

```sh
bench/run.sh                                                  # 200 files, about 1 minute
COOLDOWN=0 SIZES=500 RUNS=1 WARM_RUNS=4 bench/run.sh          # 500 files, about 9 minutes
COOLDOWN=0 RUNS=3 WARM_RUNS=10 bench/kotlinpoet-time.sh       # also coroutines-time.sh, ktor-time.sh
ITERS=10 BACKEND_THREADS=1 bench/profile.sh 500               # about 12 minutes
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

A Linux x86_64 cloud machine (4 CPUs, 15 GB), JDK 21, hyperfine 2.0.0,
with the commands above, on 19 November 2026, with `kc` at `c81677a`.

Wall time:

| Input | Files | Lines | `kc -j 1` | `kc -j 4` | `kotlinc` cold | `kotlinc` warm | Warm `kotlinc` / `kc -j 1` |
|---|---:|---:|---:|---:|---:|---:|---:|
| Generated | 500 | 202k | 5.82 s | 4.46 s | 97.9 s | 89.1 s | 15.3x |
| kotlinpoet | 39 | 8.7k | 290 ms | 210 ms | 15.5 s | 3.03 s | 10.4x |
| kotlinx-coroutines-core | 169 | 34k | 565 ms | 436 ms | 23.8 s | 5.88 s | 10.4x |
| Ktor, 3 modules | 236 | 24k | 709 ms | 598 ms | 39.5 s | 5.78 s | 8.2x |

Peak memory (resident set size):

| Input | Files | Lines | `kc -j 1` | `kc -j 4` | `kotlinc` cold | Cold `kotlinc` / `kc -j 1` |
|---|---:|---:|---:|---:|---:|---:|
| Generated | 500 | 202k | 802 MB | 912 MB | 3.42 GB | 4.3x |
| kotlinpoet | 39 | 8.7k | 89 MB | 99 MB | 966 MB | 11x |
| kotlinx-coroutines-core | 169 | 34k | 131 MB | 144 MB | 1.23 GB | 9.4x |
| Ktor, 3 modules | 236 | 24k | 91 MB | 96 MB | 971 MB | 11x |

- `kotlinc` runs with an 8 GB heap limit (`HEAP`). Warm `kotlinc`'s
  memory isn't measured.
- Since `a009992`, `kc -j 1` took 10% less time on kotlinpoet and
  coroutines, and 2% less on the generated project and Ktor. Two changes
  made it, each timed against its parent in one session: a filter of the
  class path's names rules out absent classes without building their
  names (5–8% on kotlinpoet, coroutines and the generated project, 2% on
  Ktor), and the constant pool finds entries by the indexes they hold
  instead of by concatenated strings (2–4%). The generated project's
  `kc` time moved less here than in those one-session timings, within
  the variation between sessions.
- Against the run at `a009992`, warm `kotlinc` was 5% slower on
  coroutines, 1% slower on Ktor and 4% faster on kotlinpoet. At that
  run's times kotlinpoet would be 10.9x, coroutines 9.9x and Ktor 8.1x:
  coroutines is at about 10x, and Ktor is still below it.
- Warm `kotlinc` doesn't settle at 500 files on this machine: its four
  runs took 87–102 s, against a median of 77.5 s in the run before
  (13.3x at that time). The generated row is the least certain.
- Times vary by 10–20% between sessions, for both compilers, and more
  on a busy cloud machine. To measure a change to `kc`, time both builds
  in one session.
- `kc -j 1` checks and generates code on one goroutine. A few goroutines
  of its own write the class files, and the class path is indexed and
  the standard library's default imports read alongside parsing.
- Full tables, with CPU time, are in [bench/results](../bench/results/).
  Whether `kc`'s library builds are correct (API, `-Xverify:all`, the
  libraries' own tests) is checked in [Survey of real code](survey.md).

## Where kotlinc's time goes

A profile (`bench/profile.sh`, in
[phases-500.md](../bench/results/phases-500.md)): 500 generated
files of 202k lines, warm, on the 4-CPU machine above, both compilers
on one thread.

| Work | `kotlinc` | `kc -j 1` | Ratio |
|:---|---:|---:|---:|
| Parsing | 9.2 s | 0.72 s | ~13x |
| Resolution and type checking | 28.0 s | 3.53 s | ~8x |
| Checkers (warnings and errors) | 22.8 s | none | |
| Reading the class path | 1.7 s | 0.02 s | |
| Backend, with inlining and writing | 50.8 s | 2.32 s | ~22x |
| **Total** | **112.4 s** | **6.87 s** | **16x** |

- There is no hotspot: no single lowering or checker takes more than
  about 2%. The time is spread over 93 IR lowerings and 347 checkers,
  each walking the tree. `kc`'s is spread out too: each call it checks
  is resolved once, and each lambda checked once.
- `kotlinc`'s frontend runs on one core; only its backend can use more.
  With `-Xbackend-threads=0` (a backend thread per core) and without the
  profiler it took 54.8 s, against 112.2 s for the profiled runs. `kc`
  also checks in parallel.
- `kc` writes the 10,001 class files on goroutines of their own, which
  spent 3.3 s doing so, alongside code generation.

## What the numbers don't show

They are an upper bound, not a forecast for a Go port of `kotlinc`:

- `kc` compiles a subset of Kotlin ([Unsupported Kotlin](unsupported.md))
  with simpler rules than K2's, and has no separate checkers
  ([Design](design.md)).
- Its inliner and backend are a single simple pass, without an SMAP.
- Real builds run `kotlinc` in the Gradle daemon, incrementally.
