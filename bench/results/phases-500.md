# Where kotlinc's time goes: 500 files

- openjdk version "21.0.12.1" 2026-08-18, info: kotlinc-jvm 2.4.21 (JRE 21.0.12.1)
- kotlinc warm, profiled: 33966 ms; with `-Xbackend-threads=0`: 33077 ms (profiling slows kotlinc down;
  bench/run.sh measures it without, and the phase times below include it)
- kc -j 1: parse 260.1 ms | index 5.9 ms | check 1,129.9 ms | prefetch 8.0 ms | load 44.1 ms | kmeta 8.9 ms | inline 0.5 ms | codegen 1,387.5 ms | write 631.9 ms | meta 110.1 ms
- kc -j 8: parse 112.6 ms | index 16.6 ms | check 830.6 ms | prefetch 16.4 ms | load 43.5 ms | kmeta 12.9 ms | inline 0.6 ms | codegen 560.8 ms | write 1,961.1 ms | meta 346.1 ms

## kotlinc phases (`-Xdump-perf`)

Median of runs 6-10, 501 files, 202505 lines.

| kotlinc phase | Wall time | Share |
|:---|---:|---:|
| Init | 24 ms | 0% |
| Analyze (parse, resolve, check) | 19,366 ms | 56% |
| FIR to IR | 1,506 ms | 4% |
| IR lowering | 6,209 ms | 18% |
| Bytecode generation | 7,206 ms | 21% |
| **Total** | **34,311 ms** | |

- GC pauses: 1,220 ms per run
- Classpath lookups of Kotlin classes: 409, 14 ms per run

## kotlinc profile (JFR)

69,313 samples

| Phase | Share of samples |
|:---|---:|
| Frontend: body resolution (calls, types, inference) | 25.3% |
| Bytecode generation | 23.1% |
| IR lowering | 17.7% |
| Frontend: checkers (diagnostics) | 12.9% |
| Frontend: lexing and parsing | 11.9% |
| FIR to IR | 4.2% |
| Frontend: building raw FIR | 1.5% |
| Frontend: loading classpath symbols | 1.3% |
| Frontend: declaration resolution | 0.9% |
| Other | 0.9% |
| Frontend: other | 0.3% |

82 distinct lowering classes sampled; the largest:

- (pass infrastructure): 4.4%
- ConstEvaluationLowering: 0.9%
- ForLoopsLowering: 0.6%
- JvmInlineClassLowering: 0.5%
- LocalDeclarationsLowering: 0.5%

289 distinct checker classes sampled; the largest:

- (tree traversal): 1.8%
- FirMissingDependencyClassChecker: 0.7%
- FirOptInUsageBaseChecker: 0.5%
- FirReifiedChecker: 0.4%
- FirArrayOfNullableNothingExpressionCheckerKt: 0.4%
