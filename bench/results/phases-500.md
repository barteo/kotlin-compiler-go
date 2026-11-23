# Where kotlinc's time goes: 500 files

- Picked up _JAVA_OPTIONS: -XX:+UnlockDiagnosticVMOptions -XX:DisableIntrinsic=_base64_encodeBlock,_base64_decodeBlock, info: kotlinc-jvm 2.4.21 (JRE 21.0.13.0.3+1-1-24.04.1-Ubuntu)
- kotlinc warm, profiled: 93866 ms; with `-Xbackend-threads=0`: 51312 ms (profiling slows kotlinc down;
  bench/run.sh measures it without, and the phase times below include it)
- kc -j 1: parse 416.1 ms | index 23.7 ms | check 3,103.9 ms | prefetch 24.8 ms | load 67.7 ms | kmeta 21.7 ms | inline 0.8 ms | codegen 2,300.2 ms | write 3,051.1 ms | meta 364.8 ms
- kc -j 4: parse 215.3 ms | index 34.7 ms | check 2,162.6 ms | prefetch 31.4 ms | load 55.7 ms | kmeta 12.9 ms | inline 0.9 ms | codegen 1,622.6 ms | write 4,773.8 ms | meta 826.6 ms

## kotlinc phases (`-Xdump-perf`)

Median of runs 6-10, 501 files, 202505 lines.

| kotlinc phase | Wall time | Share |
|:---|---:|---:|
| Init | 46 ms | 0% |
| Analyze (parse, resolve, check) | 51,189 ms | 57% |
| FIR to IR | 6,120 ms | 7% |
| IR lowering | 13,199 ms | 15% |
| Bytecode generation | 19,907 ms | 22% |
| **Total** | **90,461 ms** | |

- GC pauses: 3,099 ms per run
- Classpath lookups of Kotlin classes: 409, 50 ms per run

## kotlinc profile (JFR)

134,381 samples

| Phase | Share of samples |
|:---|---:|
| Bytecode generation | 22.1% |
| Frontend: body resolution (calls, types, inference) | 21.3% |
| IR lowering | 20.9% |
| Frontend: checkers (diagnostics) | 19.3% |
| FIR to IR | 8.5% |
| Frontend: lexing and parsing | 4.0% |
| Frontend: loading classpath symbols | 1.2% |
| Frontend: declaration resolution | 1.2% |
| Frontend: building raw FIR | 1.1% |
| Frontend: other | 0.2% |
| Other | 0.2% |

92 distinct lowering classes sampled; the largest:

- (pass infrastructure): 5.6%
- ConstEvaluationLowering: 1.5%
- JvmInlineClassLowering: 0.7%
- JvmArgumentNullabilityAssertionsLowering: 0.6%
- SyntheticAccessorLowering: 0.6%

339 distinct checker classes sampled; the largest:

- (tree traversal): 2.8%
- FirOptInUsageBaseChecker: 0.9%
- FirReifiedChecker: 0.7%
- FirConflictsDeclarationChecker: 0.7%
- FirMissingDependencyClassChecker: 0.5%
