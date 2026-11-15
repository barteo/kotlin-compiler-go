# kc vs kotlinc on kotlinx-coroutines-core

Kotlin/kotlinx.coroutines 1.11.0's kotlinx-coroutines-core for the JVM: 169 files, 34248 lines (common, concurrent and jvm).

- Machine: Linux x86_64, 4 CPUs
- go version go1.27.2 linux/amd64
- openjdk version "21.0.12.1" 2026-08-18
- info: kotlinc-jvm 2.4.21 (JRE 21.0.12.1+1-1-24.04.4-Ubuntu)
- kotlinc JVM options: -Xmx8g
- Cooldown before each run (outside timing): 0 s

| Command | Wall time | vs `kc -j 1` | CPU time (user+sys) | Peak memory |
|:---|---:|---:|---:|---:|
| `kc -j 1` | 796 ± 82 ms | — | 1,150 ms | 125 MB |
| `kc -j 4` | 643 ± 33 ms | 1.2x faster | 1,850 ms | 140 MB |
| `kotlinc (cold JVM)` | 28,531 ± 891 ms | 35.8x slower | 87,883 ms | 1,036 MB |

- kotlinc in a warm JVM (median of the last 5 of 10 runs in one JVM): 6884 ms
- Class files: kc 790, kotlinc 850
