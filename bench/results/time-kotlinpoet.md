# kc vs kotlinc on kotlinpoet

square/kotlinpoet 2.4.0's JVM library: 39 files, 8696 lines (commonMain and jvmMain).

- Machine: Linux x86_64, 4 CPUs
- go version go1.27.2 linux/amd64
- openjdk version "21.0.12.1" 2026-08-18
- info: kotlinc-jvm 2.4.21 (JRE 21.0.12.1+1-1-24.04.4-Ubuntu)
- kotlinc JVM options: -Xmx8g
- Cooldown before each run (outside timing): 0 s

| Command | Wall time | vs `kc -j 1` | CPU time (user+sys) | Peak memory |
|:---|---:|---:|---:|---:|
| `kc -j 1` | 300 ± 18 ms | — | 484 ms | 88 MB |
| `kc -j 4` | 211 ± 10 ms | 1.4x faster | 557 ms | 100 MB |
| `kotlinc (cold JVM)` | 17,765 ± 1,797 ms | 59.2x slower | 53,732 ms | 944 MB |

- kotlinc in a warm JVM (median of the last 5 of 10 runs in one JVM): 3438 ms
- Class files: kc 108, kotlinc 120
