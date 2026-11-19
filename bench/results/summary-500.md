# kc vs kotlinc

- Machine: Linux x86_64, 4 CPUs
- go version go1.27.2 linux/amd64
- Picked up _JAVA_OPTIONS: -XX:+UnlockDiagnosticVMOptions -XX:DisableIntrinsic=_base64_encodeBlock,_base64_decodeBlock
- info: kotlinc-jvm 2.4.21 (JRE 21.0.13.0.3+1-1-24.04.1-Ubuntu)
- kotlinc JVM options: -Xmx8g
- Cooldown before each run (outside timing): 0 s
- Output directory: /dev/shm/kc-bench-out (tmpfs)

## 500 files (202004 lines)

| Command | Wall time | vs `kc -j 1` | CPU time (user+sys) | Peak memory |
|:---|---:|---:|---:|---:|
| `kc -j 1` | 5,038 ± 259 ms | — | 7,727 ms | 758 MB |
| `kc -j 4` | 3,051 ± 166 ms | 1.7x faster | 9,365 ms | 959 MB |
| `kotlinc (cold JVM)` | 105,035 ms | 20.8x slower | 254,627 ms | 3,188 MB |

- kotlinc in a warm JVM (median of the last 2 of 4 runs in one JVM): 80606 ms
- Same program output: yes (checksum: 2194537)

