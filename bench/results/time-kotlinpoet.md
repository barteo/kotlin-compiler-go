# kc vs kotlinc on kotlinpoet

square/kotlinpoet 2.4.0's JVM library: 39 files, 8696 lines (commonMain and jvmMain).

- Machine: Linux x86_64, 4 CPUs
- go version go1.27.2 linux/amd64
- Picked up _JAVA_OPTIONS: -XX:+UnlockDiagnosticVMOptions -XX:DisableIntrinsic=_base64_encodeBlock,_base64_decodeBlock
- info: kotlinc-jvm 2.4.21 (JRE 21.0.13.0.3+1-1-24.04.1-Ubuntu)
- kotlinc JVM options: -Xmx8g
- Cooldown before each run (outside timing): 0 s

| Command | Wall time | vs `kc -j 1` | CPU time (user+sys) | Peak memory |
|:---|---:|---:|---:|---:|
| `kc -j 1` | 455 ± 24 ms | — | 691 ms | 89 MB |
| `kc -j 4` | 315 ± 21 ms | 1.4x faster | 796 ms | 103 MB |
| `kotlinc (cold JVM)` | 20,046 ± 126 ms | 44.1x slower | 61,850 ms | 965 MB |

- kotlinc in a warm JVM (median of the last 5 of 10 runs in one JVM): 3821 ms
- Class files: kc 108, kotlinc 120
