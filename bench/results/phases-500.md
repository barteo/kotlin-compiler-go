# Where kotlinc's time goes: 500 files

- openjdk version "21.0.12.1" 2026-08-18, info: kotlinc-jvm 2.4.21 (JRE 21.0.12.1+1-1-24.04.4-Ubuntu)
- kotlinc warm, profiled: 59524 ms; with `-Xbackend-threads=0`: 49645 ms (profiling slows kotlinc down;
  bench/run.sh measures it without, and the phase times below include it)
- kc -j 1: parse 539.0 ms | index 15.6 ms | check 2,210.7 ms | prefetch 16.8 ms | load 47.7 ms | kmeta 14.1 ms | inline 0.8 ms | codegen 2,352.6 ms | write 903.2 ms | meta 209.9 ms
- kc -j 4: parse 299.7 ms | index 24.5 ms | check 1,460.5 ms | prefetch 24.0 ms | load 60.9 ms | kmeta 26.1 ms | inline 0.9 ms | codegen 2,882.7 ms | write 9,604.7 ms | meta 294.9 ms

## kotlinc phases (`-Xdump-perf`)

Median of runs 6-10, 501 files, 202505 lines.

| kotlinc phase | Wall time | Share |
|:---|---:|---:|
| Init | 30 ms | 0% |
| Analyze (parse, resolve, check) | 33,772 ms | 57% |
| FIR to IR | 3,407 ms | 6% |
| IR lowering | 9,576 ms | 16% |
| Bytecode generation | 12,113 ms | 21% |
| **Total** | **58,899 ms** | |

- GC pauses: 2,403 ms per run
- Classpath lookups of Kotlin classes: 409, 34 ms per run

## kotlinc profile (JFR)

108,950 samples

| Phase | Share of samples |
|:---|---:|
| Frontend: body resolution (calls, types, inference) | 25.1% |
| Frontend: checkers (diagnostics) | 20.4% |
| Bytecode generation | 20.2% |
| IR lowering | 19.7% |
| FIR to IR | 6.4% |
| Frontend: lexing and parsing | 4.4% |
| Frontend: loading classpath symbols | 1.4% |
| Frontend: building raw FIR | 1.1% |
| Frontend: declaration resolution | 0.9% |
| Other | 0.3% |
| Frontend: other | 0.2% |

90 distinct lowering classes sampled; the largest:

- (pass infrastructure): 5.3%
- ConstEvaluationLowering: 1.2%
- JvmInlineClassLowering: 0.8%
- JvmArgumentNullabilityAssertionsLowering: 0.6%
- JvmStaticInObjectLowering: 0.6%

318 distinct checker classes sampled; the largest:

- (tree traversal): 3.0%
- FirOptInUsageBaseChecker: 0.9%
- FirConflictsDeclarationChecker: 0.9%
- FirReifiedChecker: 0.7%
- FirMissingDependencyClassChecker: 0.6%
