# kc vs kotlinc on kotlinpoet

square/kotlinpoet 2.4.0's JVM library: 39 files, 8696 lines (commonMain and jvmMain).

- Machine: Linux x86_64, 4 CPUs
- go version go1.27.2 linux/amd64
- openjdk version "21.0.12.1" 2026-08-18
- info: kotlinc-jvm 2.4.21 (JRE 21.0.12.1+1-1-24.04.4-Ubuntu)
- kotlinc JVM options: -Xmx8g
- Cooldown before each run (outside timing): 10 s

| Command | Wall time | vs `kc -j 1` | CPU time (user+sys) | Peak memory |
|:---|---:|---:|---:|---:|
| `kc -j 1` | 212 ± 8 ms | — | 359 ms | 77 MB |
| `kc -j 4` | 157 ± 13 ms | 1.3x faster | 419 ms | 87 MB |
| `kotlinc (cold JVM)` | 14,184 ± 544 ms | 67.1x slower | 41,728 ms | 952 MB |

- kotlinc in a warm JVM (median of the last 10 of 20 runs in one JVM): 2148 ms
- Class files: kc 108, kotlinc 120
