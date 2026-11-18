# Where kotlinc's time goes: 500 files

- Picked up _JAVA_OPTIONS: -XX:+UnlockDiagnosticVMOptions -XX:DisableIntrinsic=_base64_encodeBlock,_base64_decodeBlock, info: kotlinc-jvm 2.4.21 (JRE 21.0.13.0.3+1-1-24.04.1-Ubuntu)
- kotlinc warm, profiled: 109957 ms; with `-Xbackend-threads=0`: run 4: 59374 ms (profiling slows kotlinc down;
  bench/run.sh measures it without, and the phase times below include it)
- kc -j 1: parse 761.1 ms | index 26.6 ms | check 3,980.8 ms | prefetch 29.5 ms | load 94.3 ms | kmeta 22.8 ms | inline 1.0 ms | codegen 3,490.6 ms | write 1,449.0 ms | meta 322.8 ms
- kc -j 4: parse 503.5 ms | index 41.9 ms | check 2,435.1 ms | prefetch 47.1 ms | load 99.1 ms | kmeta 25.4 ms | inline 1.2 ms | codegen 1,803.7 ms | write 2,924.3 ms | meta 660.7 ms

## kotlinc phases (`-Xdump-perf`)

Median of runs 6-10, 501 files, 202505 lines.

| kotlinc phase | Wall time | Share |
|:---|---:|---:|
| Init | 57 ms | 0% |
| Analyze (parse, resolve, check) | 57,803 ms | 54% |
| FIR to IR | 6,806 ms | 6% |
| IR lowering | 19,037 ms | 18% |
| Bytecode generation | 23,085 ms | 22% |
| **Total** | **106,788 ms** | |

- GC pauses: 3,355 ms per run
- Classpath lookups of Kotlin classes: 409, 47 ms per run

## kotlinc profile (JFR)

169,300 samples

| Phase | Share of samples |
|:---|---:|
| IR lowering | 24.3% |
| Bytecode generation | 20.9% |
| Frontend: body resolution (calls, types, inference) | 19.2% |
| Frontend: checkers (diagnostics) | 18.4% |
| FIR to IR | 6.9% |
| Frontend: lexing and parsing | 5.8% |
| Frontend: building raw FIR | 1.4% |
| Frontend: loading classpath symbols | 1.3% |
| Frontend: declaration resolution | 1.2% |
| Other | 0.4% |
| Frontend: other | 0.2% |

91 distinct lowering classes sampled; the largest:

- (pass infrastructure): 6.5%
- ConstEvaluationLowering: 1.5%
- ForLoopsLowering: 0.6%
- JvmArgumentNullabilityAssertionsLowering: 0.6%
- JvmInlineClassLowering: 0.6%

344 distinct checker classes sampled; the largest:

- (tree traversal): 2.5%
- FirOptInUsageBaseChecker: 1.0%
- FirReifiedChecker: 0.7%
- FirConflictsDeclarationChecker: 0.7%
- FirMissingDependencyClassChecker: 0.7%
