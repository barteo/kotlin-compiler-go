# kc vs kotlinc on kotlinpoet

square/kotlinpoet 2.4.0's JVM library: 39 files, 8696 lines (commonMain and jvmMain).

- Machine: Darwin arm64, 8 CPUs
- go version go1.27.2 darwin/arm64
- openjdk version "21.0.12.1" 2026-08-18
- info: kotlinc-jvm 2.4.21 (JRE 21.0.12.1)
- kotlinc JVM options: -Xmx8g
- Cooldown before each run (outside timing): 10 s

| Command | Wall time | vs `kc -j 1` | CPU time (user+sys) | Peak memory |
|:---|---:|---:|---:|---:|
| `kc -j 1` | 152 ± 4 ms | — | 280 ms | 77 MB |
| `kc -j 8` | 104 ± 4 ms | 1.5x faster | 363 ms | 91 MB |
| `kotlinc (cold JVM)` | 7,004 ± 326 ms | 46.2x slower | 22,885 ms | 1,015 MB |

- kotlinc in a warm JVM (median of the last 10 of 20 runs in one JVM): 1208 ms
- Class files: kc 108, kotlinc 120
