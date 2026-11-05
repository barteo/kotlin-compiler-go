# Kotlin Analysis API on kc

This is the work toward an implementation of the Kotlin Analysis API (AA),
version 2.4.21, backed by `kc`. Its first client is KSP2, which runs on the
AA's standalone mode today.

## Why

KSP2 was profiled on a Dagger project (octi-server: 60 files, 7.7k lines).

- A warm `kspKotlin` takes about 0.91 s, and about 86% of that is AA work: FIR
  lazy resolve 38%, library classes 27%, raw FIR 15%, IntelliJ core 10%.
- A cold run takes 10–15 s.
- `kc` parses the same sources in 29 ms and indexes the class path in 44 ms.

The aim is to run processors such as Dagger, Moshi and Room, unchanged, on a
KSP fork whose AA is answered by `kc`. The targets on octi-server are a warm
`process` under 0.3 s and a cold run under 2 s.

## Design

```
KSP fork (jvm/ksp)                JVM: jvm/kc-aa (Kotlin)                Go: libkc.so
  KotlinSymbolProcessing  ──►  KaKcSession : KaSession            JNI   cmd/libkc  (C ABI, panic-safe)
  ResolverAAImpl, KS*Impl      components, KaKc*Symbol, KaKc*Type ◄──►    └─ analysis/ka  (Session, Model)
  (FIR internals → KcExtras)   handle cache, lifetime token              └─ internal/check, classpath, kmeta…
                               real KtFile PSI (kotlin-compiler 2.4.21)
                               ↔ psi node IDs (preorder index)
```

**Rounds and snapshots.**
- Each KSP round updates a Go `Session` with the sources plus the files
  generated so far, which yields an immutable `Model`.
- The class-path library is cached in the daemon, keyed by class path and
  mtimes.
- A signatures-only mode skips checking bodies, except where an omitted
  return type has to be inferred.

**The bridge.**
- `libkc.so` is loaded into the Gradle daemon's JVM and exposes a single JNI
  method, `KcNative.call(op, request) → byte[]`.
- Go objects are opaque `int64` handles into a per-`Model` table, and Go holds
  no JVM references.
- A panic surfaces as a `KcPanicException`, not a crash.
- A call costs about 0.5 µs, so a call returns a whole record (flags, names,
  supertypes, annotation headers) and the JVM caches it per handle.

**PSI.** KSP needs real `KtFile` and `KtElement` objects. The JVM side parses
the files KSP touches with kotlin-compiler 2.4.21's PSI and maps them to
`kc`'s `psi` nodes by preorder index. The two trees have the same shape
([psi.md](psi.md)), and a per-file check of the kind sequence guards the
mapping.

**KSP fork.**
- KSP2 uses a shaded AA and reaches into FIR internals, so it is forked as a
  patch series in [`jvm/ksp/patches`](../jvm/ksp/patches).
- Session creation sits behind a `KspBackend` (chosen with
  `-Dksp.backend=aa|kc`).
- FIR-internal queries sit behind `KcExtras`.
- The stock AA backend stays as `aa` and serves as the whole-run fallback.

## Milestones

| | Milestone | Status |
|---|---|---|
| M0.1 | JNI and Go runtime spike: `cmd/libkc`, `internal/kcabi`, `jvm/libkc/test.sh` | done |
| M0.2 | AA 2.4.21 API jar from the Kotlin sources: `jvm/aa-api/build.sh` | done |
| M0.3 | KSP fork compiles against the unshaded AA 2.4.21 API: `jvm/ksp` | done, see gaps |
| M0.4 | Vertical slice: list annotated source classes through a `kc` KspBackend | next |
| M1 | `psi` on Kotlin 2.4.21 | done before this work |
| M2 | Error tolerance, `types.Invalid` payload, extension-vs-top-level overload bug, `@Contextual`/`@EncodeDefault` | |
| M3 | Richer symbol model: visibility and modality enums, flags, origins, library annotation arguments, Java parameter names | |
| M4 | Queryable session `analysis/ka`: lookups, scopes, relations, overrides | |
| M5 | Bridge: `cmd/libkc` operations, `tools/kagen` wire format, `jvm/kc-aa` KaSession | |
| M6 | KSP fork integration: the `kc` backend, IC lookups, hot paths | |
| M7 | Differential conformance against stock KSP, benchmark, CI | |

### M0.1 – JNI spike

`jvm/libkc/test.sh` builds `libkc.so` and runs a stress test twice, plainly
and with the JDK's `libjsig` preloaded. Each run makes:
- 1M calls on 32 threads under GC pressure, with a Go panic every 1000th
  call;
- 2M implicit null checks and stack overflows (the JVM's own `SIGSEGV` uses)
  after the Go runtime has installed its signal handlers.

Both runs pass. A 16-byte echo costs 460–680 ns on one thread.

Not yet done: a run inside a real Gradle daemon, and a second JDK.

### M0.2 – API jar

The AA artifacts live on JetBrains' Maven repository, so `jvm/aa-api/build.sh`
builds the API jar from the Kotlin sources at `v2.4.21` instead:
- It does a sparse checkout of `analysis-api`, `analysis-internal-utils`,
  `analysis-api-platform-interface`, and the plain value classes from
  `analysis-api-impl-base` that KSP constructs (`KaAnnotationImpl`,
  `KaBaseNamedAnnotationValue`, `KaBase*TypeProjection`).
- It compiles them against `org.jetbrains.kotlin:kotlin-compiler:2.4.21` from
  Maven Central.

### M0.3 – KSP fork

`jvm/ksp/build.sh` builds the fork:
1. It checks out KSP at tag 2.3.9.
2. It applies the patches.
3. It compiles the `kotlin-analysis-api` module (the processing loop, the KS*
   symbol model and the command line) against the M0.2 jar. The result has
   275 classes.

The patches:

1. **Port to Analysis API 2.4.21.** KSP 2.3.9 was built against
   2.4.20-dev-2407. Four call sites change:
   - `mapToJvmType` → `mapToJvmTypeDescriptor`;
   - the new home of `getEffectiveVariance`;
   - `KaAnnotationImpl` lost its use-site-target parameter;
   - `EDT.updateEdt`, which kotlin-compiler's IntelliJ core doesn't have, is
     now called reflectively or replaced by setting the field.
2. **Move FIR-internal queries behind `KcExtras`.** These are type-mapping
   modes, raw types, abbreviated typealiases, package-part and container
   classes, Java annotation defaults, phase resolution, library module names
   and the binary-class cache. The stock code moves to `backend/fir/FirExtras`.
3. **Put session creation behind `KspBackend`.** The processing loop works on
   a `KspBackendSession`. `backend/fir/StandaloneBackend` (`aa`) is the old
   session code, moved, and is found with `ServiceLoader`.

## Gaps and open questions

- **Stock backend not built here.** The jar leaves out `backend/fir/`,
  `standalone/` and the Java stubs in `src/main/java`, because they need the
  AA implementation (FIR, low-level API, standalone), whose artifacts are not
  reachable from this build host.
  - The stock `aa` backend therefore has not been compiled or run against
    2.4.21.
  - Upstream's Gradle build still pins `aaKotlinBaseVersion=2.4.20-dev-2407`.
    Building the patched tree with Gradle needs the 2.4.21 AA artifacts.
- **No serialization plugin.** The fork is compiled without the
  kotlinx-serialization compiler plugin, so its `@Serializable` classes
  (incremental-compilation caches) have no serializers yet. M6 fixes this, or
  turns IC off for the first `kc` runs.
- **Java file manager.** `ResolverAAImpl.javaFileManager` casts the project's
  `JavaFileManager` service to `KotlinCliJavaFileManagerImpl`, so the `kc`
  backend must register one (M6) or the cast must move behind `KcExtras`.
- **KSP member coverage.** KSP2 uses about 308 AA members: `KaSession` 43,
  symbols 157, types 32, components 22, annotations 14, project structure
  10, signatures 6, scopes 4. None are implemented on `kc` yet. M5 tracks
  coverage through a generated `members.txt`.
