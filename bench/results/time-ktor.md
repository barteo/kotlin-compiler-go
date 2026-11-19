# kc vs kotlinc on Ktor

Ktor 3.6.0's ktor-io, ktor-utils and ktor-http JVM libraries: 236 files, 23774 lines.
Modules compile sequentially against the same compiler's preceding outputs.
Cold kotlinc starts one JVM per module; warm kotlinc runs all three in one JVM.
These are compiler timings, excluding Gradle, tests and AtomicFU bytecode transformation.

- Machine: Linux x86_64, 4 CPUs
- Revision: a009992
- go version go1.27.2 linux/amd64
- Picked up _JAVA_OPTIONS: -XX:+UnlockDiagnosticVMOptions -XX:DisableIntrinsic=_base64_encodeBlock,_base64_decodeBlock
- info: kotlinc-jvm 2.4.21 (JRE 21.0.13.0.3+1-1-24.04.1-Ubuntu)
- kotlinc JVM options: -Xmx8g
- Runs: 10 kc builds (plus one warm-up per command), 3 cold kotlinc builds, 10 builds in one JVM
- Cooldown before each complete build (outside timing): 0 s
- Options: -jvm-default=enable, kotlinx.serialization; kotlinc language 2.3, API 2.2, explicit API mode, JVM target 1.8 and multiplatform fragments
- Dependencies: atomicfu 0.33.0, coroutines 1.11.0, kotlinx-io 0.9.1, serialization 1.11.0, SLF4J 2.0.19

| Command | Wall time | vs `kc -j 1` | CPU time (user+sys) | Peak memory |
|:---|---:|---:|---:|---:|
| `kc -j 1` | 720 ± 27 ms | — | 1,393 ms | 91 MB |
| `kc -j 4` | 619 ± 20 ms | 1.2x faster | 1,729 ms | 98 MB |
| `kotlinc (cold JVM)` | 39,848 ± 222 ms | 55.4x slower | 117,983 ms | 966 MB |

- kotlinc in a warm JVM (median of the last 5 of 10 complete builds): 5736 ms
- Class files: kc 671, kotlinc 668
