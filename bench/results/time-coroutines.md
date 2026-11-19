# kc vs kotlinc on kotlinx-coroutines-core

Kotlin/kotlinx.coroutines 1.11.0's kotlinx-coroutines-core for the JVM: 169 files, 34248 lines (common, concurrent and jvm).

- Machine: Linux x86_64, 4 CPUs
- go version go1.27.2 linux/amd64
- Picked up _JAVA_OPTIONS: -XX:+UnlockDiagnosticVMOptions -XX:DisableIntrinsic=_base64_encodeBlock,_base64_decodeBlock
- info: kotlinc-jvm 2.4.21 (JRE 21.0.13.0.3+1-1-24.04.1-Ubuntu)
- kotlinc JVM options: -Xmx8g
- Cooldown before each run (outside timing): 0 s

| Command | Wall time | vs `kc -j 1` | CPU time (user+sys) | Peak memory |
|:---|---:|---:|---:|---:|
| `kc -j 1` | 565 ± 14 ms | — | 1,111 ms | 131 MB |
| `kc -j 4` | 436 ± 24 ms | 1.3x faster | 1,280 ms | 144 MB |
| `kotlinc (cold JVM)` | 23,751 ± 560 ms | 42.0x slower | 75,459 ms | 1,232 MB |

- kotlinc in a warm JVM (median of the last 5 of 10 runs in one JVM): 5881 ms
- Class files: kc 790, kotlinc 850
