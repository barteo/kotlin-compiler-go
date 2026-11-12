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
COOLDOWN=30 SIZES="100 1000" RUNS=1 WARM_RUNS=4 bench/run.sh  # the results below
COOLDOWN=10 RUNS=5 WARM_RUNS=20 bench/kotlinpoet-time.sh      # also coroutines-time.sh, ktor-time.sh
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

Apple M2 MacBook Air (4 performance and 4 efficiency cores, 16 GB),
JDK 21, at `1d1d79a` on 10 November 2026.

Wall time:

| Input | Files | Lines | `kc -j 1` | `kc -j 8` | `kotlinc` cold | `kotlinc` warm | Warm `kotlinc` / `kc -j 1` |
|---|---:|---:|---:|---:|---:|---:|---:|
| Generated | 100 | 40k | 0.60 s | 0.33 s | 17.8 s | 6.9 s | 12x |
| Generated | 1,000 | 404k | 5.98 s | 2.88 s | 80.6 s | 98.9 s | 17x |
| kotlinpoet | 39 | 8.7k | 153 ms | 104 ms | 7.5 s | 1.11 s | 7.3x |
| kotlinx-coroutines-core | 169 | 34k | 312 ms | 179 ms | 11.2 s | 2.32 s | 7.4x |
| Ktor, 3 modules | 236 | 24k | 366 ms | 244 ms | 20.0 s | 2.27 s | 6.2x |

Peak memory (resident set size):

| Input | Files | Lines | `kc -j 1` | `kc -j 8` | `kotlinc` cold | Cold `kotlinc` / `kc -j 1` |
|---|---:|---:|---:|---:|---:|---:|
| Generated | 100 | 40k | 171 MB | 238 MB | 1.41 GB | 8.3x |
| Generated | 1,000 | 404k | 1.31 GB | 1.75 GB | 4.02 GB | 3.1x |
| kotlinpoet | 39 | 8.7k | 80 MB | 93 MB | 1.03 GB | 13x |
| kotlinx-coroutines-core | 169 | 34k | 115 MB | 130 MB | 1.09 GB | 9.5x |
| Ktor, 3 modules | 236 | 24k | 82 MB | 95 MB | 1.08 GB | 13x |

- `kotlinc` runs with an 8 GB heap limit (`HEAP`). Warm `kotlinc`'s
  memory isn't measured.
- Warm `kotlinc` doesn't settle at 1,000 files on this 16 GB machine:
  its four runs took 78–99 s, mostly slower than cold.
- `kotlinc`'s times vary by 10–20% between sessions although it doesn't
  change. To measure a change to `kc`, time both builds in one session.
- Full tables, with CPU time, are in [bench/results](../bench/results/).
  Whether `kc`'s library builds are correct (API, `-Xverify:all`, the
  libraries' own tests) is checked in [Survey of real code](survey.md).

## Where kotlinc's time goes

A profile (`bench/profile.sh`, in
[phases-500.md](../bench/results/phases-500.md)): 500 generated
files of 202k lines, warm, on an 8-CPU machine, both compilers on one
thread.

| Work | `kotlinc` | `kc -j 1` | Ratio |
|:---|---:|---:|---:|
| Parsing | 5.2 s | 0.26 s | ~20x |
| Resolution and type checking | 8.0 s | 1.12 s | ~7x |
| Checkers (warnings and errors) | 4.0 s | none | |
| Reading the class path | 0.4 s | 0.01 s | |
| Backend, with inlining and writing | 14.1 s | 1.38 s | ~10x |
| **Total** | **31.7 s** | **2.77 s** | **11x** |

- There is no hotspot: no single lowering or checker takes more than
  about 2%. The time is spread over 84 IR lowerings and 272 checkers,
  each walking the tree.
- `kotlinc`'s frontend runs on one core; only its backend can use more.
  With `-Xbackend-threads=0` (a backend thread per core) and without the
  profiler it took 23.5 s, against 33.2 s for the profiled runs. `kc`
  also checks in parallel.
- Of `kc`'s backend, 0.63 s is writing 10,001 class files.

## What the numbers don't show

They are an upper bound, not a forecast for a Go port of `kotlinc`:

- `kc` compiles a subset of Kotlin ([Unsupported Kotlin](unsupported.md))
  with simpler rules than K2's, and has no separate checkers
  ([Design](design.md)).
- Its inliner and backend are a single simple pass, without an SMAP.
- Real builds run `kotlinc` in the Gradle daemon, incrementally.
