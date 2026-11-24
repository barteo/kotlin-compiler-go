# Where kotlinc's time goes: 500 files

- Picked up _JAVA_OPTIONS: -XX:+UnlockDiagnosticVMOptions -XX:DisableIntrinsic=_base64_encodeBlock,_base64_decodeBlock, info: kotlinc-jvm 2.4.21 (JRE 21.0.13.0.3+1-1-24.04.1-Ubuntu)
- kotlinc warm, profiled: 112204 ms; with `-Xbackend-threads=0`: 54759 ms (profiling slows kotlinc down;
  bench/run.sh measures it without, and the phase times below include it)
- kc -j 1: parse 722.5 ms | index 26.1 ms | check 3,534.8 ms | prefetch 24.1 ms | load 79.0 ms | kmeta 30.4 ms | inline 1.4 ms | codegen 2,324.8 ms | write 3,279.0 ms | meta 340.2 ms
- kc -j 4: parse 482.0 ms | index 46.0 ms | check 2,174.2 ms | prefetch 34.2 ms | load 72.7 ms | kmeta 27.0 ms | inline 1.3 ms | codegen 1,749.1 ms | write 6,455.0 ms | meta 802.3 ms

## kotlinc phases (`-Xdump-perf`)

Median of runs 6-10, 501 files, 202505 lines.

| kotlinc phase | Wall time | Share |
|:---|---:|---:|
| Init | 51 ms | 0% |
| Analyze (parse, resolve, check) | 61,591 ms | 55% |
| FIR to IR | 7,077 ms | 6% |
| IR lowering | 18,250 ms | 16% |
| Bytecode generation | 25,460 ms | 23% |
| **Total** | **112,429 ms** | |

- GC pauses: 3,498 ms per run
- Classpath lookups of Kotlin classes: 409, 57 ms per run

## kotlinc profile (JFR)

174,168 samples

| Phase | Share of samples |
|:---|---:|
| Frontend: body resolution (calls, types, inference) | 21.8% |
| Bytecode generation | 21.8% |
| IR lowering | 20.3% |
| Frontend: checkers (diagnostics) | 18.8% |
| FIR to IR | 6.9% |
| Frontend: lexing and parsing | 6.1% |
| Frontend: building raw FIR | 1.5% |
| Frontend: loading classpath symbols | 1.4% |
| Frontend: declaration resolution | 1.0% |
| Frontend: other | 0.3% |
| Other | 0.2% |

93 distinct lowering classes sampled; the largest:

- (pass infrastructure): 5.3%
- ConstEvaluationLowering: 1.4%
- JvmInlineClassLowering: 0.6%
- SyntheticAccessorLowering: 0.6%
- JvmArgumentNullabilityAssertionsLowering: 0.6%

347 distinct checker classes sampled; the largest:

- (tree traversal): 2.6%
- FirOptInUsageBaseChecker: 0.9%
- FirConflictsDeclarationChecker: 0.6%
- FirReifiedChecker: 0.6%
- FirMissingDependencyClassChecker: 0.4%
