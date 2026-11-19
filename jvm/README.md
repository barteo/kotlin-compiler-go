# JVM side of kc's Analysis API

This directory holds the JVM half of kc's implementation of the Kotlin
Analysis API (AA) 2.4.21. Its first client is KSP2. See
[docs/analysis-api.md](../docs/analysis-api.md) for the design and progress.

| Directory | What |
|---|---|
| `aa-api/` | `build.sh` builds the AA 2.4.21 public API jar from the Kotlin sources at tag `v2.4.21`, compiled against `org.jetbrains.kotlin:kotlin-compiler:2.4.21` from Maven Central |
| `libkc/` | the Java side of the JNI bridge to `cmd/libkc`, its stress test and its test in several class loaders |
| `ksp/` | `build.sh` checks out kc's fork of KSP2 (google/ksp's main, past release 2.3.13, and kc's commits, on branch [`kc-backend` of barteo/ksp](https://github.com/barteo/ksp/tree/kc-backend)) and compiles the `kotlin-analysis-api` module against the `aa-api` jar |
| `detekt/` | detekt 2.0.0-alpha.6 on kc: `build.sh` builds detekt-cli from the sources of detekt's fork ([`barteo/detekt`, branch `kc-analysis-api`](https://github.com/barteo/detekt/commits/kc-analysis-api)), against `aa-api` and `kc-aa`; `run.sh` runs it; `build.sh test` checks its findings on a fixture, in CI ([docs/detekt.md](../docs/detekt.md)) |
| `kc-aa/` | the `kc` KSP backend: the Analysis API answered by kc through `libkc`; `build.sh test` runs KSP's processing loop on it over the slice, Moshi and Dagger projects; `build.sh stock` also runs them on stock KSP; both run in CI (`.github/workflows/analysis-api.yml`); `loader/` is the class KSP's Gradle plugin calls, which keeps kc's class path apart from the processors'; `gradle.sh` runs the KSP tasks of Gradle builds on kc in one daemon and compares their output with stock KSP's |
