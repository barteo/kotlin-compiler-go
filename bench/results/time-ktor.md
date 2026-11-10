# kc vs kotlinc on Ktor

Ktor 3.6.0's ktor-io, ktor-utils and ktor-http JVM libraries: 236 files, 23774 lines.
Modules compile sequentially against the same compiler's preceding outputs.
Cold kotlinc starts one JVM per module; warm kotlinc runs all three in one JVM.
These are compiler timings, excluding Gradle, tests and AtomicFU bytecode transformation.

- Machine: Darwin arm64, 8 CPUs
- Revision: 1d1d79a
- go version go1.27.2 darwin/arm64
- openjdk version "21.0.12.1" 2026-08-18
- info: kotlinc-jvm 2.4.21 (JRE 21.0.12.1)
- kotlinc JVM options: -Xmx8g
- Runs: 10 kc builds (plus one warm-up per command), 5 cold kotlinc builds, 20 builds in one JVM
- Cooldown before each complete build (outside timing): 10 s
- Options: -jvm-default=enable, kotlinx.serialization; kotlinc language 2.3, API 2.2, explicit API mode, JVM target 1.8 and multiplatform fragments
- Dependencies: atomicfu 0.33.0, coroutines 1.11.0, kotlinx-io 0.9.1, serialization 1.11.0, SLF4J 2.0.19

| Command | Wall time | vs `kc -j 1` | CPU time (user+sys) | Peak memory |
|:---|---:|---:|---:|---:|
| `kc -j 1` | 366 ± 4 ms | — | 680 ms | 82 MB |
| `kc -j 8` | 244 ± 6 ms | 1.5x faster | 899 ms | 95 MB |
| `kotlinc (cold JVM)` | 19,966 ± 176 ms | 54.5x slower | 75,228 ms | 1,081 MB |

- kotlinc in a warm JVM (median of the last 10 of 20 complete builds): 2267 ms
- Class files: kc 671, kotlinc 668
