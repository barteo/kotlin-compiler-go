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
| `kc -j 1` | 627 ± 35 ms | — | 1,338 ms | 133 MB |
| `kc -j 4` | 562 ± 16 ms | 1.1x faster | 1,668 ms | 145 MB |
| `kotlinc (cold JVM)` | 22,983 ± 106 ms | 36.7x slower | 73,009 ms | 1,253 MB |

- kotlinc in a warm JVM (median of the last 5 of 10 runs in one JVM): 5587 ms
- Class files: kc 790, kotlinc 850
