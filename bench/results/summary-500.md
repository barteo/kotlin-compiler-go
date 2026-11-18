# kc vs kotlinc

- Machine: Linux x86_64, 4 CPUs
- go version go1.27.2 linux/amd64
- Picked up _JAVA_OPTIONS: -XX:+UnlockDiagnosticVMOptions -XX:DisableIntrinsic=_base64_encodeBlock,_base64_decodeBlock
- info: kotlinc-jvm 2.4.21 (JRE 21.0.13.0.3+1-1-24.04.1-Ubuntu)
- kotlinc JVM options: -Xmx8g
- Cooldown before each run (outside timing): 0 s

## 500 files (202004 lines)

| Command | Wall time | vs `kc -j 1` | CPU time (user+sys) | Peak memory |
|:---|---:|---:|---:|---:|
| `kc -j 1` | 6,866 ± 200 ms | — | 14,178 ms | 811 MB |
| `kc -j 4` | 5,307 ± 532 ms | 1.3x faster | 16,685 ms | 953 MB |
| `kotlinc (cold JVM)` | 104,473 ms | 15.2x slower | 266,270 ms | 3,283 MB |

- kotlinc in a warm JVM (median of the last 2 of 4 runs in one JVM): 87732 ms
- Same program output: yes (checksum: 2194537)

