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
- `libkc.so` is loaded into the Gradle daemon's JVM and exposes a single JNI method, which `KcNative.call(op, request) → byte[]` calls.
- A JVM lets one class loader load a native library, and a Go library can't be unloaded, while a daemon makes a class loader per KSP class path and drops it when a build ends. So the JNI method is `KcBridge`'s, a class the first `KcNative` defines in a class loader of its own that sees the JDK only. A daemon thread keeps it for the life of the JVM, and each later `KcNative`, in whichever class loader, finds it through that thread and calls it with a method handle. The class loaders share the Go side's state: its library cache, and one table of handles.
- When a class loader is collected, the thread closes the sessions and models it left open (`KcNative.atUnload`).
- KSP's Gradle plugin loads KSP's class path in one class loader, the processors in a child of it, and calls `KSPLoader` in the first. Stock KSP's class path can be a processor's parent because its Analysis API jar renames every package it bundles. kc's can't: a processor that brings its own Guava or kotlinx.serialization would get half of kotlin-compiler's or the KSP fork's (Dagger fails with `IllegalAccessError` in `com.google.common.graph`, Room's schema export with `AbstractMethodError` in a serializer). So `jvm/kc-aa/loader` is a `KSPLoader` in a jar of its own, which goes on KSP's class path with what a processor shares with KSP (KSP's API and configuration classes, the Kotlin standard library, kotlinx.coroutines), and it loads the KSP fork, kc-aa and their libraries in a class loader below its own, which no processor sees. Renaming the packages instead would need a relocating build of some 30,000 classes, with the IntelliJ core's names in strings and XML, and would run a different jar than the tests do. The loader runs KSP in a thread that ends with the run, so the thread local kotlin-compiler leaves behind (below) does not depend on what the build tool does with its threads (Gradle 9.9.1 ends its worker threads with the build), loads `libkc.so` from its own directory, and runs stock KSP in place of a failed run when the build gives it stock KSP's class path. It then runs KSP through kc-aa's `KcRun`, whose logger hands the loader the errors of the run in place of printing them, so that the errors the processors report on both are in the build log once, as stock KSP's. `tools/gradle/use-kc.init.gradle.kts` puts it on the KSP tasks' class path (`KC_KSP`), and `jvm/kc-aa/gradle.sh` tests it in a daemon.
- Go objects are opaque `int64` handles into a per-`Model` table, and Go holds
  no JVM references.
- A panic surfaces as a `KcPanicException`, not a crash: the JNI method returns a status and a message, and `KcNative` throws its own class loader's exception.
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

## kotlin-inject and kotlin-inject-anvil

kotlin-inject 0.9.0 and kotlin-inject-anvil 0.1.7 use parts of KSP that Moshi, Dagger, Room and Hilt leave alone: typealiases as qualifiers and as function types, `asMemberOf` on inherited members, `getFunctionDeclarationsByName`, `hasDefault`, KClass and array annotation arguments, `getDeclarationsFromPackage`, and components generated in one round whose supertypes later rounds read. Their output on kc matches stock KSP's because kc does the following, as FIR does:

- Source roots: Java files are read from every source root, Kotlin files from the Java roots too, and each module lists its files in the order its directories list them.
- Function classes (`Function1<A, R>` written out) are function types.
- A data object has no `copy`.
- A library Kotlin class lists its functions before its properties.
- Built-in classes get `java.io.Serializable` last. Arrays are `Cloneable` and `Serializable` and have `clone()`.
- A scope's callables come grouped by name, functions first.
- Data classes, data objects and value classes have the generated `equals`, `hashCode` and `toString`.
- A member Kotlin generates in a source class (an implicit constructor, a data class's `componentN` and `copy`) is in its class's file.
- A library property that overrides another is an override.
- A data class's `copy` parameters carry its constructor parameters' annotations.
- A library annotation's vararg parameter defaults to an empty array.
- A class that implements an interface by delegation has the delegated members.
- An annotation written through a typealias has that typealias as its type's abbreviation.

`build.sh test` and `stock` run both processors over `test/inject/src` (jars in `inject-jars.sh`) and compare with `test/inject/want`, recorded from stock KSP. `build.sh stock` also runs `test/dump/Dump.kt`, a processor that writes what KSP says about each declaration in `test/dump/src` and the order of the files, on both backends, and diffs the two outputs. Files are listed in directory order, which depends on the file system, so there is no recorded want.

## Hilt on Android

Hilt's Android processors (`@AndroidEntryPoint`, `@HiltAndroidApp`, `@HiltViewModel`) read class literals from class files and from the Java sources Hilt generates, and a build lists paths that don't exist. Their output on kc matches stock KSP's because kc does the following, as stock KSP does:

- A library path or source root that doesn't exist is left out.
- A class literal in a class file is read as its loader reads it. For a Java declaration's annotation and for any annotation parameter's default, `Void.class` is `java.lang.Void` (only `void.class` is `kotlin.Unit`), a collection is its mutable Kotlin class, an array's element is a platform type, and a generic class's type is a raw type: it prints with arguments and KSP lists none. For a Kotlin declaration's annotation, a collection is its read-only class and a generic class's type has no arguments.
- Java PSI finds the module's Kotlin source classes, so a class literal in a Java source (`@ClassKey(Screen.class)`) names the Kotlin class, also one that an earlier round generated. The classes Java PSI finds have names and nested classes only.
- Java PSI finds the classes of the Java sources a processor generated, and the packages that only generated sources are in, so a generated Java source's imports and class literals name the classes earlier rounds generated (Hilt's `@ComponentTreeDeps` lists the classes of `hilt_aggregated_deps`).
- The check of Java sources that KSP runs after a processor's error has the IntelliJ extension point and registry key that Java's type inference reads.

`build.sh test` and `stock` run Hilt's processors over `test/hilt-android/src` against hilt-android's class files and stand-ins for the Android classes (`test/hilt-android/stubs`), a three-round processor over `test/rounds/src`, a failing processor over `test/errors/src` (on kc only: stock KSP 2.3.13 fails in that check), and a processor that lists the members of a library class whose class file names a class missing from the class path, as an Android library's implementation dependency is missing from its consumers' (`test/missing`).

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
- **KSP member coverage.** The fork's bytecode uses 324 AA members; kc
  answers 221, answers 23 for some of its classes, leaves 4 to stubs, 71
  are the API's own and 5 are of types kc never makes
  (`jvm/kc-aa/members.txt`, `build.sh members`).
- **Aliases inside a class path typealias's expansion** are expanded, not
  kept as abbreviations: its right side isn't read. A source typealias's
  are kept.
- **One libkc file per daemon.** The class loaders of a JVM share one libkc (above), so they must name the same file, by path, size and modification time: another one, as after an upgrade of kc, fails with `UnsatisfiedLinkError` until the daemon restarts. Loading it next to the first would put a second Go runtime into the process. The bridge between them is not versioned either: a change to `KcBridge`'s methods needs a new JVM.
- **What keeps a dropped class loader.** libkc closes a class loader's sessions and models once the JVM has collected it, which the JVM does not do while something holds it. The IntelliJ core leaves a thread local on the thread that ran KSP (`KotlinBinaryClassCache`), AWT keeps the context class loader of the first thread that used it, and the JDK's method handle caches hold classes by soft references until memory runs short. `jvm/kc-aa/test/Loaders.java` runs KSP on threads that end, uses AWT first, and has the collector clear soft references.
- **Incremental checking.** An edit that changes only bodies checks
  only the edited files: 25 ms for a one-file edit on
  octi-server, down from 45. Every file is still lowered and has its
  written signatures resolved. An edit that changes declarations checks
  the whole module.
- **Class literals of Kotlin library annotations.** kotlinc 2.4 also writes a declaration's annotations into its metadata, and stock KSP reads them there: `Nothing::class` is `kotlin.Nothing` and `MutableList::class` is `MutableList`. kc reads the class file's annotations, where both are Java's (`java.lang.Void`, `java.util.List`). Stock KSP drops an array literal's dimension (`Array<String>::class` is `kotlin.String`); kc keeps it.
- **Kotlin members in Java PSI.** Java PSI resolves a Kotlin source class but none of its members, so the check of Java sources that KSP runs after a processor's error reports each call of a Kotlin member from Java as an unresolved reference. Stock KSP's light classes have the members.
- **JDK classes that Kotlin maps.** `java.util.Map` as a class has no declarations on kc and `java.util.Map.Entry` isn't found, so `Map.Entry.class` in a Java source's annotation is an error type.
- **`expect` declarations.** One without an actual declaration in the sources is a declaration of its file, with its members expect, without an implicit constructor and without backing fields, as on stock KSP: a processor that generates the actual one finds it, as Room does for the object a database's `@ConstructedBy` names (`test/room/src`, `test/dump/src/d/Expects.kt`). One whose actual declaration is in the sources is left out, and names resolve to the actual one: stock KSP lists both, so a processor that looks for the expect one finds none on kc.
- **Built-in members.** Members the JDK maps onto Kotlin's built-ins (`spliterator`, `stream`) are among a built-in class's declarations on kc, and only in its member scope on stock KSP. Built-in members lack some annotations (`IntrinsicConstEvaluation`, `SinceKotlin`), an enum's `getAllFunctions` lacks `java.lang.Enum`'s (`getDeclaringClass`, `describeConstable`, `finalize`), and `Array.size`'s `hasBackingField` differs.
