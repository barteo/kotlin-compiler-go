# Where kotlinc's time goes: 500 files

- openjdk version "21.0.12.1" 2026-08-18, info: kotlinc-jvm 2.4.21 (JRE 21.0.12.1+1-1-24.04.4-Ubuntu)
- kotlinc warm, profiled: 87946 ms; with `-Xbackend-threads=0`: 58369 ms (profiling slows kotlinc down;
  bench/run.sh measures it without, and the phase times below include it)
- kc -j 1: parse 653.7 ms | index 27.9 ms | check 3,055.6 ms | prefetch 28.5 ms | load 80.0 ms | kmeta 19.7 ms | inline 1.3 ms | codegen 3,385.6 ms | write 1,520.8 ms | meta 283.2 ms
- kc -j 4: parse 414.1 ms | index 33.4 ms | check 1,872.1 ms | prefetch 36.9 ms | load 90.9 ms | kmeta 23.1 ms | inline 1.0 ms | codegen 1,580.0 ms | write 2,670.4 ms | meta 541.8 ms

## kotlinc phases (`-Xdump-perf`)

Median of runs 6-10, 501 files, 202505 lines.

| kotlinc phase | Wall time | Share |
|:---|---:|---:|
| Init | 43 ms | 0% |
| Analyze (parse, resolve, check) | 50,046 ms | 59% |
| FIR to IR | 4,674 ms | 6% |
| IR lowering | 13,105 ms | 15% |
| Bytecode generation | 16,892 ms | 20% |
| **Total** | **84,760 ms** | |

- GC pauses: 3,114 ms per run
- Classpath lookups of Kotlin classes: 409, 47 ms per run

## kotlinc profile (JFR)

140,704 samples

| Phase | Share of samples |
|:---|---:|
| Frontend: body resolution (calls, types, inference) | 27.6% |
| IR lowering | 20.5% |
| Bytecode generation | 19.3% |
| Frontend: checkers (diagnostics) | 19.2% |
| FIR to IR | 6.2% |
| Frontend: lexing and parsing | 3.4% |
| Frontend: loading classpath symbols | 1.5% |
| Frontend: declaration resolution | 1.0% |
| Frontend: building raw FIR | 0.9% |
| Other | 0.2% |
| Frontend: other | 0.2% |

93 distinct lowering classes sampled; the largest:

- (pass infrastructure): 5.7%
- ConstEvaluationLowering: 1.2%
- JvmInlineClassLowering: 0.7%
- JvmArgumentNullabilityAssertionsLowering: 0.6%
- SyntheticAccessorLowering: 0.5%

330 distinct checker classes sampled; the largest:

- (tree traversal): 2.8%
- FirOptInUsageBaseChecker: 1.0%
- FirConflictsDeclarationChecker: 0.8%
- FirReifiedChecker: 0.7%
- FirMissingDependencyClassChecker: 0.5%
