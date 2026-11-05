# Where kotlinc's time goes: 500 files

- openjdk version "21.0.12.1" 2026-08-18, info: kotlinc-jvm 2.4.21 (JRE 21.0.12.1)
- kotlinc warm, profiled: 40664 ms; with `-Xbackend-threads=0`: 22389 ms (profiling slows kotlinc down;
  bench/run.sh measures it without, and the phase times below include it)
- kc -j 1: parse 266.1 ms | index 5.8 ms | check 918.9 ms | prefetch 8.2 ms | load 57.8 ms | kmeta 11.3 ms | inline 0.5 ms | codegen 1,322.8 ms | write 586.1 ms | meta 93.5 ms
- kc -j 8: parse 96.1 ms | index 12.9 ms | check 532.3 ms | prefetch 13.7 ms | load 54.0 ms | kmeta 9.2 ms | inline 0.6 ms | codegen 497.1 ms | write 1,662.8 ms | meta 255.4 ms

## kotlinc phases (`-Xdump-perf`)

Median of runs 6-10, 501 files, 202505 lines.

| kotlinc phase | Wall time | Share |
|:---|---:|---:|
| Init | 22 ms | 0% |
| Analyze (parse, resolve, check) | 19,228 ms | 50% |
| FIR to IR | 2,358 ms | 6% |
| IR lowering | 7,619 ms | 20% |
| Bytecode generation | 9,135 ms | 24% |
| **Total** | **38,362 ms** | |

- GC pauses: 1,248 ms per run
- Classpath lookups of Kotlin classes: 409, 14 ms per run

## kotlinc profile (JFR)

65,549 samples

| Phase | Share of samples |
|:---|---:|
| Bytecode generation | 25.1% |
| Frontend: body resolution (calls, types, inference) | 22.2% |
| IR lowering | 17.8% |
| Frontend: checkers (diagnostics) | 14.1% |
| Frontend: lexing and parsing | 11.1% |
| FIR to IR | 5.5% |
| Frontend: building raw FIR | 1.2% |
| Frontend: loading classpath symbols | 1.1% |
| Other | 0.9% |
| Frontend: declaration resolution | 0.6% |
| Frontend: other | 0.3% |

88 distinct lowering classes sampled; the largest:

- (pass infrastructure): 4.5%
- ConstEvaluationLowering: 1.1%
- JvmInlineClassLowering: 0.6%
- ForLoopsLowering: 0.5%
- SyntheticAccessorLowering: 0.5%

290 distinct checker classes sampled; the largest:

- (tree traversal): 1.9%
- FirMissingDependencyClassChecker: 0.7%
- FirOptInUsageBaseChecker: 0.7%
- FirReifiedChecker: 0.4%
- FirConflictsDeclarationChecker: 0.4%
