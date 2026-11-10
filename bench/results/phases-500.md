# Where kotlinc's time goes: 500 files

- openjdk version "21.0.12.1" 2026-08-18, info: kotlinc-jvm 2.4.21 (JRE 21.0.12.1)
- kotlinc warm, profiled: 33218 ms; with `-Xbackend-threads=0`: 23496 ms (profiling slows kotlinc down;
  bench/run.sh measures it without, and the phase times below include it)
- kc -j 1: parse 260.3 ms | index 5.8 ms | check 1,116.7 ms | prefetch 8.0 ms | load 49.1 ms | kmeta 9.9 ms | inline 0.5 ms | codegen 1,380.7 ms | write 629.6 ms | meta 109.3 ms
- kc -j 8: parse 101.2 ms | index 11.9 ms | check 693.8 ms | prefetch 16.0 ms | load 55.7 ms | kmeta 10.5 ms | inline 0.5 ms | codegen 483.6 ms | write 1,641.5 ms | meta 289.3 ms

## kotlinc phases (`-Xdump-perf`)

Median of runs 6-10, 501 files, 202505 lines.

| kotlinc phase | Wall time | Share |
|:---|---:|---:|
| Init | 23 ms | 0% |
| Analyze (parse, resolve, check) | 17,625 ms | 56% |
| FIR to IR | 1,647 ms | 5% |
| IR lowering | 5,803 ms | 18% |
| Bytecode generation | 6,646 ms | 21% |
| **Total** | **31,745 ms** | |

- GC pauses: 1,359 ms per run
- Classpath lookups of Kotlin classes: 409, 15 ms per run

## kotlinc profile (JFR)

42,731 samples

| Phase | Share of samples |
|:---|---:|
| Frontend: body resolution (calls, types, inference) | 25.8% |
| IR lowering | 18.9% |
| Bytecode generation | 15.9% |
| Frontend: lexing and parsing | 15.8% |
| Frontend: checkers (diagnostics) | 13.6% |
| FIR to IR | 4.8% |
| Frontend: building raw FIR | 1.6% |
| Frontend: loading classpath symbols | 1.3% |
| Other | 1.1% |
| Frontend: declaration resolution | 1.0% |
| Frontend: other | 0.3% |

84 distinct lowering classes sampled; the largest:

- (pass infrastructure): 5.1%
- ConstEvaluationLowering: 1.5%
- JvmArgumentNullabilityAssertionsLowering: 0.5%
- ForLoopsLowering: 0.4%
- JvmLocalDeclarationPopupLowering: 0.4%

272 distinct checker classes sampled; the largest:

- (tree traversal): 2.1%
- FirMissingDependencyClassChecker: 0.6%
- FirOptInUsageBaseChecker: 0.6%
- FirReifiedChecker: 0.4%
- FirArrayOfNullableNothingExpressionCheckerKt: 0.4%
