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
with the commands above, on 15 November 2026: the libraries at `f5f5cce`,
the generated project at `0f87173`.

Wall time:

| Input | Files | Lines | `kc -j 1` | `kc -j 4` | `kotlinc` cold | `kotlinc` warm | Warm `kotlinc` / `kc -j 1` |
|---|---:|---:|---:|---:|---:|---:|---:|
| Generated | 500 | 202k | 7.13 s | 4.01 s | 98.1 s | 83.9 s | 12x |
| kotlinpoet | 39 | 8.7k | 300 ms | 211 ms | 17.8 s | 3.44 s | 11x |
| kotlinx-coroutines-core | 169 | 34k | 796 ms | 643 ms | 28.5 s | 6.88 s | 8.6x |
| Ktor, 3 modules | 236 | 24k | 849 ms | 698 ms | 45.7 s | 6.96 s | 8.2x |

Peak memory (resident set size):

| Input | Files | Lines | `kc -j 1` | `kc -j 4` | `kotlinc` cold | Cold `kotlinc` / `kc -j 1` |
|---|---:|---:|---:|---:|---:|---:|
| Generated | 500 | 202k | 755 MB | 1.03 GB | 3.36 GB | 4.4x |
| kotlinpoet | 39 | 8.7k | 88 MB | 100 MB | 944 MB | 11x |
| kotlinx-coroutines-core | 169 | 34k | 125 MB | 140 MB | 1.04 GB | 8.3x |
| Ktor, 3 modules | 236 | 24k | 87 MB | 94 MB | 946 MB | 11x |

- `kotlinc` runs with an 8 GB heap limit (`HEAP`). Warm `kotlinc`'s
  memory isn't measured.
- Warm `kotlinc` doesn't settle at 500 files on this machine: its four
  runs took 67–103 s.
- `kotlinc`'s times vary by 10–20% between sessions although it doesn't
  change. To measure a change to `kc`, time both builds in one session.
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
| Parsing | 4.0 s | 0.65 s | ~6x |
| Resolution and type checking | 26.8 s | 3.06 s | ~9x |
| Checkers (warnings and errors) | 17.9 s | none | |
| Reading the class path | 1.4 s | 0.03 s | |
| Backend, with inlining and writing | 34.7 s | 3.39 s | ~10x |
| **Total** | **84.8 s** | **7.13 s** | **12x** |

- There is no hotspot: no single lowering or checker takes more than
  about 2%. The time is spread over 93 IR lowerings and 330 checkers,
  each walking the tree.
- `kotlinc`'s frontend runs on one core; only its backend can use more.
  With `-Xbackend-threads=0` (a backend thread per core) and without the
  profiler it took 58.4 s, against 87.9 s for the profiled runs. `kc`
  also checks in parallel.
- Of `kc`'s backend, 1.52 s is writing 10,001 class files.

## What the numbers don't show

They are an upper bound, not a forecast for a Go port of `kotlinc`:

- `kc` compiles a subset of Kotlin ([Unsupported Kotlin](unsupported.md))
  with simpler rules than K2's, and has no separate checkers
  ([Design](design.md)).
- Its inliner and backend are a single simple pass, without an SMAP.
- Real builds run `kotlinc` in the Gradle daemon, incrementally.
