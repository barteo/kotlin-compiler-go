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
with the commands above, on 18 November 2026, with `kc` at `2daf0f3`.

Wall time:

| Input | Files | Lines | `kc -j 1` | `kc -j 4` | `kotlinc` cold | `kotlinc` warm | Warm `kotlinc` / `kc -j 1` |
|---|---:|---:|---:|---:|---:|---:|---:|
| Generated | 500 | 202k | 9.59 s | 5.21 s | 116.8 s | 79.5 s | 8.3x |
| kotlinpoet | 39 | 8.7k | 455 ms | 315 ms | 20.0 s | 3.82 s | 8.4x |
| kotlinx-coroutines-core | 169 | 34k | 876 ms | 603 ms | 27.5 s | 6.31 s | 7.2x |
| Ktor, 3 modules | 236 | 24k | 991 ms | 755 ms | 46.5 s | 6.11 s | 6.2x |

Peak memory (resident set size):

| Input | Files | Lines | `kc -j 1` | `kc -j 4` | `kotlinc` cold | Cold `kotlinc` / `kc -j 1` |
|---|---:|---:|---:|---:|---:|---:|
| Generated | 500 | 202k | 769 MB | 1.00 GB | 3.54 GB | 4.6x |
| kotlinpoet | 39 | 8.7k | 89 MB | 103 MB | 965 MB | 11x |
| kotlinx-coroutines-core | 169 | 34k | 128 MB | 139 MB | 1.05 GB | 8.2x |
| Ktor, 3 modules | 236 | 24k | 89 MB | 98 MB | 987 MB | 11x |

- `kotlinc` runs with an 8 GB heap limit (`HEAP`). Warm `kotlinc`'s
  memory isn't measured.
- Warm `kotlinc` doesn't settle at 500 files on this machine: its four
  runs took 78–114 s.
- Times vary by 10–20% between sessions, for both compilers. This
  session's machine was slower than the previous one's (15 November):
  `kc` at `e225d5b`, which measured those numbers, took 8.55 s on the
  generated project and 403 ms on kotlinpoet here, against 7.13 s and
  300 ms then. The current `kc` is 5–9% slower than `e225d5b` in the
  same session. To measure a change to `kc`, time both builds in one
  session.
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
| Parsing | 8.8 s | 0.76 s | ~12x |
| Resolution and type checking | 25.1 s | 3.98 s | ~6x |
| Checkers (warnings and errors) | 22.4 s | none | |
| Reading the class path | 1.6 s | 0.03 s | |
| Backend, with inlining and writing | 48.9 s | 3.49 s | ~14x |
| **Total** | **106.8 s** | **9.59 s** | **11x** |

- There is no hotspot: no single lowering or checker takes more than
  about 2%. The time is spread over 91 IR lowerings and 344 checkers,
  each walking the tree.
- `kotlinc`'s frontend runs on one core; only its backend can use more.
  With `-Xbackend-threads=0` (a backend thread per core) and without the
  profiler it took 59.4 s, against 110.0 s for the profiled runs. `kc`
  also checks in parallel.
- Of `kc`'s backend, 1.45 s is writing 10,001 class files.

## What the numbers don't show

They are an upper bound, not a forecast for a Go port of `kotlinc`:

- `kc` compiles a subset of Kotlin ([Unsupported Kotlin](unsupported.md))
  with simpler rules than K2's, and has no separate checkers
  ([Design](design.md)).
- Its inliner and backend are a single simple pass, without an SMAP.
- Real builds run `kotlinc` in the Gradle daemon, incrementally.
