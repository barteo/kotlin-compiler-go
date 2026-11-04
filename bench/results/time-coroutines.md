# kc vs kotlinc on kotlinx-coroutines-core

Kotlin/kotlinx.coroutines 1.11.0's kotlinx-coroutines-core for the JVM: 169 files, 34248 lines (common, concurrent and jvm).

- Machine: Linux x86_64, 4 CPUs
- go version go1.27.2 linux/amd64
- openjdk version "21.0.12.1" 2026-08-18
- info: kotlinc-jvm 2.4.21 (JRE 21.0.12.1+1-1-24.04.4-Ubuntu)
- kotlinc JVM options: -Xmx8g
- Cooldown before each run (outside timing): 10 s

| Command | Wall time | vs `kc -j 1` | CPU time (user+sys) | Peak memory |
|:---|---:|---:|---:|---:|
| `kc -j 1` | 560 ± 80 ms | — | 823 ms | 110 MB |
| `kc -j 4` | 464 ± 48 ms | 1.2x faster | 1,376 ms | 121 MB |
| `kotlinc (cold JVM)` | 21,741 ± 512 ms | 38.9x slower | 67,259 ms | 1,054 MB |

- kotlinc in a warm JVM (median of the last 10 of 20 runs in one JVM): 4531 ms
- Class files: kc 790, kotlinc 850
