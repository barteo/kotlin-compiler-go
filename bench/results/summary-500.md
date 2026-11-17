# kc vs kotlinc

- Machine: Linux x86_64, 4 CPUs
- go version go1.27.2 linux/amd64
- openjdk version "21.0.12.1" 2026-08-18
- info: kotlinc-jvm 2.4.21 (JRE 21.0.12.1+1-1-24.04.4-Ubuntu)
- kotlinc JVM options: -Xmx8g
- Cooldown before each run (outside timing): 0 s

## 500 files (202004 lines)

| Command | Wall time | vs `kc -j 1` | CPU time (user+sys) | Peak memory |
|:---|---:|---:|---:|---:|
| `kc -j 1` | 7,126 ± 387 ms | — | 10,806 ms | 755 MB |
| `kc -j 4` | 4,008 ± 226 ms | 1.8x faster | 12,914 ms | 1,026 MB |
| `kotlinc (cold JVM)` | 98,128 ms | 13.8x slower | 244,815 ms | 3,358 MB |

- kotlinc in a warm JVM (median of the last 2 of 4 runs in one JVM): 83920 ms
- Same program output: yes (checksum: 2194537)

