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

A cloud VM with 4 vCPUs (Intel Xeon) and 16 GB, Linux x86_64, JDK 21,
at `e619cc1` on 4 November 2026.

Wall time:

| Input | Files | Lines | `kc -j 1` | `kc -j 4` | `kotlinc` cold | `kotlinc` warm | Warm `kotlinc` / `kc -j 1` |
|---|---:|---:|---:|---:|---:|---:|---:|
| Generated | 100 | 40k | 1.27 s | 0.82 s | 32.4 s | 15.9 s | 13x |
| Generated | 1,000 | 404k | 11.59 s | 6.02 s | 140.1 s | 120.0 s | 10x |
| kotlinpoet | 39 | 8.7k | 212 ms | 157 ms | 14.2 s | 2.15 s | 10x |
| kotlinx-coroutines-core | 169 | 34k | 560 ms | 464 ms | 21.7 s | 4.53 s | 8.1x |
| Ktor, 3 modules | 236 | 24k | 660 ms | 557 ms | 36.0 s | 5.05 s | 7.7x |

Peak memory (resident set size):

| Input | Files | Lines | `kc -j 1` | `kc -j 4` | `kotlinc` cold | Cold `kotlinc` / `kc -j 1` |
|---|---:|---:|---:|---:|---:|---:|
| Generated | 100 | 40k | 172 MB | 219 MB | 1.52 GB | 8.8x |
| Generated | 1,000 | 404k | 1.31 GB | 1.79 GB | 4.89 GB | 3.7x |
| kotlinpoet | 39 | 8.7k | 77 MB | 87 MB | 952 MB | 12x |
| kotlinx-coroutines-core | 169 | 34k | 110 MB | 121 MB | 1.05 GB | 9.6x |
| Ktor, 3 modules | 236 | 24k | 75 MB | 83 MB | 935 MB | 12x |

- `kotlinc` runs with an 8 GB heap limit (`HEAP`). Warm `kotlinc`'s
  memory isn't measured.
- Warm `kotlinc` gains little at 1,000 files: its four runs took
  113–145 s, against 140 s cold.
- On 4 vCPUs, `kc -j 4` is only 1.2–1.9x as fast as `kc -j 1`, and
  `kc`'s times vary more between runs than on the earlier runs' Apple M2,
  by up to 14%.
- `kotlinc`'s times vary by 10–20% between sessions although it doesn't
  change. To measure a change to `kc`, time both builds in one session.
- Full tables, with CPU time, are in [bench/results](../bench/results/).
  Whether `kc`'s library builds are correct (API, `-Xverify:all`, the
  libraries' own tests) is checked in [Survey of real code](survey.md).

## Where kotlinc's time goes

A profile (`bench/profile.sh`, in
[phases-500.md](../bench/results/phases-500.md)): 500 generated
files of 202k lines, warm, on the machine above, both compilers on one
thread.

| Work | `kotlinc` | `kc -j 1` | Ratio |
|:---|---:|---:|---:|
| Parsing | 3.2 s | 0.54 s | ~6x |
| Resolution and type checking | 15.4 s | 2.21 s | ~7x |
| Checkers (warnings and errors) | 12.0 s | none | |
| Reading the class path | 0.8 s | 0.03 s | |
| Backend, with inlining and writing | 27.5 s | 2.35 s | ~12x |
| **Total** | **58.9 s** | **5.10 s** | **12x** |

- There is no hotspot: no single lowering or checker takes more than
  about 2%. The time is spread over 90 IR lowerings and 318 checkers,
  each walking the tree.
- `kotlinc`'s frontend runs on one core. With `-Xbackend-threads=0` it
  took 49.6 s, against 59.5 s for the profiled runs; part of that
  difference is the profiling itself. `kc` checks and generates code
  in parallel.
- Of `kc`'s backend, 0.90 s is writing 10,001 class files.

## What the numbers don't show

They are an upper bound, not a forecast for a Go port of `kotlinc`:

- `kc` compiles a subset of Kotlin ([Unsupported Kotlin](unsupported.md))
  with simpler rules than K2's, and has no separate checkers
  ([Design](design.md)).
- Its inliner and backend are a single simple pass, without an SMAP.
- Real builds run `kotlinc` in the Gradle daemon, incrementally.
