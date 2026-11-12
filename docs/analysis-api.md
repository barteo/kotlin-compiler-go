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
- KSP2 uses a shaded AA and reaches into FIR internals, so it is forked:
  branch [`kc-backend` of barteo/ksp](https://github.com/barteo/ksp/tree/kc-backend),
  google/ksp's main (past release 2.3.13) with one commit
  per change. Until 2026-11-11 the fork was a
  patch series in `jvm/ksp/patches`, whose patches are the branch's
  commits in order.
- Session creation sits behind a `KspBackend` (chosen with
  `-Dksp.backend=aa|kc`).
- FIR-internal queries sit behind `KcExtras`.
- The stock AA backend stays as `aa` and serves as the whole-run fallback.

## Gaps and open questions

- **Slice simplifications.** These go when the symbol model and the
  queryable session bring the facts in:
  - Accessors read their flags from PSI modifiers, not from `kc`.
  - ~~There are no backing-field symbols, so `@field:` annotations are dropped.~~
    Fixed.
  - ~~Library type aliases and top-level callables aren't known.~~ Fixed.
  - ~~Java sources are ignored.~~ Fixed.
  - ~~The PSI join checks each node's PSI class, not yet the whole kind
    sequence.~~ Fixed.
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
- ~~**Generic annotations' type arguments** (`@A<String>`) aren't in the
  model.~~ They are.
- ~~**Aliases inside an alias's expansion** are expanded, not kept as
  abbreviations.~~ A source typealias's are kept; a
  class path typealias's right side isn't read.
- **One libkc per daemon.** A JVM lets one class loader load libkc, and
  the KSP plugin makes a class loader per KSP class path, so a daemon runs
  kc for one KSP class path only; another fails with
  `UnsatisfiedLinkError` until the daemon restarts. Loading a
  copy of the library from another path would load a second Go runtime
  into the process, which is untested; failing over to the `aa` backend
  needs the stock backend (above).
- **Incremental checking.** An edit that changes only bodies checks
  only the edited files: 25 ms for a one-file edit on
  octi-server, down from 45. Every file is still lowered and has its
  written signatures resolved. An edit that changes declarations checks
  the whole module.
