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
  KSP 2.3.9 with one commit per change. Until 2026-11-11 the fork was a
  patch series in `jvm/ksp/patches`; the patch numbers below are the
  branch's commits in order.
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
1. It checks out the fork at the commit it pins: KSP at tag 2.3.9 with
   the patches below as commits.
2. It compiles the `kotlin-analysis-api` module (the processing loop, the KS*
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
build scripts, which pin the fork's commit; the processors' and stock KSP's jars too.
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
| kc | 14 | `complexAliasing`, `smokeTest`, `typeArgs`, `regression_1513_annotation`, `transitiveAliases` |

- `complexAliasing` fails on both: `DaggerProvider` (an alias of a
  class the test doesn't put on the class path in this setup) is an error
  type, which the processor rejects. Under kctfork the test's class path
  is the test JVM's, which has more on it.
- `smokeTest`, `typeArgs`, `regression_1513_annotation`: a generic
  annotation's type arguments as written, `@A<String>(...)`. kc's model
  has an annotation's class and arguments but not its type arguments
  (the checker only counts them), so `KtTypeReference.type` of `String`
  there is unsupported.
- `transitiveAliases`: an alias whose expansion uses other aliases,
  `typealias Alias41<Z, Q> = (Alias43<Z>) -> Alias47<Q>`, expands to
  `(List<Z>) -> List<Q>`: kc keeps the abbreviation of the alias written
  (`Alias41`) but not those inside its expansion.

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
  reachable from this build host.
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
- **Generic annotations' type arguments** (`@A<String>`) aren't in the
  model (step 20).
- **Aliases inside an alias's expansion** are expanded, not kept as
  abbreviations (step 20).
- **Incremental checking.** A changed Kotlin file re-checks the whole
  module in Go (step 17); only parsing is per file.
