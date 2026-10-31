# kc vs kotlinc

- Machine: Darwin arm64, 12 CPUs
- go version go1.27.2 darwin/arm64
- openjdk version "21.0.12.1" 2026-08-18
- info: kotlinc-jvm 2.4.21 (JRE 21.0.12.1)
- kotlinc JVM options: -Xmx4g

## 200 files (28804 lines)

| Command | Wall time | vs `kc -j 1` | CPU time (user+sys) | Peak memory |
|:---|---:|---:|---:|---:|
| `kc -j 1` | 163 ± 4 ms | — | 287 ms | 69 MB |
| `kc -j 12` | 51 ± 1 ms | 3.2x faster | 399 ms | 90 MB |
| `kotlinc (cold JVM)` | 9,275 ± 21 ms | 57.0x slower | 38,300 ms | 1,059 MB |

- kotlinc in a warm JVM (median of the last 3 of 5 runs in one JVM): 3134 ms
- Same program output: yes (checksum: 467923)

