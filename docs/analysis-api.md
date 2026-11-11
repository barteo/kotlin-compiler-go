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
| M0.1 | JNI and Go runtime spike: `cmd/libkc`, `internal/kcabi`, `jvm/libkc/test.sh` | done; also in a Gradle daemon and on JDK 17 (step 21) |
| M0.2 | AA 2.4.21 API jar from the Kotlin sources: `jvm/aa-api/build.sh` | done |
| M0.3 | KSP fork compiles against the unshaded AA 2.4.21 API: `jvm/ksp` | done, see gaps |
| M0.4 | Vertical slice: list annotated source classes through a `kc` KspBackend: `jvm/kc-aa/build.sh test` | done, see gaps |
| M1 | `psi` on Kotlin 2.4.21 | done before this work |
| M2 | Error tolerance, `types.Invalid` payload, extension-vs-top-level overload bug, `@Contextual`/`@EncodeDefault` | done (two items moved, see M2) |
| M3 | Richer symbol model: visibility and modality enums, flags, origins, library annotation arguments, Java parameter names | done: source types, flags and modality (step 1); KaTypes on the JVM (2); annotation arguments (3); library classes (4); inherited members, overrides, subtyping (5); packages, library top-level declarations, alias abbreviations (6); JVM type mapping (7); backing fields (8); Java sources in the module (15). Moshi, Dagger, Room and Hilt byte-identical to stock KSP 2.3.6 (steps 7–9, 14) |
| M4 | Queryable session `analysis/ka`: lookups, scopes, relations, overrides | done: `ka.Session.Update` and `Model` with `Class` and `Package` lookups on the class path (steps 4, 6), shared library (11), lookups recorded for incremental processing (16); scopes, relations and overrides are answered on the JVM over the model (step 5) |
| M5 | Bridge: `cmd/libkc` operations, `tools/kagen` wire format, `jvm/kc-aa` KaSession | done: `internal/kaops` operations (open, update, close, class, package), the model wire format, written by hand in `internal/kaops` and `KcModel.kt` (no `tools/kagen` generator was needed), the `KaSession` proxy and symbol, type and annotation classes; member coverage in `jvm/kc-aa/members.txt` (step 19) |
| M6 | KSP fork integration: the `kc` backend, IC lookups, hot paths | done: KSP's own test suite on the kc backend, `build.sh ksp` (step 15): 134 of 136 pass, the other 2 disabled in KSP, Java sources in the main module included; IC lookups recorded by kc's checker and replayed into KSP's tracker, checked against stock KSP on a fixture (step 16). Open: the whole-run fallback to `aa` (see gaps) |
| M7 | Differential conformance against stock KSP, benchmark, CI | in progress: `bench/ksp` (steps 11, 17): on octi-server warm 773 ms on stock KSP, 176 ms on kc; cold 6.2–6.4 s and 3.5–3.6 s (step 11's CDS archive: 1.9–2.1 s on kc); CI: `.github/workflows/analysis-api.yml` (step 12) |

### M0.1 – JNI spike

`jvm/libkc/test.sh` builds `libkc.so` and runs a stress test twice, plainly
and with the JDK's `libjsig` preloaded. Each run makes:
- 1M calls on 32 threads under GC pressure, with a Go panic every 1000th
  call;
- 2M implicit null checks and stack overflows (the JVM's own `SIGSEGV` uses)
  after the Go runtime has installed its signal handlers.

Both runs pass. A 16-byte echo costs 460–680 ns on one thread.

**Step 21: in a Gradle daemon, and on JDK 17.** `jvm/libkc/test.sh gradle`
runs the same test inside a Gradle daemon (`jvm/libkc/gradle`, Gradle
8.14.3), the way the KSP Gradle plugin runs KSP2: a worker without
isolation loads the classes with a `URLClassLoader` that a static of the
plugin's keeps by class path (`KspAATask`'s `isolatedClassLoaderCache`),
so a later build of the daemon with the same KSP class path reuses it.
Each of two daemons, one plain and one started with `libjsig` preloaded,
runs three builds:
1. loads the library and runs the test;
2. runs it again with the class loader kept from build 1;
3. runs it with another class path, so a new class loader, which must
   fail with `UnsatisfiedLinkError` ("already loaded in another
   classloader"): a JVM lets one class loader load a native library.

| | plain | with libjsig | Gradle daemon, builds 1 and 2 | daemon with libjsig |
|---|---|---|---|---|
| JDK 21.0.12 | pass (step M0.1) | pass | pass, pass | pass, pass |
| JDK 17.0.20 | pass, 430 ns | pass, 479 ns | pass, pass | pass, pass |

Every run made its 1M calls with 992 panics caught, 2M implicit null
checks and 20 recovered stack overflows; the echo cost 390–700 ns. JDK 17
came from Ubuntu's `openjdk-17-jdk-headless`. The JDK warns, with
`libjsig`, that chaining through `signal()` is deprecated: the Go runtime
installs its handlers with `sigaction`, so the warning is from the JVM's
own check, not a failure.

What build 3 means for kc: within one daemon, the kc backend works for
every build whose KSP class path is the same one, and a build with
another KSP class path (another KSP or kc version, or two projects whose
KSP class paths differ) can't load libkc and fails until the daemon
restarts. See gaps.

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
4. **Find source file facades through `KcExtras`.** `getOwnerJvmClassName`
   asked `KtFile.findFacadeClass()`, which needs light classes. Backends
   that don't have them answer `KcExtras.fileFacadeClass` instead; FIR's
   still asks the light class.

### M0.4 – Vertical slice

`jvm/kc-aa/build.sh test` runs KSP's processing loop on the `kc` backend over
a 3-file project (`jvm/kc-aa/test/slice`). The processor
(`test/PrintAnnotated.kt`) prints every symbol that `getSymbolsWithAnnotation`
returns, and the output must match `want.txt`: classes, nested classes, an
object, an enum entry, functions, a getter (`@get:`), value and type
parameters, a constructor `val`, and a file annotation.

**Go.**
- `analysis/ka` checks the module and builds a declaration `Model`. A
  declaration has its kind, name, class ID, visibility, flags, annotations
  (class ID, use-site target, psi node), type and value parameters, and
  members.
- `internal/kaops` serves it over the bridge: open, update and close,
  operations 10–12.
- A declaration's psi node is its preorder index in `kc`'s lossless tree,
  counted over every node, leaves included.

**JVM (`jvm/kc-aa`).**
- `KcBackend` is a `KspBackend` found with `ServiceLoader`. Its session:
  - creates a `KotlinCoreEnvironment`, which also gives `ResolverAAImpl` the
    `KotlinCliJavaFileManagerImpl` it casts to;
  - registers a `KaSessionProvider`;
  - opens a `kc` session on the module's Kotlin sources and libraries.
- Each update decodes the model and joins it with kotlin-compiler's PSI of
  the same files, through the preorder index, into a `KcRound`. A round is the
  symbols, an index by PSI, and an index by pointer key. The next update
  replaces the round and invalidates its lifetime token.
- `KaSession` is a `java.lang.reflect.Proxy`. A call goes to the `KcAnalysis`
  method with the same name and parameter types; an extension member takes
  its receiver first.
  - Implemented: `getSymbol` for files, declarations and classes;
    `findClass`, `findClassLike`, `findTypeAlias`; the file, declared-member,
    static-declared-member and combined scopes; `containingSymbol` and
    `containingFile`; `KtTypeReference.type` for annotation types;
    `fullyExpandedType`; `isMarkedNullable`; and `buildClassType`.
  - Any other abstract member throws
    `UnsupportedOperationException("kc: KaSession.name(types)")`.
- The symbol classes (`KcSymbols.kt`) extend stubs that `gen/GenStubs.kt`
  generates from the API jar. A stub member throws
  `UnsupportedOperationException("kc: Class.member")`, so a run names the
  first missing member.

### M2 – Error tolerance

`ka.Update` no longer stops at the first error. The errors become the
model's `Diagnostics`, which travel over the bridge, and the JVM side prints
them with `-Dkc.diagnostics`.

- **Lowering.** A syntax error used to drop the whole top-level declaration
  around it. Now a class keeps its members and loses only the one with the
  error, as the stock PSI does. Every class body already lowers each member
  on its own and reports that member's errors. A file with a psi error
  element still always gets at least one diagnostic, so the compiler can
  never accept broken code.
- **Checking.** The checker already went on after resolution errors.
  Unresolved names leave the rest of the module resolved, and an unresolved
  annotation reaches the JVM as a `KaClassErrorType`, which KSP skips.
- **Checker crashes.** A panic in a `checkBodies` worker is now raised again
  on `CheckContext`'s caller, with the worker's stack, instead of killing the
  process. If the checker crashes on a module with syntax errors, `ka`
  checks the files without errors and models the broken ones from syntax
  alone.
- **Tests.** The slice has a file with a syntax error and an unresolved
  annotation (`test/slice/src/c/Broken.kt`), and `analysis/ka` has
  `TestErrors`.
- **Extension vs. top-level overload.** In a class, `log(TAG, INFO) { }`
  called `this.log(priority, ...)`, an `Any` extension, instead of the
  imported top-level `log(tag: String, priority, ...)`. Inside a class,
  `call` tries an extension of `this` before top-level functions, and
  `onReceiver` can't try the extension first and fall back afterwards,
  because a lambda is checked only once. The guard, `thisExtension`, used
  `mayTake`, which knows only the types of literals, locals and `this`, so a
  property argument like `TAG` fitted any parameter. Now, when the call has a
  lambda, the arguments' types decide (`takeTypes` over `argTypes`), as they
  already did for members.
  - kotlinc picks the top-level function as well.
  - This bug caused 369 of the 393 errors on octi-server. That project isn't
    in this container, so the zero-error exit hasn't been rechecked on it.
  - Test: `testdata/run/extensionVsTopLevel`, which also has the cases
    where the extension must still win.
- **Moved: the `types.Invalid` payload.** The model doesn't export types
  yet. When M3/M4 do, an error type's presentable text, which is what KSP
  shows for a `KaClassErrorType`, comes from the declaration's psi
  `TYPE_REFERENCE` node. That node is already in the model. `types.Type`,
  which is copied by value throughout the checker, then doesn't need to
  grow.
- **Moved: `@Contextual` and `@EncodeDefault`.** These are gaps in kc's
  serialization plugin codegen, and they don't touch the Analysis API path:
  `ka` runs no compiler plugins, and KSP2 sees no plugin-generated members
  either. They stay on the compiler backlog. `TestSerialization` needs
  kotlinc and the kotlinx.serialization jars, which this container doesn't
  have.

### M3/M4 – Symbol model (in progress)

**Step 1: source types, flags and modality.** `ka.Decl` now carries:
- `Modality`: final, open, abstract or sealed. It is the modality Kotlin
  implies, not only the one written: an interface member without a body is
  abstract, an override or interface member with one is open, and a private
  or top-level one is final.
- `Flags`: companion, data, inner, value, fun, expect, actual, inline,
  suspend, operator, infix, tailrec, external, const, lateinit, mutable,
  vararg, noinline, crossinline, has-default, override and reified.
- `Variance`: a type parameter's declaration-site variance.
- `Type`: a callable's return type, a property's or parameter's type (a
  vararg's element type), a type alias's expansion, or an enum entry's class.
  Omitted return and property types are the inferred ones.
- `Receiver`, `Supertypes` (with the implied `kotlin/Enum<E>`,
  `kotlin/Annotation` or `kotlin/Any`) and `Bounds`.

A `ka.Type` (`analysis/ka/types.go`) is a class type with arguments and
projections, a type parameter type, or an error type:
- Class types use Kotlin class IDs. Primitives, ranges and the Java classes
  that are Kotlin classes (`java/lang/Object` and so on) are mapped. A
  function type is `kotlin/FunctionN` or `kotlin/coroutines/SuspendFunctionN`,
  with the extension flag and the number of context parameters.
- A type parameter type points at its `TYPE_PARAMETER` node, following kc's
  substitution origins back to the declared parameter. It may be definitely
  non-null (`T & Any`).
- An error type's text is the type as written, taken from its psi node. This
  is what M2 deferred for the `types.Invalid` payload.
- Nullable and flexible (platform) are flags.

`ka` records the type of every written type reference through the
checker's `TypeRef` recorder hook, so receivers and bounds that have no symbol field
still resolve. The wire format (`internal/kaops`) and `KcModel.kt` carry all
of this. `KcSymbols.kt` now reads modality, flags and variance from kc instead
of PSI modifiers; property accessors still read theirs from PSI.

Test: `TestTypes` in `analysis/ka`, run with the JDK.

Known gaps in this step:
- Types are expanded: a use of a type alias has no abbreviation.
- An intersection type other than `T & Any` (from smart casts or inference)
  is an error type.
- An inferred platform type is nullable (`String?`), not flexible
  (`String!`), because kc picks one when it infers a declaration's type.
- Accessor flags and modality aren't in the model.
- Library declarations aren't in the model yet.

**Step 2: KaTypes on the JVM.** `jvm/kc-aa/.../KcTypes.kt` turns the model's
types into `KaType`s: class types (with qualifiers for nested classes),
function types (receiver, context parameters, suspend), type parameter types
(linked to the source `KaTypeParameterSymbol`), flexible, definitely non-null
and error types, with star and variance projections. They back:
- `returnType` on functions, constructors, properties, accessors, parameters
  and enum entries (a constructor's is its class's type); `receiverParameter`;
  `superTypes` on classes; `upperBounds` on type parameters; and
  `expandedType` on type aliases.
- `KtTypeReference.type` for any reference in a declaration's signature,
  mapped by walking the written type alongside the model's type.
- `getBuiltinTypes`, `isMarkedNullable`, `isNullable`, `withNullability`,
  `lowerBoundIfFlexible`/`upperBoundIfFlexible`, `getExpandedSymbol`,
  `isAnyType` and `directSupertypes`.

The slice (`test/slice/src/d/Types.kt`) prints each declaration's types
through KSP. Known differences from kotlinc and things still missing:
- An inferred Java return type is `String?` in kc and `String!` in kotlinc.
- ~~`directSupertypes` doesn't substitute the type's arguments.~~ Fixed in
  step 5.
- ~~`getFullyExpandedType` returns its argument, and
  `KcExtras.abbreviatedTypeAlias` returns null: there is no alias
  abbreviation yet.~~ Fixed in step 6.
- ~~`buildClassType` ignores its builder's arguments and nullability.~~
  Fixed in step 4.
- ~~Library class types know only their class ID.~~ Fixed in step 4, except
  for the Kotlin built-ins that are Java classes (see step 4).
- ~~`isSubtypeOf` and `semanticallyEquals` aren't implemented.~~ Fixed in
  step 5.

**Step 3: annotation arguments.** The checker hands the values it evaluates
for each annotation to an optional `check.ValueRecorder`. That includes
SOURCE-retention classes: the program's are checked as usual, and a
library's (`@Suppress`) are evaluated with their errors dropped, since kc
doesn't otherwise check them. `analysis/ka/values.go` turns those values
into `ka.Value`s:
- the kinds are constants of every Kotlin constant type (unsigned too),
  enum entries, class literals, nested annotations, arrays, and
  "unsupported" for values kc didn't compute;
- `Annotation.Args` holds the arguments as written, in parameter order. A
  vararg parameter without arguments is left out, as in the Analysis API.
- `Decl.Default` is the default of an annotation class's constructor
  parameter, nested-annotation defaults included.
- Each value points at its expression's psi node. Argument expressions are
  re-parsed from tokens, so they are found by position: the leftmost
  operand's line and column, mapped to the outermost expression that starts
  there.

On the JVM (`KcAnnotationValues.kt`), the values become `KaAnnotationValue`s
and `KaConstantValue`s with their `sourcePsi`. They back:
- `KaAnnotation.arguments` (nested annotations too);
- `evaluateAsAnnotationValue`, which KSP uses for defaults;
- `memberScope`, which KSP uses to find the constructor that carries those
  defaults.

The slice (`test/slice/src/e/Args.kt`) prints every argument through KSP's
`KSAnnotation.arguments`, defaults included. Known gaps:
- ~~Library annotation classes have no defaults.~~ Fixed in step 4.
- `evaluateAsAnnotationValue` answers only the expressions kc evaluated as
  annotation arguments or defaults; for any other expression it returns
  null.
- ~~`memberScope` of a source class is its declared members, with no
  inherited members or fake overrides.~~ Fixed in step 5.
- Arguments are ordered by parameter. That may differ from kotlinc's order
  when named arguments are written out of order.
- A position that doesn't line up with the psi (non-ASCII text earlier on
  the line) leaves the value without `sourcePsi`.
- Arguments of annotations on local declarations aren't in the model.

**Step 4: library classes.** A `Model` now keeps its class path open, and
`Model.Class(id)` (`analysis/ka/library.go`) builds a class-path class's
`Decl` on demand, with its members and nested classes, and caches it.
`Model.Close` closes the class path.
- **Kotlin classes** come from their `kotlin.Metadata`: constructors
  (primary flagged), properties, functions, enum entries and nested classes,
  with the flags, visibility, modality and types the metadata gives. Fake
  overrides are left out. Suspend function types become
  `SuspendFunctionN`.
- **Java classes** come from the checker's class symbol plus the class
  file's details (`classfile.ParseDetails`: annotations with their
  arguments, parameter annotations, `MethodParameters` names, annotation
  defaults, nested classes). Java types are flexible. Members are flagged
  Java, statics static, and fields are `KindJavaField`. Package visibility
  is its own value. A Java annotation class gets a primary constructor whose
  parameters are its methods, with their `AnnotationDefault`s.
- **Annotations** are read with their arguments. Unsigned values are found
  from the annotation class's parameter types. Compiler-internal ones
  (`kotlin.Metadata`, the JetBrains nullability annotations, the Java
  meta-annotations Kotlin maps) are left out of Kotlin declarations.
- **Wire:** operation 13 (`class`: model, class ID → 0 or 1 decls). The
  model handle now lives until the next update or the session's close.

On the JVM, `KcRound.libraryClass` loads a class with its outermost class
through `KcOps.cls`, and builds its symbols with the same builder as source
files, minus the psi:
- origins are `LIBRARY` or `JAVA_LIBRARY`;
- a type parameter type without a node resolves by name through the
  enclosing declarations' type parameters;
- pointers to library symbols restore by reloading the class.

They back `findClass`/`findClassLike`, class types' `symbol`, the member
and static scopes, `getContainingSymbol`, `isStatic`, Java visibilities,
and `KcExtras.javaAnnotationDefault`, which KSP asks for a library
annotation parameter's default. That fills in `KSAnnotation.arguments`
defaults for library annotations. `buildClassType` now runs its builder,
which KSP needs for a constructor's return type.

The slice prints `@Deprecated` and `@java.beans.JavaBean` arguments with
their library defaults, and the declarations of four library classes
through `Resolver.getClassDeclarationByName`: a Kotlin annotation, a generic
Kotlin class, a generic nested Java class and a Java enum. Known gaps:
- The Kotlin built-ins that are Java classes in the class path
  (`kotlin/String`, `kotlin/collections/List`) have no members: kc doesn't
  read `.kotlin_builtins`.
- Private members of Java classes, and Java enums' `values`/`valueOf`, are
  left out.
- Annotations on Kotlin properties' fields and accessors, and on
  extension receivers, aren't read.
- A `Class<?>` parameter of a Java annotation stays `java/lang/Class`, not
  `KClass`.
- Java parameter names without `MethodParameters` are `p0`, `p1`, … (FIR
  does the same for class files).
- Library top-level callables and type aliases aren't loaded yet.
- ~~Scopes have no inherited members or fake overrides; the static scope
  has no inherited Java statics.~~ Fixed in step 5.
- A constructor's `typeParameters` are its class's; FIR may differ.

**Step 5: inherited members, overrides and subtyping.** All of it runs on
the JVM, over the declarations and KaTypes `KcRound` already holds, so
source and library classes share one mechanism and the wire is unchanged.
The one Go change is to the built-ins (`analysis/ka/library.go`):
`kotlin/Any` is synthesized with its constructor and `equals`, `hashCode`
and `toString`; the built-ins that are Java classes get the mapped class's
supertypes, kept to `kotlin/*` ones (`String : Comparable<String>,
CharSequence`, `Int : Number, Comparable<Int>`, `List<E> : Collection<E>`);
and `FunctionN` extends `Function<R>`.

- **Substitution** (`KcSubstitution.kt`): `KcSubstitutor` replaces type
  parameters in one pass, through class and function type arguments,
  flexible bounds and definitely-non-null types; a replaced `T?` becomes
  nullable. A class type's substitutor maps its class's parameters to its
  arguments, a star to the first bound (or `Any?`). They back
  `createSubstitutor`, `createInheritanceTypeSubstitutor`,
  `substitute`/`asSignature` on callables (with `KcFunctionSignature` and
  `KcVariableSignature`), and `directSupertypes`, which now substitutes.
- **Subtyping** (`KcTypeRelations`): error types follow the policy, then
  flexible bounds, nullability, `Any`/`Nothing`, type parameters through
  their bounds, and class types through a breadth-first walk of substituted
  supertypes, comparing arguments by declaration-site and use-site
  variance. It backs `isSubtypeOf` (types, class IDs, symbols),
  `semanticallyEquals` (subtype both ways, so `String!` equals `String`),
  `isSubClassOf` and `isDirectSubClassOf`.
- **Member scopes** (`KcMembers.kt`, `KcRound.members`, cached per class):
  the class's own non-static callables, then what it inherits, one member
  per signature. Each supertype's scope comes in with that supertype's
  substitutor, minus private members. A declared member that matches an
  inherited one (same kind, name, receiver presence and parameter counts,
  equal receiver and parameter types once substituted, type parameters
  matched by position) overrides it. Unmatched inherited members are
  grouped by the same test and deduplicated by their original declaration:
  - one left whose types don't change is kept as it is, as FIR keeps it;
  - one left whose types change becomes a substitution override
    (`SUBSTITUTION_OVERRIDE`);
  - several left become an intersection override
    (`INTERSECTION_OVERRIDE`), typed from the first non-abstract one.
  
  Fake overrides delegate to their original for psi, annotations, name,
  flags and type parameters. Their pointers are `<class key>/~<index>` and
  restore by rebuilding the class's scope. The static scope adds the
  statics of Java superclasses that aren't hidden.
- **Overrides:** `directlyOverriddenSymbols` (declared originals, unwrapping
  fake overrides; accessors answer for their property),
  `allOverriddenSymbols`, `intersectionOverriddenSymbols` and
  `fakeOverrideOriginal`. Also `sealedClassInheritors` (source classes in
  source order) and `isVisibleInClass` (a private member only in its own
  class).
- **PSI join:** the preorder numbering now skips KDoc's inner nodes, which
  kc's psi folds into one leaf. Before this, any KDoc comment shifted every
  later node of its file.

The slice (`test/slice/src/f/Inherit.kt`) prints, through KSP,
`getAllFunctions`/`getAllProperties` of a generic hierarchy with origins,
parents, `findOverridee` and `asMemberOf`; `isAssignableFrom` for seven
pairs; `getSealedSubclasses`; and `Resolver.overrides`. `want.txt` was
checked by hand against FIR's behaviour (and, since step 10, against stock
KSP). Known gaps:
- Java collection methods aren't mapped to Kotlin's: in
  `class Bag : AbstractList<String>`, `override val size` doesn't override
  Java's `size()`, and both appear (fixed in step 15). Java collection
  supertypes are read-only Kotlin types, not `(Mutable)List<E>!`.
- The built-ins have only `Any`'s members.
- Fake overrides keep their originals' type parameters; FIR makes fresh
  ones.
- Subtyping has no captured or intersection types.
- Data class members (`componentN`, `copy`) aren't in scopes. (Enum
  `values`/`valueOf`/`entries` are since step 10.)
- An inner class's outer type arguments aren't substituted.
- Override matching approximates the checker's: no erasure-based Java
  overrides, and no special cases such as `CharSequence.get`/`charAt`
  (the renames are mapped since step 15).
- Package-private members of other packages stay in scopes.
- Pointer restore of fake overrides isn't exercised by the slice: KSP
  doesn't create such pointers on these paths.

**Step 6: packages, library top-level declarations and type aliases.**

- **Package listing (Go).** `classpath` lists the classes of a package,
  from a per-package index built once with the class path's index.
  `ka.Model.Package(pkg)` (cached, thread-safe) returns:
  - the package's top-level classes, Kotlin's and Java's, without file
    facades, multi-file classes and their parts, synthetic classes and
    classes the sources declare;
  - its top-level Kotlin functions, properties and type aliases, read from
    the parts that the class path's `.kotlin_module` files list (every copy:
    module names repeat across jars), decoded the way library class members
    are (`kfunction`/`kproperty`, now shared).

  A function's or property's `ClassID` is the class that declares it on the
  JVM: the file facade, or the multi-file class's part
  (`kotlin/collections/CollectionsKt__CollectionsKt`), as FIR's
  `JvmPackagePartSource.classId` has it. It's on the wire as op 14,
  `pkg(model, fqName) → exists, classes, decls`. `exists` is also true for
  a package that has only subpackages.
- **Alias abbreviations (Go).** A type written as a typealias carries the
  typealias type as `Type.Abbrev`: the alias's class ID, its type arguments
  as written, and the type's nullability. The type itself is the expansion.
  - In sources, the checker's `AliasRecorder` names the typealias a type
    reference resolved through. Typealiases written as the alias's
    arguments are carried into the expansion's arguments (`L<S>` gives
    `List<String as S>`).
  - The checker resolves the default imports' class typealiases
    (`ArrayList`, `HashMap`, …) straight to their classes, without telling
    the recorder. `defaultAlias` gets them back from the reference's name:
    an unqualified or `kotlin.collections.`-qualified name of a default
    import's typealias that stands for that very class, unless an explicit
    import or a non-default star import that has the name says otherwise.
  - In libraries, `kmeta` decodes `Type.abbreviatedType` and the alias
    class id of an abbreviation, and the encoder writes them back.
    A library typealias's `Type` is its expanded type, falling back on the
    underlying type, and keeps the underlying type's abbreviation
    (`typealias A = B`).
  - On the wire, a type ends with its abbreviation: an optional type.
- **JVM.**
  - `KcPackageSymbol` (pointer `pkgsym:<fq>`) is the parent of library
    top-level declarations; their pointers are
    `pkg:<fq>/<kind>:<name>:<i>`.
  - `findPackage`: a package of the sources (or a parent of one) or of the
    class path.
  - `getPackageScope`: the sources' top-level declarations, then the
    library's callables and type aliases, then the library's classes.
  - `findTopLevelCallables(package, name)`: sources first, then the class
    path.
  - `findTypeAlias` and `findClassLike` load library type aliases.
  - Typealias types are `KcClassType`s with `isAlias` set, whose symbol is
    the `KaTypeAliasSymbol`. A type written as an alias is its expansion
    with that alias type as its `abbreviation`, which is what KSP shows
    through `abbreviationOrSelf`. Function types keep theirs too, and
    `KcExtras.abbreviatedTypeAlias` returns it.
  - `KcRound.expand` expands an alias type: it substitutes the alias's
    parameters into its expanded type and keeps the alias as the result's
    abbreviation. `getFullyExpandedType`, `buildClassType` on a type alias
    symbol, substitution, subtyping and `isNullable` expand first. Type
    equality ignores the abbreviation, as FIR's does.
  - `KcExtras.packagePartClass`: the declaration's `ClassID` for a library
    callable; the file facade (`JvmFileClassUtil`) for a source one.
    `fileFacadeClass` (KSP patch 4) is the same file facade.

The slice adds `test/slice/src/g/Alias.kt` and prints, through KSP:
`getDeclarationsFromPackage` for a source package and filtered for
`kotlin.collections` and `java.util.function`;
`getFunctionDeclarationsByName` and `getPropertyDeclarationByName` with
`includeTopLevel`, with `getOwnerJvmClassName`; and, for properties typed
with typealiases, the declaration their type names, the alias's type and
what it expands to. `want.txt` was checked by hand against FIR's behaviour
(and, since step 10, against stock KSP). Known gaps:
- `defaultAlias` is a heuristic on the reference's name. A source typealias
  the checker resolves without the recorder (a nested typealias, or one of
  a non-default package) isn't recorded.
- Annotations on library typealiases aren't decoded.
- Private library top-level declarations are listed; FIR lists them too, but
  KSP filters them only by visibility checks it doesn't always run.
- ~~`KcExtras.binaryContainerClass` returns null, so KSP's
  `hasBackingField` asks the symbol's `hasBackingField`, which kc doesn't
  answer yet for any property.~~ Answered in step 8.
- Expanding an alias turns a star argument into the parameter's bound.
- Package scopes don't list subpackages.
- A library package's classes are all loaded when its scope is first built.

#### Step 7: JVM type mapping; Moshi codegen end to end

The first real processor on the `kc` backend: Moshi's KSP codegen
(`moshi-kotlin-codegen` 1.15.2, with kotlinpoet 1.13.2) runs over a small
model module and generates the same adapters kotlinc+KSP would.

- **JVM type mapping (`KcJvmTypes.kt`).** `mapToJvmTypeDescriptor`,
  `mapToJvmType(type, mode)` and `isPrimitiveBacked` give the descriptor
  of a type's erasure, as FIR's type mapper does in
  `TypeMappingMode.DEFAULT`:
  - typealiases are expanded first;
  - flexible types use their lower bound, and `T & Any` uses `T`;
  - a type parameter is its first bound's erasure, boxed (`Any?` if it has
    none);
  - `Int` is `I`, `Int?` and `Array<Int>`'s element are `Ljava/lang/Integer;`,
    and `IntArray` is `[I`;
  - `Array<in T>` and `Array<*>` are `[Ljava/lang/Object;`;
  - `Nothing` is `Ljava/lang/Void;`, and an error type is
    `Lerror/NonExistentClass;`;
  - Kotlin classes that are Java classes are mapped through
    `JavaToKotlinClassMap` (`kotlin.collections.MutableMap` is
    `java/util/Map`, `Map.Entry` is `java/util/Map$Entry`), and
    `SuspendFunctionN` is `kotlin/jvm/functions/Function(N+1)`;
  - a value class is its primary constructor parameter's type, unless the
    type is nullable and the underlying type is nullable, a primitive or a
    type parameter: `Id?` for a `Long` stays `Lh/Id;`, while `Name?` for a
    `String` is `Ljava/lang/String;`. Library value classes (`UInt`,
    `Duration`) work the same way.

  `Unit` stays `kotlin/Unit`; KSP turns it into `V` itself. `mapToJvmType`
  ignores the mode.
- **`test/RunKsp.kt`.** A general harness: KSP's processing loop on a
  chosen backend (`-Dksp.backend=kc|aa`), with the processors on its class
  path found through `ServiceLoader`. Processor options come from
  `-Dksp.arg.NAME=VALUE`. It prints the generated files and fails on any
  error the processors log.
- **`build.sh test`** now also downloads Moshi's codegen and its
  dependencies into `build/m2`, retrying on Maven Central's 429s. It runs
  them over `test/moshi/src` (a `@JsonClass` data class with defaults, a
  `@Json(name)` property, an enum, a generic class, a class with a body
  `var` and a `@Transient` property, and nested generic and `IntArray`
  properties). It compares the four adapters and four ProGuard files with
  `test/moshi/want` and prints "moshi: ok".
- **Checks of the output.**
  - kotlinc 2.4.21 compiles the models with the generated adapters, and so
    does kc. A round trip through Moshi gives the same output with either
    compiler: defaults filled through the synthetic constructor (the mask
    and `DefaultConstructorMarker` signature match kotlinc's), `e_mail`
    renamed, null `email` omitted, `Page<User>` through `types[0]`, the
    `theme` default kept, and the `IntArray` written.
  - The ProGuard rules name the defaults constructor
    `<init>(long,java.lang.String,java.lang.String,java.util.List,m.Role,int,kotlin.jvm.internal.DefaultConstructorMarker)`,
    which is what the mapping above gives.
- **Slice.** `test/slice/src/h/Jvm.kt` and the `jvm:` mode print
  `Resolver.mapToJvmSignature` for a class, its constructor, functions and
  properties. They cover primitives, boxing, every kind of array, bounded
  and unbounded type parameters, mapped built-ins, function and suspend
  function types, `Nothing`, nested classes, and source and library value
  classes.
- **Fix.** A class kc doesn't know (`KcLibraryClassSymbol`) has no flags,
  so the mapping asks only `KcClassSymbol`s whether they're value classes.

Known gaps:
- `KcExtras.optimalModeForReturnType`/`optimalModeForValueParameter`
  (`Resolver.getJavaWildcard`, which Room uses), `libraryModuleName` and
  `kotlinBinaryClass` are still unsupported.
- `mapToJvmType` ignores the mode: there are no generic signatures, no
  wildcards, and no value-class wrapping modes.
- A value class whose underlying type is a type parameter or another value
  class is mapped one level at a time, without FIR's special cases.
- ~~`hasBackingField` is still unanswered. Moshi doesn't ask for it.~~
  Answered in step 8.
- Moshi's reflective (`moshi-kotlin`) path and the `generateAdapter =
  false`/`generator` options aren't exercised.

#### Step 8: backing fields; Dagger end to end

The second real processor: Dagger's KSP processor (`dagger-compiler`
2.59.2, built on XProcessing) runs over a small component module on the
`kc` backend and generates a working component.

- **Backing fields.** XProcessing lists a class's fields through KSP's
  `hasBackingField`, so the model now has `FlagBackingField` (`ka.go`,
  mirrored as `KcDecl.BACKING_FIELD`), with FIR's meaning:
  - a source property has one if it has a default accessor, uses `field`,
    or declares an explicit backing field, and isn't abstract, `expect` or
    delegated (`lateinit`, `const` and constructor properties have one; an
    interface property and an extension property don't);
  - a library property has one if its metadata names a JVM field
    (`Pair.first` does; `Regex.options`, a getter over a private
    `_options`, doesn't);
  - substitution and intersection overrides have none.

  `TestBackingField` covers these cases.
- **Empty modules.** `check.conflicts` indexed the first file of a module
  with none and panicked. A KSP module with only Java sources has none.
  It now returns early, and `TestEmpty` checks such a module.
- **Panic stacks.** A checker panic that `ka` turns into an error now
  carries the panicking goroutine's frames, so a crash inside libkc can
  be found from the JVM's exception message.
- **`buildTypeParameterType`** (`KSClassDeclaration.asType` with type
  arguments) and **a `KotlinModificationTrackerFactory`** registered in the
  project. KSP builds `KotlinAlwaysAccessibleLifetimeToken`s for type
  projections and synthetic annotations, and they watch the source
  tracker. kc's trackers never change: its own token ends with the round.
- **`build.sh test`** downloads Dagger and its dependencies into
  `build/m2` and runs the processor over `test/dagger/src`:
  - an `@Inject` class and a `@Singleton` one with a `@Named` dependency;
  - `Lazy` and `Provider` requests;
  - a `@Module object` with `@Provides` and `@IntoSet`;
  - a `@Module interface` with `@Binds`;
  - members injection of `lateinit` properties;
  - a `@Component` with a `@Component.Factory` taking a `@BindsInstance Int`.

  It compares the seven Java files with `test/dagger/want` and prints
  "dagger: ok".

  The processor jars come first on the class path, because
  `kotlin-compiler` bundles an older Guava that breaks Dagger's
  `common.graph` use. Real KSP loads processors in a class loader of their
  own. The Moshi and Dagger steps now drop KSP's `caches` directory before
  comparing. A stray, untracked `test/moshi/want/caches` had made the Moshi
  comparison pass only locally.
- **Checks of the output.**
  - Scopes and qualifiers are right: `@ScopeMetadata("javax.inject.Singleton")`
    and `@QualifierMetadata("javax.inject.Named")`.
  - `Repo_Factory.newInstance` takes `int port`.
  - javac compiles the generated sources against the models compiled by
    kotlinc 2.4.21, and against the models compiled by kc.
  - A driver gives the same results with both: the singleton is shared,
    the binding resolves, the set holds the plugin, and members injection
    sets both properties.

Known gaps:
- ~~No stock-KSP reference output.~~ Stock KSP 2.3.6's embeddable jar
  runs here (only the AA 2.4.21 standalone jars are blocked):
  `build.sh stock` diffs `test/dagger/want` against it (step 9).
- Hilt, assisted injection, subcomponents, map multibindings, and Java
  sources in the module aren't exercised.
- Source annotation-class properties report a backing field (as FIR's
  source properties do); library ones don't.

#### Step 9: Dagger on octi-server, matched against stock KSP

The plan's end-to-end target: Dagger over octi-server (60 files, 7.7k
lines, ktor and kotlinx-serialization), through the `kc` backend, against
stock KSP 2.3.6 (`symbol-processing-aa-embeddable`, which runs here; only
the AA 2.4.21 standalone jars are blocked).

- **Output.** The 31 generated Java files (the `_Factory` classes,
  `DaggerAppComponent`) are byte-identical to stock KSP 2.3.6's.
- **`getContainingDeclaration` and `getContainingModule`.** A library
  symbol's module is a `KcLibraryModule` (a `KaLibraryModule` over the
  configured libraries, one for all of them); substitution, intersection
  and delegated overrides take their class's module; the rest are the
  source module's. `KcExtras.libraryModuleName` returns null, so KSP
  names library modules "main".
- **`build.sh stock`** does what `build.sh test` does, then runs Moshi
  and Dagger on stock KSP 2.3.6 from Maven Central (aa-embeddable, with
  its intellij coroutines build) over the same fixtures, and compares the
  output with the same `want` directories ("moshi: stock KSP ok", "dagger:
  stock KSP ok"). Both match, so the fixtures' expected output is now
  checked against stock KSP, not only against kc. Stock KSP runs with
  language version 2.3, its compiler's.
- **`RunKsp`** takes several source roots (`SRC_DIR[:SRC_DIR...]`) and
  `-Dksp.languageVersion`, and exits explicitly, since stock KSP leaves
  non-daemon threads running.
- **Checker: octi-server checks with no errors** but the unresolved
  `DaggerAppComponent`, which Dagger generates. Before this step kc
  reported 304 errors there, which KSP ignores but a kc compile wouldn't:
  - **Extensions of a lambda's receiver give way** to a top-level function
    that takes the arguments, as those of `this` already did (M2):
    `embeddedServer(Netty) { log(TAG, INFO) { "…" } }` calls
    `log(tag: String, …)`, not `Application.log(priority, …)` (298
    errors). In `recvCall`, when only extensions of the receiver fit and
    the call has a lambda, they must take the argument types if a
    top-level function may take the call (`extensionsGiveWay`).
    `testdata/run/extensionVsTopLevel` covers it.
  - **Ifs without else at the end of a try** that ends a lambda are
    statements, like those at the end of an if that ends one, and so are
    those ending such an if's branches: `Mutex.withLock { try { if (a)
    return@withLock; if (b) { f(); if (c) g() } } catch (e: E) { … } }`
    (4 errors). `testdata/run/lambdaTailTry.kt` covers it.

Known gaps:
- One library module for all libraries, so symbols from different jars
  share a module, and `libraryModuleName` is null (kc doesn't decode the
  module name from class metadata).
- The octi-server check isn't in `build.sh`: it needs octi-server's
  Gradle class path. It ran from the profiling session's checkout.

#### Step 10: the slice against stock KSP

Stock KSP 2.3.6 runs here (step 9), so it is now the oracle for the slice
too: `PrintAnnotated` built against it prints the same symbols over the
same sources. The first run differed from `want.txt` in 42 lines; fixed:
- **Constructor properties' annotations.** An untargeted annotation on a
  constructor `val` is on the parameter and, if it may be on a property,
  on the property too; if it may be on a field and not a property, on the
  field (Kotlin 2.4's PropertyParamAnnotationDefaultTargetMode, which AA
  2.3 already shows). `ka` gives the property those annotations
  (`propAnnotations`, step 14); `KcRound.property` no longer drops them.
- **Platform types of inferred declarations.** `fun f() =
  System.getProperty("x")` returns `String!`: a checker type with `P` set
  (and `N`, which platform types also have) is flexible, not nullable.
- **Java type parameters without bounds** are bounded by `Any!`, not
  `Any?`.
- **Implicit constructors.** A source class that declares none has an
  implicit primary constructor whose origin is `SOURCE_MEMBER_GENERATED`
  (KSP: `SYNTHETIC`), as is every source declaration without a node. A
  class with only secondary constructors has no primary one; the model
  added one before.
- **Enum members.** Every enum class, source, Kotlin library or Java, has
  static `values()`, `valueOf(value: String)` and, with the standard
  library, `entries: EnumEntries<E>`, as FIR synthesizes them; a Java
  enum's own `values`/`valueOf` methods are replaced by them.
- **Built-in classes in packages.** `Model.Package` lists the classes of
  the package's `.kotlin_builtins` (`kotlin/collections/List`,
  `kotlin/Int`), which have no class files.

`build.sh stock` now runs the slice on stock KSP too and compares the
difference from `want.txt` (its `<`/`>` lines) with
`test/slice/stock.diff` ("slice: stock KSP differs as recorded"). Those
are the gaps left, 25 lines:
- A Java enum's private constructor (`<init>()`), and private members of
  Java classes generally (`SimpleEntry.serialVersionUID`), aren't in
  `declarations`.
- Java getters that implement Kotlin properties: FIR adds properties
  `key`/`value` to `AbstractMap.SimpleEntry` (from `getKey`/`getValue`,
  which implement `Map.Entry.key`/`value`) and drops the getters.
- Java signatures aren't enhanced from the Kotlin members they override:
  `SimpleEntry.equals` takes `Any?` (from `Any.equals`), not `Any!`.
- Java collection methods aren't mapped to Kotlin's (step 5's gap):
  `Bag`'s `size` doesn't override `AbstractCollection.size`. Fixed in
  step 15.
- `asMemberOf` of a star-projected type (`f.Base<*>`) substitutes `Any?`
  where FIR has the captured type, which KSP prints as `Any`.

#### Step 11: benchmark

`bench/ksp/run.sh` times KSP's processing loop on a project with stock
KSP 2.3.6 and with the fork on kc, each in a JVM of its own and run after
run, as a Gradle daemon runs `kspKotlin` (`KspBench.kt`), then cold in
fresh JVMs. Each run prints session setup (until the provider's `create`),
processing (the processors' `process` calls), the time between rounds,
finish and teardown. `KC_TIMING=1` adds kc's own split on stderr:
`kc-go:` lines for `ka.Session.Update` (parse, class-path open, lower,
signature checking, body checking, model) and `kc-jvm:` lines for
`KcSession` (environment, `KcOps.open`, the native update and its decoding,
PSI lookup, `KcRound`). `JFR=N` records samples from run N on;
`bench/ksp/Classify.java` splits them by phase and by code. octi-server
isn't vendored: its sources and Gradle class paths are inputs.

octi-server, Dagger, 61 Kotlin files, 31 generated files; 4 CPUs, JDK 21,
20 runs in one JVM (warm is the median of the last 10), and 3 cold JVMs:

| ms | cold JVM | warm total | warm setup | warm process | between rounds |
|---|---|---|---|---|---|
| stock KSP 2.3.6 | 8100–8700 | 990 | 128 | 829 | 6 |
| kc (baseline) | 3780–3960 | 976 | 442 | 156 | 375 |
| kc, shared library, signatures only | 3190–3510 | 412 | 181 | 130 | 86 |
| kc, second round kept | 3030–3120 | 304 | 159 | 150 | 0 |
| kc, PSI parsed in parallel | 2850–3190 | 284 | 125 | 146 | 0 |

Processing is 5x faster on kc, and cold runs 2x, but kc's warm total is no
better: each run checks the module twice, once per round, and each check
costs about 350 ms in Go:

| ms per update | parse | open | lower | signatures | bodies | model |
|---|---|---|---|---|---|---|
| kc (baseline) | 30–65 | 20–60 | 12–15 | 105–150 | 70–90 | 55–120 |
| kc, shared library, signatures only | 17–31 | 0–0.4 | 7–12 | 18–26 | 5 | 2 |

On the JVM, the first round's `KcRound` takes about 155 ms (IntelliJ
parsing the files' PSI on first use); the second's 12–40 ms. The
environment, `KcOps.open` and decoding take under 15 ms together.

Dagger generates only Java, so the second round checks the same Kotlin
sources again. The rest of step 11 makes the warm path cheaper in order
of these numbers.

**Shared library.** Most of the check was reading class-path classes:
each update opened the class path anew, and the checker read every class
the sources use from the jars again. Now a `ka.Session`'s updates share
one library, and so do the sessions of a process whose configuration is
the same and whose jars haven't changed (size and modification time of
each jar and of the JDK's `lib/modules`; `analysis/ka/libcache.go`). The
two libraries no session uses that were used last are kept for the next
KSP run. The checker keeps what it learns of the class path's classes
(supertypes, intersections, Java members) in the library and what it
learns of the program's in each check's own `Universe`
(`check.Universe.cache`), so checks one after another or at the same time
can share a library. A class path with a directory (whose classes can
change without its time changing) or with Java sources isn't shared
across sessions.

**Signatures only.** `ka.Update` checks with
`check.Options{Signatures: true}`: declarations, supertypes, signatures
and the bodies whose types are inferred, annotations and their arguments,
annotation classes, but not the other bodies. A model's `Diagnostics`
therefore lists only the errors found that far: errors inside bodies
whose types are declared aren't reported. Parsing and lowering run in
parallel too.

With these the Go update costs about 55 ms.

**Second round kept.** The model is of the Kotlin sources and the
libraries (Java sources don't reach it), and KSP doesn't edit sources
between rounds, so a round that generated no Kotlin, as Dagger's don't,
keeps the round before: `KcSession.addFiles` updates only when a Kotlin
file was added.

**PSI parsed in parallel.** Each KSP run has a new IntelliJ environment,
whose PSI parses the sources again when `KcRound` first walks them. The
files are walked in parallel now: the first round's `KcRound` went from
77–111 ms to 30–44 ms.

**Where the warm time goes now** (JFR from run 11, main thread): setup is
three quarters Go (the one update, about 55–65 ms a run) and the rest
IntelliJ and `KcRound`; of processing, Dagger, XProcessing and
JavaPoet/google-java-format take 62%, KSP's symbol layer 20%, and kc
(Go, the kc-aa layer and IntelliJ) 18%. The warm target (under 0.3 s) is
met; processing alone is 5.7x faster than stock KSP's.

Without help the cold target (under 2 s) isn't met: a fresh JVM takes
about 3 s, of which setup is 1.6 s (the IntelliJ environment, about
500 ms; the first update, which reads the class path's classes, 280–310
ms; the first `KcRound`, whose PSI IntelliJ parses with code not yet
compiled, 500–660 ms) and processing 1.3 s. A cold JFR recording puts
64% of processing in Dagger, XProcessing and JavaPoet and 16% in kc: the
processors' own class loading and warm-up.

Running the first update while the environment is made hides it (the
session waited 0.2 ms for it) but saved only about 60 ms of cold setup:
on 4 CPUs a cold JVM is busy compiling, and the environment took 690 ms
and the update 435 ms side by side. It was dropped.

What helps is class-data sharing: with an archive of the classes a run
loads (`-XX:+AutoCreateSharedArchive -XX:SharedArchiveFile=…`, JDK 19
and later; `JVM_OPTS` in `run.sh`, whose classes are jars for it), cold
runs take

| ms, cold JVM | no archive | archive |
|---|---|---|
| stock KSP 2.3.6 | 8100–8700 | 5810–6130 |
| kc | 2850–3190 | 1870–2110 |

so kc meets the cold target with an archive, and is 3x faster than stock
KSP with one. In Gradle the archive is the daemon's (`org.gradle.jvmargs`
in `gradle.properties`), and cold is the first `kspKotlin` of a daemon:
later runs are warm.

Next steps: the environment, its PSI and the Go check kept across KSP
runs (step 17).

#### Step 12: CI

`.github/workflows/analysis-api.yml` runs the JVM half on every pull
request: `jvm/libkc/test.sh` (the JNI stress test, with and without
libjsig) and `jvm/kc-aa/build.sh stock` (the slice, Moshi and Dagger, and
since step 14 Room and Hilt, on the kc backend, compared with their recorded output, and on stock KSP
2.3.6). The Analysis API jar and the KSP fork are cached, keyed by their
build scripts and the patches; the processors' and stock KSP's jars too.
From nothing, on 4 CPUs, both take under 3 minutes. The Go half's tests
(`analysis/...`, `internal/kaops`, `internal/check`) run in PR checks
with the rest. The benchmark isn't run in CI: octi-server isn't vendored,
and shared runners' timings vary too much to gate on.

#### Step 14: Room and Hilt

With `dl.google.com` reachable, Room 2.8.4 and Hilt 2.59.2 run on the kc
backend over `test/room/src` (entities with an embedded class, a foreign
key, indices and type converters; a DAO interface with `Flow`, `suspend`,
vararg and `@Transaction` queries; an abstract DAO class; a database) and
`test/hilt/src` (an injected singleton, `@InstallIn` modules with
`@Provides`, `@IntoSet` and `@Binds`, entry points, and a
`@DefineComponent` with its builder and scope). Their output is
byte-identical to stock KSP 2.3.6's: Room's three `_Impl` files, Hilt's
twelve Java files (factories, `hilt_aggregated_deps` and the
`definecomponent` metadata classes). `build.sh test` runs both
("room: ok", "hilt: ok"), `build.sh stock` both on stock KSP as well.
The class paths are what Gradle resolves for `room-compiler` and
`hilt-compiler`, but for KSP's API and the standard library; Room's
androidx jars come from Google's Maven repository (`GOOGLE_MAVEN_REPO`).

One fix was needed, found by Room ("An entity must have at least 1
property annotated with @PrimaryKey"): where Kotlin puts an annotation
written without a use-site target. Room reads `@PrimaryKey`,
`@ColumnInfo`, `@Embedded` and `@Relation`, whose targets are `FIELD`
and `METHOD`, from the backing field, which kc had no symbol for. Now,
as FIR does (stock KSP too, in Kotlin 2.4's param-property mode):
- An untargeted annotation on a constructor `val` is the parameter's if
  it may be on a parameter (`ctorParamAnnotations`), the property's if it
  may be on a property, and otherwise the backing field's if it may be on
  a field. The same goes for a property declared in the body, but that
  an annotation that may be on neither stays on the property
  (`propAnnotations`).
- The model marks an annotation placed on the field this way `Implied`
  (on the wire, a flag after the target): it is the field's but has no
  use-site target, as in FIR.
- `KaPropertySymbol.backingFieldSymbol` is a `KcBackingFieldSymbol` for
  every property with a backing field, holding the annotations with
  target `field` or `delegate`, written or implied. KSP adds them to the
  property's annotations.

`test/slice/src/a/Targets.kt` checks the rules (`PrintAnnotated`'s
`annots:` mode lists the annotations of a class's parameters,
properties, accessors and setter parameters with their targets). It
matches stock KSP but for one difference, two lines of `stock.diff` (27
lines now): stock KSP also lists `@param:Other` of a constructor `var`
among the property's annotations, which kc doesn't.

Gap: fake overrides (substituted and intersection properties) have no
backing field symbol.

#### Step 15: KSP's own test suite

`jvm/kc-aa/build.sh ksp` runs KSP's unit tests on the kc backend: the
tests of `kotlin-analysis-api`'s `KSPUnitTestSuite`, as
`AAConfiguredUnitTestSuite` configures them, with the processors of
`test-utils` and `kotlin-analysis-api/src/test/.../processor`, compiled
against the fork. KSP's own harness is built on Kotlin's test framework
(`org.jetbrains.kotlin.test.*`), which isn't published, so
`ksptest/KspSuite.kt` does what `AbstractKSPTest`/`AbstractKSPAATest`
do:
- reads the test list from the suite's sources (`runTest`,
  `runFailingTest`, `runThrowingTest`; `@Disabled` tests are skipped);
- splits each test data file into modules (`// MODULE: name(deps)(friends)`)
  and files (`// FILE:`), padding each file so that it keeps its line
  numbers, as Kotlin's test framework does;
- compiles the library modules with K2JVMCompiler (in a class loader of
  its own) and javac, with the same arguments;
- runs `KotlinSymbolProcessing` over the last module with the test's
  processor (`// TEST PROCESSOR:`, `// PROCESSOR INPUT:`) and the same
  `KSPJvmConfig` (`incremental = true`), and compares `toResult()` with
  the text between `// EXPECTED:` and `// END`.

One difference: every module's libraries include the Kotlin standard
library, where KSP's harness passes only the other modules' outputs and
lets FIR's built-ins stand in for it.

`build/ksptest/results.txt` has a line per test, with a `.diff` (or
`.log`, for a run that threw) per failing test under `build/ksptest/work`.
`test/ksp/pass.txt` lists the tests that pass, and the build fails if
one of them stops passing; `KSP_UPDATE=1` rewrites it. `test/ksp/skip.txt`
gives a reason class for every other test. The whole suite takes about a
minute.

Baseline: 35 of 136 pass, 2 are disabled. The other 99, by class:

| Class | Tests |
|---|---|
| `java-source`: the main module has Java sources, which don't reach the model | 66 |
| `unsupported`: `KaKotlinPropertySymbol.initializer`, `KcExtras.optimalMode*` | 5 |
| `local-declarations`: no symbols for local declarations | 4 |
| `annotation-target`: receiver, grouped file annotations, `@all:` | 4 |
| `type-annotations`, `type-alias` | 3 each |
| `class-literal`: Kotlin classes not mapped to Java ones for `getAnnotationsByType` | 2 |
| one each: data-class `componentN`/`copy`, star projections, library sealed inheritors, Java collection members, generated Java, `KFunction` kinds, friends' `internal`, flexible types, error types, companion naming, constructor `val` `isVal` | 12 |
| `ka-differs-from-aa`: kc gives the expected output, which a `runFailingTest` records AA as not giving | 1 |

##### Java sources in the module

The main module's Java files now reach the model, as they reach a kc
build:
- `ka.Config.JavaSources` goes through `driver.AddJavaSources`, the code
  `kc` builds use, so the checker sees Java source classes and Kotlin
  calls into them type-check. The library cache from step 11 is bypassed
  when there are Java sources.
- Java source classes come into the model as library classes, built from
  the class-file view that `javasrc` gives (`Set.Details` adds annotations,
  access and parameter names in source order). They carry origin
  `JAVA_SOURCE` and their file's path on the wire.
- On the JVM, `KcBackend` parses the Java files with the core environment's
  `PsiManager`. `KcRound` maps each `PsiClass` to its class symbol and each
  `PsiMethod`/`PsiField` to its member, so `psi`, `getNamedClassSymbol` and
  `getCallableSymbol` work both ways.
- Members of a Java class are in FIR's order: instance fields, instance
  methods and inner classes, then constructors, then static fields, static
  methods and nested classes. Each group keeps source order, or class-file
  order for a library class.
- Fork patch 0005: `KSAnnotationJavaImpl` resolves a Java annotation whose
  class is Kotlin through `KcExtras.javaAnnotationClass`, since without
  light classes `PsiAnnotation.resolveAnnotationType()` finds only Java
  classes. kc resolves the name against the file's imports, package and
  outer classes.
- The runner writes Java files under `java-sources/<module>/`, as Kotlin's
  test framework does, so `RecordJava*` processors print the same paths.

Result: 47 of 136 pass. The remaining 87, by class:

| Class | Tests |
|---|---|
| `java-source`: Java sources reach the model, but the test fails further on (data-class and enum members, Java collection types as `MutableList`, accessor overrides, docs of package-private fields, error types for unresolved Java supertypes, ...) | 53 |
| `unsupported`, `local-declarations`, `annotation-target` | 5, 4, 4 |
| `type-annotations`, `type-alias` | 3 each |
| `class-literal` | 2 |
| one each: as in the baseline, plus `checker-gap` (Java `@NotNull`/`@Nullable` are ignored) | 12 |
| `ka-differs-from-aa` | 1 |

##### Kotlin-only fixes

Each fix below keeps `build.sh stock` and the slice unchanged.

- **Scope order.** A `KaScope`'s `declarations` follow FIR's order: the
  callables, grouped by name in order of first appearance with functions
  before properties, then the classifiers, then the constructors. KSP's
  `declarations` read them in that order. +2 (47 → 49).
- **Data class members.** A source data class gets `componentN` for each
  primary constructor property, then `copy`, after its own members.
  `equals`, `hashCode` and `toString` are generated after FIR, so FIR's
  declarations don't have them. +1.
- **`initializer`.** A const val's value comes through `Decl.Default`. For
  a library property it is read from the `ConstantValue` of its field,
  which for a companion's const vals is in the outer class. For a source
  one it is evaluated as annotation arguments are. Other source properties
  get a `KaNonConstantInitializerValue` of their initializer. KSP's
  `hasBackingField` reads it for library properties. +2 (→ 52).
- **Local declarations.** `Decl.Locals` lists the local variables,
  functions and classes written directly in a function's, constructor's or
  accessor's body, with `Local` visibility; a local class has no ClassID.
  Their annotations are checked in signatures mode, and an unchecked local
  variable's written type is resolved. KSP's `declarations` of a function
  or accessor read them. Nested blocks, anonymous objects and destructuring
  declarations are not listed. +2 (→ 54).
- **Grouped annotations.** Lowering expands `@[A B]` and `@t:[A B]` into
  one annotation per entry, each with the group's target, in modifier
  lists, file annotation lists, parameters, types, statements and
  expressions; kc had skipped them. A setter's parameter keeps its
  annotations, which the model gives the property with target
  `setparam`. A function or constructor parameter written without a type
  no longer drops its declaration: it has an invalid type, and the parse
  error is reported with the checker's. `@file:` annotations on a
  declaration stay on it, and KSP gives them to the file. +3 (→ 57).
- **Test directives.** The runner reads `//FILE:` and `//MODULE:`
  without a space, as Kotlin's test framework does; six test files write
  them so, and had run as a single file. +2 (→ 59).
- **Destructuring declarations.** `val (@A a, b) = e` in a body is a
  local `KindDestructuring` declaration (14 in the ABI) whose Params are
  its entries, local properties with their own annotations; kc-aa makes a
  `KaDestructuringDeclarationSymbol` with those `entries`, as KSP's
  `declarations` expects. The parser and lowering keep the entries'
  annotations (they had been rejected), and the checker checks them. An
  entry has a type only when it is written: the model doesn't check bodies,
  so the inferred types of locals stay unknown. +1 (→ 60).
- **Small gaps.** A type with no name (an unresolved one the checker
  didn't name) is a plain `KaErrorType`, not a class type. A constructor
  parameter's `generatedPrimaryConstructorProperty` is the property its
  `val`/`var` declares, which KSP reads for `isVal`/`isVar`. A module's
  code before its first `// FILE:` is a file named after the test, as in
  Kotlin's test framework. A library property's getter and setter
  annotations (`@get:JvmName`) come from the accessors' class-file
  methods. +4 (→ 64).
- **Library module names, sealed inheritors, package scopes.** A library
  declaration carries its module's name, from the class's or the facade's
  `kotlin.Metadata` module, so KSP's `internal` checks and mangled names
  see the library's module. A library sealed class carries its
  `sealedSubclasses`. A package scope lists, as FIR does, source then
  library callables, then source classes, Java source classes and library
  classes, then library type aliases; packages with only Java sources are
  packages. A Java source enum's synthetic entries property is the class's,
  so KSP gives it no accessors. A typealias reached other than through the
  default imports (by its package, an explicit or star import, or the
  file's own package) that expands to a class is the type's abbreviation,
  as with `kotlin.collections.ArrayList`. The Java source parser reads a
  method with no return type as a misnamed constructor, as IntelliJ's Java
  PSI does, instead of failing the file. +4 (→ 68).
- **Annotation arguments as written.** An annotation's arguments come in
  the order they were written, named ones included, as FIR gives them,
  and so do a nested annotation's defaults after them. A class literal
  of a generic class written without arguments, `HashMap::class`, has
  star arguments. A Java source annotation method's `default` marks its
  parameter as having a default. A collection literal argument of a
  parameter whose type didn't resolve is still an array. A library
  parameter without a default gives KSP no default value unless it is an
  array (an empty one), as FIR's converter does. +2 (→ 70).
- **Type annotations.** `@A T` on a declared type, a supertype, a type
  argument or a function type reaches `KaType.annotations`, the
  abbreviation's too. The checker takes a Java annotation class as an
  annotation parameter's type. Still missing: type annotations on Java
  source supertypes, and creating a Java annotation in a Kotlin
  annotation's default (kc can't compile it either). +1 (→ 71).
- **Type arguments as written.** Annotations and nested types on an
  inner class's outer arguments and on a function type's receiver,
  parameters and return type follow the written type. `KFunctionN`
  and `KSuspendFunctionN` are function types, the suspend ones
  suspend. Types built with KSP's builder compare their projections
  structurally. A type argument inside an unresolved type is an error
  type. Still missing: FIR's argument-count mismatch error types
  (`<ERROR TYPE: A<Int> % Unexpected extra 1 type argument(s)>`). +5
  (→ 76).
- **Friend paths.** `ka.Config.FriendPaths` (kotlinc's
  `-Xfriend-paths`) reaches the class path, so a friend module's
  `internal` declarations resolve. The open request carries them after
  the class path. The error type of an omitted return type whose call
  doesn't resolve is named after the callee, as FIR names it. +1 (→ 77).
- **Hidden Java members and local class types.** Library Java classes
  keep their package-private and private members
  (`check.Library.KeepHidden`), as FIR's declared scopes have them;
  javac's enum constructors lose their synthetic name and ordinal
  parameters. Class types of local classes carry their symbol.
  `asKaType` turns Java class types into flexible Kotlin types, and a
  Java constructor's type parameters have no container, as FIR drops
  them. The slice now lists `Thread.State`'s constructor and
  `SimpleEntry`'s fields, as stock KSP does. +6 (→ 83).
- **Object constructors.** An object has an implicit private
  constructor, and an enum class's constructor is private, as FIR
  gives them. +2 (→ 85).
- **Java accessors of Kotlin properties, Java annotation order.** A Java
  class's `getX()`/`setX()` that implement an inherited Kotlin property
  `x` aren't functions of its member scope: FIR folds them into the
  property. A library Java class lists its run-time visible annotations
  before the class-file-only ones, as FIR reads them (Kotlin classes keep
  the class-file-only ones first). Unknown library classes have no type
  parameters instead of throwing. +2 (→ 87).
- **Implicit bounds, Java synthetic origins, Java parse recovery.** A
  type parameter without a written bound has no `upperBounds`, as in FIR,
  so KSP makes a synthetic `Any?` reference (and a star projection's
  member types become `Any`, as stock KSP prints them: the slice's
  recorded differences drop by 5). A Java annotation class's methods are
  `JAVA_SYNTHETIC_PROPERTY` properties at the method, with getters.
  kc's Java source parser leaves out a member it can't parse, keeping the
  rest of the file, as IntelliJ's Java PSI does; before, the whole file
  was dropped. +1 (→ 88).
- **Library backing-field annotations.** A library Kotlin property's
  backing-field annotations are read from its class file's field, and the
  property carries them with target `field`, as `@field:` ones from source
  do. +1 (→ 89).
- **Java platform types and type mapping modes.** A Java collection's
  flexible type has the read-only collection as its upper bound,
  `(MutableList<T>..List<T>?)`, and a Java array's has an out-projected
  one, `(Array<T>..Array<out T>?)`, as in FIR. Type hash codes take
  nullability and both flexible bounds in, so `String`, `String?` and
  `String!` stay apart in a hash set while still comparing equal where FIR's
  do. `KcExtras.optimalModeForReturnType`/`ValueParameter` port Kotlin's
  `getOptimalModeForSignaturePart`, so `Resolver.getJavaWildcard` works (a
  generic value class's underlying type isn't looked through yet). +7 (→ 96).
- **Built-in classes from `.kotlin_builtins`.** A built-in class (`List`,
  `Map` and its nested `Entry`, `Comparable`, `String`, `Int`, …) is now
  built from the class path's `.kotlin_builtins` metadata, members
  included, instead of a stub with supertypes only; `Serializable` is kept
  from the mapped Java class, and `kotlin/Any` stays hand-made. An
  intersection override takes its types from the member whose return type
  is the most specific, so `MutableList.iterator` returns `MutableIterator`.
  In the slice, `Bag.size` now overrides `List.size` (stock: Java
  `AbstractCollection.size`, folded into a property, which kc doesn't do
  yet). +1 (→ 97).
- **Unresolved types in Java sources, initialized extension properties.**
  A Java source field or method whose type the class path can't resolve
  is kept, typed `<error Name>!` from its descriptor's simple name, rather
  than dropped; the field becomes a `KaJavaFieldSymbol` read from the
  class file. An extension property whose type can't be inferred still
  resolves its written receiver and type, which `asMemberOf` reads.
  +1 (→ 98).
- **FIR's Java-to-Kotlin member mapping.** A Java class's declared scope
  is built as FIR's Java use-site scope builds it:
  - a getter (and setter) implementing an inherited Kotlin property
    becomes a `KaSyntheticJavaPropertySymbol` of that name that overrides
    the property, with accessors of origin `JAVA_SYNTHETIC_PROPERTY`. The
    method stays a function too when its name doesn't give the property's
    back (`getFoo` for `val Foo`);
  - built-in getters map by their JVM names (`size`, `keySet`, `values`,
    `entrySet`, `length`), and `remove(int)`, `charAt` and `xxxValue`
    are renamed to `removeAt`, `get` and `toXxx` when they implement them;
  - `equals`/`hashCode`/`toString` aren't members of Java interfaces;
  - an inherited Java getter and an inherited Kotlin property fold into a
    synthetic property of the class;
  - a Java method overrides only with a covariant return type;
  - a Java source supertype that doesn't resolve is an error type, not
    `kotlin.Any` (javasrc records it in `classfile.Info.Unresolved`).

  The slice's `Bag.size` now overrides `AbstractCollection.size` as in
  stock KSP (stock.diff: 14 → 11 lines). +5 (→ 103).
- **Erroneous supertypes stay.** A class that inherits from a final
  class, or from a class it doesn't initialize (`class A : B`), still has
  that supertype, as in FIR; the checker reported the error and dropped
  it, so the model showed an error type. +1 (→ 104).
- **Supertypes in place, invisible Java classes kept.** A Java source
  class's unresolved supertypes keep their places among the resolved ones
  (`J : I0, <ERROR TYPE: I1>, I2`), as FIR lists them; they all came first.
  A Kotlin class keeps the type of a package-private Java class of another
  package (a supertype a processor generated without `public`), as FIR
  does: the checker reports it and, like it, now keeps the class; Kotlin
  `internal` classes of other modules still give error types.
  `KSP_JAVA_OPTS=-Dkc.diagnostics` prints kc's diagnostics during the
  suite. +1 (→ 105).
- **JDK classes Kotlin maps are Java classes.** `findClass` of
  `java/util/Collection`, `java/lang/String` and the other JDK classes by
  their own names gave KOTLIN_LIB classes; FIR's are Java classes, and
  KSP's `isAssignableFrom` maps only Java classes to Kotlin's
  (`shouldMapToKotlinForAssignabilityCheck`). +1 (→ 106).
- **Class literals by their Java names; annotation argument fixes.**
  `Throwable::class` with `import java.lang.Throwable`, or
  `java.util.Map.Entry::class` written out, named the Kotlin class
  (`kotlin/Throwable`), which KSP's `getAnnotationsByType` can't load with
  `Class.forName`; FIR keeps the Java class, and so does the model now,
  where the file writes the Java name. A positional annotation argument
  after named ones in their own places, `@A(x = 1, 2)`, went to the first
  parameter; it takes its own place, as in Kotlin. A negative annotation
  parameter default, `= -2`, pointed at its literal, not the
  PREFIX_EXPRESSION KSP evaluates. `buildClassType` of a class that
  doesn't exist gives an error type, as in FIR. +2 (→ 108).
- **Unresolved class literals in annotation arguments.**
  `@A(type = Missing::class)` gave an unsupported value, which KSP's
  `validate()` accepts; FIR keeps a class literal of an error type, and so
  does the model, so the annotated class is invalid. +1 (→ 109).
- **Java annotations in Kotlin annotation defaults; local annotation values.**
  `val r: Retention = Retention(RetentionPolicy.SOURCE)` in an annotation
  class was a checker error: a Java annotation class has no constructor. In
  an annotation parameter's default the checker now calls one made from the
  annotation's methods (`value` a vararg when it is an array, a parameter
  optional when the method has a default), as kotlinc does. Annotations on
  local declarations now record their arguments too, which
  `getSymbolsWithAnnotation(.., inDepth = true)` reaches. No count change:
  testAnnotationValue_kt still differs on a backticked class with `$` in
  its name and on a local class's class literal.
- **Class literals of `$`-named and local classes.** A source class's ID
  came from its JVM name with every `$` read as nesting, so
  `` `SomeClass$WithDollarSign` `` became `SomeClass.WithDollarSign`; it now
  follows the class's outer classes. A local class's type was an error
  type: in signatures mode the checker doesn't declare local classes. It now
  puts a name-only symbol of each in scope for its own annotations and the
  statements after it, and the model writes its type as a class type with a
  local ID (package and name) whose `Param` is the class's declaration
  node; on the JVM it is a `KcClassType` with a local `ClassId` and the
  local class symbol, as in FIR. +1 (→ 110).
- *Type annotations on Java supertypes; Kotlin enum entries in Java
  annotation arguments.* `class Sub implements @B(a = @A(i = 42)) Parent`:
  the Go model keeps a Java source's annotation names only, so
  `superTypes` of a Java source class reads the annotations on each
  extends or implements reference from its PSI and converts them to the
  model's annotations and values (constants through Java's constant
  evaluator, nested annotations, arrays, enum entries), with node 0, so
  they go through the same `KaAnnotation` path as Kotlin's. A Java
  annotation argument `RGB.G`, an entry of a Kotlin enum, doesn't resolve
  in Java PSI without light classes; fork patch 0006 adds
  `KcExtras.javaEnumEntry`, which finds the qualifier's class as
  `javaAnnotationClass` finds an annotation's (now `javaClassNamed`), for
  `calcValue` to look the entry up. +2 (→ 112).
- *A Java field missing its `;`; Java parameters' annotations.* A field
  whose `;` is missing before the class's closing brace made javasrc drop
  the whole file; Java PSI reports it and keeps the field, so javasrc now
  does too (its initializer stops at an unmatched closing bracket). The
  javasrc parser also kept no annotations on method parameters, so
  `Details` gave none, and a Java method's annotated parameters lost
  them in the model. +1 (→ 113).
- *Java raw types.* `KcExtras.isRawType` was false for every type, so a
  Java `List` field printed as `List<E>`. The model writes a raw type as
  a flexible class type without arguments whose class has type
  parameters, which is what `isRawType` now checks. +1 (→ 114).
- *Library class files through the Java file manager.* KSP reads a
  library class's checked exceptions, member order, backing fields and
  Java annotation defaults from its class file, through the core
  environment's Java file manager in `getAnalysisScope()`. The scope was
  empty, and the class path was registered only when the module had Java
  sources, so `getJvmCheckedException` gave nothing. The scope is now the
  whole project, the class path is always registered with PSI-backed class
  file reading (`USE_PSI_CLASS_FILES_READING`), and `kotlinBinaryClass`
  reads the Kotlin view of a class file through `KotlinBinaryClassCache`.
  Registering the roots for every module may add to environment setup;
  its cost is still to be measured with `KC_TIMING`. +1 (→ 115).
- *Java source fields' PSI.* `KcJavaFieldSymbol.psi` was always null, so
  a Java field's `docString` was null; it now gives the `PsiField`.
  +1 (→ 116).
- *A method's missing `;`.* javasrc dropped a bodiless method whose `;`
  was missing before the class's closing brace; Java PSI reports it and
  keeps the method, and so does javasrc now. +1 (→ 117).
- *Nameless object declarations.* `object : Any {}` in a class body is a
  syntax error ("Name expected") that FIR keeps, as a class named
  `<no name provided>`, which KSP lists. Lowering dropped the declaration;
  it now keeps it under that name and reports the error with the file's
  other errors. +1 (→ 118).
- *A field's missing `;` before the next member.* javasrc skipped a
  field initializer up to the next `;`, so `String s = "str"` without one
  swallowed the field after it. The initializer now ends at the next
  member's modifier, where Java PSI ends the expression. +1 (→ 119).
- *`@all:` on constructor parameters.* An `@all:` annotation on a
  constructor property applies to its parameter too, as it does to the
  property, its accessors and its field. +1 (→ 120).
- *Annotations on an extension's receiver type.* `fun @receiver:A
  String.f()` puts `A` on the receiver type, which AA models as the
  receiver parameter's annotation; KSP matches the type reference's entries
  against it by PSI. The model now carries them as the declaration's
  annotations with target `receiver`, from the AST's receiver annotations
  and, for libraries, from the method's receiver parameter (a property's
  getter's first). On the JVM those written inside the receiver type
  reference, and the library ones, are the receiver parameter's; a
  `@receiver:` annotation on the declaration stays the declaration's, as in
  FIR. +1 (→ 121).
- Constructors and synthetic enum members of Java classes (122 pass, testLocations): a Java source class's constructors return their own copies of the class's type parameters, as AA has them — it builds the class's from PSI and the constructor's from FIR, two unequal symbols, so KSP reports an annotated one once for each. A Java enum's values(), valueOf() with its parameter, and a source enum's entries have origin SOURCE_MEMBER_GENERATED (KSP's SYNTHETIC, so their types have no reference elements), as in FIR; a library enum's entries keeps JAVA_LIBRARY. testParent then differed only where stock gives a Java library class its class-file PSI (a parent `File: HSV.class`), fixed below.
- Library types keep their annotations (`@A T` in a library signature):
  kmeta decodes the JvmProtoBuf `typeAnnotation` extension's other
  annotations, with their arguments, into `Type.Annotations` (and encodes
  them back), and the library builder turns them into model annotations.
  testTypeAlias passes (123).
- Partial error types (124 pass, testErrorTypes): in the checker's
  Signatures mode, which only analysis/ka uses, a class type with an
  argument that doesn't resolve keeps its class, `Map<String, Missing>`
  with an error type argument rather than an error type as a whole, and a
  generic call with the wrong number of type arguments has its candidates'
  return class with error type arguments, `mutableMapOf<String, Missing,
  Int>()` a `MutableMap<ERROR, ERROR>`, as FIR has them. A processor sees
  this in a first round, before the classes it generates exist. Also, an
  inner class type's arguments are split among its qualifiers, so KSP's
  qualifier of `X<String>.Z<Int>` is `X<String>`.
- Java nullability annotations (125 pass, testJavaNonNullTypes): a Java
  member's type annotated with a known not-null or nullable annotation
  (JetBrains, AndroidX and the older Android ones, Eclipse, Checker
  Framework, FindBugs, Lombok, RxJava; not JSR-305) is a Kotlin non-null
  or nullable type rather than a platform type, for Java sources and class
  files alike (the class reader now keeps parameter annotations). The
  checker does this only when asked (`driver.Options.JavaTypes`, which
  analysis/ka sets), so kc builds are unchanged. Also, an error type
  that names no class, such as an unresolved call's, is nullable, as in
  FIR; an unresolved class reference stays not null.
- The `all` use-site target (126 pass, testAnnotationsInDependencies):
  `@all:A` on a property is the property's, its backing field's, its
  getter's and its setter parameter's, and a constructor property's
  parameter's, each where A's targets allow (without @Target, the
  defaults). The copies carry the target they land on, where FIR keeps
  `all`. A library property's setter parameter has the annotations on its
  setter method's last parameter.
- Java inner classes (127 pass, testReferenceElement): a Java inner (not
  static) class sees its outer class's type parameters, so a member of
  `Base<T, P>.Inner<P>` typed `T` is `Base.T`, and a field typed
  `J<String>.H` keeps its outer class's arguments (the class reader reads
  the `LJ<TT;>.H;` signature form, which javasrc now writes too). The
  model lists only the class's own type parameters, and drops javac's
  outer-instance parameter from its constructors, as FIR does. With
  analysis/ka's `driver.Options.JavaTypes` only (renamed from
  `JavaNullability`, as it now covers both), so kc builds are unchanged.
- Java enum member order (128 pass, testResolveJavaType): a Java enum's
  members are in FIR's order too, its entries among its fields.
- JDK members of the Kotlin collections (129 pass,
  testAllFunctions_kt_inherits_java): `kotlin/collections/{Iterable,
  Collection, List, Set, Map}` and their mutable classes get the members
  of their JDK classes that Kotlin's don't declare, as FIR's JVM mapped
  scope adds them: `List.getFirst()`, `Collection.stream()`,
  `Iterable.forEach(Consumer)`, `Collection.toArray(IntFunction)`. The
  checker's new `Library.JavaMethods` types them against the Kotlin
  class's type parameters. Left out: those covered by a Kotlin member (by
  JVM name and parameter count, `removeAt` as `remove`), the hidden
  `toArray()`, `reversed()`, `equals` and `hashCode`, and on the
  read-only classes the ones that mutate (`removeIf`, `sort`, `addFirst`,
  `Map.remove(key, value)`, …). Iterator and ListIterator, whose type
  parameters Kotlin names differently, get none.
- Unclosed property types (130 pass, testReplaceWithErrorTypeArgs): a
  property whose type's argument list isn't closed, `val x: Bar<out P!!>`,
  is kept with its type as far as it parsed, `Bar<out P>`, as FIR keeps
  it, rather than dropped. The rest of the declaration, which psi leaves
  in one error element, initializer included, is lost, as in FIR. The
  syntax error is still reported, so a kc build still fails.
- Kotlin constants in Java annotation arguments (131 pass,
  testAnnotationWithDefault): `@KotlinAnnotation(ConstKt.DebugKt)` names
  a `const val` in a Kotlin source file's facade class, which Java PSI
  can't resolve without light classes, so the argument was null. Fork
  patch 0008 adds `KcExtras.javaConstant`, which `calcValue` asks when
  the reference doesn't resolve. `KcRound` finds the qualifier as
  `javaClassNamed` finds a class, among source file facades
  (`JvmFileClassUtil`'s facade name) and source classes and their
  companions, and returns the `const val`'s value. Java annotations in the
  model (`javaValue`) use it too.
- Java source classes in package scopes (132 pass, testGetPackage): FIR
  lists a package's Java source classes in the iteration order of the
  fastutil `ObjectOpenHashSet` that `KotlinCliJavaFileManagerImpl.knownClassNamesInPackage`
  collects their names in (`mapToSetOrEmpty` and the package scope's
  `buildSet` keep it), D, L, C for C.java {C, D} and L.java.
  `packageDeclarations` collects them in the same set (fastutil is in
  kotlin-compiler) instead of file order.
- Class-file PSI for Java library classes (133 pass, testParent): AA
  gives a Java class from the class path its `ClsClassImpl`, so KSP's
  `parent` and `containingFile` of such a class are its .class file
  (`File: HSV.class`), and a Java library enum's synthetic `entries` is
  that class too, so KSP gives it no getter. `KcRound.classFilePsi` finds
  it with `JavaPsiFacade.findClass` in the core environment, which has the
  class path, only when asked (`KcInfo.findPsi`), and caches it per round
  so it stays the same object. Members keep no PSI; FIR gives them their
  Cls elements, which nothing in the suite or the processors reads yet.
- A Java import without its `;` (134 pass, testJavaAnnotatedUtil): Java
  PSI reports the missing `;` of a package or import declaration and
  parses on, and AA builds a Java source class's symbol from its PSI, so
  stock KSP sees every class of such a file. kc's `javasrc` parser
  dropped the whole file; it now keeps the declaration and the classes
  after it, returning the error with the file as for a malformed member.

Result: 134 of 136 pass. The remaining 2 are the tests KSP itself disables:

| Class | Tests |
|---|---|
| `disabled`: `@Disabled` in `KSPUnitTestSuite` | 2 |

#### Incremental processing

In incremental mode (`incremental = true`, Gradle's default), KSP keeps a
record of lookups: which (scope, name) pairs each source file's processing
resolved. On the next build it reprocesses only the files whose lookups hit
a changed declaration. For Java sources, KSP records the lookups itself
(`IncrementalContextAA.recordLookup*`). For Kotlin sources, the front end
records them: the stock backend's `beginRound` registers KSP's
`DualLookupTracker` with FIR, which records every name it resolves.

On kc, the checker records them instead, and the backend replays them
into the tracker (step 16): `recordsLookups` is true, and incremental
runs reprocess only the dirty files, as on stock KSP.

Patch 0007 added `KspBackendSession.recordsLookups` for a backend that
records no lookups. When a run is asked to be incremental on such a
backend, KSP deletes `caches.uptodate` from the caches directory before
processing and again after it:
- **Before:** `IncrementalContextBase` sees no marker and rebuilds. Every
  file is dirty, so every build processes every file.
- **After:** the next run, on any backend, rebuilds too, rather than
  trusting caches that miss the Kotlin sources' dependencies.
- The incremental context itself stays on, so KSP's own lookups for Java
  sources are still recorded. KSP's `testRecordJava*` tests check them;
  turning incremental off outright failed those 6.

No backend takes that path now. It stays as the fallback for one that
doesn't record lookups.

#### Step 16: incremental lookups

kc's checker records the names it looks up, and KSP's incremental caches
on kc get the Kotlin sources' dependencies the way FIR gives them on stock
KSP.

The pieces:
- **Checker** (`internal/check/lookups.go`): a `Recorder` that also
  implements `LookupRecorder` is told each (file, scope, kind, name) once
  per file. They are recorded where FIR records them
  (`FirLookupTrackerComponent`'s callers):
  - each simple name in the package and class scopes it is searched in:
    the enclosing classes, their supertypes and companions, the file's
    package and the star imports;
  - a resolved class in its package or outer class;
  - a member's name in its receiver's class and supertypes;
  - a callable or property in its owner, or in the package of its facade;
  - the classes of resolved types, type arguments included;
  - explicit imports in their package or class.

  Locals and parameters are found before any scope FIR records, so they
  aren't recorded. Neither are explicit and default imports as scopes:
  FIR's scopes for them have no owners. `TestLookups` checks a two-file
  fixture.
- **Wire** (`analysis/ka`, `internal/kaops`): `Session.Update` collects
  them per file, sorted and deduplicated, into `File.Lookups`. kaops
  writes them after the file's declarations (`lookups = n:Uint (scope
  kind name)*n`), and `KcModel` decodes them as `KcFile.lookups`.
- **Replay** (`KcBackend.beginRound`): KSP calls `beginRound` at the
  start of each round. The backend records each file's lookups into
  `DualLookupTracker` the first time it sees the file: first the
  sources, then, in later rounds, the generated files. It records them
  under the file's virtual-file path, which is what `KSFile.filePath`
  gives. `recordsLookups` is true.

All lookups are replayed as `ScopeKind.PACKAGE`, class scopes too. FIR does
the same: `IncrementalPassThroughLookupTrackerComponent` records every
lookup with `PACKAGE`, so on stock KSP `DualLookupTracker.classTracker`
(fed only by `CLASSIFIER` lookups) stays empty. KSP finds the files that
a changed class member dirties through its symbol lookups,
(`a.B`, `member`) included. The Go side and the wire keep the real kind.

The checker resolves every declaration and body of every file, eagerly.
FIR, on stock KSP, resolves only what processing asks for, lazily. So kc
records a superset of stock KSP's lookups. Extra lookups only dirty more
files, never fewer, so the outputs stay correct. One known source of
extras: the checker's probes for inner classes and type aliases
(`innerNamed`) look a member's name up as a classifier in the enclosing
scopes too.

`build.sh test` checks it on `test/ic`, six files with Moshi's codegen:
- `m/Address.kt` (`@JsonClass` `Address`);
- `m/User.kt` (uses `Address`);
- `m/Order.kt` (uses `User`);
- `m/Settings.kt` (declares a `street` of its own);
- `n/Label.kt` (reads `Address.street`, unannotated);
- `n/Util.kt` (a top-level `street()`).

`RunKsp` takes `-Dksp.modified`, `-Dksp.removed` and
`-Dksp.incrementalLog`, as Gradle's KSP task passes them. The check runs
three times:
1. An incremental run, with every file dirty.
2. `test/ic/edit/m/Address.kt` (adds a property) copied over the source,
   then an incremental run told that `Address.kt` was modified. Its dirty
   set, read from `caches/logs/kspDirtySet.log`, must be
   `test/ic/dirty.txt`: `Address`, `User`, `Order` (it uses `User`,
   whose file is dirty, and KSP propagates dirtiness), `Label`.
   `Settings` and `Util` stay clean.
3. A clean, non-incremental run over the edited sources. Its outputs must
   be the second run's: the clean files' adapters are kept, and the dirty
   ones are regenerated.

`build.sh stock` runs the same test on stock KSP 2.3.6. Its outputs match
too, and its dirty set differs only as recorded in `test/ic/stock.diff`.
Stock KSP leaves `n/Label.kt` clean: it has no annotations, and no
processor asks for its declarations, so FIR never resolves its signature
or body and records nothing for it. kc's checker resolved
`a.street` and recorded (`m.Address`, `street`), so on kc `Label.kt`
is reprocessed. That is the superset above: more work, the same outputs.

#### Step 17: the environment, PSI and checks kept across runs

A Gradle daemon runs `kspKotlin` again and again on the same module,
mostly with no source changed, or with one. Step 11 left each run making
a new IntelliJ environment, parsing every file's PSI and checking the
module in Go twice, once per round. Now both halves keep what they can.

**JVM: an environment per configuration** (`KcEnvironment.kt`). A
`KcEnvironment` owns the `KotlinCoreEnvironment` and its own disposable,
not the run's, and registers kc's `KaSessionProvider` and modification
trackers once. `KcEnvironments.acquire` gives a run the idle environment
of its configuration, and `release` takes it back when the session
closes; the two used last are kept, the others disposed. The key is the
class path and friends (path, size and modification time of each jar),
the JDK's `lib/modules`, the Java source roots, and each Java source
file's stamp: Java PSI resolves names through the Java file manager's
index of the roots, built once per environment, so a changed Java source
makes a new environment. `-Dkc.keepEnvironment=false` turns reuse off.
`KcSessionProvider` drops the run's session on release, so a stale
`analyze` on a kept environment fails with "no KSP run is using this
environment" instead of answering from the last run.

`KcEnvironment.psiFile` gives the PSI parsed before when the file's text
(`LoadTextUtil.loadText`) is the cached view provider's; otherwise it
drops the view provider and parses the file again. The old file's PSI is
left alone, so the last run's symbols over it stay as they were. A
generated file that KSP writes again with the same text keeps its PSI
too: `bench/ksp`'s `KspBench` now writes every run to the same output
directory, as Gradle does (`build/generated/ksp`); the per-run temporary
directory gave the generated files new paths each run.

**Go: checks kept** (`analysis/ka/reuse.go`). `Session.Update` reads the
texts first. The last two checks are kept: for a Kotlin-only module in
the library its sessions share, for a module with Java sources (checked
against a library of its own, step 15) in a process-wide cache by the
class path's key, together with that library. An update whose files and
texts are a kept check's gets that check's files and diagnostics back,
without parsing or checking (`reused=0.3` ms). An update whose texts
differ parses only the changed files again, reusing the other files'
snapshots, and checks the module whole: the checker resolves a file's
declarations against the whole module's (an inferred type, an overload,
a supertype from any file), and keeping a module's declarations from one
check to the next would mean making the checker incremental. That is
left for later; a one-file edit costs a whole check (about 120 ms of
signatures on octi-server, below).

Not done: parsing only the files KSP touches. KSP's `getAllFiles` and
`getSymbolsWithAnnotation` walk every source file, and `KcRound` maps
each file's nodes to its PSI up front, so every file is touched; with the
PSI kept it is parsed once per daemon anyway.

**Numbers.** octi-server (`github.com/d4rken-org/octi`, `server`
module, HEAD 6e3323a, 60 Kotlin files, 7740 lines; Dagger, 31 generated
Java files; the Gradle class paths from step 9), 4 CPUs, JDK 21, 20 runs
in one JVM, warm the median of the last 10, and 3 cold JVMs:

| ms | cold JVM | warm total | warm setup | warm process | between rounds |
|---|---|---|---|---|---|
| stock KSP 2.3.6, before | 6640–10530 | 799 | 109 | 690 | 5 |
| kc, before | 3700–4320 | 701 | 182 | 225 | 300–520 |
| stock KSP 2.3.6, after | 6190–6440 | 773 | 111 | 652 | 5 |
| kc, after | 3470–3610 | 176 | 19 | 130 | 17–28 |

(Before is this branch before step 17, with the benchmark's fixed output
directory. Its warm kc total is above step 11's 284 ms: Dagger's
generated Java now reaches the model (step 15), so the second round
checks again, against a library of its own, about 350–450 ms.)

A warm kc run is 4x faster than before and 4.4x faster than stock KSP's.
Per warm run kc now spends about 5 ms per round in the native update (4
of it encoding the model), 3 ms on PSI, and 6–9 ms in `KcRound`; the
environment is ready in 0.3 ms. The rest is the processors: Dagger,
XProcessing and JavaPoet. The first run of a daemon (cold) is unchanged,
as nothing is kept yet.

The Go cache is tested by `TestReuse` and `TestReuseJava`
(`analysis/ka/reuse_test.go`): the same texts give the same files back, a
changed file is parsed again and the others' snapshots are kept, and a
Java module's check is kept with its library. `build.sh ksp` (134 pass)
and `build.sh stock` run with environments kept: KSP's tests run one
after another in one JVM, many with the same class path.

#### Step 18: the PSI join checks the kind sequence

kc joins its `psi` nodes to IntelliJ's PSI by preorder index (step 5),
and before this checked only each node's PSI class at the places it
used one. Now the join checks each file whole. The model carries, per
file, the number of kc's psi nodes and an FNV-1a 64 hash of their kinds
in preorder (`psi.Snapshot.KindSequenceHash`, a kind being its line in
`psi/kinds.txt`, 0 for one not there); kaops writes them after the
lookups (`nodes:Uint kinds:Uint`). On the JVM, `KcKinds` maps each
IntelliJ element type to the same numbers by name (the static fields of
`KtNodeTypes`, `KtStubElementTypes`, `KtTokens`, `KDocTokens` and
`TokenType`, from `kinds.txt` in the jar), and `KcRound` hashes the
preorder it already walks. A file whose count or hash differs fails the
round with `KcPsiMismatch` (the file and both counts) rather than give
symbols the wrong PSI. `-Dkc.checkKinds=false` turns it off.

It holds on everything the gates run: KSP's 134 tests, the slice, Moshi,
Dagger, Room, Hilt and octi-server.

Cost: one identity-map lookup and a multiply per node, in the parallel
walk `KcRound` already makes, and the hash per file in Go. Measured on
octi-server, kc only, 30 runs per JVM, two JVMs each with and without
the check, alternating (warm medians of the last 20 rounds):

| | `KcRound`, warm | warm total | first run's `KcRound` (rounds 1, 2) |
|---|---|---|---|
| checked | 8.4–9.2 ms | 143–148 ms | 558–572, 149–173 ms |
| `-Dkc.checkKinds=false` | 8.1–8.8 ms | 141–142 ms | 530–574, 120–143 ms |

That is under 1 ms a round warm, and about 25 ms in a cold JVM's second
round, where the code isn't compiled yet; both are near the runs' noise.

#### Step 19: member coverage

`jvm/kc-aa/members.txt` lists every Analysis API member the KSP fork's
bytecode uses (calls, field reads and method references to
`org.jetbrains.kotlin.analysis.api`) and whether kc answers it.
`build.sh members` regenerates it with `gen/Members.kt`, which reads the
fork's jar with ASM and classifies each member against kc-aa's classes:

- **kc**: a `KcAnalysis` method the `KaSession` proxy dispatches to, or
  an override in each of kc's classes implementing the owner;
- **partial**: some of those classes override it, the others leave it to
  a generated stub;
- **stub**: kc leaves it to a stub, or the proxy has no method for it; a
  call throws `UnsupportedOperationException("kc: ...")`;
- **api**: the Analysis API answers it itself (a default method, a static
  or concrete member, a constructor, a field), possibly through members
  listed here;
- **none**: an abstract member of a type kc has no class for.

| | total | kc | partial | stub | api | none |
|---|---|---|---|---|---|---|
| all | 310 | 204 | 28 | 4 | 69 | 5 |
| `KaSession` and its components (`session`) | 53 | 47 | | 2 | 4 | |
| symbols | 162 | 83 | 27 | 1 | 49 | 2 |
| types | 32 | 28 | | | 2 | 2 |
| components | 23 | 15 | | | 8 | |
| annotations | 14 | 13 | 1 | | | |
| other (scopes, signatures, project structure, ...) | 26 | 18 | | 1 | 6 | 1 |

The stubs: `KaSession.resolveToCall` and `getExpectsForActual`,
`KaSourceModule.stableModuleName` and `KaEnumEntrySymbol`'s initializer.
The `none` members are of types kc never makes: call infos, local
variable symbols, captured and intersection types. The partials are
mostly members that kc's symbol classes for one origin answer and others
(library or Java) leave to stubs, such as `psi` on a library symbol. No
gate reaches a stub; a run that does names the member.

#### Step 20: kotlinpoet's KSP interop tests

kotlinpoet's `interop:ksp` tests (`TestProcessorTest`, 17 tests, and
`KsTypesTest`, 2) run on kc with `build.sh kotlinpoet`. It clones
kotlinpoet at `KOTLINPOET_TAG` (2.4.0, 97e504b), fetches the
`kotlinpoet-jvm`/`kotlinpoet-ksp` jars of that version, compiles the
test processor and the tests unchanged, and runs them with JUnit on kc
and on stock KSP 2.3.6.

The tests use kotlin-compile-testing (kctfork), which can't run in the
fork's JVM: it brings kotlin-compiler-embeddable, whose
`org.jetbrains.kotlin` classes are the same names as the unshaded
kotlin-compiler 2.4.21 the fork runs on (stock KSP's aa-embeddable
relocates them to `ksp.org.jetbrains.kotlin`). So
`test/kotlinpoet/Compilation.kt` stands in for the part of kctfork's API
the tests use (`KotlinCompilation`, `SourceFile.kotlin`, `configureKsp`,
`kspProcessorOptions`, `kspSourcesDir`): `compile()` runs KSP on the
sources and nothing else. The generated code isn't compiled, so the exit
code is KSP's; every assertion on generated text is kotlinpoet's own.

| | pass of 19 | fail |
|---|---|---|
| stock KSP 2.3.6 | 18 | `complexAliasing` |
| kc | 18 | `complexAliasing` |

- `complexAliasing` fails on both, the same way: `DaggerProvider` (an
  alias of a class the test doesn't put on the class path in this setup)
  is an error type, which the processor rejects. Under kctfork the
  test's class path is the test JVM's, which has more on it.
- kc first passed 14: `smokeTest`, `typeArgs` and
  `regression_1513_annotation` failed on a generic annotation's type
  arguments, and `transitiveAliases` on the aliases inside an alias's
  expansion. Step 23 fixes both.

#### Step 22: the AA implementation's repository, checked again

The stock `aa` backend and the whole-run fallback from kc to it need the
Analysis API implementation 2.4.21 (FIR, low-level API, standalone),
published only in JetBrains' repositories. On 2026-11-07 every one of
them was still refused by this build host's network proxy (CONNECT 403):
`redirector.kotlinlang.org/maven/kotlin-ide-plugin-dependencies`,
`maven.pkg.jetbrains.space/kotlin/p/kotlin/kotlin-ide-plugin-dependencies`,
`packages.jetbrains.team/maven/p/kt/...` and
`cache-redirector.jetbrains.com`. Maven Central has no
`analysis-api-standalone-for-ide`. So the `aa` backend is still not
built on 2.4.21 and there is no fallback; nothing in the network
settings was changed.

#### Step 23: generic annotations' type arguments, aliases inside an alias

Two model gaps that step 20 found, both now closed:

- **Generic annotations' type arguments**, `@A<String, List<Int>>`. The
  checker only counted them; now it resolves them (reported to a
  `Recorder` as types written, like any other), ka's `Annotation` has
  them as `TypeArgs` (nil for `*`), and the wire's `annot` ends with
  them (`typeArgs:types`). On the JVM, `KcRound` records the types of
  the entry's type argument references, and `KcAnalysis.getType` gives
  the entry's type, `A<String, List<Int>>`, with them; before, it had no
  arguments and `KtTypeReference.type` of `String` was unsupported.
- **Aliases inside an alias's expansion.** For
  `typealias Alias41<Z, Q> = (Alias43<Z>) -> Alias47<Q>`, the expansion
  of `Alias41<Alias23, out Alias77<...>>` kept the abbreviation
  `Alias41` and those of its arguments (`substAbbrevs`), but the parts
  written as aliases on the right side lost theirs: `(List<Alias23>) ->
  List<out ...>`. ka now builds the model of the right side as written
  (a source typealias's, from the types the checker recorded for it) and
  puts its arguments' abbreviations into the expansion with the
  typealias's type parameters replaced by the arguments:
  `(Alias43<Alias23>) -> Alias47<out Alias77<Alias73<Int>>>`, as FIR
  has it. The right side's own top-level alias stays out (the
  expansion's abbreviation is the alias written), and a class path
  typealias's right side isn't read.

kotlinpoet's interop tests now pass 18 of 19 on kc, as on stock KSP
(step 20); `build.sh ksp` still passes 134 and `build.sh stock` is
unchanged. Tests: `analysis/ka/annottypeargs_test.go`,
`internal/kaops` `TestValues`.

#### Step 24: what a one-file edit costs, and what incremental checking needs

`BenchmarkUpdateEdit` (`analysis/ka/bench_test.go`, with `KA_BENCH` as
for `BenchmarkUpdate`) updates a module again and again with one file's
text changed each time (a comment added at its end, so its parse
differs but not its declarations). On octi-server's 60 files, 4 CPUs,
the median of 40 warm edits (`KC_TIMING`), in ms:

| read | parse | lower | signatures | bodies | model | total |
|---|---|---|---|---|---|---|
| 0.5 | 1.4 | 4.2 | 26.0 | 4.2 | 5.0 | 45.5 |

The 120 ms of step 17 was the first check of a session; once the class
path's classes are read, a one-file edit costs about 45 ms, a quarter of
a warm KSP run on kc (176 ms). Parsing is already per file. The rest is
per module: `signatures` is mostly inferring the types of expression
bodies and initializers without written types (Ktor's DSL lambdas in
octi), through every file.

Making it per file needs more than a cache:

- The checker writes its results into the syntax trees: resolved
  symbols, annotation classes, known types, about 50 assignments in
  `internal/check`. So a file's lowered tree can't be kept from one check
  to the next and checked again, and the checker would need to keep its
  results beside the trees, or lowering would have to be redone per
  check (4 ms) with the symbols kept apart.
- A file's declarations are resolved against the module's, so keeping
  the unchanged files' symbols needs knowing which of them depend on the
  edited file: its declarations' signatures as before (a body-only edit)
  is the common case and the safe one to start with, if the file has no
  declaration whose type is inferred from its body.
- The model of an unchanged file holds node IDs and types, which a
  body-only edit elsewhere doesn't change, so the model's files could be
  kept with the symbols.

This is a change to the checker's design, not to ka, and for octi it
would save about 40 ms of a 176 ms warm run. Step 25 takes a narrower
route to most of it.

#### Step 25: body-only edits check only the edited files

Step 24's route, moving the checker's results out of the syntax trees,
turned out to be a rewrite of codegen too: `internal/codegen` reads
them from the trees (about 400 uses of `.Sym` alone). This step gets
most of the gain without it, from what the checker already does lazily:
it infers an omitted type when something needs it (`FunSym.Infer`,
`VarSym.Infer`), and `inferAll` only forces that up front.

- **`check.Options.Only`** is the files to check. Every file is still
  declared and has its written signature types resolved, but inference
  (`inferAll`), `defaultImpls` (which forces the inferred types of
  supertypes' properties), exposure, serialization, boxing and clash
  checks, the parallel signature check and the per-file errors run only
  for those files. The others' omitted types are inferred only if a
  checked file needs them. `Only` nil is the whole check, unchanged.
- **ka** (`analysis/ka/reuse.go`, `signature.go`) checks only the
  changed files when an update has the files and Java texts of the last
  check, that check had no syntax errors, and each changed file declares
  what it did. `signatureHash` hashes a parse's tokens and the tree's
  shape, but leaves out whitespace, comments and the block bodies of
  functions, accessors, initializers and secondary constructors. A
  block body doesn't make a declaration's type: without a written
  return type, a function with one returns Unit. The other files' model
  files and diagnostics are kept from that check. Any other update is
  checked whole, as before.

**Tests.** `TestIncremental` makes body edits (statements, a local
class, a comment, an initializer, a secondary constructor, a body with an
error) and compares each incremental model, declarations, lookups and
diagnostics included, with a whole check of the same texts (made with
the caches off). `TestIncrementalSignature` checks that changing an
inferred type, a written return type or adding a declaration checks the
module whole. `TestIncrementalModule` does the same on a real module
(`KA_BENCH`): it adds a statement to the first function body of each
file in turn. On octi-server all 42 files with a function body give the
whole check's model, and so do all 78 when the main and test sources
are one module. A mutation that keeps the edited file's old model fails
every case.

**Numbers.** `BenchmarkUpdateEdit` on octi-server, median of 40 warm
edits, in ms:

| | read | parse | lower | signatures | bodies | model | total |
|---|---|---|---|---|---|---|---|
| whole check (step 24) | 0.5 | 1.4 | 4.2 | 26.0 | 4.2 | 5.0 | 45.5 |
| edited file only | 0.5 | 1.5 | 5.0 | 15.3 | 2.3 | 0.2 | 25.5 |

What is left runs for every file: lowering (5 ms) and resolving the
written signature types (`resolveSignatures`, 7–9 ms). Making those per
file needs the checker to resolve written types lazily, as it does
omitted ones, and to keep lowered trees, which it can't while it writes
its results into them.

`build.sh ksp` passes 134, `build.sh stock` is unchanged, and kotlinpoet
still passes 18 of 19.

#### Step 26: TachiyomiExhaustive, and what stock KSP2's processors see

KSP's own benchmark, `TachiyomiExhaustive`, now runs on both backends
(`bench/ksp/exhaustive/run.sh`): 559 Kotlin files, 136 jars, and
upstream's processor, which visits every node through the KSP API.
`-Dbench.dump=FILE` makes `KspBench` write what the processor sees
(each node's kind, name, location, resolved type and overridee), sorted,
so the two backends can be compared line by line. The bundle has no
`android.jar`, so `android.*` classes are missing on both backends and
much of the code doesn't compile: that is where kc and FIR differed most.

The fixes, each with a Go test in `analysis/ka` or `internal/check`:

- **Missing library classes** (`check.Library.ErrorTypes`): a Java or
  Kotlin library member whose type names a class that isn't on the class
  path is kept, with an error type naming the class, instead of being
  dropped. FIR keeps it too.
- **Local declarations** (`localsOf`): under `Options.Signatures`, a
  function or accessor body with a local declaration whose type isn't
  written is checked, to infer that type. FIR does this for KSP's
  `declarations` of a function. Warm runs reuse the kept check, so they
  don't pay for it.
- **Code that doesn't compile**, under the signature check only, as in
  FIR:
  - An error type fits any extension receiver, and any receiver fits an
    error type's.
  - A cast has the type it casts to, whatever it casts.
  - A function type with an error part is a `FunctionN` with error
    arguments.
  - A call with an error argument still resolves.
- **Error type names** (ka's `calleeName`):
  - an unresolved callee or reference gives its own name;
  - an uninferred type parameter gives its name (`T` of `apply`, `K` of `?:`);
  - a resolved callee's unresolved return type gives that type's name.
- **Typealiases:** the inferred type of a constructor call through a
  default typealias keeps the alias (`ArrayList()`).
- **Annotations:** a constructor property's annotations go only to its
  parameter, as the `@Target` rules say.

Of the 56839 lines of stock's dump, kc's now differs in 386 stock-only
and 317 kc-only lines. Before this step it was 4747 and 2117. What is
left, and why:

| lines | difference | why |
|---|---|---|
| ~310 | error types and their names in code that doesn't compile | FIR names some error types after the candidate it tried; kc doesn't keep one. A few expressions also infer differently when an operand is an error type. |
| ~95 | `(Unit..Unit?)`, `(Int..Int?)`, and `(MutableList<T>..List<T>)` after a `@NonNull` | kc's types can't make Unit or a primitive a platform type. They also don't record a collection's Java origin once an annotation fixes its nullability. |
| ~65 | other flexibility: `T` from a platform receiver (`apply`), the common supertype of a platform type and a non-null one | inference keeps the non-null form |
| 50 | anonymous object types | kc names the JVM class (`f$1`); FIR has an anonymous symbol with no name |
| 28 | `@Contextual`, `@StringRes` and `@DrawableRes` on type arguments in bodies | annotations of types that kc doesn't resolve in bodies |
| 2 | `ViewBoundsCheck.FLAG_*`, a package-private library field read from a file in its package | kc reads library classes' public and protected members only |

`build.sh stock` passes (slice, Moshi, IC, Dagger; Room and Hilt can't be
fetched in this container).

**Numbers** (4 CPUs, JDK 21, Exhaustive, `bench/ksp/run.sh`; stock 7
runs in one JVM, kc 10). Hot is BenchRunner's: the fastest of runs 4–6.
Warm is the median of the runs after the first.

| ms | stock KSP2 | kc | ratio |
|---|---|---|---|
| cold JVM, first run | 23184 | 5882 (setup 3752, process 2049) | 3.9x |
| Hot | 6570 | 741 | 8.9x |
| warm (median) | 6764 (setup 683, process 6148) | 639 (setup 145, process 434) | 10.6x |
| fastest run | 6336 | 601 | 10.5x |

The cold run is far from 10x: kc's setup (the IntelliJ environment and
PSI for 559 files, 3.7 s) is most of it. Phase 3 of
[ksp-perf.md](ksp-perf.md) is for that.

#### Step 27: warm runs reuse the kept round

A warm run with no edits checked nothing again (step 18 kept the check),
but still encoded the whole model over JNI, decoded it and built a new
round. Now:

- **Kept rounds.** A model names its check (ka's `Model.Check`, the same
  for an update that reuses a check whole). The JVM sends the checks of
  the rounds its kept environment holds (up to 3, which own their
  models' handles). If the update's check is one of them, Go doesn't
  encode the model, and the session reuses that round and its member
  caches. This cut setup from 130 to about 20 ms.
- **Members by signature.** `KcMemberBuilder` matched every member
  against every candidate. It now buckets them by kind, name and counts,
  which a match requires. This cut process from 547 to 349 ms.
- **Paths and PSI.** `psiFile` compares a file's bytes with the bytes its
  PSI was parsed from, instead of decoding its text again. Source paths
  are canonicalized once per process.

TachiyomiExhaustive, 4 CPUs, JDK 21, 10 runs in one JVM:

| ms | stock KSP2 (step 26) | kc | ratio |
|---|---|---|---|
| warm (median) | 6764 | 308 (setup 22, process 287) | 22x |
| Hot | 6570 | 308 | 21x |
| fastest run | 6336 | 234 | 27x |

What is left of a warm run is KSP's own work and the processor's walk
through kc-aa's symbols.

#### Step 28: one-file edits

`KspBench`'s edit mode (`run.sh`'s `EDIT=auto`) runs KSP incrementally
over a copy of the sources. Before each run it edits one file, a body
edit and a signature edit in turn. Both backends reprocess the same 3
files.

- **Lookups.** KSP's lookup storage update drops the dirty files'
  lookups and adds the tracker's. A fork patch passes the dirty files to
  `KspBackendSession.beginRound`, so kc-aa replays only their lookups.
  This took 160–300 ms of lookup cache updates off each round.
- **Signature edits check only the files they reach** (`declEdits`).
  These are the edited files, the files whose recorded lookups find a
  name the edit declares (before or after) or look in a class it
  declares, and the files declaring the same names. Any file a checked
  file's changed model reaches is checked next (`dependents`), until
  none is left. More than half the module is checked whole.
- **The rest is lowered as declarations.** A partial check lowers the
  files it doesn't check with `lower.Declarations`. Block bodies are
  empty except for a leading `contract { }` call, which `signatureHash`
  now hashes. Expression bodies of functions with a written return type
  are left out too. The checker records no lookups in those files and
  skips their opt-in walk.
- **Lookups for extensions.** `x.f()` and `x.p` now record the name in
  the file's package and star imports when they find no member, or find
  an extension, because an added extension may be found there.
  `TestIncrementalModuleDecl` caught this: it makes 1170 signature edits
  across Tachiyomi, one at a time, and compares each with a whole check.
- **Fixes found on the way:**
  - A race: two goroutines could infer the omitted type of an unchecked
    file at once.
  - A leak: `types.MakeArgs` interned type lists in a class-path
    symbol's table, which kept every check alive (40–120 MB per check).
- **Model delta.** The JVM sends its kept rounds' checks as bases. A
  model file that is the same as a base's is sent as that check, and the
  JVM reuses the `KcFile` it already decoded. This cut encode from 15–88
  to 1 ms and decode from 14–27 to 2 ms. A round also skips walking and
  hashing the PSI of files whose PSI and model nodes are unchanged
  (`KcTrees`).

Numbers, 12 runs in one JVM (runs 2–12 are edits, 3 files dirty on both
backends):

| ms | stock KSP2 | kc | ratio |
|---|---|---|---|
| body edit (median) | 871 | 204 | 4.3x |
| signature edit (median) | 881 | 296 | 3.0x |
| fastest edit | 784 | 178 | 4.4x |

Before this step kc took 405 ms for a body edit and 973 ms for a
signature edit. A kc edit run now splits as:

- Go update, 100–210 ms:
  - lower 30–60 ms, for all 559 files;
  - signatures 45 ms after a body edit, 95–125 ms after a signature edit
    (serial over every file: imports, parameter types, conflicts);
  - bodies under 4 ms;
  - model 5 ms.
- Round, 30–60 ms: kc-aa builds every file's symbols again.
- KSP's own setup and teardown, about 40 ms.

10x needs the edit run under about 85 ms. That means not lowering and
resolving the unchanged files at all, which needs the checker's module
state kept across checks, and a round built per file on demand. Both
are open.

#### Step 29: Room on kc and on stock KSP2

Room's jars are now fetched here, and `build.sh stock` passes Room's and
Hilt's cases on both backends. `bench/ksp/room/run.sh` times Room's
processor (room-compiler 2.8.4) on two workloads:

- **small:** `test/room/src`, one file with 2 entities and 2 DAOs.
- **large:** `gen.sh`'s module of 100 files. Each has 2 entities (an
  `@Embedded` class, type-converted columns, a unique index, a foreign
  key), a `@Relation` class, a projection, and a DAO of 12 methods
  (`suspend`, `Flow`, `IN` lists, `@Transaction`). An `AppDb` lists all
  200 entities.

Before timing, the script checks that both backends generate the same
files: 3 for small, 101 for large.

Numbers, 4 CPUs, JDK 21. Warm is the median of 10 runs in one JVM, cold
the median of 3 fresh JVMs, and edits the medians of 12 incremental runs:

| ms | stock KSP2 | kc | ratio |
|---|---|---|---|
| large, warm | 8104 | 3716 | 2.2x |
| large, Hot | 8449 | 3665 | 2.3x |
| large, cold | 20829 | 11750 | 1.8x |
| large, body edit | 9655 | 4877 | 2.0x |
| large, signature edit | 9712 | 4662 | 2.1x |
| small, warm | 555 | 135 | 4.1x |
| small, cold | 5134 | 2761 | 1.9x |
| small, body edit | 735 | 209 | 3.5x |
| small, signature edit | 596 | 218 | 2.7x |

Room bounds the ratio. In JFR profiles of warm runs on the large module
(runs 3–6, main thread), Room's own work takes about 2.7 s a run on
both backends:

- XProcessing: about 9% of stock's samples and 15% of kc's.
- Room, with its sqlite-jdbc query verification: about 10% of stock's
  and 19% of kc's.
- KotlinPoet and JavaPoet writing the output: about 13% of stock's and
  23% of kc's.

The backend takes about 6.1 s a run on stock (the Analysis API) and
about 1.3 s on kc (kc-aa and libkc), so the backend alone is about 4.7x
faster. KSP's symbol layer takes about 0.6 s on both. Even if the
backend took no time at all, the large module couldn't get better than
about 2.9x.

Edits gain nothing over full runs on either backend. Both reprocess the
same 2 files, but the `@Database` aggregates every entity, so Room
generates all 101 files again.

#### Step 30: Hilt on kc and on stock KSP2

`bench/ksp/hilt/run.sh` times Hilt's processors (hilt-compiler 2.59.2,
with dagger-compiler's) on two workloads:

- **small:** `test/hilt/src`, one file.
- **large:** `gen.sh`'s module of 100 files. Each has:
  - `@Inject` classes, one `@Singleton` and one that injects the
    previous file's class;
  - a `@Binds` module, and an object module with `@Named`, `@IntoSet`
    and `@IntoMap` bindings, both `@InstallIn(SingletonComponent::class)`;
  - an `@EntryPoint`;
  - an `@AssistedInject` class with its factory.

  Every tenth file also defines a scoped child component.

There is no Hilt root (`@HiltAndroidApp` is Android's), so Hilt
generates factories and its aggregated dependencies, but no components.
It runs 28 rounds. Both backends generate the same files: 12 for small,
1260 for large. `bench/ksp/workload.sh` now holds what the Room and Hilt
scripts share, including that check.

Numbers, as in step 29:

| ms | stock KSP2 | kc | ratio |
|---|---|---|---|
| large, warm | 4415 | 1486 | 3.0x |
| large, Hot | 4716 | 1500 | 3.1x |
| large, cold | 17545 | 8708 | 2.0x |
| large, body edit | 5294 | 2396 | 2.2x |
| large, signature edit | 5253 | 2487 | 2.1x |
| small, warm | 457 | 82 | 5.6x |
| small, cold | 4073 | 2262 | 1.8x |
| small, body edit | 407 | 218 | 1.9x |
| small, signature edit | 512 | 235 | 2.2x |

An edit of `Feature000.kt` dirties all 100 files on both backends,
because each file injects the previous file's class.

JFR samples of warm runs on the large module (runs 3–6, main thread):

| samples | stock KSP2 | kc |
|---|---|---|
| Hilt and Dagger (XProcessing, Dagger, JavaPoet/KotlinPoet/google-java-format) | 1756 | 1740 |
| writing the 1260 files (`CodeGeneratorImpl.createNewFile`, in native I/O) | 878 | 1394 |
| the backend (Analysis API on stock; kc-aa and libkc on kc) | about 6700 | 338 |
| all | 10167 | 4066 |

On stock KSP the symbol layer's classes are shaded under `ksp.`, so
Classify counts them with the Analysis API, and its backend figure is
approximate.

On kc, about 77% of a run is Hilt itself and writing its output, the
same work as on stock. The backend is 8%, about 170 ms. So even a
backend that took no time would leave the large module at about 3x. The
writes show more samples on kc, probably because its runs write the
same files closer together.

#### Step 31: Moshi on kc and on stock KSP2

`bench/ksp/moshi/run.sh` times Moshi's codegen (moshi-kotlin-codegen
1.15.2, with build.sh's class path for it) on two workloads:

- **small:** `test/moshi/src`, one file of four models.
- **large:** `gen.sh`'s module of 100 files. Each has four
  `@JsonClass(generateAdapter = true)` classes, in test/moshi's shapes:
  - a data class with nullable, defaulted, `@Json`-renamed, list, map and
    enum properties;
  - a generic page of it;
  - a class with `var` and `@Transient` properties;
  - one nesting them.

  Files 1 to 9 of each ten also use file 0's models.

Moshi generates an adapter and a ProGuard rule per class, in two rounds.
Both backends generate the same files: 8 for small, 800 for large.

Numbers, as in steps 29 and 30. Cold is the median of three fresh JVMs.

| ms | stock KSP2 | kc | ratio |
|---|---|---|---|
| large, warm | 2911 | 1039 | 2.8x |
| large, Hot | 2769 | 1008 | 2.7x |
| large, cold | 11022 | 5802 | 1.9x |
| large, body edit | 1016 | 732 | 1.4x |
| large, signature edit | 1071 | 704 | 1.5x |
| small, warm | 271 | 31 | 8.7x |
| small, cold | 2967 | 1666 | 1.8x |
| small, body edit | 438 | 94 | 4.7x |
| small, signature edit | 363 | 97 | 3.7x |

An edit of `Model000.kt` dirties 10 files on both backends: the file
itself and the nine that use its models.

JFR samples of runs 3–6 on the large module (main thread):

| samples | warm, stock | warm, kc | edit, stock | edit, kc |
|---|---|---|---|---|
| Moshi's codegen and KotlinPoet | 1260 | 1262 | 241 | 245 |
| writing the generated files (`createNewFile`, native I/O) | 1042 | 1527 | 133 | 123 |
| KSP's incremental bookkeeping (`IncrementalContextBase`) | | | about 1000 | about 1040 |
| the backend (Analysis API on stock; kc-aa, libkc and kc's IntelliJ core on kc) | about 4400 | about 170 | about 1650 | about 930 |
| all | 7132 | 3378 | 3084 | 2417 |

As in step 30, Classify counts stock's shaded KSP classes with the
Analysis API, so stock's backend figures are approximate.

What bounds the ratio:

- **Warm runs:** on kc, writing the 800 files is 45% of a run and Moshi
  with KotlinPoet 37%. The backend is 5%, about 40 ms. A backend that
  took no time would leave the ratio near 3x.
- **Edit runs:** KSP's own incremental bookkeeping (updating its caches
  and output maps, and computing the dirty set) takes about 260 ms per
  run on both backends. That is close to half of kc's 700 ms.

kc's share of an edit run is about 160 ms (`KC_TIMING=1`), in two
module updates, each with `reused=0`:

- **Round 1:** the 100 sources, with one changed. About 45 ms; their
  signatures take 18 ms.
- **Round 2:** the same 100 sources plus the 40 regenerated adapters.
  About 110 ms; parsing the adapters takes 15–45 ms and their bodies
  22 ms.

Both updates check the whole module's signatures again, though only one
file changed. Reusing the unchanged files' results there, and checking
generated files' declarations without their bodies, are what is left
for kc on this workload. Even with both, the bookkeeping and the writes
would hold the large edit ratio under about 2x.

#### Step 32: Room, Hilt and Moshi after the output, bookkeeping and lookup fixes

Steps 29–31 left kc at 2–3x over stock on the three processor workloads.
This step cuts what the KSP fork and kc spend around the processors, and
adds a third edit: `EDIT=leaf` in each workload's `run.sh` edits the last
generated file (`r/Item099.kt`, `h/Feature099.kt`, `m/Model099.kt`),
which no other file uses. `EDIT=auto` still edits file 000, which Hilt's
and Moshi's other files use.

Changes:

- **Output write-behind (fork patch 0010).** `CodeGeneratorImpl` hands
  each processor an in-memory stream. When it closes, a pool of up to
  four threads writes the bytes, skipping a file whose bytes on disk are
  already the same. The pool is drained before anything reads the
  outputs: the next round's files, `outputs` and `generatedFile`, and
  the end of the run. Write errors are rethrown there.
- **Backup copies (fork patch 0011).** `IncrementalContextBase.updateOutputs`
  skips a backup or restore copy whose target has the same length and
  modification time. `copyWithTimestamp` keeps both, so the test is
  exact.
- **Round updates reuse the last check of the same files.** KSP's rounds
  alternate between file sets (the sources, then the sources with the
  generated files). `checkCache.find` now prefers the most recent check
  with the same paths, so each round's update after an edit is
  incremental (`reused=99`) instead of a whole check (`reused=0`).
  `TestIncrementalRounds` covers it.
- **Per-call lookups on Room's hot paths:**
  - The fork's `analyze { }` keeps the module's `KaSessionProvider` with
    the module, instead of a project service lookup per call, and skips
    the `ThreadLocal` on the thread that set it (patch 0012).
  - `KcAnalysis`'s proxy remembers the `KaSession` methods it has no
    implementation for. `computeIfAbsent` stores no null, so each such
    call ran `getMethod` and its `NoSuchMethodException` again.
  - A class type keeps its class's symbol once kc has it.
  - `asSuper` keeps its answers per pair of classes for the round: a miss
    always, and the supertype of a class type without arguments.

  A class kc doesn't have yet can turn up later in the round (Room's
  type converters do), so neither cache keeps answers that involved
  one.

Numbers: 4 CPUs, JDK 21, `run.sh 10`, both backends back to back on the
same machine. Warm is the median of runs 2–10 in one JVM, edits the same
with `COLD=0`, cold the median of two fresh JVMs. Stock varies by up to
30% between batches on this machine, so compare within a row.

| ms | stock KSP2 | kc | ratio | steps 29–31 |
|---|---|---|---|---|
| Room large, warm | 8782 | 2755 | 3.2x | 2.2x |
| Room large, edit Item000 | 11068 | 3548 | 3.1x | 2.0x |
| Room large, leaf edit | 10850 | 3550 | 3.1x | 2.2x |
| Room large, cold | 24051 | 11582 | 2.1x | |
| Room small, warm | 647 | 144 | 4.5x | |
| Room small, edit | 651 | 213 | 3.1x | |
| Hilt large, warm | 4525 | 861 | 5.3x | 3.0x |
| Hilt large, edit Feature000 | 5971 | 1751 | 3.4x | 2.2x |
| Hilt large, leaf edit | 585 | 341 | 1.7x | 1.2x |
| Hilt large, cold | 17000 | 7681 | 2.2x | |
| Hilt small, warm | 465 | 70 | 6.6x | |
| Hilt small, edit | 470 | 229 | 2.1x | |
| Moshi large, warm | 2419 | 629 | 3.8x | 2.8x |
| Moshi large, edit Model000 | 774 | 245 | 3.2x | 1.4x |
| Moshi large, leaf edit | 421 | 131 | 3.2x | 1.3x |
| Moshi large, cold | 9846 | 5239 | 1.9x | 1.9x |
| Moshi small, warm | 247 | 20 | 12x | 8.7x |
| Moshi small, edit | 265 | 50 | 5.3x | 4.7x |

The leaf column of steps 29–31 is from the start of this step on the
same machine: stock 10696, 858 and 597 ms; kc 4961, 704 and 474 ms.

The leaf edit dirties one file on both backends. On Room it costs as much
as a full run: Room's database processor reads every entity and DAO
again whenever it runs.

Where kc's time goes now:

- **Room:** the processors. Room, XProcessing and KotlinPoet are about
  90% of kc's run, and take as long on stock, so 10x is out of reach; a
  backend that took no time would get about 3.5x. In JFR samples the
  share of kc's own JVM layer fell from 12% to about 8%; the
  `KcAnalysis` proxy's reflection is the largest piece left (about 2%).
- **Hilt edits:** the generated Java. Dagger writes Java sources, and a
  module with Java sources gets a new library for every update: the JDK
  and class path are opened again, and the Kotlin sources are checked
  whole (`reused=0`), because the Java source classes in the library's
  class table refer to one check's Kotlin classes. On the leaf edit that
  update is 170 of the 186 ms between rounds; on the small module it is
  155 ms. Splitting the class table into a shared class-path part and a
  per-check part for Java sources is the next lever for Hilt and Dagger.
  Even then the leaf edit would be near 3x: its 28 rounds of processors
  take about 50 ms.
- **Moshi edits:** fixed costs around 14 ms of processing. Of a 131 ms
  leaf edit, JFR puts about 70% of the setup and between-rounds samples
  in kc's two module updates. Each one lowers all 100 files again and
  declares their signatures (15–25 ms), even with 99 reused. About 22%
  is KSP's own bookkeeping (`updateCachesAndOutputs`, `calcDirtyFiles`).
  GC pauses are about 20 ms per run.

What 10x would take is in `docs/ksp-perf.md`.

#### Step 33: Java sources on the shared library, and a lighter round

Step 32 left three kc-side costs on the edits: a new library and a whole
check for every update of a module with Java sources (Hilt and Dagger),
lowering every file again on each update, and kc-aa building every
file's symbols again each round.

Changes:

- **A class-path overlay for Java sources.** `classpath.Path.Overlay`
  returns a path that adds classes on top of a base path without
  touching it. `driver.AddJavaSources` adds a check's Java sources to an
  overlay of the shared class path, so the shared `Path` is never
  mutated. `IsSource` and `HasSources` look only at the overlay's own
  classes; everything else falls through to the base.
- **A per-check class table.** `check.Library.WithSources` returns a
  library whose class table is a child of the shared one. The child
  loads the overlay's source classes itself (marked
  `types.ClassSym.Source`) and asks the parent for everything else, then
  the program's Kotlin classes. The parent never sees the child, so jar
  classes resolve only to jar classes. Results that involve a source
  class (Java methods, nested classes, inherited defaults, interned type
  arguments) go to the check's own caches instead of the shared ones.
  This differs from kotlinc only when a Java source shadows a jar class:
  jar signatures keep the jar class.
- **ka shares the library with modules that have Java sources.** An
  update with Java sources now takes the module's kept library and
  layers a per-check `library` over it (`library.withJava`): its own
  declarations for the source classes, and the base's for the rest.
  `acquireJava`, which opened the JDK and class path for every update,
  is gone. The check cache is shared too, so a Kotlin body edit with
  unchanged Java texts is incremental. `TestSharedLibraryJava` and
  `TestIncrementalJava` cover it.
- **No provenance for declarations-only files.** `lower` records no
  provenance, annotation map or negation map for files that are only
  declared; their provenance was never read.
- **kc-aa registers pointer keys lazily.** `KcRound`'s builder put every
  source symbol's string key into `byKey`, which only symbol-pointer
  restores read. The keys now wait in a list until the first restore of
  the round. The round's identity maps start at the previous round's
  sizes. On a Moshi leaf edit this halved the builder's JFR samples
  (592 to 291) and cut kc's per-round build (`round=`) from a median of
  about 19.5 ms to 13.5 ms.

Not done: plan B2, keeping a lowered tree per snapshot and cloning it
for each update. `BenchmarkUpdateEdit` on Moshi's 100 files puts all of
an update's lowering at about 2 ms (4–12 ms in the JVM, with GC), so a
clone could save at most about 3 ms per update, less than the run's
noise. B1 gave no measurable time either; it stays because it allocates
less.

Numbers as in step 32 (4 CPUs, JDK 21, `run.sh 10`, both backends back
to back):

| ms | stock KSP2 | kc | ratio | step 32 |
|---|---|---|---|---|
| Room large, warm | 7159 | 1991 | 3.6x | 3.2x |
| Room large, edit Item000 | 7942 | 2911 | 2.7x | 3.1x |
| Room large, leaf edit | 8208 | 2618 | 3.1x | 3.1x |
| Room large, cold | 16047 | 9472 | 1.7x | 2.1x |
| Room small, warm | 429 | 130 | 3.3x | 4.5x |
| Room small, edit | 664 | 185 | 3.6x | 3.1x |
| Hilt large, warm | 3940 | 647 | 6.1x | 5.3x |
| Hilt large, edit Feature000 | 3608 | 1131 | 3.2x | 3.4x |
| Hilt large, leaf edit | 461 | 167 | 2.8x | 1.7x |
| Hilt large, cold | 16681 | 6639 | 2.5x | 2.2x |
| Hilt small, warm | 401 | 42 | 9.5x | 6.6x |
| Hilt small, edit | 426 | 95 | 4.5x | 2.1x |
| Moshi large, warm | 2195 | 407 | 5.4x | 3.8x |
| Moshi large, edit Model000 | 564 | 225 | 2.5x | 3.2x |
| Moshi large, leaf edit | 328 | 106 | 3.1x | 3.2x |
| Moshi large, cold | 8044 | 4145 | 1.9x | 1.9x |
| Moshi small, warm | 215 | 23 | 9.3x | 12x |
| Moshi small, edit | 281 | 43 | 6.5x | 5.3x |

The ratio column compares within a row; stock moved by up to 30% from
step 32's batch (Hilt's edit of Feature000: 5971 then 3608 ms), and the
short kc runs by about 30 ms, so ratios on rows kc didn't touch (Room,
the edits of file 000) moved both ways. kc's own times are the steadier
comparison: Hilt's leaf edit went from 341 to 167 ms, Hilt small's edit
from 229 to 95 ms, Hilt's edit of Feature000 from 1751 to 1131 ms, and
Moshi's edit of Model000 from 245 to 225 ms. Measured just after the Java change alone, Hilt
small's edit was 62 ms (6.1x) and Hilt large's leaf edit 158 ms (2.5x).

`KC_TIMING=1` on Hilt's leaf edit now reads `reused=99` and `open=0.0`
for the update with Java sources: no library is opened, and only the
edited file is checked. Its 170 ms update from step 32 is gone.

Where kc's time goes now:

- **Room:** still the processors, about 90% of kc's run.
- **Hilt leaf edit:** 28 rounds of Dagger and Hilt processors take about
  50 ms; each round's update is lowering (about 25 ms over the run in
  the JVM) and signatures (about 13 ms). KSP's setup and bookkeeping
  are about 40 ms.
- **Moshi leaf edit:** of about 106 ms, setup is about 35 ms (kc's
  module updates and KSP's bookkeeping) and processing about 11 ms.
  kc-aa's round building is now about a third of the JVM samples, spread across `info`,
  type-reference navigation and the maps. G1 young pauses add about
  8 ms per run; they come from KSP's heap as much as kc's.

#### Step 34: less per-update work in Go and in kc-aa's round

After step 33, each update of a leaf edit cost kc about 15–35 ms in Go
(`lower` plus `signatures`, with 99 of 100 files reused) and 10–17 ms
in kc-aa building the round's symbols, twice per run. This step cuts
both without keeping state across checks.

Changes in Go (`internal/check`):

- **`resolveSignatures` runs in parallel**, one file per goroutine, up
  to the update's jobs. Each checker resolves its own file's
  declarations; the race detector finds no races over `internal/...`'s
  tests, the compile corpus included.
- **`conflicts` compares only what is reported.** With `Options.Only`,
  it compares the declarations of the packages that have a checked
  file, and the members of the checked files' classes. A class's
  members and a file facade's are compared within one container, so the
  checked files' errors are the same.
- **No type references recorded for the files not checked.** As with
  lookups, a Recorder isn't told the type references of a file whose
  earlier results are kept. Each one had cost a lock and a map entry.

`BenchmarkUpdateEdit` on Moshi's 100 files (a body edit of one file,
99 reused) went from about 13.1 to 9.4 ms per update. In the JVM,
`KC_TIMING`'s `signatures` went from 7–21 ms to about 6 ms.

Changes in kc-aa (`KcRound`):

- **A source file's type references are recorded when one of them is
  looked up.** The Builder walked every declaration's type references
  against PSI. Now each file's walk waits until the first lookup of one
  of its type references, annotation entries or annotation values
  (`typeOf`, `annotationClass`, `valueOf`).
- **So are its symbols by PSI.** `KaSession.symbol`, type parameters'
  symbols and local classes go through `KcRound.symbolOf`, which records
  the file's entries first. Inserting every declaration into an identity
  map was most of `Builder.register`.
- `KC_TIMING` now splits the round's build out of `round=` as `build=`.

A leaf edit's KSP rounds process the dirty file, so most files'
entries are never recorded. `build=` went from 10–17 ms to about 8 ms
per round.

Not done: keeping the declared program across body edits (step 33's
"fixed costs per update"). The model builder needs the edited file's
new tree and provenance, since positions and node numbers move; the
checker and Universe key state on declaration nodes (`classes`,
`inlineDecls`, `done`, `serialInterfaces`); and signatures mode still
checks a body whose locals have inferred types. That is a larger
refactor than the 10 ms per update left in Go (`lower` about 5 ms,
`signatures` about 6 ms in the JVM).

Numbers as in step 32, kc's fastest run (min) beside the median, as
these changes are smaller than the runs' noise:

| ms | stock KSP2 | kc | ratio | kc min | step 33 kc (min) |
|---|---|---|---|---|---|
| Room large, warm | 7125 | 2236 | 3.2x | 2147 | 1991 (1917) |
| Room large, edit Item000 | 8440 | 2745 | 3.1x | 2669 | 2911 (2667) |
| Room large, leaf edit | 8642 | 2659 | 3.3x | 2562 | 2618 (2568) |
| Room small, warm | 492 | 116 | 4.2x | 100 | 130 (97) |
| Room small, edit | 496 | 150 | 3.3x | 122 | 185 (165) |
| Hilt large, warm | 3322 | 612 | 5.4x | 537 | 647 (588) |
| Hilt large, edit Feature000 | 3650 | 1012 | 3.6x | 913 | 1131 (999) |
| Hilt large, leaf edit | 494 | 158 | 3.1x | 111 | 167 (149) |
| Hilt small, warm | 317 | 49 | 6.5x | 46 | 42 (40) |
| Hilt small, edit | 378 | 79 | 4.8x | 74 | 95 (80) |
| Moshi large, warm | 2295 | 434 | 5.3x | 388 | 407 (392) |
| Moshi large, edit Model000 | 580 | 248 | 2.3x | 206 | 225 (181) |
| Moshi large, leaf edit | 445 | 119 | 3.7x | 90 | 106 (80) |
| Moshi small, warm | 260 | 19 | 13.7x | 17 | 23 (21) |
| Moshi small, edit | 267 | 46 | 5.8x | 39 | 43 (37) |

Cold runs: Moshi 8570 / 4567, Hilt 14861 / 6310, Room 17232 / 9233.

The phases these changes touch are faster in every `KC_TIMING` run,
but the whole runs move by less than their spread: a leaf edit's two
updates and two rounds save about 15–20 ms when no GC pause lands in
them, and G1's young pauses (13–33 ms, about one per run) land in a
different phase each run. Hilt's leaf edit and both Hilt edits are
faster in median and minimum. Moshi's large edits are slower in this
batch's median and minimum (leaf 119 against 106 ms), within the
10–30 ms by which the same code moves between batches.

What is left on kc's side of a leaf edit, from `KC_TIMING` and JFR:
each update's `lower` (about 5 ms in the JVM) and `signatures` (about
6 ms), kc-aa's round build (about 8 ms), and decoding and the native
call around them; KSP's own setup and bookkeeping is about as much
again.

#### Step 35: a round builds only the files KSP reaches

Step 34 left kc-aa's round build at about 8 ms per round, and a JFR
profile of Moshi's leaf edit still had `KcRound.<init>` as the largest
kc frame. It also had restores (`KSFileImpl.defer` → `KcPointer`) flushing
every file's pointer keys. `KC_TIMING` now prints, for each round, how
many source files the previous round built, had a key restored from, and
looked up by PSI or type reference. On a leaf edit those are 2, 0 and 2
of 100: KSP's rounds process the dirty file and reach few others.

Changes in kc-aa (`KcRound`):

- **A source file's symbols are built the first time they're needed.**
  The round makes each file's `KcFileSymbol` up front, with its
  annotations, and builds its declarations on the first of:
  - its `declarations`;
  - a class or type alias it declares, found through an index of the
    model's class IDs (`sourceClass`, `knownTypeAlias`), which `findClass`,
    `classSymbol` and package scopes go through;
  - a pointer key of the file restored;
  - a lookup by its PSI, type references, annotation entries or values.
  
  A class ID declared twice keeps the eager build's choice: the last
  file's class and the first file's type alias.
- **Pointer keys wait per file.** Every key of a file's symbols starts
  with the file symbol's (`file:<path>`), so a restore records only the
  keys of the file it names, not every file's.
- **The type reference walk waits too.** `info` queued the declaration's
  reference (from step 34) but still walked its PSI for the references;
  the walk itself now waits with the file's other deferred records.

`build=` went from about 8 ms to about 2 ms per round on Moshi's and
Hilt's leaf edits.

Changes in Go (`analysis/ka`):

- **Signature edits check only the files they reach with Java sources
  too.** `declEdits` refused any module with Java sources, so every
  signature edit of Hilt's second round, which has Dagger's generated
  Java, checked all 112 files (about 50 ms in the JVM). It now applies
  when the Java texts are unchanged. A Kotlin lookup through a Java class
  already records its Kotlin supertypes' scopes, and a Java signature's
  Kotlin types record their classes when used. As a safeguard, the
  classes of the Java sources that name a changed class, or a changed
  file's facade class, count as changed. A changed file that may rename
  its facade (`@JvmName`, `@JvmMultifileClass`) checks the whole module.
  `TestIncrementalJavaDecl` compares six such edits with a whole check.
  Hilt's second-round updates went from about 50 to 13–25 ms.

Tried and dropped: a higher GOGC in libkc. The GC takes about a quarter
of an update's CPU, and GOGC=400 (with a 768 MiB soft limit) cut
`BenchmarkUpdateEdit` from 9.1 to 7.2 ms. But Room's runs, whose live
Go heap is 90–190 MB, were about 140 ms slower with it in two A/B pairs,
which outweighs 2 ms per update on Moshi and Hilt.

Room's processing is mostly Room's own code: in a JFR profile of its
leaf edit, kc's `KaSession` handler is about 11% of the samples and the
proxy dispatch around it about 2%.

Numbers as in step 34 (10 runs per case, kc's fastest beside the
median):

| ms | stock KSP2 | kc | ratio | kc min | step 34 kc (min) |
|---|---|---|---|---|---|
| Room large, warm | 7091 | 2342 | 3.0x | 2122 | 2236 (2147) |
| Room large, edit Item000 | 8673 | 2719 | 3.2x | 2619 | 2745 (2669) |
| Room large, leaf edit | 8650 | 2780 | 3.1x | 2667 | 2659 (2562) |
| Room small, warm | 524 | 117 | 4.5x | 101 | 116 (100) |
| Room small, edit | 577 | 143 | 4.0x | 110 | 150 (122) |
| Hilt large, warm | 3979 | 534 | 7.5x | 481 | 612 (537) |
| Hilt large, edit Feature000 | 3928 | 1069 | 3.7x | 998 | 1012 (913) |
| Hilt large, leaf edit | 658 | 128 | 5.1x | 108 | 158 (111) |
| Hilt small, warm | 366 | 52 | 7.0x | 46 | 49 (46) |
| Hilt small, edit | 375 | 69 | 5.4x | 63 | 79 (74) |
| Moshi large, warm | 2202 | 464 | 4.7x | 447 | 434 (388) |
| Moshi large, edit Model000 | 691 | 165 | 4.2x | 155 | 248 (206) |
| Moshi large, leaf edit | 387 | 81 | 4.8x | 74 | 119 (90) |
| Moshi small, warm | 232 | 19 | 12.2x | 17 | 19 (17) |
| Moshi small, edit | 227 | 40 | 5.7x | 36 | 46 (39) |

Cold runs: Moshi 8170 / 4349, Hilt 14782 / 6343, Room 18132 / 8954.

The leaf edits are faster in median and minimum: Moshi's from 119 to
81 ms, Hilt's from 158 to 128 ms. Moshi's edit of Model000, whose
dirty files reach few others, went from 248 to 165 ms. Hilt's
Feature000 edit dirties all 100 files and builds them all, as before.
Stock's runs move between batches too (Hilt's warm 3322 then 3979 ms),
so the ratios move more than kc's times. Room is unchanged within noise.

What is left on kc's side of a Moshi leaf edit (about 80 ms): two
updates in Go of about 15 ms each (`lower` about 5, `signatures` about
6), a round build of about 2 ms each, and KSP's own setup, between-round
and teardown bookkeeping (`calcDirtyFiles`, `updateCachesAndOutputs`,
the lookup storage), about as much again.

#### Step 36: KSP's caches stay in memory, and updates reuse lowered declarations

After step 35, a Moshi leaf edit took about 80 ms on kc. Roughly 30 ms
was kc's two Go updates. Another 35–40 ms was KSP's own incremental
bookkeeping, which stock KSP pays too. A JFR profile at 1 ms sampling
showed where KSP's part went:

- **JSON maps.** Every run decoded the `symbols`, `sealed` and
  `sourceToOutputs` maps (`PersistentMap.deserialize`) and wrote all
  three back at the end, changed or not. `sourceToOutputs` was written
  twice.
- **Lookup storages.** Every run opened Kotlin IC's two `LookupStorage`s
  and closed them at the end. A fresh storage starts with cold page
  caches, so `calcDirtyFiles`' lookups of the edited file's symbols read
  from disk (`PersistentMapImpl.get`), and opening and forcing them cost
  more.

`relativeTo` and `DirtinessPropagator`'s inverted map, which step 36's
plan also listed, were under 1 ms per run together, so they are
unchanged.

KSP-side changes (fork patch 0013). These are in KSP, not in kc's
backend. Stock KSP 2.3.x runs without them, so the comparison below
includes them on kc's side only:

- **The maps stay in memory between runs on the same caches.**
  `PersistentMap` keeps each map as last read or written in the process,
  with its file's size, modification time (ns) and file key. A later
  run gets a copy while the file still has that stamp, and decodes it
  otherwise. A map equal to the kept copy, whose file still matches, is
  not written again. The second `sourceToOutputs` write is gone. The
  copy always mirrors the file, so error and rebuild paths need nothing
  special. A missing file (a deleted caches directory) drops the entry.
- **The lookup storages stay open.** A process-wide pool, keyed by
  storage and base directory, hands a run the storage the last run put
  back. It does so only while every file in the storage's directory has
  the stamp it had then; otherwise the storage is closed and opened
  again. A run takes the storage out of the pool, so two runs on one
  caches directory never share one. It flushes and puts it back at the
  end (`LookupStorageWrapper.release`), and a run that fails closes it.

`KspBench` gets `bench.clean=N,M`, which deletes the output and caches
directories before those runs, as a clean build in the same daemon
would. Moshi's leaf edit with cleans before runs 4, 5 and 9 dirties all
100 files on those runs and 1 file on the others. Its Model000 edit
dirties 10 files on each incremental run, with or without cleans.

Moshi's leaf edit went from about 81 to 62 ms, and Hilt's from about
128 to 85 ms. In the warm JFR profile, `KotlinSymbolProcessing.execute`
went from 697 to 453 samples over 36 runs.

kc-side changes (Go):

- **Lowered declarations are kept across updates.** An update that
  checks only some files lowered every other file as declarations only
  (`lower.Declarations`), getting the same trees as the update before.
  `analysis/ka` now keeps each parse's lowered declarations
  (`declcache.go`) for as long as the parse itself is kept. The cache is
  keyed by `weak.Pointer[psi.Snapshot]`, and `runtime.AddCleanup`
  removes an entry once its snapshot is collected. The checker writes
  into the trees it checks, so each update gets a copy.
- **The copy is generated.** `syntax.Clone` is a typed deep copy, made
  from the AST's types by `TestCloneGenerated`, which also fails when
  `clone_gen.go` is stale. A lowered tree can hold a node twice: a
  delegated property's accessors hold the property and its reference.
  `Clone` keeps such nodes shared through a map of copies, used only for
  a file that needs it, and remaps `File.Annotated`'s node keys.
  `TestClone` checks that copies of the lowered test sources (both
  `File` and `Declarations`) are equal to the originals (`syntax.Diff`)
  and share no node, slice or map with them. A copy costs about half a
  lowering, since Moshi's constructor parameters carry defaults and
  annotations.
- **`conflicts` compares fewer declarations.** It compared every
  declaration of a checked file's package. From the package's
  unchecked files, only declarations with the same name as one in a
  checked file can conflict with the checked file's, so only those are
  compared.

`BenchmarkUpdateEdit` on Moshi's large module, editing the leaf file:
about 8.9 ms before, 7.6 with the declarations kept, and 6.7 with both
changes.

Measured and left alone:

- Running the error-only passes on checked files only, and the smaller
  allocations in the checker's setup. `errorf`'s `Sprintf` and those
  passes are about 1% of an update's CPU.
- `lookupAlias`, the alias lookup of every type reference, is about 9%,
  and data classes' members about 6%. Both are part of re-resolving
  every file's signatures on every update, which keeping the declared
  program across updates would remove. That is the remaining large cut
  on kc's side (step 34).

The batch as in step 35 (10 runs per case, kc's fastest beside the
median). The machine was noisier than in step 35's batch: rows where
nothing changed for kc, like Room, moved by up to 10% each way.

| ms | stock KSP2 | kc | ratio | kc min | step 35 kc (min) |
|---|---|---|---|---|---|
| Room large, warm | 7067 | 2514 | 2.8x | 2336 | 2342 (2122) |
| Room large, edit Item000 | 8154 | 2796 | 2.9x | 2605 | 2719 (2619) |
| Room large, leaf edit | 8331 | 2998 | 2.8x | 2625 | 2780 (2667) |
| Room small, warm | 435 | 103 | 4.2x | 80 | 117 (101) |
| Room small, edit | 644 | 180 | 3.6x | 157 | 143 (110) |
| Hilt large, warm | 3916 | 692 | 5.7x | 601 | 534 (481) |
| Hilt large, edit Feature000 | 4113 | 1161 | 3.5x | 1077 | 1069 (998) |
| Hilt large, leaf edit | 535 | 102 | 5.2x | 71 | 128 (108) |
| Hilt small, warm | 403 | 59 | 6.8x | 53 | 52 (46) |
| Hilt small, edit | 279 | 53 | 5.3x | 49 | 69 (63) |
| Moshi large, warm | 2270 | 458 | 5.0x | 383 | 464 (447) |
| Moshi large, edit Model000 | 610 | 167 | 3.7x | 148 | 165 (155) |
| Moshi large, leaf edit | 405 | 75 | 5.4x | 66 | 81 (74) |
| Moshi small, warm | 204 | 23 | 8.9x | 21 | 19 (17) |
| Moshi small, edit | 211 | 32 | 6.6x | 29 | 40 (36) |

Cold runs: Moshi 9015–9537 / 4312–4622, Hilt 14746–15723 / 5712–6488,
Room 16082–16623 / 8134–9094.

**Ten runs flatter kc's leaf edits.** `warm` is the median of the second
half of the runs. With 30 runs, stock's JIT keeps warming up: its leaf
edits went to 246 ms (Moshi) and 375 ms (Hilt), and kc's medians were
76 and 69 ms (minimums 43 and 61). That puts the ratios at 3.2x and
5.4x, not the 5x of ten runs. kc's Moshi runs are bimodal: body edits
take 54 ms and signature edits 77 ms. Later steps should report leaf
edits from 30 runs or more.

The leaf edits that KSP's caches dominated are faster in median and in
minimum. Moshi's minimum went from 74 to 66 ms in the batch, and to
43 ms in the 30-run pair. Hilt's went from 108 to 71 ms. The warm
cases, which process all files, didn't use these paths, so their
differences are this batch's noise.

#### Step 37: a body edit keeps the edited file's model, without a check

After step 36, a leaf body edit on Moshi took about 54 ms with kc. Each
KSP run makes two Go updates, one per round, and `BenchmarkUpdateEdit`
put one update at about 6.4 ms: 1.3 ms to lower (a copy of 99 kept
trees), 3.6 ms to resolve all 100 files' signatures, and GC for about a
third of the CPU. Re-resolving was the cost step 36 left.

**Keeping the checker's Universe across updates was weighed and set
aside.** That was the cut steps 34 and 36 pointed to. Symbols point into
syntax trees, and the phases build links both ways: sealed subclasses,
overrides, alias files. `resolveClass` is not idempotent (it makes a new
constructor and appends to the class's properties). The checker keeps
declared and body state together in about 100 fields. Taking a new tree
for one file under the others' declarations means separating all of
that, and the results must still equal a whole check's.

**A body edit doesn't need a check at all, most of the time.** ka checks
signatures only (`check.Options{Signatures: true}`). In that mode
`checkSignatures` looks into a block body for one thing: the local
declarations written directly in it, whose inferred types it checks, and
which the model lists (`Decl.Locals`, from `fileBuilder.locals`). The
model takes nothing else from a body. `bodyEdits` already guarantees
that no token outside block bodies changed (`signatureHash`). So when a
file's change lies inside one body that declares no locals, before or
after the edit, what the check finds in the file is exactly what the
last check found. Only the psi node IDs after the body move, and the
line and column of what comes after the change.

`analysis/ka/bodyreuse.go` takes such files over (`shiftedFile`):

- **One region.** The common prefix and suffix of the old and new texts
  give the change. It must lie strictly inside the braces of one block
  body: the outermost one around it whose owner is a function, a
  secondary constructor or an initializer.
- **Only declarations' bodies, in classes and objects.** The owner's
  ancestors must be classes, objects and class bodies up to the file. A
  function of an object expression in a property's initializer may be
  checked to infer the property's type. An accessor's body decides
  whether its property has a backing field, so accessors are left out.
- **No contract, no locals.** A leading `contract { }` call changes what
  callers see. A body that declares a variable, a destructuring, a
  function, a class, an object or a type alias directly, before or after
  the edit, is checked. So is one with an annotated or labeled
  statement, which may be such a declaration.
- **The trees are verified node by node.** The new body must start where
  the old one did and end moved by the text's change. The node counts
  must differ by exactly the body's change in size. Every node before
  the body must have the same kind and start, and every node after it
  the same kind and start moved by the change. That is microseconds for
  one file, and it makes the renumbering exact.
- **The model is copied with its IDs renumbered.** `remapNodes` copies a
  `File` by reflection, so a field added to the model later can't be
  missed. IDs before the body stay, IDs after it move by the change in
  node count, and an ID inside the body fails the shift. Pointer
  sharing is kept, and values that hold no IDs (strings, lookups, the
  snapshot) are shared. It panics on an unexported field it would skip.
- **Diagnostics move.** The file's diagnostics go to offsets through the
  old text's line starts (columns are bytes), move by the text's change
  if they come after it, and go back through the new line starts. A
  diagnostic inside the change fails the shift.
- **The new text lowers cleanly.** An update lowers its changed files
  in full before it looks for body edits, and takes `bodyEdits` only if
  none has a parse or lowering error. So an edit that breaks a body, or
  adds a construct kc can't lower (even inside a lambda, where
  `hasLocals` doesn't look), is checked and keeps its diagnostic. Step
  36's kept declarations are keyed by snapshot, so the shifted file's
  new snapshot is lowered fresh.

An update whose body-edited files all shift checks nothing. Its model
is the last one with those files swapped in, its diagnostics are the
last ones with those files' replaced, and the cache entry keeps the new
texts and snapshots as a clean check would (`tm.Mark("shifted")`). If
some files shift and others don't, the others are checked as before,
and the shifted ones take their copies instead of the old models.

**Java sources.** Hilt's second round has generated Java files, and a
check with Java sources builds a library from them. A shifted update
changes no Kotlin declarations and no Java text, so it reuses the
library the last check kept (`prev.lib`). Without that, Hilt's second
round never shifted.

Tests (`analysis/ka/bodyreuse_test.go`) compare each update with a whole
check of the same sources (`sameModel`, with diagnostics). They cover
edits that shift (a statement, a literal, comments and blank lines, a
lambda, a deletion, a constructor's body, the last function, an edit on
one line, a top-level function) and edits that must be checked (a body
gaining, editing or losing a local, a local function, an accessor, an
object expression, two bodies, a contract, a syntax error, and an
unsupported construct in a lambda). One test inserts text into every body
in turn in one session, as a daemon's runs do. Another checks that
`remapNodes` reaches every node ID of a model. `TestShiftedModule`
gives each file of a `KA_BENCH` module a function and edits each file's
first body that may shift, one file at a time. Every file shifted and
equalled a whole check on Moshi's, Hilt's and Room's modules (100, 100
and 102 files) and on the Exhaustive module's 559 files of real code.
`TestIncrementalJava`
asserts that a Java module's body edit shifts and that the Java class
equals a whole check's.

`BenchmarkUpdateEdit` with a body edit (`KA_BENCH_RETURN`, a statement
added to a function in Model099): 6.6 ms, 9.7 MB and 54,600 allocations
before; 1.3 ms, 0.64 MB and 1,760 allocations after. The shift itself
takes about 0.4 ms. Signature edits are unchanged. (The module's
generated sources have no function bodies, so the benchmark's
comment edit, outside any body, still takes the partial check.)

The KSP bench, leaf edits over 30 runs, with output checked against
stock's (`CHECK`). Both rounds of body-edit runs shift.

| ms | stock KSP2 | kc | ratio | step 36 kc (30 runs) |
|---|---|---|---|---|
| Moshi, warm | 198 | 49 | 4.0x | |
| Moshi, body edit | 292 | 40 (min 29) | 7.3x | 54 |
| Moshi, signature edit | 266 | 64 | 4.2x | 77 |
| Hilt, warm | 407 | 82 | 5.0x | |
| Hilt, body edit | 407 | 67 | 6.1x | |
| Hilt, signature edit | 434 | 89 | 4.9x | |

Room's small module, `EDIT=auto`, 10 runs: warm 507 / 199, body edit
509 / 218, signature edit 522 / 199. Room's processor dominates, so the
shift doesn't show there.

What is left in a kc body-edit run is mostly outside the Go update: on
Moshi, kc-aa's setup takes about 6 ms beyond the update, processing
about 9, teardown about 6, and the time between rounds about 5.

The batch as in step 36 (10 runs per case, kc's fastest beside the
median):

| ms | stock KSP2 | kc | ratio | kc min | step 36 kc (min) |
|---|---|---|---|---|---|
| Room large, warm | 7251 | 2271 | 3.2x | 2011 | 2514 (2336) |
| Room large, edit Item000 | 8710 | 2530 | 3.4x | 2486 | 2796 (2605) |
| Room large, leaf edit | 9156 | 2986 | 3.1x | 2398 | 2998 (2625) |
| Room small, warm | 567 | 132 | 4.3x | 96 | 103 (80) |
| Room small, edit | 620 | 181 | 3.4x | 152 | 180 (157) |
| Hilt large, warm | 4148 | 745 | 5.6x | 593 | 692 (601) |
| Hilt large, edit Feature000 | 3900 | 1226 | 3.2x | 717 | 1161 (1077) |
| Hilt large, leaf edit | 575 | 120 | 4.8x | 66 | 102 (71) |
| Hilt small, warm | 353 | 61 | 5.8x | 54 | 59 (53) |
| Hilt small, edit | 451 | 60 | 7.5x | 56 | 53 (49) |
| Moshi large, warm | 2172 | 446 | 4.9x | 409 | 458 (383) |
| Moshi large, edit Model000 | 596 | 169 | 3.5x | 124 | 167 (148) |
| Moshi large, leaf edit | 354 | 54 | 6.6x | 46 | 75 (66) |
| Moshi small, warm | 292 | 25 | 11.7x | 22 | 23 (21) |
| Moshi small, edit | 200 | 34 | 5.9x | 28 | 32 (29) |

Cold runs: Moshi 7948–8142 / 4507–5141, Hilt 14359–18138 / 6527–7217,
Room 18366–20241 / 9502–10536.

Moshi's leaf edit moved most: a median of 54 ms against step 36's 75,
with body edits at 52 ms and signature edits at 86 (stock 458 and 376).
In 10 runs Hilt's leaf edits are noisier than in the 30-run pair above:
body edits 103 ms and signature edits 146 (stock 599 and 714). Hilt's
edits of Feature000 dirty all 100 files and don't take this path, and
Room's processors dominate its rows, so their differences are this
batch's noise. Moshi's one-file test is the first row past 10x, but
stock's warm run there was 292 ms against 204 in step 36's batch. Taken
against that, it is about 8x.

#### Step 38: a body-edit run reads only the edited file

After step 37, a Moshi leaf body edit took about 40 ms with kc (30 runs).
A JFR of 50 body-edit runs (`bench.editKind=body`, below), split by
Classify into main-thread samples at about 1 ms each:

| phase | samples | of which |
|---|---|---|
| setup and between rounds | 864 | libkc (Go) 370, IntelliJ core 201, KSP 193, kc-aa 100 |
| processing | 348 | the processors (KotlinPoet, Moshi) 228, KSP 46, kc-aa 32, IntelliJ core 28, libkc 14 |
| the harness | 177 | |

Go was the largest share: about 7.5 ms per run, in two updates of a
shifted file. Their timers said `read` 1.1–2.2 ms, `parse` 0.7–1.3 ms
and `shifted` 0.4–0.8 ms, and a profile of `BenchmarkUpdateEdit` agreed:
`os.ReadFile` of all 100 files was about a quarter of an update's CPU,
and the copy of the model about a seventh.

**The updates read only what changed.** kc-aa passes paths, and every
update read every file. A library now keeps the texts its updates read
(`shared.read`, `analysis/ka/readcache.go`), with each file's stamp:
size, modification and change times, device and inode. An update stats
each file and reads it only if the stamp differs. A write moves the
change time, which can't be set back, and a rename over the file moves
the inode. As in git's index, a file whose modification or change time
is within two seconds of when it was read isn't trusted, and is read
again until it is older: a coarse clock may not move for a second
write. Windows and other systems without those fields read every file.
The cache is on the library, not the session, because kc-aa opens a
session for every KSP run: a per-session cache missed all 100 files in
each run's first round.

**Work is handed out without a wake-up per item.** `parallel` gave out
indexes over an unbuffered channel. In a daemon, Go's threads are parked
between updates, and with items that take microseconds (a stat, a kept
parse) waking a thread per item cost more than the items: a read of 100
stats still took 1–2 ms in the JVM. Workers now take the next index from
an atomic counter, and the caller is one of them.

**The model copy skips fields without IDs.** `remapNodes` precomputes,
for each struct type of the model, the fields that hold node IDs, and
copies only those; the rest are shared with the struct's copy.

`BenchmarkUpdateEdit` with a body edit: 1.40 ms, 0.64 MB and 1,760
allocations before; 0.86 ms, 0.42 MB and 1,370 after. In the JVM, each
update's native call went from 2.5–4 ms to about 1 ms (`read` 0.2–0.3
ms, `parse` 0.2 ms, `shifted` 0.4 ms), and libkc's samples in the same
JFR from 384 to 132.

Tests: `TestReadCache` checks that a file is read again after a write
that keeps its size and modification time, after a rename over it, and
while it is within the window, and only then; `TestReadCacheSessions`
that a second session's update reads no unchanged file.

**kc warms up over more runs than the bench made.** Body-edit runs,
in groups of 20 or 30 from one JVM: runs 11–30 had a median of 36 ms,
31–50 of 29, 51–70 of 29, and 71–100 of 25 (minimum 21). Stock's JIT
keeps warming too (step 36). So the leaf edits below are 100 runs on
each backend, with the medians and minimums of runs 51–100, body edits
(odd runs) and signature edits (even runs) apart, and output checked
against stock's (`CHECK`):

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit | 254 | 32 | 7.9x | 25 |
| Moshi, leaf signature edit | 249 | 58 | 4.3x | 43 |
| Hilt, leaf body edit | 426 | 42 | 10.1x | 39 |
| Hilt, leaf signature edit | 445 | 68 | 6.5x | 58 |

Hilt's leaf body edit is the first edit past 10x. The fast and slow
runs differ in every phase, the processors' too (9 ms in a fast Moshi
run, 24–44 in a slow one), so the spread is the JVM's and the
machine's, not kc's.

The batch of steps 36 and 37 (medians of 10 runs, so the leaf edits
are still warming up), after step 38. Stock was 5–15% slower than in
step 37's batch on every module, so the machine was too; compare the
ratios:

| ms | stock KSP2 | kc | ratio | kc min | step 37 kc (min) |
|---|---|---|---|---|---|
| Room large, warm | 8179 | 2760 | 3.0x | 2417 | 2271 (2011) |
| Room large, edit Item000 | 9549 | 3053 | 3.1x | 2953 | 2530 (2486) |
| Room large, leaf edit | 9501 | 2996 | 3.2x | 2727 | 2986 (2398) |
| Room small, warm | 541 | 133 | 4.1x | 92 | 132 (96) |
| Room small, edit | 588 | 166 | 3.5x | 149 | 181 (152) |
| Hilt large, warm | 4364 | 778 | 5.6x | 664 | 745 (593) |
| Hilt large, edit Feature000 | 4889 | 1203 | 4.1x | 986 | 1226 (717) |
| Hilt large, leaf edit | 665 | 84 | 7.9x | 54 | 120 (66) |
| Hilt small, warm | 427 | 67 | 6.4x | 61 | 61 (54) |
| Hilt small, edit | 419 | 80 | 5.2x | 63 | 60 (56) |
| Moshi large, warm | 2406 | 536 | 4.5x | 512 | 446 (409) |
| Moshi large, edit Model000 | 748 | 175 | 4.3x | 135 | 169 (124) |
| Moshi large, leaf edit | 413 | 73 | 5.7x | 31 | 54 (46) |
| Moshi small, warm | 240 | 23 | 10.4x | 22 | 25 (22) |
| Moshi small, edit | 291 | 39 | 7.5x | 33 | 34 (28) |

Ten runs are too few for the leaf edits: Hilt's went from 4.8x to 7.9x,
while Moshi's median fell from 6.6x to 5.7x though its minimum went from
46 to 31 ms, which is the warm-up and spread above; the 100-run table is
the measure. The whole-module edits, which change a file every other
file depends on, and Room, whose processing dominates, stay within the
noise of step 37's.

What a fast Moshi body-edit run (about 30 ms) is made of now, from the
second JFR: the processors about 9 ms; KSP's incremental bookkeeping
about 5 ms (updating the lookup caches, backing up outputs, symbol
collection of the edited and generated files, `calcDirtyFiles`); kc-aa
building the rounds about 4 ms (psi and `build`); libkc about 2.5 ms;
the rest IntelliJ core and KSP's symbol layer. Signature edits are
still about twice as slow, because their updates resolve every file's
signatures again (4–6 ms per update).

#### Step 39: a signature edit checks only what looks up the declarations that changed

After step 38, a leaf signature edit on Moshi took about 58 ms with kc
against 32 for a body edit. `bench.editKind=signature` (new: every run
changes the appended function's return type) and the `checked=` count
now on each `kc-go: update` line showed where the extra time went: each
run's second round checked 5 files, not 1. Round 2 holds the 100
sources and the 4 adapters Moshi generated for Model099 in that run.
Only Model099 changed, but `declEdits` counted every name it declares as
changed, and the adapters look up `Model099User` and the rest, so they
were checked too, with their bodies' locals (`bodies` 3–4 ms, `model`
0.6 ms).

**Only the top-level declarations whose texts changed count.**
`declHashes` (`signature.go`) hashes each top-level declaration of a
parse as `signatureHash` hashes a file (no bodies, no trivia, depths
from the file), by name, and hashes the rest (package, imports, file
annotations, declarations without names) together. A declaration's hash
doesn't depend on where it is in the file. `changedDecls` compares the
old and new parses of a changed file. If the rest differs, every
declaration counts as before. Otherwise only the names whose hashes
differ, in order (an added, removed or reordered overload counts), give
`declaredNames` the declarations, their members and their classes
(`addModelOf`, `addSyntaxOf`).

**A declaration whose text didn't change may still change with one that
did** (`val x = f()`, a class using a type alias of the same file). The
checked files' models find it: `dependents` now compares, for every
checked file, each top-level declaration's model, grouped by name, with
the last check's (`changedModels`), and adds the names whose models
differ. It used to compare whole files, and only those whose texts were
the same, because a changed text moves psi node IDs, which models hold.
`sameNodes` compares models as `reflect.DeepEqual` does but takes any
two node IDs (and snapshots) as equal: no file's model holds another
file's node IDs, so other files can't see them move.

Tests: `TestIncrementalDecl` has new cases, with how many files the
update's last check checks (`lastChecked`): a function whose return type
changes the inferred type of a property and a class member of its file
(2 files), a type alias a class of its file uses (2), a declaration
added before the others, which moves every node after it (1), an added
function nobody uses (1), an unrelated function's type (2), and an
import (1, every declaration counted). Under the old rule the moved
declarations checked 3. `TestIncrementalModuleDecl` (a function added
to each file, and its first top-level callable and class renamed, each
compared with a whole check):
every edit equalled a whole check, and all but one were partial checks:
200 on Moshi's module, 200 on Hilt's, 204 on Room's (203 partial), and
1,170 on the Exhaustive module's 559 files. `TestShiftedModule` and
`TestIncrementalModule` pass on the same four.

The KSP bench, Moshi and Hilt, leaf edits, 100 runs on each backend, medians and
minimums of runs 51–100, output checked against stock's (`CHECK`).
Each signature-edit run's second round now checks 1 file:

| ms | stock KSP2 | kc | ratio | kc min | step 38 kc |
|---|---|---|---|---|---|
| Moshi, leaf body edit | 200 | 28 | 7.1x | 23 | 32 |
| Moshi, leaf signature edit | 205 | 51 | 4.0x | 36 | 58 |
| Moshi, signature edits only (`bench.editKind=signature`) | | 44 | | 36 | |
| Hilt, leaf body edit | 430 | 43 | 10.0x | 35 | 42 |
| Hilt, leaf signature edit | 436 | 66 | 6.6x | 54 | 68 |

Stock was faster on Moshi than in step 38's session (about 200 ms
against 254, in two runs of 100), so the Moshi ratios are lower though
kc's times are too.

A signature-edit run still costs about twice a body-edit run. Its two
updates each lower the 99 unchanged files from the declaration cache
(2–2.5 ms, a copy of each tree) and resolve every file's signatures
(3.5–7 ms in the JVM). The allocation profile of the second shows what
building the checker's symbols again costs: a fifth of the bytes are the
`componentN` and `copy` functions of data classes (`dataMembers`), and a
`FunSym` is 584 bytes. Some runs' `signatures` phase takes 12–30 ms.
Those runs line up with no Go GC (one every two runs, 12–28 ms of
concurrent marking); the phase runs on all four cores while the JVM's
compiler threads still warm up.

#### Step 40: a round indexes classes from IDs parsed once per file

A JFR of 130 Moshi body-edit runs on kc (`EDIT=leaf
bench.editKind=body`), by share of the main thread's samples:

| | share |
|---|---|
| KSP's `updateCaches` (lookup caches: path normalising, `CompressedAppendableFile` writes) | 6.4% |
| KSP's `calcDirtyFiles` | 5.2% |
| KSP's `registerGeneratedFiles` | 2.3% |
| kc-aa `KcRound.<init>` | 8.4% |
| of which `KcRound.index` | 4.7% |
| the processors (Moshi, KotlinPoet) | about 20% |

`KcRound.index` maps every class of every file to its source, so each
round called `ClassId.fromString` for every class of the 100–104 files,
though only one changed. `KcFile` now keeps its parsed class IDs (with
whether each is a type alias), and `KcModel.update` reuses unchanged
files' `KcFile`s across updates, so a round parses only the new files'.

KSP's three steps are the same work on both backends; they are in the
fork, not kc's to cut, and stay as they are.

The KSP bench, Moshi, leaf edits, 100 runs on each backend, medians and
minimums of runs 51–100, output checked against stock's (`CHECK`):

| ms | stock KSP2 | kc | ratio | kc min | step 39 kc |
|---|---|---|---|---|---|
| Moshi, leaf body edit | 202 | 24 | 8.4x | 17 | 28 |
| Moshi, leaf signature edit | 200 | 51 | 3.9x | 30 | 51 |

A body-edit run's phases, kc's medians: setup 5, process 8, between 3,
teardown 5.

#### Step 41: KSP stats each output once, and libkc's library classes are kept

A JFR of 130 Moshi body-edit runs on kc after step 40, counting native
samples too (`Classify` does), showed where the run's own time still
went outside the processors. kc's share was small: each of the two
updates about 1 ms in libkc, and psi and the round's build about 1 ms
each per round. The largest single cost was KSP's:

- **`updateOutputs`** backs up the run's outputs and restores the clean
  ones on every run: about 1,600 `copy` checks on Moshi's 800 outputs,
  each with five `stat` calls (`isFile`, `length` and `lastModified` of
  the copy, `length` and `lastModified` of the original). It had 171
  native samples, more than libkc's two updates.
- **`relativeTo` and `toRelativeString`**, for every source and output
  path, split both paths into components and normalize them
  (`calcDirtyFiles`' `getRelativeFile`, `updateOutputs`' `bak`).
- **libkc's library classes**: each round asked libkc again for every
  class-path class it built (29 a run, 0.6 ms of JNI, encoding and
  decoding), though Go keeps them modeled.

Fork patch 0014 reads each file's attributes once
(`Files.readAttributes`; size and time to the millisecond, as before),
and cuts a path that is the base, a separator and a plain relative path
directly; any other path takes the old way. Like patches 0010–0013,
this is KSP-side and runs on kc's side of the comparison only: stock is
KSP 2.3.6 as released.

kc-aa's environment, whose key holds the class path's files and
stamps, now keeps the library classes libkc modeled, by outermost class
ID, and starts over for another module name or other friends. A Java
source's class, and a class-path class a Java source declares too, is
always asked for. Warm runs make no class calls.

Not kept: `KcEnvironment.psiFile` reads every file to compare it with
the bytes its PSI was parsed from (45 native samples). Trusting libkc's
reuse of a file's model instead made no difference to the `psi` timer
(0.6 ms against 0.7), so the change was dropped.

The KSP bench, leaf edits, 100 runs on each backend, medians and
minimums of runs 51–100, output checked against stock's (`CHECK`).
Moshi's stock times are step 40's, from the same session:

| ms | stock KSP2 | kc | ratio | kc min | step 40 kc |
|---|---|---|---|---|---|
| Moshi, leaf body edit | 202 | 22 | 9.2x | 16 | 24 |
| Moshi, leaf signature edit | 200 | 47 | 4.3x | 34 | 51 |
| Hilt, leaf body edit | 476 | 37 | 12.9x | 29 | 43 (step 39) |
| Hilt, leaf signature edit | 487 | 64 | 7.6x | 48 | 66 (step 39) |

Stock's Hilt runs were slower than in step 39's session (476 against
430), as Moshi's were faster in step 39's. Within one session, batches
of kc's Moshi body edits gave medians of 21 and 22 ms.

A Moshi body-edit run is now setup 5, process 8 (the processors about
6), between 3, teardown 4. What is left outside the processors is
KSP's own bookkeeping (lookup caches flushed every run, dirtiness, the
generated files' symbols; about 3 ms), libkc's two updates (about 2 ms,
most of it reading and stat-ing the files), and kc-aa's psi and builds
(about 2 ms).

#### Step 42: KSP's output checks run in parallel, and listing a class's declarations builds no member scope

Step 41's JFR (90 recorded Moshi body-edit runs, main-thread samples,
execution and native, about 1 ms each) left these as the run's largest
items outside the processors: KSP's `updateOutputs` (126 samples),
`calcDirtyFiles` (61), `updateLookupCache` (58) and
`registerGeneratedFiles` (40), libkc's two updates (166 native) and
`KcRound.<init>` (60).

**`updateOutputs`** still compared every output with its backup (step
41: two `stat` calls each), about 1,600 pairs a run. Fork patch 0015
runs the comparisons on a parallel stream, from 64 pairs on; the copies
that are needed (about 4 in a warm run) are made in order, one at a
time, as `copyWithTimestamp` creates missing parent directories with a
non-overwriting copy. KSP-side, and on kc's side of the comparison only,
as patches 0010–0014.

**`registerGeneratedFiles` and `calcDirtyFiles`** collect the names a
file declares (`SymbolCollector`): KSP lists each class's declarations,
which asks kc for the class's declared member scope. `KcRound.declared`
built the class's whole member scope (inherited members, override
matching: `KcMemberBuilder.build`) and returned its own members unless
it was a Java class. So every run built the member scopes of the four
adapters Moshi had generated. It now builds them only for a Java class,
whose declarations are FIR's view of its members
(`hasJavaDeclarations`, shared with the builder).

Under KSP's `execute`, the main thread's samples per 90 runs:

| | step 41 | step 42 |
|---|---|---|
| `SymbolCollector` | 55 | 20 |
| `KcMemberBuilder.build` | 29 | 4 |
| `updateOutputs` (execution + native) | 126 | 68 |
| all of `execute` (execution + native) | 1,090 | 964 |

Dropped: sending round 2's update only the new paths. Go stats the
files in parallel already, and `read` is 0.2 ms an update.

The KSP bench, Moshi, leaf edits, 100 runs on each backend, in two
batches one after the other, medians and minimums of runs 51–100, output
checked against stock's (`CHECK`):

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit, batch 1 | 238 | 21 | 11.3x | 16 |
| Moshi, leaf body edit, batch 2 | 188 | 20 | 9.4x | 17 |
| Moshi, leaf signature edit, batch 1 | 236 | 41 | 5.8x | 26 |
| Moshi, leaf signature edit, batch 2 | 188 | 39 | 4.8x | 32 |
| Hilt, leaf body edit | 476 (step 41) | 38 | 12.5x | 28 |
| Hilt, leaf signature edit | 487 (step 41) | 66 | 7.4x | 55 |

Stock's medians moved by 50 ms between the two batches, kc's by 1 ms.
Over both, Moshi's body edit is about 10.4x (213 against 20.5).
Batches of kc on the same code gave trimmed means of 20.5 to 22.5 ms
before these cuts, so the totals alone can't show cuts this size; the
samples above do.

#### Step 43: a partial check resolves only the signatures it can reach

After step 42, a signature edit still cost about twice a body edit
(Moshi 40 ms against 20.5, Hilt 66 against 38). Each of a signature
run's two Go updates checks one file (step 39), but `check.CheckOptions`
ran `resolveSignatures` over every file of the module: all of their
functions, properties and members, with their types. That was the
largest item of a signature-edit update's CPU profile, 2.27 s of the
`BenchmarkUpdateEdit` profile against 0.8 s for lowering.

**ka passes the files the checked ones can reach**
(`check.Options.Resolve`, from `reachable` in `ka.go`). A top-level
declaration is reached if a reached text writes a name it declares (its
own, or the name of a class it nests). Its own text is then reached too,
but not the rest of its file. The starting points are:
- the checked files' whole texts;
- the top-level operators and the functions a construct calls by
  convention (`check.IsOperatorName`: `plus`, `getValue`, `componentN`...),
  which code calls without writing their names;
- the declarations whose names the Java sources write, whose members
  may return a Kotlin class that no Kotlin text names.

So an inferred type of another file (`val x = f()`), a supertype, a
typealias, an extension, and the members of every class a reached text
names are all reached through the names that lead to them.

The checker resolves the signatures only of the files that hold a
reached declaration. In `CheckOptions`, `resolveSignatures`,
`forwardInlinable`, `checkMemberImports`, `inheritDefaults`,
`delegations`, the `inferNamed` loop and `conflicts` run over those
files. Every file is still declared, with its imports, typealiases and
supertypes, so every class has its symbol and supertypes, and names and
sealed hierarchies resolve as before. If more than half the files are
reached, every file is resolved. `KC_NO_REACH` turns it off.

Each file's identifiers, whole and per top-level declaration, are kept
with its parse (`checkCache.names`), so an update finds only the changed
file's again.

Following whole files reached too much on Hilt. Each `FeatureN.kt`
names `Feature(N-1)Clock` in a constructor, so whole files chained to
62.8 of 100 files per edit. Per declaration, it reaches 2.0, because
`Feature098Clock`'s own text names no other file. A declaration of a
reached file that isn't reached itself still has its signature
resolved, with the classes it names (every file declares them), but no
member of theirs.

Tests:
- `TestIncrementalDecl` has three new cases, with how many files the
  check resolves (`lastResolved`):
  - a member reached only through another file's inferred return type;
  - an operator extension no file names;
  - a file with one reached and one unreached declaration.
- `TestIncrementalJavaDecl` has one new case: a Kotlin class only a
  Java source names.
- Without the operator seed and the Java seed, those models differ from
  a whole check.
- `TestIncrementalModuleDecl` now logs the files resolved per edit.
  Every edit still equals a whole check: 2.4 of 100 files on Moshi,
  2.0 of 100 on Hilt, and 52.5 of 102 on Room (renaming an entity
  checks `AppDb.kt`, which names all 100, so those edits resolve every
  file). On the Exhaustive module it was 478.6 of 559, mostly the
  fallback.
- `TestShiftedModule` and `TestIncrementalModule` pass on all four
  modules.
- `build.sh stock` and `build.sh ksp` (134/136) pass.

`BenchmarkUpdateEdit` (Moshi, `KA_BENCH_SIGNATURE`: a function added
to Model099 each time) took 5.5–6.6 ms per update, against 9.4–9.8
with `KC_NO_REACH`. `resolveSignatures` fell from 2.27 s to 0.29 s of
the profile. Most of what is left is lowering the 99 unchanged files
as declarations from the declaration cache (a copy of each tree), and
GC.

This session ran on a slower machine than step 42's: kc's body edits,
whose path didn't change, took 26–28 ms on Moshi against 20.5, and
56 ms on Hilt against 38. So the step was measured A/B on the same
machine: kc only, signature edits only (`EDIT_KIND=signature`), four
batches each with the cut on and off, alternating. Medians and minimums
of runs 51–100:

| ms | on | off (`KC_NO_REACH`) |
|---|---|---|
| Moshi, leaf signature edit, batch 1 | 42 (min 33) | 48 (min 36) |
| Moshi, leaf signature edit, batch 2 | 41 (min 31) | 54 (min 37) |
| Hilt, leaf signature edit, batch 1 | 73 (min 52) | 82 (min 62) |
| Hilt, leaf signature edit, batch 2 | 70 (min 54) | 79 (min 60) |

That is about 8 ms off a Moshi signature run and 9 off a Hilt one.
Setup and between, the two updates, each fall by 3–5 ms. With
`KC_TIMING`, a warm update's `signatures` phase is now 2–3 ms, against
3.5–7 in step 39, and `lower` is 2–3 ms.

The KSP bench on the same machine, both backends, leaf edits
alternating body and signature, 100 runs, output checked against
stock's (`CHECK`):

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit | 187 | 26 | 7.2x | 21 |
| Moshi, leaf signature edit | 186 | 40 | 4.7x | 31 |
| Hilt, leaf body edit | 583 | 56 | 10.4x | 43 |
| Hilt, leaf signature edit | 572 | 86 | 6.7x | 69 |

A second Moshi batch gave kc 28 and 44 ms. Stock's medians in that
batch were 362 and 380 ms, with teardowns of 178–194 ms against 28 in
the first, so it isn't used for ratios. These ratios are lower than
step 42's, though kc's signature path is faster, because this machine
slowed kc's unchanged body path by about a quarter, and stock's Moshi
runs by less.

#### Step 44: files a partial check can't reach become stubs

After step 43, a partial check resolved signatures only in the files it
could reach, but each update still lowered every unchanged file and
declared them all. The `BenchmarkUpdateEdit` profile (Moshi,
`KA_BENCH_SIGNATURE`, 5.9 ms per update) showed:
- `syntax.Clone` copying the 99 unchanged files' declarations out of
  the declaration cache: 2.24 s of `Update`'s 4.59. The checker mutates
  trees, so each update needs its own copy.
- GC background marking, largely for those copies: 3.26 s.
- `declare`, `resolveSupertypes` and `resolveImports` over every file:
  about 1.1 s.

**A file a partial check can't reach is given a stub, not a copy**
(`declStub` in `declcache.go`). The stub is a `syntax.File` with the
cached tree's path, package and facade, and no declarations. The checker
then declares only its package, so `import a.b.*` and qualified names
still resolve. `Update` now computes the reach (`reachable`, now
index-based) before lowering, for body edits and declaration edits
alike. The dependents loop recomputes it as `dirty` grows. A whole
check, or a reach over half the files, stubs nothing.

The name closure now covers one lookup that step 43's didn't need,
because every file used to be declared:
- **Sealed hierarchies.** A `when`'s exhaustiveness counts a sealed
  class's subclasses (`ClassSym.Subclasses`). The checker builds that
  list from the subclasses' declared supertypes, and nothing need write
  a subclass's name. So a reached name of a source `sealed` class or
  interface also reaches every declaration whose supertype list writes
  that name. `snapNames` records each declaration's sealed names, from
  the psi's modifier lists, and its supertype identifiers.

These are never stubbed, though nothing else names them:
- files with file annotations (`@file:JvmName`, `@file:JvmMultifileClass`);
- multifile parts;
- files with `expect` declarations;
- files whose parse has errors (those aren't cached anyway).

The checker reports no clash between two files' facades, so for facades
the guard is only a precaution. Java names were already handled: the
Java texts' identifiers seed the closure (step 43), and `SetKotlin`
gets the stubs, which hold no classes.

`check.Options.Resolve` (step 43) is gone. Every file that isn't a stub
holds a reached declaration, so the checker resolves all it is given.

Tests:
- **`TestIncrementalDecl` has three new cases:**
  - **A sealed class's subclass in a file nothing names.** The whole
    check reports the `when` as not exhaustive. Without the sealed rule,
    the partial check's model differs.
  - **An import of a package only an unreached file declares.** A stub
    without its package gives "unresolved import".
  - **Two files with one `@file:JvmName`.**
- **Module tests:**
  - `TestIncrementalModuleDecl`, `TestIncrementalModule` and
    `TestShiftedModule` pass on Moshi, Hilt, Room and Exhaustive.
  - The files reached per edit are step 43's: 2.4, 2.0, 52.5 and 478.6.
- **JVM checks:** `build.sh stock` and `build.sh ksp` (134/136) pass.

`BenchmarkUpdateEdit` took 3.2–3.9 ms per update, against 5.5–6.6 in
step 43 and 9.4–10.3 with `KC_NO_REACH`, which now turns off both steps.
In the profile:
- `syntax.Clone` fell from 2.24 s to 0.02 s.
- `declare` is gone.
- GC marking fell from 3.26 s to 0.81 s.

The largest item left is `declEdits`' search for the files that look up
a changed declaration: `addLookingUp` → `declaredNames.declaredBy`,
0.50 s of `Update`'s 2.81.

With `KC_TIMING`, a warm partial update's `signatures` phase is now
0.8–0.9 ms, against 5.5–7.8 with `KC_NO_REACH`. Its `lower` phase is
1.5–2.0 ms, against 2.2–2.8. The `lower` timer also covers `declEdits`
and `bodyEdits`, so what is left of it is mostly that search.

The bench ran on the same machine as step 43's. Kc only, signature edits
only, two batches each with the cut on and off, alternating. Medians and
minimums of runs 51–100:

| ms | on | off (`KC_NO_REACH`) |
|---|---|---|
| Moshi, leaf signature edit, batch 1 | 34 (min 29) | 49 (min 36) |
| Moshi, leaf signature edit, batch 2 | 41 (min 31) | 47 (min 38) |
| Hilt, leaf signature edit, batch 1 | 61 (min 48) | 80 (min 61) |
| Hilt, leaf signature edit, batch 2 | 58 (min 46) | 79 (min 61) |

Steps 43 and 44 together take about 10 ms off a Moshi signature run
(step 43 alone: 8) and about 20 off a Hilt one (step 43 alone: 9).
Setup and between, the two updates, each fall by 4–6 ms on Moshi and
by 7–12 ms on Hilt.

The KSP bench on the same machine, both backends, leaf edits
alternating body and signature, 100 runs, output checked against
stock's (`CHECK`):

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit | 216 | 25 | 8.6x | 21 |
| Moshi, leaf signature edit | 217 | 35 | 6.2x | 28 |
| Hilt, leaf body edit | 505 | 43 | 11.7x | 36 |
| Hilt, leaf signature edit | 495 | 54 | 9.2x | 47 |

The bench's body edit declares no locals, so it runs no check and this
step doesn't touch it. Yet Hilt's body edit took 43 ms here, against 56
in step 43's batch, and stock's Hilt edits ran in about 500 ms, against
about 575. The machine's speed varies that much between batches, so the
same-batch A/B above is this step's measure. On it, step 44 alone is
about 2 ms of Moshi's signature run and 11 of Hilt's.

#### Step 45: changed names are tested against each file's hashed keys

After step 44, a signature edit's update took 3.2–3.9 ms in
`BenchmarkUpdateEdit` (Moshi) and 5.0–6.0 ms in the JVM's `native`
timer. A third of that went on finding the files a changed declaration
reaches: `declEdits` (`addLookingUp`) and then `dependents` scanned every
unchanged file's lookups and declarations (`declaredNames.hit`,
`declaredBy`). Each step hashed a `[2]string` key, and `declaredBy`
called `strings.ReplaceAll` per class. That was 0.9 s of `Update`'s
2.81 s, while the names searched for were the few the edit changed.
`KC_TIMING` didn't show it: `dependents` and `keepCheck` ran after the
timer's last mark.

**Each model now has its keys** (`fileKeys` in `keys.go`): the sorted
64-bit hashes (`hash/maphash`, one seed per process) of
- its lookups' scopes and names;
- its lookups' scopes alone;
- its declarations' scopes and names, walked as `declaredBy` walks them.

`checkCache.keysOf` computes them when a check is kept and keeps them
with the models (`checked.keys`). An unchanged file's model is the same
`*File` from one check to the next, so an update computes only the
checked files' keys. `keep` drops the keys of models no kept check
holds. `declaredNames.seal` hashes the changed names and classes, and
`intersects` searches for them in each file's keys. It binary-searches
when the sets are far apart in size and merges them otherwise. A hash
that collides can only add a file to those checked.

The tests turn on `verifyKeys`: each file the keys pass over is checked
against `hit` and `declaredBy` too, and a miss panics. `TestFileKeys`
compares the two for every single name and class a small module looks
up or declares, and `TestIntersects` covers both of `intersects`'
paths. `KC_TIMING` now marks `dependents` and `keep`.

This step's plan also had `reachable` keep its name index across updates.
After this step `reachable` was 0.10 s of `Update`'s 1.70 s, about
0.1 ms per update, so it wasn't done: an index kept across updates holds
the parses it indexed until it is rebuilt.

Tests:
- `TestIncrementalModuleDecl`, `TestIncrementalModule` and
  `TestShiftedModule` pass on Moshi, Hilt, Room and Exhaustive with
  `verifyKeys` on, with the files resolved per edit unchanged (2.4,
  2.0, 52.5, 478.6).
- With `newFileKeys` leaving out declarations, `TestIncrementalDecl`
  panics on the first file the keys miss.
- `build.sh stock` and `build.sh ksp` (134/136) pass.

`BenchmarkUpdateEdit` (Moshi, `KA_BENCH_SIGNATURE`) took 2.1–2.9 ms per
update, against 3.2–3.9 in step 44. In the profile, the two scans fell
from 0.9 s to 0.04 s, and `Update` from 2.81 s to 1.70 s. What is left
is spread out:
- `checkFiles`, 0.37 s;
- reading the texts, 0.21 s;
- the edited file's model and lowering, about 0.5 s;
- `changedModels`, 0.13 s.

There is no switch for this step, so it was measured against step 44's
libkc, built from 6ab8413, swapped in between batches on the same
machine. Kc only, signature edits only, two batches each, alternating.
Medians and minimums of runs 51–100, and of `native` per update:

| ms | step 45 | step 44 | `native`, 45 against 44 |
|---|---|---|---|
| Moshi, leaf signature edit, batch 1 | 31 (min 26) | 35 (min 29) | 3.2 against 5.2 |
| Moshi, leaf signature edit, batch 2 | 30 (min 23) | 37 (min 29) | 3.0 against 5.3 |
| Hilt, leaf signature edit, batch 1 | 56 (min 47) | 62 (min 45) | 4.0 against 6.0 |
| Hilt, leaf signature edit, batch 2 | 52 (min 40) | 57 (min 45) | 3.8 against 5.6 |

Each update's `native` falls by about 2 ms, and a run by 5–6 ms. With
`KC_TIMING`, the phases now add up to `native`. On Moshi they are:

| phase | ms |
|---|---|
| read | 0.3 |
| parse | 0.3 |
| lower (with `declEdits`) | 0.8, against 1.5 |
| signatures | 1.0 |
| bodies | 0.3 |
| model | 0.1 |
| dependents | 0.1 |
| keep | 0.1 |

The KSP bench, both backends, leaf edits alternating body and signature,
100 runs, output checked against stock's (`CHECK`):

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit | 232 | 26 | 8.9x | 22 |
| Moshi, leaf signature edit | 225 | 33 | 6.8x | 26 |
| Hilt, leaf body edit | 504 | 41 | 12.3x | 35 |
| Hilt, leaf signature edit | 481 | 48 | 10.0x | 43 |

Hilt's signature edit is now at 10x. Moshi's first batch gave kc 27 and
32 ms, but stock's teardowns were 98–101 ms there, against 45–46 in the
second, so its ratios aren't used. A Moshi signature run is now 7 ms
over a body run. Its two updates take about 3 ms each, and a body run's
take about 1.

#### Step 46: the JVM keeps unchanged files' PSI by libkc's text hash

After step 45, a Moshi body-edit run took 26–27 ms with kc. A JFR of it
showed two things to cut, one in the bench and one in kc-aa.

**The bench's layout.** KspBench passed the first source directory as
`projectBaseDir`, and the outputs were under a sibling directory. Every
output path relative to the base was then `../out/...`. That missed
fork patch 0014's fast path, so `updateOutputs`' backup paths went
through `toRelativeString`'s slow path: 304 of the 3962 Java samples
under `execute`. Gradle passes the project directory, which holds both
the sources and `build/generated/ksp`. KspBench now passes the
directory that holds both, for both backends. That brought kc's body run
to 25 ms, and stock's Moshi teardown from about 45 ms to 30.

**kc's rounds.** With `KC_TIMING`, a body run's kc time was about
6.7 ms:

| phase | ms |
|---|---|
| session (`environment` 0.4, `open` 0.7) | 1.1 |
| round 1: update, psi, build | 1.3, 1.1, 1.0 |
| round 2: update, psi, build | 1.3, 0.9, 0.5 |

`psi` found each of about 100 files' virtual file
(`refreshAndFindFileByPath`, a stat each), and `KcEnvironment.psiFile`
read each file's bytes to compare them with the bytes its PSI was
parsed from. That ran every round, though libkc had just found the
same texts by their stamps. The session walked the source roots twice,
once for `.kt` files and once for `.java` ones, with a stat per entry.
And the second round's update parsed the edited file again: it reuses
the last run's check of the same round (`checkCache.find`), whose text
for that file is the old one.

Four changes:
- **libkc's file records carry a text hash** (`kaops`'s `textHash`, the
  64-bit FNV-1a of the text as read, which is the file's bytes).
  `KcEnvironment` keeps, by path, the virtual file and the hash of the
  bytes `psiFile` last read for it. A round whose `KcFile.text` matches
  gets the cached PSI, if its view provider is still the file's, with no
  stat and no read. Otherwise `psiFile` runs as before and records the
  hash of what it read. A file read between libkc's read and the JVM's
  gets a hash that differs from libkc's, so the next round reads it
  again. `TestTextHash` checks `textHash` against FNV-1a's published
  values; the JVM's `textHash` is the same loop.
- **Parses are shared across kept checks.** `checkCache.snapshot`
  looks in every check the cache keeps for a parse of the same path and
  text. The second round takes the edited file's parse from the first
  round's check.
- **Source listings are cached** (`SourceWalk` in `KcBackend.kt`). Each
  directory's listing is kept with its modification time, which adding,
  removing or renaming an entry changes. A walk then stats only the
  directories. As in libkc's read cache, a directory modified within 2 s
  of being listed is listed again until it is older.
- `build`'s error for a missing file now says "no Kotlin PSI", which
  covers both causes.

After the change:

| phase | ms, step 46 | step 45 |
|---|---|---|
| session | 0.4 | 1.1 |
| round 1 psi | 0.3 | 1.1 |
| round 2 psi | 0.1 | 0.9 |
| round 2's Go parse | 0.1 | 0.3 |

That is about 1.7 ms off kc's 6.7 per body run. What is left of kc's
rounds is the two updates (about 1.3 ms each in the JVM, 1.1–1.2 of
it `native`), IntelliJ parsing the edited file (round 1's `build`), and
the second round's shift of the edited file's model (0.5 ms).

Tests:
- `TestIncrementalModuleDecl`, `TestIncrementalModule` and
  `TestShiftedModule` pass on Moshi, Hilt, Room and Exhaustive, with
  the files resolved per edit unchanged (2.4, 2.0, 52.5, 478.6).
- `build.sh stock` and `build.sh ksp` (134/136) pass, and the bench's
  `CHECK` finds kc's output the same as stock's on Moshi and Hilt.
- `internal/classfile`'s and `internal/driver`'s tests fail in this
  sandbox only. The proxy's `JAVA_TOOL_OPTIONS` banner gets into the
  JVM's output, and with no UTF-8 locale a non-ASCII class name breaks.
  With the variable unset and `LANG=C.UTF-8`, they pass.

Moshi body edits only, kc only, six batches: 23, 22, 23, 23, 22 and 21
ms (medians of runs 51–100), against 25 with the new layout and step
45's code, and 26–27 with the old layout.

The KSP bench, both backends, leaf edits alternating body and signature,
100 runs, output checked against stock's (`CHECK`):

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit | 200 | 22 | 9.1x | 17 |
| Moshi, leaf signature edit | 207 | 28 | 7.4x | 21 |
| Hilt, leaf body edit | 484 | 36 | 13.4x | 31 |
| Hilt, leaf signature edit | 476 | 43 | 11.1x | 37 |

Both Hilt edits are past 10x. A second Moshi batch gave kc 21 and 26 ms,
but stock's teardowns there were 190–205 ms, against 29–30 in the
first, so its ratios aren't used. On Moshi, stock is faster than in
step 45's rows (232 and 225 ms) because of the bench's layout: 10x is
now 20 ms. The rest of a kc body run is the processors (about 9 ms,
half of it KotlinPoet writing the adapter) and KSP's bookkeeping (about
5): reads and writes of its on-disk lookup and symbol maps, spread over
`calcDirtyFiles`, `DirtinessPropagator` and `updateCaches`.

#### Step 47: KSP checks clean outputs against their backups while the processors run

After step 46, a Moshi body run took 22 ms with kc: setup 4, processing
9, kc's second round 2 and teardown 4. A JFR of runs 51–100 put about
1.7 ms per run of the teardown in `updateOutputs`:

- 33 main-thread samples (about 0.7 ms per run) in `upToDate`,
  `attributes` and the backup paths;
- `ThreadPark` events in 22 of the 50 runs, 51.6 ms in all, while the main
  thread waited for fork patch 0015's parallel check.

`updateOutputs` checks every clean output against its backup (size and
millisecond time), which is about 800 outputs and 1600 stats on Moshi,
on every run. In a daemon almost all of them are up to date.

The clean outputs are known long before the end of the run. They are
outputs in `sourceToOutputsMap`. That map is in memory once
`calcDirtyFiles` returns, and only `updateCaches` changes it, at the end
of the run and only for dirty sources.

Fork patch 0016 starts the same check of every output in the map on a
common-pool thread when `calcDirtyFiles` returns (`startOutputChecks`):

- The main thread only snapshots the map's lists, which are replaced,
  never changed.
- `updateOutputs` then takes the set of outputs found up to date. It
  checks again only the pairs that are missing from the set and the
  outputs this run wrote. A processor may have changed those, and the
  output writer writes no others.
- The dirty outputs' backups are checked as before.
- Backups change only in `updateOutputs`.
- The one difference: an outside change to a clean output or to its
  backup made while the processors run is seen by the next run, as a
  change made just after this run would be. Gradle cleans the outputs
  before KSP starts.
- `KSP_NO_PRECHECK=1` turns the check off, for A/B batches.

A first version checked in parallel, as patch 0015 does at the end. On
Moshi it cut the teardown by as much, but processing got 0.7–1.5 ms
slower in two of four batches. The processors and the output writers
use the other cores. One thread finishes well within the about 11 ms it
has, and in three alternating batches it did as well as the parallel
version or better (body 20 against 20 ms, signature 25 against 25,
means 21.3 against 21.8 and 26.2 against 27.2).

`build.sh`'s incremental check now also runs with the outputs deleted
before the second run, as Gradle does
(`incremental, outputs deleted: ok`). Until now the restore of clean
outputs from their backups had no test, because the outputs were always
still there.

With the change, the JFR has 4 main-thread samples in `updateOutputs`
and no parks.

A/B on one machine, kc only, leaf edits alternating body and signature,
medians of runs 51–100 (means in brackets):

| ms | on | off (`KSP_NO_PRECHECK=1`) |
|---|---|---|
| Moshi body, 3 batches | 21 (22.2), teardown 2 (2.4) | 22 (22.7), teardown 4 (3.9) |
| Moshi signature, 3 batches | 25 (27.0) | 27 (27.8) |
| Hilt body, 2 batches | 34.5 (36.1), teardown 2 (1.6) | 38 (40.1), teardown 5 (4.8) |
| Hilt signature, 2 batches | 41 (44.5) | 44 (45.9) |

The teardown drops by 1.5–3 ms on both projects. On Moshi the totals
gain less, 0.5–2 ms, because processing's mean rises by up to 0.8 ms
while the check runs beside it.

The KSP bench, both backends, 100 runs, output checked against stock's
(`CHECK`):

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit | 272 | 20 | 13.6x | 16 |
| Moshi, leaf signature edit | 274 | 24 | 11.4x | 20 |
| Hilt, leaf body edit | 461 | 37 | 12.5x | 31 |
| Hilt, leaf signature edit | 460 | 44 | 10.5x | 36 |

Stock's Moshi teardown was 116 ms in this batch, against 29–30 in step
46's first batch. Step 46's second batch had already shown that it
varies from batch to batch. Against step 46's faster stock (200 and 207
ms), kc's Moshi rows are 10.0x and 8.6x.

What is left of KSP's bookkeeping on Moshi, by the same JFR:
- `updateLookupCache`, about 1.3 ms per run;
- `cachesUpToDateFile.createNewFile`, 37 samples in this JFR and 3 in
  step 46's, so it varies;
- `DirtinessPropagator`, about 0.7 ms;
- `CodeGeneratorImpl.drain`'s wait for the output writers, about
  0.4 ms.

#### Step 48: KSP skips lookup-cache updates that change nothing

After step 47, a Moshi body run's largest piece of KSP bookkeeping was
`updateLookupCache`. It took 65 of 937 main-thread samples, about 1.4 ms
per run:
- `LookupStorageWrapperImpl.flush`, 42 samples: `PersistentHashMap.force`
  writing appended records;
- `LookupStorage.addAll`, 17;
- `release`, 5.

Kotlin's `LookupStorage.update(tracker, F, removed)` drops the dirty
files' ids (`removeLookupsFrom`), then gives them new ids and appends
their records (`addAll`). Records for files outside `F` that the
tracker names are appended under their existing ids. So every daemon
run rewrites and flushes the dirty files' lookups, even when they are
what they were, and the storage grows by that much each run.

Fork patch 0017 keeps, with each lookup storage that `LookupStorages`
holds open between runs, what this process knows the storage holds,
per file. A file the last update or removal was of has its lookups
exactly. A file an update only appended to has at least those lookups.
On Hilt those are the library class files that the processing of a
dirty file looked at.

An update leaves:
- exactly the tracker's records for each file in `F`;
- for any other file, what it held plus the tracker's records.

That is the storage as it was when it already holds exactly the
tracker's records for each file in `F`, and at least them for each
other file the tracker names. `update` then returns without touching
the storage, and `flush` writes only after a change.

The bookkeeping:
- A storage opened again knows nothing.
- `removeLookupsFrom` records its files as empty.
- An update with removed files clears what is known.
- `KSP_NO_LOOKUP_SKIP=1` turns the skip off.
- `KSP_LOOKUP_VERIFY=1` makes each update that could be skipped run
  anyway, and fails if `get` then differs for any symbol the tracker
  holds.

A first version required every file the tracker names to be in `F`.
That never held on Hilt, whose processors record lookups against
classes in jars (`hilt-core`, `dagger`, the JDK). Treating those files
as "at least" fixed it.

A temporary log showed when the skip fires:
- on every Moshi run after the first, body and signature, since the
  edited file's lookups don't depend on the function's type;
- on every Hilt body run;
- on no Hilt signature run, since the edited file's lookups differ by
  one between `Int` and `Any`.

`build.sh` runs each KSP run in a fresh JVM, so the skip never fires
there. The tests are bench batches:
- `KSP_LOOKUP_VERIFY=1` on Moshi and Hilt, leaf edits (100 runs) and
  `EDIT=auto` (Model000, 10 files dirty, 100 runs; Feature000, 100
  files dirty, 40 runs): all runs OK, no difference;
- `EDIT=auto` with the skip on and off: the same `dirty=` on every run.

In the new JFR, `updateLookupCache` takes 29 samples, with no `flush` and
no `addAll`. What is left is normalizing the tracker's paths, the
storage directory's stamps and the set comparison.

A/B on one machine, kc only, leaf edits alternating body and signature,
medians of runs 51–100 (means in brackets):

| ms | on | off (`KSP_NO_LOOKUP_SKIP=1`) |
|---|---|---|
| Moshi body, 3 batches | 18 (20.1), teardown 1 (1.4) | 21 (22.2), teardown 3 (2.9) |
| Moshi signature, 3 batches | 24 (24.8) | 25 (26.6) |
| Hilt body, 2 batches | 34 (35.4), teardown 2 (1.8) | 35 (37.0), teardown 2 (2.4) |
| Hilt signature, 2 batches | 41 (44.0) | 42.5 (43.7) |

The KSP bench, both backends, 100 runs, output checked against stock's
(`CHECK`):

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit | 218 | 18 | 12.1x | 14 |
| Moshi, leaf signature edit | 219 | 22 | 10.0x | 17 |
| Hilt, leaf body edit | 366 | 34 | 10.8x | 28 |
| Hilt, leaf signature edit | 376 | 40 | 9.4x | 33 |

Stock varies from batch to batch again. Its Hilt teardown was 120 ms
here against 185–206 in step 47's batch, and its Moshi teardown 54
against 116. Against step 46's Moshi stock (200 and 207 ms), kc's
Moshi rows are 11.1x and 9.4x.

What is left of KSP's bookkeeping on a Moshi body run:
- `calcDirtyFiles`, 55 samples, about 1.1 ms (`DirtinessPropagator`
  15);
- the rest of `updateCachesAndOutputs`, including the
  `cachesUpToDate` marker's `createNewFile` (19).

#### Step 49: libkc keeps reachable's index between updates

After step 48, Hilt's signature edit was the one row under 10x (9.4x).
Its runs make two incremental updates in libkc, one per round, and each
checks one file. In the round timings an update took about 3.5–4 ms in
the JVM, of which lowering took about 1.0 ms, signatures 0.8–1.1 and
bodies 0.3–0.5.

In a CPU profile of the Go bench's Hilt signature update
(`BenchmarkUpdateEdit`, `KA_BENCH_SIGNATURE=1`, edit
`h/Feature099.kt`), these are the shares of `Update`:

| Part | Share |
|---|---|
| `checkFiles` | 22% |
| `parallel` (the per-file fan-out) | 15% |
| `reachable` | 13% |
| `lowerFiles` | 13% |
| `declEdits`, `dependents`, `bodyEdits` | 5–6% each |

`reachable` (step 43) finds the files a changed declaration reaches by
name. Each time, it indexed all of the module's files first, through
their cached `fileNames`:
- the declarations that declare each name;
- the sealed classes' names;
- the declarations whose supertype lists write each name.

That came to 1,360 files on Hilt's second round. Between two daemon
updates, only the edited file's parse differs.

The check cache now keeps the indexes of the last four updates, keyed by
their files' paths. A KSP run's rounds alternate between two lists, the
second with the generated files. An update over the same paths takes
the kept index and changes the entries only of the files whose
`fileNames` differ, by pointer. The cache keeps those by parse, so an
unchanged file's pointer is the same. Removing a file's entries drops
its declarations from each name's list and decrements the sealed
counts. The traversal is unchanged.

The switches:
- `KC_NO_REACH_INDEX=1` builds the index for each update, as before.
- In tests, each kept index is compared with a fresh one, and any
  difference panics.

`TestReachIndexKept` applies 11 edits cumulatively to one session:
- sealed classes and subclasses added and removed;
- supertypes changed;
- an operator added;
- a conflicting function;
- a class removed and a class renamed.

Each update is compared with a whole check. Breaking the sealed
decrement made the comparison panic, as it should.

The Go bench, Hilt signature edit, three runs each:
- on: 2.52, 2.44 and 2.49 ms per update;
- off: 2.84, 2.73 and 2.74 ms;
- so about 0.26 ms less per update.

In the KSP bench, six alternating batches per project (on, off, off, on,
on, off), the medians of the JVM's `update` over runs 51–100 (means in
brackets):

| ms, both updates | on | off (`KC_NO_REACH_INDEX=1`) |
|---|---|---|
| Moshi signature | 7.0 (7.95) | 7.2 (8.44) |
| Hilt signature | 7.9 (8.67) | 8.3 (9.10) |
| Moshi body | 2.7 (3.11) | 2.8 (3.11) |
| Hilt body | 3.3 (3.84) | 3.4 (3.84) |

So that is 0.2–0.5 ms off a signature run's updates. Body runs take
libkc's shifted path, which doesn't check. The run totals didn't move
outside the batches' spread: Hilt signature was 48 (51.0) on against 49
(49.9) off, and Moshi's 27 (28.2) against 26 (27.7).

Two other things were tried first and dropped:
- **Parsing the edited file in kc-aa while libkc updates.** The JVM
  would parse the file on another thread, overlapping the native update,
  instead of after it.
  - Round 1's JVM time fell by 0.0–0.2 ms: Moshi body 2.40 against 2.60,
    Moshi signature 4.60 against 4.70, Hilt body 2.90 against 3.05, and
    Hilt signature 5.35 against 5.35.
  - The parse it hides is 0.5–0.7 ms. But waking the thread takes
    0.1–1.2 ms, and the PSI step after it still takes 0.2–0.3. On four
    cores the native update itself slowed by about 0.6 ms from the
    contention.
  - The first, in-order A/B looked like a regression. Batches in ABBA
    order showed it was noise. Every A/B since is in that order.
- **The caches' up-to-date marker.** Measured in place, its create took
  161 µs and its delete 63 µs (medians). JFR overstates native frames
  like those, so there was little to take.

The KSP bench, both backends, 100 runs, output checked against stock's
(`CHECK`), medians of runs 51–100:

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit | 209 | 21 | 10.0x | 17 |
| Moshi, leaf signature edit | 211 | 23 | 9.2x | 20 |
| Hilt, leaf body edit | 469 | 40 | 11.7x | 31 |
| Hilt, leaf signature edit | 477 | 47 | 10.1x | 35 |

This session's machine was slower than step 48's: kc's Moshi body run
took 21 ms here, and 22 in the A/B batches, against 18 there, with the
same code on that path. Stock's Hilt runs were back at about 470 ms,
against step 48's 366.

#### Step 50: small updates run on the calling thread

After step 49, every row was at 10x except Moshi's signature edit (9.2x
on that session's slower machine). A signature run pays for libkc's two
partial checks, which a body run doesn't. Each checks one file. In the
JVM in step 49, these took 2.9 and 3.6 ms (`native`):

| Round | read | parse | lower | signatures | bodies |
|---|---|---|---|---|---|
| 1 | 0.4 | 0.3 | 0.5 | 1.0 | 0.3 |
| 2 | 0.3 | 0.1 | 1.0 | 1.5 | 0.3 |

The Go bench takes 1.95 ms for the same update. It runs updates back to
back, though, while a daemon's come 15–20 ms apart, with the JVM running
in between.

So a harness outside the repository (an outside Go module over
`ka.Open` and `Session.Update`) times Moshi's real edit, which flips
`kspBenchEdit()` between `Int` and `Any`. Before each update it sleeps
15 ms and sweeps 64 MB. Medians of updates 100–300:

| Update | 4 jobs (the default) | 2 jobs | 1 job |
|---|---|---|---|
| Moshi signature | 2.04–2.23 | 2.22 | 1.78–1.83 |
| Hilt signature | 2.28 | | 1.97 |
| Moshi body (shifted) | 0.87 | | 0.83 |

Run back to back with no sleep, it is 1.54 against 1.34 ms. The sleep
alone adds 0.35–0.45 ms to both, from parked threads and cold caches.

With one job, the saving is in lowering (0.45 against 0.39 ms) and
signatures (0.71 against 0.54). Each fan-out wakes Go threads that are
parked between updates:
- `parallel` over all 100 sources for the parse, though 99 take a kept
  parse;
- the lowering of the other files as stubs and declarations;
- the checker's signature and body workers.

For a one-file check, that costs more than the threads save.

Step 50 makes `Update` keep small updates on the calling goroutine:
- The parse takes the kept parses serially and calls `parallel` only on
  the files left to parse. One left starts no goroutine; a cold update
  still parses them all in parallel.
- A partial check whose dirty files, plus the non-dirty files it reaches
  (stubs aren't counted), number at most `smallCheck` (8) lowers and
  checks them on one goroutine.
- The count is taken again when `dependents` widens the check, and a
  whole check uses the session's jobs.
- `KC_ALWAYS_PARALLEL=1` restores the fan-outs.

The read (a stat per file) measured the same either way and is
unchanged.

Results:

| | after | `KC_ALWAYS_PARALLEL=1` |
|---|---|---|
| Harness, Moshi signature, two runs | 1.86, 1.83 ms | 2.17, 2.14 ms |
| Harness, Hilt signature, two runs | 2.15, 2.22 ms | 2.38, 2.56 ms |
| Harness, Moshi body | 0.91–0.92 ms | 0.90–0.99 ms |
| Go bench, Moshi signature | 1.90 ms | 2.02 ms |
| Go bench, Hilt signature | 2.08 ms | 2.23 ms |
| Go bench, `KA_BENCH_RETURN` | the same | the same |

In the KSP bench, six alternating batches per project (on, off, off, on,
on, off), the sums of both rounds' JVM `update` per run, medians (means
in brackets) of runs 51–100:

| ms | on | off (`KC_ALWAYS_PARALLEL=1`) |
|---|---|---|
| Moshi signature, `update` | 5.4 (5.84) | 6.1 (6.91) |
| kc-go `lower` | 0.9 | 1.2 |
| kc-go `signatures` | 1.3 | 1.9 |
| kc-go `bodies` | 0.7 | 0.6 |
| Hilt signature, `update` | 5.7 (6.40) | 6.1 (7.01) |
| kc-go `lower` | 1.1 | 1.3 |
| kc-go `signatures` | 1.1 | 1.6 |
| kc-go `bodies` | 1.0 | 0.8 |
| Moshi body, `update` | 2.2 | 2.2 |
| Hilt body, `update` | 2.7 | 2.6 |

That is 0.4–0.7 ms off a signature run's updates, a little more than
the harness predicted. The one-goroutine body check is 0.1–0.2 ms slower,
but lowering and signatures more than make up for it. Run totals moved
within the batches' spread: Moshi signature 21 (22.4) against 22 (23.8),
Hilt signature 39 (40.9) against 38 (39.5).

**The JVM side.** Round 2's `build` took 0.8–1.0 ms, though it parsed
nothing, and JFR saw only a fifth of it. `KcRound`'s constructor runs a
`parallelStream` over every file to join kc's tree with IntelliJ's,
though a daemon's round finds all but at most one joined before.
`KC_TIMING` now marks its parts and counts the trees a round joins
afresh (`joined=`). In a Moshi batch:
- the join took 0.7 ms in round 1 (one tree joined: IntelliJ parsing
  the edited file) and 0.2 ms in round 2 (Hilt 0.9 and 0.3);
- building the files' symbols took 0.2 ms;
- the rest took nothing.

Round 2 joins no tree, yet the join takes 0.2–0.3 ms. A version that
took the kept trees on the calling thread, and joined only the files
left, didn't change it: 0.20 (0.25) against 0.20 (0.32) on Moshi, and
0.30 (0.32) against 0.30 (0.27) on Hilt. Waking the common pool cost
0.06–0.09 ms in a scratch test, too little to matter. So that version
was dropped and the counts kept. What round 2's join pays for is still
open, as is the edited file's IntelliJ parse in round 1 (0.7 ms).

The KSP bench, both backends, 100 runs, output checked against stock's
(`CHECK`), medians of runs 51–100:

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit | 184 | 16 | 11.5x | 13 |
| Moshi, leaf signature edit | 194 | 20 | 9.7x | 16 |
| Hilt, leaf body edit | 325 | 34 | 9.6x | 28 |
| Hilt, leaf signature edit | 334 | 39 | 8.6x | 34 |

This batch ran about 15% faster than step 49's on both backends: kc's
Moshi body run took 16 ms against 21, and stock's process phase 96 ms
against 108.

Stock's Hilt teardown was 78 ms here against 190 in step 49, as in step
48 (120 against 185–206). So Hilt's ratios swing with stock: against
step 49's stock Hilt runs (469 and 477), kc's are 13.8x and 12.2x.

#### Step 51: the edited file's PSI is parsed while libkc updates

Step 50 left IntelliJ's parse of the edited file in round 1's tree join
(0.6–0.8 ms with `joined=1`), plus about 0.15 ms making its PSI. Both
ran on the calling thread after libkc's update returned. KSP passes
the files Gradle says were modified (`KSPConfig.modifiedSources`), so
the session knows before the update which file's PSI it will need.

Now, before calling `KcOps.update` for a session's first round,
`KcSession` hands the modified Kotlin sources to
`KcEnvironment.parseAhead`. On the common pool, it:
- finds and reads each file;
- makes its PSI with `psiFile` (which records the hash of the bytes it
  read);
- walks the tree, which has IntelliJ parse it;
- leaves the tree in `KcTrees` under its PSI.

The calling thread waits for it after the update and touches nothing of
the environment before then. The round's `psiFile(path, libkc's hash)`
finds the same hash and takes the PSI made ahead. The join takes the
tree made ahead, checks it as one it made, and keeps it. Trees no round
took are dropped after the join. A file that changes between the two
reads has another hash and is made again, as before. A file that fails
ahead is left to the round. `KC_NO_PARSE_AHEAD=1` turns it off.

`KC_TIMING` now prints the parse's parts (`kc-jvm: ahead`) and splits
the round's update into `native` and `wait`. It also marks the round's
field initializers (`fields`, which take nothing). `KcTimer` no longer
counts its own formatting in the next part.

The parse's parts, medians of runs 51–100:

| ms | Moshi | Hilt |
|---|---|---|
| waking a pool thread | 0.1 | 0.1 |
| making the PSI | 0.1–0.2 | 0.2 |
| the parse | 0.6–0.7 | 0.7–0.9 |
| all, body runs (mean) | 1.1 (1.8) | 1.5 (1.9) |

The parse costs what it did on the calling thread. A signature run's
round-1 update (about 2.5 ms) almost always outlasts it. A body run's
(about 1 ms) often doesn't: the calling thread then waits, 0 ms at the
median but 0.4–0.7 ms on average.

In the KSP bench, six alternating batches per project (on, off, off,
on, on, off; off is `KC_NO_PARSE_AHEAD=1`), sums of both rounds,
medians (means in brackets) of runs 51–100:

| ms | Moshi on | Moshi off | Hilt on | Hilt off |
|---|---|---|---|---|
| body, `update` | 2.8 (3.20) | 2.3 (2.68) | 3.6 (3.95) | 2.8 (3.32) |
| body, `psi` | 0.2 | 0.4 | 0.5 | 0.6 |
| body, `join` | 0.3 | 0.9 | 0.4 | 1.1 |
| body, the three | 3.3 | 3.6 | 4.5 | 4.5 |
| signature, `update` | 5.9 (6.46) | 5.6 (5.90) | 6.8 (7.67) | 6.0 (6.94) |
| signature, `psi` | 0.2 | 0.4 | 0.5 | 0.6 |
| signature, `join` | 0.4 | 0.9 | 0.4 | 1.1 |
| signature, the three | 6.5 | 6.9 | 7.7 | 7.7 |

The join and the PSI step lose 0.7–0.8 ms, but round 1's update grows
by 0.3–0.8 ms. Part of that is the wait. The rest is libkc itself,
slower with the parse beside it:
- round 1's `native` takes 1.1 against 1.0 ms on Moshi's body runs,
  and 1.6 against 1.1 on Hilt's;
- its `read`'s mean goes from 0.38 to 0.62 ms (Moshi) and from 0.56 to
  0.90 (Hilt), though its median stays at 0.3;
- round 2, with nothing beside it, takes the same either way.

The machine has four cores. The parse competes with libkc's parallel
`read` and with Go's idle GC workers, and both sides with each other's
caches. So the step takes 0.3–0.4 ms off a Moshi run and nothing off a
Hilt one. Run totals moved within the batches' spread:

| | on | off |
|---|---|---|
| Moshi body | 18 (19.4) | 19 (19.5) |
| Moshi signature | 22 (23.0) | 22 (24.1) |
| Hilt body | 37 (38.8) | 36 (37.8) |
| Hilt signature | 44 (46.1) | 42 (43.9) |

That leaves two leads:
- Make a file's tree lazy in `Builder`, so the round would wait for
  the parse only when it first builds that file's symbols, not after
  the update.
- Start the parse after libkc's `read`, not beside it.

The KSP bench, both backends, 100 runs, output checked against stock's,
medians of runs 51–100:

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit | 359 | 17 | 21x (10.8x against step 50's stock) | 15 |
| Moshi, leaf signature edit | 340 | 21 | 16x (9.2x against step 50's stock) | 16 |
| Hilt, leaf body edit | 335 | 35 | 9.6x | 29 |
| Hilt, leaf signature edit | 335 | 40 | 8.4x | 33 |

Stock's Moshi teardown took 192–204 ms here, against 30–116 in earlier
batches, so its Moshi ratios swing with it; its Hilt teardown took
92–93. kc's Moshi totals are 1 ms above step 50's comparison batch,
though the A/B on one machine puts them 0.3–0.4 ms below: a batch's
machine state moves the totals more than this step does.

#### Step 52: round 2 takes the edited file's model from round 1

After step 51, a signature run cost 3–4 ms more than a body run, and
most of the difference was libkc's second update, after KSP adds the
generated files: about 2.5 ms on Moshi and 3 on Hilt (`native`),
against about 1 for a body run's. That update compares the files with
the check it finds for the same paths, the last run's round 2, in
which the edited file had the other text. So `declEdits` made it dirty
and the update checked it again, though round 1 had checked it moments
before with the same text and the same parse (`cache.snapshot` already
took the parse from there).

Now, once `declEdits` has the dirty files, `checkCache.borrow` looks
for another kept check that has them with the same text and parse: round
1's. It takes that check's models of the dirty files if:
- that check is clean, of Kotlin sources only, reports nothing in them,
  and has no file the update doesn't;
- each Kotlin file the update has that it doesn't, or has with another
  text, `prev` has with the same text, so what it declares is `prev`'s
  model of it, and kc parses each of the update's Java sources;
- no dirty file looks up or declares a name those Kotlin and Java files
  declare (`declaredNames.affects`, the rule `declEdits` stands on);
- no dirty file writes, as a word, the name of a package they make:
  one none of the lending check's files is in or inside, and the class
  path doesn't have. A package missing from round 1 that a file names
  is reported there, so the rule above leaves it out; this covers the
  rest;
- with Java sources, no Java text names a class or facade of the dirty
  files whose models differ from `prev`'s.

Then, as an update that only shifted models does, it takes `prev`'s
models of the rest and `prev`'s library: its Java classes see the
Kotlin classes as they were, which the last rule allows. If `dependents`
finds another file that looks up what changed, the update drops the
borrowed models and checks as before. `KC_NO_BORROW=1` turns it off.
The JVM needs no change: a borrowed `*File` is round 1's, which
`kaops` encodes as reused, so the round takes the decoded file and the
tree joined in round 1.

The first version left out updates with Java sources, which is all of
Hilt's round 2s: its processors generate Java. Its tests cover generated
Java files, one whose class a dirty file then resolves to, one that names
the changed facade, and new packages with parents the class path has or
hasn't.

In the KSP bench, six alternating batches per project (on, off, off,
on, on, off; off is `KC_NO_BORROW=1`), round 2's update in signature
runs, medians (means in brackets) of runs 51–100 (Moshi's batches ran
the first version, which took the same path there):

| ms | Moshi on | Moshi off | Hilt on | Hilt off |
|---|---|---|---|---|
| round 2's update, libkc's parts after `parse` | 0.4 (borrow) | 2.0 (check) | 0.7 (borrow) | 2.2 (check) |
| round 2's `native` | 0.8 (1.45) | 2.5 (2.77) | 1.6 (1.91) | 3.0 (3.44) |
| round 2's `update` (JVM) | 1.0 (1.62) | 2.7 (2.98) | 1.8 (2.11) | 3.2 (3.67) |
| run total, signature | 20 (21.6) | 22 (23.5) | 40 (40.5) | 42 (44.9) |
| run total, body | 18 (20.3) | 19 (19.1) | 38 (39.6) | 36 (37.7) |

Round 2's signature update now costs what a body run's does. It
borrowed on 49 of each batch's 50 signature runs. Body runs take the shifted path
as before; their totals moved within the batches' spread.

The KSP bench, both backends, 100 runs, output checked against stock's,
medians of runs 51–100:

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit | 227 | 18 | 12.6x | 15 |
| Moshi, leaf signature edit | 223 | 19 | 11.7x | 18 |
| Hilt, leaf body edit | 368 | 35 | 10.5x | 29 |
| Hilt, leaf signature edit | 378 | 39 | 9.7x | 33 |

Stock's Moshi teardown took 72–75 ms here, against 192–204 in step 51's
batch; its Hilt teardown took 112–115. Moshi's signature edit is now
above 10x against this batch's stock and step 50's (194 ms, 10.2x).
A kc signature run now costs about 1 ms more than a body run, on
both projects: the check of the edited file in round 1.

#### Step 53: Hilt's overhead outside the processors (no change kept)

After step 52, a Hilt run took 35 ms (body edit) or 39 (signature edit)
against stock's 368 and 378, and the processors themselves about 20 of
that. This step profiled Hilt, to find the largest piece outside
processor code that kc or the fork controls, and cut it.

Two kc batches of 100 runs on Hilt, a leaf edit, JFR from run 51 (the
recording slows the runs about 2x; one sample is about 1.4 ms a run at
this scale, calibrated against `KC_TIMING`'s libkc time):

| samples (50 runs) | signature edit | body edit |
|---|---|---|
| `process`, all | 630 | |
| …of it, not processor code (KSP's symbol layer 102, kc-aa 64, IntelliJ 44, libkc 11) | 221 | |
| setup, between rounds and teardown, all | 463 | 332 |
| …`updateCachesAndOutputs` | 88 | 61 |
| ……updating and flushing the lookup storages | 48 | 17 |
| ……writing the symbol maps (`PersistentMap.flush`) | 15 | 12 |
| …`calcDirtyFiles` (symbols of the edited file, `DirtinessPropagator`, paths) | 42 | 29 |
| …`registerGeneratedFiles` (the symbols of each generated Java file) | 40 | 34 |
| …`IncrementalContextAA`'s construction | 12 | 10 |

Inside `process`, the 221 samples that aren't processor code are spread
over about 30 KSP and kc-aa calls; the largest is
`getSymbolsWithAnnotation`'s `mapAnnotatedSymbols` (24). In the Go
benchmark (`BenchmarkUpdateEdit`, Hilt, `h/Feature099.kt`), round 1's
signature update takes 2.07 ms against a body edit's 0.91, spread over
the check (0.59), parsing (0.27), the signature hash (0.15), lowering
(0.15), `dependents` (0.14) and reading (0.13): about 0.3 ms of it looked
cuttable.

The largest single piece was `registerGeneratedFiles`, on every Hilt
run: after each round, KSP visits every declaration of every file the
round generated, to record the symbols it defines. Most of those files
have the same text as the run before, and the symbols depend only on the
text. So fork patch 0018 kept each generated file's last text and
symbols for the process and reused them when the text matched
(`KSP_SYMBOL_VERIFY=1` batches on Moshi and Hilt matched the symbols
collected anew on every run). Under JFR, `registerGeneratedFiles` went
from 40 samples to none, but `mapAnnotatedSymbols` went from 24 to 57:
the processors' first `getSymbolsWithAnnotation` of each round now built
the generated files' declarations, which the visit used to build first.

Six alternating batches per project (on, off, off, on, on, off; off is
`KSP_NO_SYMBOL_REUSE=1`), medians (means in brackets) of runs 51–100:

| ms | Hilt on | Hilt off | Moshi on | Moshi off |
|---|---|---|---|---|
| run total, body | 37 (39.3) | 35 (36.6) | 19 (20.8) | 19 (19.9) |
| run total, signature | 40 (43.5) | 37 (40.5) | 21 (22.4) | 20 (21.8) |
| `process`, body | 25 (26.4) | 22 (23.1) | 9 (10.0) | 9 (9.4) |
| `process`, signature | 25 (26.9) | 23 (24.4) | 9 (10.4) | 9 (9.5) |

The patch made Hilt 2–3 ms slower, all of it in `process`; kc's own
parts (`KC_TIMING`) didn't move. The JFR batch put the extra time in
the processors' own code (JavaPoet's `emit`, `KspFiler.write`). Patch
0016's check of clean outputs against their backups (about 2,500
`stat`s on one thread) runs while the processors do, on a 4-core machine
that also runs the output writer. With that check off
(`KSP_NO_PRECHECK=1`, four alternating batches), the patch saved
1.5–2.5 ms instead (Hilt body 38.5 against 41, signature 41.5 against
43), with `process` equal. So the visit had left room for the backup
check, and without it the check competed with the processors. Both
settings are still slower than step 52's (the check on, no patch: 35
and 37), so the patch was reverted.

Leads for later:
- the backup check's cost while the processors run, now measured
  through the patch: fewer `stat`s (outputs and backups this process
  wrote or copied and whose stamps still match), or a later start;
- a delta update of the lookup storages on signature runs, where patch
  0017's skip never fires (about 0.9 ms more than a body run's update
  and flush);
- `calcDirtyFiles` (about 1 ms, spread);
- round 1's signature update in Go (about 0.3 ms that looks cuttable).

Step 54 found that this step's A/B is biased (see there): only its "off"
arm set an environment variable, and that alone made runs about 2 ms
faster. The patch may not have been slower at all; it needs another A/B
with the variable set in both arms. (Step 55 ran it: the patch saves
1–2 ms in `between` and leaves `process` as it was, so it is back.)

#### Step 54: the lookup storages updated by delta

On Hilt's signature runs, fork patch 0017's skip never fires: the edited
file looks up `Int` in one run and `Any` in the next. Then
`LookupStorageWrapperImpl.update` ran Kotlin's full update
(`BuildUtilKt.update` in the compiler jar):
- `removeLookupsFrom(F)` for the dirty files;
- then `addAll` of every record the run's tracker has: thousands of
  appends to `lookupMap`, nearly all of records the storage already
  held, which the flush then wrote.

Step 53's profile put that at 48 samples on signature runs against 17
on body runs.

Writing it showed something about KSP's own layout:
- The tracker has files as absolute paths (`virtualFile.path`).
- The storage's path converter keeps paths as they are.
- The dirty files KSP passes are relative to the base directory.

So `removeLookupsFrom(F)` finds nothing to drop, in stock KSP too: a
dirty file's old records stay, and each update only adds. A record a
file no longer makes keeps it dirty for later edits of that symbol.
That is conservative, not wrong. kc keeps that behaviour.

Fork patch 0018 writes only what changed, when the process knows each
dirty file's records (`known`, patch 0017's):
- a dirty file that looks up only more than it did gets those records;
- one that no longer looks up something it did is removed, as the full
  update does it. If its records are still there afterwards (`get` of
  a symbol it still looks up names it), it gets only the new ones;
  otherwise it gets them all again;
- any other file the tracker names gets the records the storage isn't
  known to hold;
- a run that writes nothing doesn't flush.

Every update, skipped or not, also made each tracker path relative to
the base directory first. Paths outside it (library classes) take
`relativeTo`'s slow path, 7 samples on signature runs. Those paths are
now kept for the process. `KSP_NO_LOOKUP_DELTA=1` turns both off.

`KSP_LOOKUP_VERIFY=1` runs the full update after the delta and fails if
that changes what `get` returns for any symbol either touches.
- Hilt (leaf, 100 runs; `EDIT=auto`, 40) and Moshi (leaf, 100) passed.
- `dirty=` was the same in every run with and without the delta.
- `build.sh stock` is 11 ok and `build.sh ksp` is 134/136.

Under JFR (Hilt signature runs, 50 each), `updateLookupCache` went from
48 samples to 38:
- `addAll` 10 → 3;
- the flush 16 → 11, now mostly the persistent maps' fixed cost (forcing
  the mapped files, writing the append caches).

**The A/B, and a bias in the earlier ones.** The first A/B, like step 53's,
set `KSP_NO_LOOKUP_DELTA=1` in the "off" arm and nothing in the "on"
arm. The "off" arm came out 2–3 ms faster on Hilt, as in step 53, and
again in `process` and `between`, which the patch doesn't touch. Batches
in reverse order (off, on, on, off) gave the same result.

With the variable set in both arms (`=0` for on, `=1` for off), the
difference reversed and moved into the teardown. So the size of the
environment alone shifts these runs by about 2 ms, as large as the
effects being measured. That is the known bias of environment size on
memory layout.

Steps 48–52 also set their switch only in the "off" arm. Their "on" arms
were faster all the same, so if anything the bias understated those
gains. From here on, both arms set the switch.

Equal environments, medians (means in brackets) of runs 51–100. Hilt has
six batches (on, off, off, on, on, off); Moshi has four (on, off, off,
on):

| ms | Hilt on | Hilt off | Moshi on | Moshi off |
|---|---|---|---|---|
| teardown, body | 1.0 (1.6) | 2.0 (2.2) | 2.0 (1.9) | 1.0 (1.4) |
| teardown, signature | 2.0 (1.9) | 3.0 (2.8) | 2.0 (1.8) | 1.0 (1.5) |
| run total, body | 36 (39.8) | 38 (38.9) | 19 (20.5) | 18 (19.0) |
| run total, signature | 40 (41.9) | 41 (42.5) | 21 (21.8) | 20 (21.4) |

On Hilt the teardown drops by 0.6–0.9 ms, on body runs (the path memo)
and on signature runs (both parts). Moshi's teardown is about 0.4 ms
higher with the patch, in all ten of its batches, with or without the
bias. Only the memo applies there, since patch 0017 skips every Moshi
update, and why it would cost anything is open. Its totals moved within
the batches' spread.

The KSP bench, both backends, 100 runs, output checked against stock's,
medians of runs 51–100:

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit | 199 | 21 | 9.5x | 14 |
| Moshi, leaf signature edit | 205 | 21 | 9.8x | 17 |
| Hilt, leaf body edit | 337 | 36 | 9.4x | 30 |
| Hilt, leaf signature edit | 334 | 39 | 8.6x | 32 |

Stock ran faster in this batch than in step 52's: its teardown took
42 ms on Moshi against 72–75, and 65–71 on Hilt against 112–115. kc's
runs are where they were. Against step 52's stock, the ratios are
10.8x and 10.6x (Moshi) and 10.2x and 9.7x (Hilt). Stock's teardown, and
so the ratio, moves by tens of ms from one batch to the next.

#### Step 55: step 53's symbol reuse, re-tested with equal environments

Step 53's A/B set `KSP_NO_SYMBOL_REUSE=1` only in its "off" arm, the
bias step 54 found. So this step put the patch back, unchanged but for
its switches, as fork patch 0019 on top of 0018. It keeps each generated
file's last text and the symbols `registerGeneratedFiles` collected from
it, and reuses the symbols when the text is the same. The switches now
check for `1`, like 0017's and 0018's, so `KSP_NO_SYMBOL_REUSE=0` keeps
reuse on (step 53's checked only that the variable was set).

`KSP_SYMBOL_VERIFY=1` collects every reused file's symbols again and
fails if they differ. It ran with `KSP_LOOKUP_VERIFY=1`, so 0018's delta
stays checked with 0019 on top. Hilt (leaf, 100 runs; `EDIT=auto`, 40)
and Moshi (leaf, 100) passed on every run. `build.sh stock` is 11 ok and
`build.sh ksp` is 134/136.

Six batches per project, kc only (on, off, off, on, on, off; `=0` for
on, `=1` for off), medians (means in brackets) of runs 51–100:

| ms | Hilt on | Hilt off | Moshi on | Moshi off |
|---|---|---|---|---|
| run total, body | 36 (38.3) | 38 (39.6) | 20 (21.4) | 23 (23.0) |
| run total, signature | 40 (40.8) | 41 (42.3) | 21 (22.0) | 23 (24.1) |
| `between`, body | 3 (3.6) | 4 (4.7) | 2 (2.7) | 3 (3.3) |
| `between`, signature | 3 (3.8) | 5 (5.6) | 2 (2.6) | 3 (3.0) |
| `process`, body | 25 (26.3) | 25 (26.3) | 9 (10.7) | 10 (11.0) |
| `process`, signature | 25 (26.2) | 25 (25.5) | 9 (10.4) | 11 (11.1) |

`between` is where `registerGeneratedFiles` runs after each round. On
Hilt it is lower in every "on" batch than in every "off" batch: 1–2 ms,
as the profile predicted. `process` doesn't grow. So step 53's slowdown
came from the bias, not from the patch: the processors' first
`getSymbolsWithAnnotation` does build the declarations the visit used to
build (step 53's JFR), but that costs less than the visit did. Moshi's
totals drop by about 2 ms, more than its `between` alone, though one of
its "off" batches was slow throughout.

With stock in the same batches (`BACKENDS="stock kc" CHECK=1`), Hilt
again ran on, off, off, on with equal environments. kc's medians:

| ms | on | off |
|---|---|---|
| Hilt, body | 37 | 36 |
| Hilt, signature | 39.5 | 40 |
| `between`, both kinds | 3 | 4 |

The totals there are within the spread; `between` drops in every batch.
A first comparison batch had kc's Hilt runs at 41 and 43 ms, 4 ms of it
in `process`, but no later batch, on or off, repeated it.

The KSP bench, both backends, 100 runs, output checked against stock's,
medians of runs 51–100 (Hilt pools the two "on" batches above):

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit | 203 | 18 | 11.3x | 14 |
| Moshi, leaf signature edit | 197 | 19 | 10.4x | 15 |
| Hilt, leaf body edit | 355.5 | 37 | 9.6x | 28 |
| Hilt, leaf signature edit | 359 | 39.5 | 9.1x | 30 |

Stock's teardown moved between 66 and 166 ms from one Hilt batch to the
next, so the Hilt ratios ranged from 8.8x to 11.8x across the four
batches. kc moved by 3–4 ms.

#### Step 56: a KSP run's second round reuses its first round's sources

A KSP run's rounds are updates of one libkc session, and KSP only adds
files between them. Yet each update statted every source again (and read
those changed within the racy window), and parsed every Java source
again: about 0.3 ms of reading and, on Hilt, 0.3 ms of Java parsing in
round 2 (step 55's `KC_TIMING`). Two changes:

- **Fixed sources.** `ka.Config.FixedSources`, which kc-aa sets (OpKaOpen
  gained a trailing `fixed:Bool`), says the files don't change while the
  session is open. An update then takes the text a source had in the
  session's last update without statting its file, and reads only the
  files that are new: in round 2, the files round 1 generated.
- **Java parses kept.** The library keeps the parse of each Java source
  by path and text (`javacache.go`), and parses only the sources whose
  texts changed. A parsed `javasrc.File` is only read once parsed, so
  models share it. It keeps the parses of the last two updates, not one:
  Hilt's Java files are generated in round 1 and are sources of round 2
  only, so the next run's round 1, which has no Java sources, would drop
  them before its round 2. The first A/B, with one update kept, showed no
  hit at all; their texts are mostly the same from run to run.

`KC_NO_ROUND_REUSE=1` turns both off. `build.sh stock` is 11 ok and
`build.sh ksp` is 134/136; a new test checks that a session with fixed
sources doesn't see a file changed on disk, reads a new one, and parses a
Java source again only when its text changes or after two updates
without it.

Six batches per project, kc only (on, off, off, on, on, off; `=0` for
on, `=1` for off), means of round 2's phases (`KC_TIMING`, 0.1 ms
resolution) and medians (means in brackets) of the run totals, runs
51–100. Moshi is from the first A/B (it has no Java sources), Hilt from
the second, with two updates' parses kept:

| ms | Hilt on | Hilt off | Moshi on | Moshi off |
|---|---|---|---|---|
| round 2 read, body | 0.32 | 0.35 | 0.25 | 0.41 |
| round 2 read, signature | 0.29 | 0.41 | 0.25 | 0.41 |
| round 2 parse, body | 0.00 | 0.32 | 0.00 | 0.00 |
| round 2 parse, signature | 0.01 | 0.38 | 0.00 | 0.00 |
| run total, body | 35 (37.5) | 35 (36.6) | 17 (18.5) | 19 (20.2) |
| run total, signature | 37 (40.5) | 38 (39.9) | 20 (20.9) | 20 (22.1) |

Round 2's Java parse is gone on Hilt and its read drops by 0.05–0.1 ms:
what is left is reading the 12 files round 1 generated, which KSP writes
anew every run. On Moshi the read drops by 0.16 ms. That is 0.4 ms a Hilt
run and 0.16 ms a Moshi one, below the spread of the totals, which don't
show it: Hilt's first "on" batch was slow throughout (38 and 39 ms), and
Moshi's 2 ms on body runs is more than the phases saved (one of its
"off" batches was slow). There was no stock run: the ratios stay step
55's.

#### Step 57: round 1's read without the goroutine fan-out

Steps 55 and 56's `KC_TIMING` logs show one stall that no other phase
has: round 1's Go `read` was 1 ms or more in 30–43% of the runs (Hilt 65
and 61 of 150 per edit kind, Moshi 43 and 45). It was bimodal, 0.2–0.4 ms
or 1.2–1.8 ms, in both arms of each A/B. Other phases reach 1 ms in a
few runs, apart from signature runs' `bodies` (15–23 of 150). A GC would
land on random phases.

Round 1's update is the first Go work after the processors, in a new
session, and `readCache.texts` sent every source's stat to `parallel`,
four goroutines here. A stat is a syscall: a goroutine that blocks in it
can wait for its thread to be woken, with the JVM's threads busy (step
51's parse-ahead among them), while the update waits for it. Warm, 100
stats take about 0.13 ms one after another.

The files the last update read are now statted on the calling
goroutine; only the files to read go to `parallel`, when there are more
than `smallCheck` (8). Files without a last read (a first update, new
files) are read as before, so a cold start still reads in parallel.
`KC_NO_SERIAL_STAT=1` turns it off. `KC_TIMING` now marks `library`, the
shared library's lookup, apart from `read`. A new test reads 24 changed
files through the parallel branch. `build.sh stock` is 11 ok and
`build.sh ksp` is 134/136.

Six batches per project, kc only (on, off, off, on, on, off; `=0` for
on, `=1` for off), runs 51–100: round 1's `read` (mean, runs at 1 ms or
more out of 75) and the run totals' medians (means in brackets):

| ms | Hilt on | Hilt off | Moshi on | Moshi off |
|---|---|---|---|---|
| round 1 read, body | 0.21, 0 | 0.78, 29 | 0.22, 0 | 0.78, 28 |
| round 1 read, signature | 0.21, 0 | 0.84, 25 | 0.21, 0 | 0.66, 20 |
| round 2 read, body | 0.27 | 0.25 | 0.15 | 0.37 |
| round 2 read, signature | 0.63 | 0.41 | 0.13 | 0.29 |
| run total, body | 37 (38.1) | 36 (37.4) | 20 (20.9) | 21 (22.4) |
| run total, signature | 39 (40.3) | 40 (42.0) | 21 (21.9) | 22 (22.1) |

The stall is gone: no "on" run's round 1 read reaches 1 ms, and its mean
drops by 0.5–0.6 ms; `library` stays under 0.1 ms. Hilt's signature
round 2 mean is one slow run's. Moshi's totals drop by about 1 ms; Hilt's
batch medians spread from 34 to 38 ms, which covers the difference.

The KSP bench, both backends, 100 runs, output checked against stock's,
medians of runs 51–100:

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit | 297 | 21 | 14.1x | 15 |
| Moshi, leaf signature edit | 315 | 22 | 14.3x | 16 |
| Hilt, leaf body edit | 401 | 35 | 11.5x | 30 |
| Hilt, leaf signature edit | 401 | 37 | 10.8x | 33 |

Stock was slow in this batch: its teardown was 139 ms on Moshi and 120 ms
on Hilt (42–75 and 65–115 in steps 52–55), and its Moshi runs were about
100 ms slower than step 55's. Against step 55's stock, kc's runs here are
9.0–9.7x on Moshi and 9.7–10.2x on Hilt. kc's Hilt runs are 2–2.5 ms
faster than step 55's (37 and 39.5), but its Moshi runs are 3 ms slower
(18 and 19), as were this step's Moshi A/B batches in both arms (19–23),
so the machine, not kc alone, moved between the steps.

#### Step 58: the Java sources' classes, and absent names, kept across rounds

The class-path classes libkc models are kept by kc-aa's environment
(`KcSession.libraryDecl`), so a warm run asks libkc only for the classes
of its Java sources, which change with them, and for names libkc has no
class of, which it didn't keep. On Hilt that is 24 `KcOps.cls` calls a
run (`kc-jvm: lookups class=24/0.4–0.5`): 12 for the generated Java
classes of round 2, and 12 in round 1 for those same classes, not
generated yet, and package names (`h`, `dagger`). A trace of each call
put the Go `Model.Class` at 0–3 µs, the whole call at 1–30 µs inside Go,
and 5–60 µs seen from the JVM: the cost is the call, and its
message, not the model.

libkc already keeps a check's library, the Java sources' classes with
it, when an update finds the Java sources and the Kotlin classes they
name as they were (the shifted and borrowed updates set `m.lib =
prev.lib`). Hilt's round 2 takes one of those on both edit kinds: its
generated Java texts are the same run after run. So each library over
Java sources now gets an id unique in the process (`Model.JavaID`), sent
with the model (`model = handle check java same ...`), and kc-aa keeps
the Java classes it was given, nulls too, by that id, for the last four
ids. The class reply says whether the class path, the sources left
out, has no class of the name (`Model.Absent`; `n:Uint decl*n
absent:Bool`), and kc-aa keeps those names with the class-path classes;
a class of the program is absent from the class path too, and a Java
source's class is asked by its id first. `KC_NO_JAVA_DECLS=1` turns both
off. New tests: `TestJavaID` (same id after a body edit, another after
a Java edit, 0 without Java sources, `Absent` of a source class) and
`TestClass`'s absent flag. `build.sh stock` is 11 ok and `build.sh ksp`
is 134/136.

Six batches per project, kc only (on, off, off, on, on, off), runs
51–100, medians (means in brackets):

| ms | Hilt on | Hilt off | Moshi on | Moshi off |
|---|---|---|---|---|
| class calls per run | 0 | 24 (0.5 ms) | 0 | 0 |
| run total, body | 35 (36.5) | 38 (38.9) | 20 (21.0) | 20 (21.0) |
| run total, signature | 37 (40.2) | 41 (42.2) | 22 (22.7) | 20 (21.8) |

Moshi makes no class calls in either arm: its rows are the spread. Hilt's
batch medians were 34–36 on and 35–41 off for body runs, 36–40 and 37–44
for signature runs, and the "off" batch between two "on" ones matched
them: the cut is the 0.5 ms the calls took, the rest of the difference
the batches' spread.

The KSP bench, both backends, 100 runs, output checked against stock's,
medians of runs 51–100:

| ms | stock KSP2 | kc | ratio | kc min |
|---|---|---|---|---|
| Moshi, leaf body edit | 209 | 17 | 12.3x | 14 |
| Moshi, leaf signature edit | 219 | 17 | 12.9x | 15 |
| Hilt, leaf body edit | 436 | 31 | 14.1x | 27 |
| Hilt, leaf signature edit | 418 | 33 | 12.7x | 28 |
| Room, leaf body edit | 9054 | 3076 | 2.9x | 2668 |
| Room, leaf signature edit | 8967 | 2870 | 3.1x | 2494 |

kc ran faster in this batch than in the A/B's (Hilt 31 and 33 against 35
and 37), and stock's Hilt teardown was slow again (169–182 ms, 65–115 in
steps 52–55). Against step 55's stock, kc's runs here are 11.5x and 10.9x
on Hilt and 11.9x and 11.6x on Moshi. Room's ratio is bounded by its
processors (about 3.5x, the ceiling in `docs/ksp-perf.md`).

#### Step 59: the sources no round parsed yet, parsed while libkc updates

Step 59 turns to cold runs, the first run in a fresh JVM. Run 1 of step
58's batches put most of kc-aa's own cold time after libkc's updates:

- **Moshi.** Round 1's join (IntelliJ parsing the 100 sources) took
  203–255 ms after a native update of about 200 ms. Round 2's join of
  the 400 generated files took 805–906 ms after a native update of
  566–637 ms.
- **Hilt.** Round 1's join took 250–278 ms. In round 2, `KcSession.build`
  loading the PSI of the 1260 generated Java files one by one took
  426–545 ms.

Step 51's parse-ahead already overlapped the two, but only for the files
Gradle reports as modified. Making the join lazy, per file, was looked at
and set aside:

- KSP's annotation walk visits every new file.
- Building a file's symbols reads its PSI (`psiOf`, accessors, annotation
  filters).
- KSP reads Java annotations and `psi.classes` from PSI.

So every new file would be parsed anyway, serially under the round's
lock.

**The change.** `KcSession.update` now parses ahead the modified files
plus every Kotlin file no round has joined yet (`KcTrees.has`). That is
every Kotlin file in a fresh environment's first round, and the
generated ones in the rounds after. A warm run's other files are already
joined, so warm runs parse ahead what they did before.

`parseAhead` makes the PSI one file after another, since `psiFile` isn't
thread-safe, and that part is cheap. It then parses in a `parallelStream`
when there are more than 8 files. The round's join takes the trees from
`trees.takeAhead`, and `checkKinds` checks them as before.
`KC_NO_AHEAD_NEW=1` keeps step 51's list, of modified files only.

A first version also loaded the new Java files' classes ahead, in
parallel. Hilt's round 2 then waited 460 ms (423–525) for the ahead work,
against 490 ms (450–556) loading serially in `build`. IntelliJ parsed the
Java files no faster in parallel, and libkc's update slowed beside them,
so Java files are left to `build` again.

`build.sh stock` is 11 ok and `build.sh ksp` is 134/136 on both versions.
The bench's check (kc generates what stock does) passed on Moshi and Hilt.

**The A/B.** Cold runs, kc only, `COLD=4` per batch plus the bench's own
run 1. Moshi and Hilt ran six batches (on, off, off, on, on, off: 15 JVMs
per arm); Room ran four (on, off, off, on: 6 JVMs per arm). The table
shows medians, with ranges in brackets. "Round" is the round's whole
`KC_TIMING` line: native update, wait, PSI, join, symbols and build.

| ms | Moshi on | Moshi off | Hilt on | Hilt off | Room on | Room off |
|---|---|---|---|---|---|---|
| cold run | 5677 (5400–6103) | 6028 (5857–6535) | 8593 (7927–9012) | 8780 (8370–9110) | 11860 (11246–12318) | 12620 (12520–13293) |
| setup | 1321 | 1415 | 1491 | 1628 | 1562 | 1757 |
| process | 2155 | 2180 | 4870 | 4914 | 8432 | 8596 |
| round 1 | 409 | 525 | 525 | 634 | 610 | 745 |
| round 1 join | 24 | 217 | 33 | 282 | 45 | 332 |
| round 2 | 1249 | 1444 | 648 | 652 | 1389 | 1830 |
| round 2 join | 90 | 793 | 0 | 0 | 139 | 1116 |

What the numbers show:

- **Libkc's update slows beside the parse.** On 4 CPUs, Moshi's round 2
  native update took 1131 ms against about 600. Its Go phases slowed
  too: parse 320 ms against 144, bodies 440 against 217. So the overlap
  gives back about a quarter of the join, not all of it.
- **The cuts.** Room gains the most, 760 ms a run, of which round 2's
  400 generated Kotlin files are 440 ms. Moshi gains 350 ms. Hilt gains
  about 190 ms, all of it round 1's; its round 2 is Java.
- **A disturbed Hilt batch.** One Hilt A/B, run right after `build.sh
  ksp`, had its first batches at 9–10.7 s and showed "on" slower in
  `process`. Six clean batches afterwards, in the table, show `process`
  equal in both arms (4870 against 4914).

**Against stock.** Stock KSP2 cold, `COLD=4` plus run 1 (5 JVMs), medians:

| ms | stock KSP2 | kc (step 59) | ratio | kc before (off) | ratio before |
|---|---|---|---|---|---|
| Moshi, cold run | 11411 | 5677 | 2.0x | 6028 | 1.9x |
| Hilt, cold run | 19570 | 8593 | 2.3x | 8780 | 2.2x |
| Room, cold run | 24818 | 11860 | 2.1x | 12620 | 2.0x |

A cold run is mostly JIT warm-up and setup that both backends share:
KSP's, the processors' and IntelliJ's. Of kc's own work, what is left
cold is:

- the environment, about 0.5–0.6 s;
- libkc's first update, about 0.2–0.45 s;
- round 2's native update, slowed by the parse beside it;
- Hilt's serial Java PSI, about 0.5 s.

#### Step 60: libkc's first update beside the environment (reverted)

A fresh JVM's session acquires its environment first, about 0.6 s of
`KotlinCoreEnvironment`. Only then does it run libkc's first update,
0.25–0.5 s. That update needs only the session's handle and its source
paths: a new environment has no kept rounds to share (`reusableChecks`
and `baseFiles` are empty).

Step 60 opened libkc and listed the sources first. When no kept
environment matched the key (`KcEnvironments.has`), it started the first
update on another thread, and the round's update took its result.
`KC_NO_EARLY_UPDATE=1` kept the old order. `build.sh stock` was 11 ok,
`build.sh ksp` 134/136, and the bench's check passed on Moshi.

Cold A/B, as in step 59 (15 JVMs per arm for Moshi and Hilt, 6 for Room),
medians:

| ms | Moshi on | Moshi off | Hilt on | Hilt off | Room on | Room off |
|---|---|---|---|---|---|---|
| round 1 | 375 | 457 | 429 | 500 | 497 | 627 |
| setup | 1412 | 1428 | 1458 | 1448 | 1620 | 1694 |
| cold run | 5981 | 6067 | 8515 | 8697 | 12371 | 12779 |

What the A/B shows:

- **Round 1 gains.** Libkc's update finished while the environment was
  made (`kc-jvm: early update=270–300`), and round 1 then waited only
  for step 59's parse, which needs the environment and so can't start
  before it.
- **The environment loses about as much.** Beside libkc's update it
  slowed by about 80 ms: a median of 676 ms against 597, over all three
  projects' 36 JVMs per arm. Setup, where the change lands, moved by −16,
  +10 and −74 ms.
- **The totals' differences aren't the change's.** They come from
  processing and round 2, which it doesn't touch.

A cold run on these 4 CPUs is bound by the CPU: the JIT's compiler
threads, IntelliJ's and the processors' class loading, and libkc share
them, so moving work beside other work gains little. Step 59 gained
where its parse ran beside libkc's single-threaded phases and the
round's waits; here the environment's construction was itself busy.
Step 11 had tried the same overlap and dropped it for the same reason:
the session waited 0.2 ms for the update, but cold setup saved only
about 60 ms. Reverted. A cold cut has to remove work, not overlap it:

- a slimmer environment, which the plan's Phase 3 targets;
- fewer classes loaded;
- less parsed.

## Gaps and open questions

- **Slice simplifications (M0.4).** These go when M3/M4 bring the facts in:
  - Accessors read their flags from PSI modifiers, not from `kc`.
  - ~~There are no backing-field symbols, so `@field:` annotations are dropped.~~
    Fixed in step 14.
  - ~~Library type aliases and top-level callables aren't known.~~ Fixed in
    step 6.
  - ~~Java sources are ignored.~~ Fixed in step 15.
  - ~~The PSI join checks each node's PSI class, not yet the whole kind
    sequence.~~ Fixed in step 18.
  - `dropCaches` does nothing. Dropping PsiManager's caches needs IntelliJ
    extension points that the core environment doesn't register, and sources
    don't change between rounds.

- **Stock backend not built here.** The jar leaves out `backend/fir/`,
  `standalone/` and the Java stubs in `src/main/java`, because they need the
  AA implementation (FIR, low-level API, standalone), whose artifacts are not
  reachable from this build host (checked again in step 22).
  - The stock `aa` backend therefore has not been compiled or run against
    2.4.21.
  - Upstream's Gradle build still pins `aaKotlinBaseVersion=2.4.20-dev-2407`.
    Building the patched tree with Gradle needs the 2.4.21 AA artifacts.
- **No serialization plugin.** The fork is compiled without the
  kotlinx-serialization compiler plugin. Nothing needs it: the
  incremental caches (`PersistentMap`) use hand-written `KSerializer`s
  for their keys and values (`FileSerializer`, `SymbolSerializer`), and
  step 16 runs them on kc.
- **Java file manager.** `ResolverAAImpl.javaFileManager` casts the project's
  `JavaFileManager` service to `KotlinCliJavaFileManagerImpl`. The `kc`
  backend gets one from `KotlinCoreEnvironment`, whose setup is part of the
  cold-start cost that M7 measures.
- **KSP member coverage.** The fork's bytecode uses 310 AA members; kc
  answers 204, answers 28 for some of its classes, leaves 4 to stubs, 69
  are the API's own and 5 are of types kc never makes
  (`jvm/kc-aa/members.txt`, `build.sh members`, step 19).
- ~~**Generic annotations' type arguments** (`@A<String>`) aren't in the
  model (step 20).~~ They are (step 23).
- ~~**Aliases inside an alias's expansion** are expanded, not kept as
  abbreviations (step 20).~~ A source typealias's are kept (step 23); a
  class path typealias's right side isn't read.
- **One libkc per daemon.** A JVM lets one class loader load libkc, and
  the KSP plugin makes a class loader per KSP class path, so a daemon runs
  kc for one KSP class path only; another fails with
  `UnsatisfiedLinkError` until the daemon restarts (step 21). Loading a
  copy of the library from another path would load a second Go runtime
  into the process, which is untested; failing over to the `aa` backend
  needs the stock backend (below).
- **Incremental checking.** An edit that changes only bodies checks
  only the edited files (step 25): 25 ms for a one-file edit on
  octi-server, down from 45. Every file is still lowered and has its
  written signatures resolved. An edit that changes declarations checks
  the whole module.
