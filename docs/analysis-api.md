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
  method, `KcNative.call(op, request) → byte[]`. kc-aa calls it through
  `KcBridge`, which loads `KcNative` once per JVM, in a class loader that
  outlives KSP's (see "One libkc per daemon" below).
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
- KSP2 uses a shaded AA and reaches into FIR internals, so it is forked:
  branch [`kc-backend` of barteo/ksp](https://github.com/barteo/ksp/tree/kc-backend),
  google/ksp's main (past release 2.3.13) with one commit
  per change.
- Session creation sits behind a `KspBackend` (chosen with
  `-Dksp.backend=aa|kc`).
- FIR-internal queries sit behind `KcExtras`.
- The stock AA backend stays as `aa` and serves as the whole-run fallback.

## Gaps and open questions

- **Slice simplifications.** These go when the symbol model and the
  queryable session bring the facts in:
  - Accessors read their flags from PSI modifiers, not from `kc`.
  - `dropCaches` does nothing. Dropping PsiManager's caches needs IntelliJ
    extension points that the core environment doesn't register, and sources
    don't change between rounds.

- **Stock backend not built here.** The jar leaves out `backend/fir/`,
  `standalone/` and the Java stubs in `src/main/java`, because they need the
  AA implementation (FIR, low-level API, standalone), whose artifacts are not
  reachable from this build host.
  - The stock `aa` backend therefore has not been compiled or run against
    2.4.21.
  - Upstream's Gradle build still pins `aaKotlinBaseVersion=2.4.20-dev-2407`.
    Building the patched tree with Gradle needs the 2.4.21 AA artifacts.
- **No serialization plugin.** The fork is compiled without the
  kotlinx-serialization compiler plugin. Nothing needs it: the
  incremental caches (`PersistentMap`) use hand-written `KSerializer`s
  for their keys and values (`FileSerializer`, `SymbolSerializer`), and
  they run on kc.
- **Java file manager.** `ResolverAAImpl.javaFileManager` casts the project's
  `JavaFileManager` service to `KotlinCliJavaFileManagerImpl`. The `kc`
  backend gets one from `KotlinCoreEnvironment`, whose setup is part of the
  cold-start cost measured in `bench/ksp`.
- **KSP member coverage.** The fork's bytecode uses 310 AA members; kc
  answers 204, answers 28 for some of its classes, leaves 4 to stubs, 69
  are the API's own and 5 are of types kc never makes
  (`jvm/kc-aa/members.txt`, `build.sh members`).
- **Aliases inside a class path typealias's expansion** are expanded, not
  kept as abbreviations: its right side isn't read. A source typealias's
  are kept.
- **One libkc per daemon.** A JVM lets one class loader load a native
  library, and the KSP plugin gives kc-aa a new class loader for each KSP
  class path and, from 2.3.13, for each build. So `KcBridge` loads
  `KcNative`, and libkc, once per JVM, in a class loader of its own
  (parent: the platform class loader) that a private `MBeanServer` keeps
  for the life of the JVM; each later kc-aa class loader finds it there.
  (System properties would not do: the Gradle daemon restores them after
  each build.) What remains:
  - libkc, its Go runtime and its caches outlive the builds of a daemon;
    a rebuilt `libkc.so`, or another `kc.lib`, takes a new daemon
    (`gradle --stop`).
  - A kc-aa of another ABI fails with an `UnsatisfiedLinkError` naming
    both ABIs until the daemon restarts.
- **Incremental checking.** An edit that changes only bodies checks
  only the edited files: 25 ms for a one-file edit on
  octi-server, down from 45. Every file is still lowered and has its
  written signatures resolved. An edit that changes declarations checks
  the whole module.
