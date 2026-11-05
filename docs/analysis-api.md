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
| M0.4 | Vertical slice: list annotated source classes through a `kc` KspBackend: `jvm/kc-aa/build.sh test` | done, see gaps |
| M1 | `psi` on Kotlin 2.4.21 | done before this work |
| M2 | Error tolerance, `types.Invalid` payload, extension-vs-top-level overload bug, `@Contextual`/`@EncodeDefault` | done (two items moved, see M2) |
| M3 | Richer symbol model: visibility and modality enums, flags, origins, library annotation arguments, Java parameter names | in progress: source types, flags, modality; KaTypes on the JVM |
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
- `directSupertypes` doesn't substitute the type's arguments.
- `getFullyExpandedType` returns its argument, and
  `KcExtras.abbreviatedTypeAlias` returns null: there is no alias
  abbreviation yet.
- `buildClassType` ignores its builder's arguments and nullability.
- Library class types know only their class ID, and a type parameter of a
  library class is a placeholder symbol.
- `isSubtypeOf` and `semanticallyEquals` aren't implemented.

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
- Library annotation classes have no defaults: `memberScope` is empty for a
  library class until library symbols land.
- `evaluateAsAnnotationValue` answers only the expressions kc evaluated as
  annotation arguments or defaults; for any other expression it returns
  null.
- `memberScope` of a source class is its declared members, with no
  inherited members or fake overrides.
- Arguments are ordered by parameter. That may differ from kotlinc's order
  when named arguments are written out of order.
- A position that doesn't line up with the psi (non-ASCII text earlier on
  the line) leaves the value without `sourcePsi`.
- Arguments of annotations on local declarations aren't in the model.

## Gaps and open questions

- **Slice simplifications (M0.4).** These go when M3/M4 bring the facts in:
  - Accessors read their flags from PSI modifiers, not from `kc`.
  - There are no backing-field symbols, so `@field:` annotations are dropped.
  - Library classes are known by class ID only.
  - `buildClassType` doesn't run its builder, and library type aliases
    aren't known.
  - Java sources are ignored.
  - The PSI join checks each node's PSI class, not yet the whole kind
    sequence.
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
  kotlinx-serialization compiler plugin, so its `@Serializable` classes
  (incremental-compilation caches) have no serializers yet. M6 fixes this, or
  turns IC off for the first `kc` runs.
- **Java file manager.** `ResolverAAImpl.javaFileManager` casts the project's
  `JavaFileManager` service to `KotlinCliJavaFileManagerImpl`. The `kc`
  backend gets one from `KotlinCoreEnvironment`, whose setup is part of the
  cold-start cost that M7 measures.
- **KSP member coverage.** KSP2 uses about 308 AA members: `KaSession` 43,
  symbols 157, types 32, components 22, annotations 14, project structure
  10, signatures 6, scopes 4. M0.4 implements the slice's subset (see above).
  M5 tracks coverage through a generated `members.txt`.
