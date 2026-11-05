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
| `kc -j 1` | 143 ± 8 ms | — | 267 ms | 76 MB |
| `kc -j 8` | 97 ± 2 ms | 1.5x faster | 345 ms | 85 MB |
| `kotlinc (cold JVM)` | 6,864 ± 176 ms | 48.0x slower | 21,387 ms | 988 MB |

- kotlinc in a warm JVM (median of the last 10 of 20 runs in one JVM): 1230 ms
- Class files: kc 108, kotlinc 120
