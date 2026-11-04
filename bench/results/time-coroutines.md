# kc vs kotlinc on kotlinx-coroutines-core

Kotlin/kotlinx.coroutines 1.11.0's kotlinx-coroutines-core for the JVM: 169 files, 34248 lines (common, concurrent and jvm).

- Machine: Darwin arm64, 8 CPUs
- go version go1.27.2 darwin/arm64
- openjdk version "21.0.12.1" 2026-08-18
- info: kotlinc-jvm 2.4.21 (JRE 21.0.12.1)
- kotlinc JVM options: -Xmx8g
- Cooldown before each run (outside timing): 10 s

| Command | Wall time | vs `kc -j 1` | CPU time (user+sys) | Peak memory |
|:---|---:|---:|---:|---:|
| `kc -j 1` | 320 ± 5 ms | — | 531 ms | 111 MB |
| `kc -j 8` | 176 ± 1 ms | 1.8x faster | 755 ms | 127 MB |
| `kotlinc (cold JVM)` | 10,860 ± 446 ms | 33.9x slower | 36,971 ms | 1,115 MB |

- kotlinc in a warm JVM (median of the last 10 of 20 runs in one JVM): 2215 ms
- Class files: kc 790, kotlinc 850
