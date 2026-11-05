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
| `kc -j 1` | 285 ± 5 ms | — | 464 ms | 107 MB |
| `kc -j 8` | 159 ± 5 ms | 1.8x faster | 683 ms | 118 MB |
| `kotlinc (cold JVM)` | 10,910 ± 390 ms | 38.3x slower | 37,396 ms | 1,146 MB |

- kotlinc in a warm JVM (median of the last 10 of 20 runs in one JVM): 2496 ms
- Class files: kc 790, kotlinc 850
