# Plan: Java statics and ambiguity

This plan covers how `kc` scopes Java static members and when it reports a name or call as ambiguous. The goal is to bring both in line with K2, and so cut the diagnostics tests where `kc` disagrees with `kotlinc`.

## Where things stand

The committed baseline (`bench/results/diagnostics.tsv`, at `0f87173`) has 286 *accepts invalid* and 118 *rejects valid* tests. The counts in the request (420 and 178) came from an older baseline. The theme was re-derived from the TSV and the test sources by running the harness on a scratch checkout of v2.4.21. The run, with `-list`, `-errors` and `-marks`, reproduces the baseline exactly: 42 accepts invalid, 13 rejects valid, 1 unsupported.

One of those 42, `scopes/specialFunctionInJava_1.kt`, is about built-in name mapping (`byteValue` vs `toByte`) and falls outside this theme. That leaves 41 accepts invalid, 13 rejects valid and 1 unsupported. Seven of them, listed under [Left to the inference plan](#left-to-the-inference-plan), belong to `docs/plans/inference.md`. **This plan therefore targets 37 accepts invalid, 10 rejects valid and 1 unsupported.**

On the request's note about `check.Library.JavaTypes()`: it is already on. #102 set `JavaTypes: true` in `cmd/kc/main.go:119`, and `analysis/ka/libcache.go:66` sets it too. `j+k/primitiveOverrides/notNullAnnotated.kt` and `triangleWithNotNullType.kt` now *accept valid*. `j+k/primitiveVsBoxedOverloads.kt` is counted *rejects invalid*, but only by accident: `kc` reports an error on the `foo2` line, which expects none, and misses the expected `OVERLOAD_RESOLUTION_AMBIGUITY` on `foo4`. Milestone 6 has to fix both lines together, or the test regresses to *accepts invalid*.

## Failing tests by root cause

Paths are relative to `compiler/testData/diagnostics/`. AI means accepts invalid, RV rejects valid. For RV tests, `kc`'s error is quoted.

### A. Java statics reached through Kotlin classes, or through supertypes that should clash (8 AI)

- `tests/scopes/inheritance/statics/jjkj.kt`, `oneInterfaceManyTimes.kt`: `K.a`, `K.b` and `K.bar()`, where `K` is a Kotlin class extending the Java class `C`. K2 reports `UNRESOLVED_REFERENCE`; `kc` resolves them through `javaField` and `javaStatics`.
- `staticsFromjava.kt`, `staticsFromjavaAfterKotlin.kt`: `X.foo()`, `XN.foo()`, `Y.bar()`, `D.foo()`, where the qualifier is a Kotlin class. The same calls unqualified inside those classes are valid, and `kc` already gets those right.
- `tests/j+k/javaStaticFieldViaKJInheritance.kt`: `MyInterfaceEx.MY_STATIC_FIELD`, where `MyInterfaceEx` is a Kotlin interface extending a Java one.
- `nameClash0.kt`, `nameClash1.kt`, `nameClash2.kt`: `E.field`, where the Java class `E` inherits `field` from two Java interfaces. K2 reports `OVERLOAD_RESOLUTION_AMBIGUITY`. `kc` takes the first field `javaField` meets, breadth-first.
  - `nameClash1` is the subtle case. `B extends A` redeclares `field`, but `E implements A, B` still sees two fields.
  - `oneInterfaceManyTimes` is the counter-case. One field reached along several paths is not ambiguous (`D.a`, `E.a`).

### B. Unqualified statics and supertype companions in a Kotlin class body (2 AI, 3 RV, 1 AI)

- `tests/visibility/javaInterfaceFieldDirectAccess.kt` and `tests/problems/enumEntryFieldShadow.kt` (AI): an interface field of a Java interface that the Kotlin class implements *directly* is not in scope unqualified. `inheritedStatic` finds it anyway, because `javaField` walks interfaces.
- `tests/scopes/inheritance/companionObjectsOrder.kt` (RV): `unresolved reference: foo` for `foo()` inside `class C : A()`, where `foo` is an extension of `A.Companion`. A superclass's companion is an implicit receiver for extension calls too, not only for its members.
- `tests/visibility/packagePrivateStatic.kt` (RV `class Base is not public`) and `packagePrivateStaticViaTypeAlias.kt` (RV `class PackagePrivateGrandparentAbstractClass is not public`): a public static of a package-private Java class, reached through a public subclass, is accessible, both qualified and unqualified.
- `tests/secondaryConstructors/companionObjectScope.kt` (AI, `NONE_APPLICABLE`): in a delegation call, `C` resolves to the nested `object C`, not to the companion's `val C`, so the `+` is inapplicable. `kc` takes the companion property.

### C. Qualifier forms (5 AI)

- `tests/SafeCallOnFakePackage.kt` and `testsWithStdLib/resolve/javaStaticMembers.kt`:
  - `Test?.FOO` and `System?.out` are `NO_COMPANION_OBJECT`, because the receiver of `?.` is an expression, and a class without a companion is not one.
  - `foo?.s`, where `foo` is a package, is `EXPRESSION_EXPECTED_PACKAGE_FOUND`.
  - `kc` treats `?.` on a qualifier like `.` and reports neither diagnostic anywhere.
- `tests/javac/qualifiedExpression/visibleClassVsQualifiedClass.kt` and `tests/qualifiedExpression/PackageVsClass2.kt`:
  - In `package other` with a `class a`, `a.b()` means class `a`'s member `b`, which doesn't exist. `kc` falls back to the package `a` and finds `a.b`.
  - `PackageVsClass2`'s `a.a` is the same mistake.
- `tests/jvm/classpath/prohibitNestedClassesByDollarName.kt`: ``test.`Outer$Nested`()`` and ``test.`JavaOuter$JavaNested`()`` must not find nested classes by their binary names.

### D. Ambiguity between imports at one level (11 AI, 2 RV)

Explicit imports:

- `tests/j+k/properties/staticFieldPropertyOverloads.kt` (`import a.JClass.foo` and `import b.foo`).
- `tests/imports/EnumEntryVsStaticAmbiguity3.kt` (two explicit imports of `SOME`).
- `tests/resolve/ambiguityOnPropertiesWithTheSamePackageName.kt` (two imported extension properties `d`).

In all three, `kc` uses only the first member import (`memberImports`; later ones go to `moreMemberImports`, `java.go:796–800`).

Star imports of class members:

- `EnumEntryVsStaticAmbiguity.kt`, `EnumEntryVsStaticAmbiguity2.kt`, `EnumEntryVsStaticAmbiguity4.kt`: `import first.KtNodeTypes.*` and `import second.SomeEnum.*` both supply `SOME`. `importMembers` (`imports.go:246`) keeps the first and silently skips the rest.

Classifiers from star imports:

- `testsWithStdLib/resolve/ambiguousTypeReferences/typeCase.kt` (`HashMap` from `java.util.*` and `some.*`).
- `qualifierEnumEntryOrCompanionMemberCase2.kt`, `qualifierStaticObjectOrCompanionMemberCase2.kt`, `qualifierOrStaticObjectCase.kt` (three or two `Some` classes).
- `tests/javac/imports/AllUnderImportsAmbiguity.kt` (`x` from `a.*` and `b.*`).

In all of these, `findClassifier1` (`java.go:962`) takes the first package that has the name.

Rejects valid:

- `tests/multimodule/varargConflict.kt` (RV `ambiguous call: 4 candidates for 'foo' take (Int)`): two modules declare the same `p.foo(Int)` and `p.foo(vararg Int)`. K2 drops equivalent candidates (same signature, different modules) and then prefers the non-vararg one.
- `tests/regressions/kt26303.kt` (RV `ambiguous call: 2 candidates for 'foo' take ()`): `42()` with `import foo.foo as invoke` and `import foo.invoke`. The renamed `foo` is not an `operator`, so it isn't a candidate for the invoke convention.

### E. Duplicate Java members and most-specific rules (5 AI, and one test passing by accident)

- `tests/j+k/selectMoreSpecific.kt`: two `foo()` methods in one Java class that differ only in return type, and two such static `bar()`s. `javaMethods` dedups by `paramKey`, keeping the first, so the ambiguity disappears.
- `tests/j+k/brokenCode/fieldDuplicates.kt`: two fields named `foo` in one Java class. `cls.JavaFields` is a map, so one is lost.
- `tests/multimodule/duplicateMethod/differentGenericsInParams.kt`, `classGenericsInParamsIndexMismatch.kt`: `b?.foo(null, null)` after `b is C`, where `B` comes from two modules with different `foo` signatures. The candidates are not equivalent, so K2 reports an ambiguity. `kc`'s `dedupe` merges them.
- `testsWithStdLib/callableReferences/javaStatic.kt`: `foo3(JavaClass::bar)`. Both `foo3` overloads take one of the two `bar`s (static `bar(String)` and instance `bar(CharSequence)`), so the call is ambiguous. `kc` picks one.
- `tests/j+k/primitiveVsBoxedOverloads.kt` (see above).

### F. Assignment operators (4 AI, 2 RV)

- `tests/operatorsOverloading/AssignOperatorAmbiguity.kt`: `var i = 1; i += 34` with an `Any.plusAssign` extension in scope. `compoundOp` is never reached for built-in operand types.
- `plusAssignOnArray.kt`, `AssignOperatorAmbiguityExtension.kt`, `tests/callResolution/exponentialArraySetWithPlusAssign.kt`: `a[i] += v`, where both `a.get(i).plusAssign(v)` and `a.set(i, a.get(i).plus(v))` work. `indexCompound` (`operator.go:313`) takes `plusAssign` whenever it exists and never reports ambiguity.
- `plusAssignOnVarAndCollections.kt` (RV `assignment operators ambiguity: 'minusAssign' and 'minus' both apply`) and `testsWithStdLib/java/intersectFlexibleAndMutable.kt` (RV, the same for `plusAssign`): `var list = ArrayList<Int>(); list -= 2`. `list - 2` is a `List<Int>`, which can't be assigned back to `list`, so only `minusAssign` applies. `compoundOp` (`operator.go:333`) doesn't check that.

### G. References to Java fields and to `entries` (2 RV, 1 unsupported, 1 AI)

- `tests/expresssions/javaFieldCallable.kt` (RV `unresolved reference: staticField`) and `tests/callableReference/property/javaStaticFieldViaImport.kt` (unsupported, `property reference`): `JavaClass::staticField`, `JavaClass::nonStaticField` and `::publicFinal` via a star import.
- `tests/enum/entries/primitiveReference.kt` (RV `unresolved reference: entries`): `Some::entries`.
- `testsWithStdLib/resolve/ambiguousTypeReferences/constructorVsSyntheticValuesPrioritized.kt` (AI): `SomeClass::entries`, where the enum also has a nested `class entries`, is `OVERLOAD_RESOLUTION_AMBIGUITY`.

### H. Java sources naming nested classes inherited through Kotlin classes (1 RV)

- `tests/javac/inheritance/InheritanceWithKotlin.kt` (RV `unresolved reference: javaInner`, `kotlinInner3`): `UseKotlinInner.java extends KotlinClass` names `JavaInner` and `KotlinInner3`. These are nested classes of supertypes two levels up, through Kotlin classes. `internal/javasrc` doesn't resolve them, so the getters' types fail.

### Left to the inference plan

These seven fail on ambiguity diagnostics, but the cause is inference or lambda and callable-reference analysis. They belong in `docs/plans/inference.md`:

- AI `tests/callResolution/kt50386.kt`: two generic `presenter` overloads with a lambda returning a generic class.
- AI `tests/resolve/overloadConflicts/emptyLambdaUnitNoDiscrimination.kt`: an empty lambda must not prefer `() -> Unit` (`byLambdaResult`).
- AI `tests/unsignedTypes/unsignOverloadVsNonIntLike.kt`: after `a as UByte` (`CAST_NEVER_SUCCEEDS`), `a` is `String & UByte`, so both `foo`s apply. Smart cast after an impossible cast.
- AI `tests/secondaryConstructors/generics3.kt`: `super(x, 1)` with `x: T1`, where `T2 : T1` is wanted. This is subtyping of type parameters in delegation calls.
- RV `tests/callableReference/resolve/resolveReferenceAgainstKFunctionAndKPrpoerty.kt` (`ambiguous call: 2 candidates for 'foo' take (lambda)`): explicit type arguments should make only the `KProperty1` overload fit.
- RV `tests/problems/kt24230.kt` (`ambiguous call: 2 candidates for 'bar' take (lambda, Int, lambda)`): a callable reference with a lambda.
- RV `testsWithStdLib/ambiguityWithUnsignedSpecificity.kt` (`ambiguous call: 3 candidates for 'of' take (K)`): an argument whose type variable is not yet inferred, against `ULong`, `Int` and `Long`.

## How kc resolves names today

`kc` has no single scope tower. It resolves through ordered fallbacks inside two functions, picks a level whose candidates "may take" the call, and judges ambiguity only inside that level.

**Simple names (values)** are resolved by `expr1`'s `Ident` case (`expr.go:74–198`), in this order:
1. `entries` inside the enum.
2. Implicit receivers (`recvMember`).
3. `lookupVar` (`expr.go:363`): locals, then program enum entries, then `memberProp`, then `topVar`.
4. Inside a class, a top-level hit first gives way to `receiverProp` and `inheritedStatic` (`object.go:392`).
5. Then `importedMember`, `entries`, built-in companions, and a class used as an object.

`topVar1` (`check.go:1648`) orders explicit imports, file, package, program star imports, library stars and default packages.

**Calls** are resolved by `call`'s `Ident` case (`expr.go:2359–2682`), in this order:
1. Invokable locals.
2. Local functions (`scope.findFunsFor`).
3. Implicit receivers (`recvCall`, `onReceiver`).
4. Inside a class: instance methods, an invokable property, extensions on `this`, `staticInScope` (`object.go:415`) and `objectMethods`.
5. `callableImport` (`imports.go:95`).
6. `topFunsFor` over `topFunLevels` (`check.go:1595`): explicit imports, then package, then program stars.
7. `kotlinFuns` over `kotlinFunLevels` (`java.go:1092`), which takes the first non-empty level.

A level gives way when nothing in it passes `mayTake` (`receiver.go:602`). A few sites trial-resolve and roll back (`expr.go:2558`). Star-imported packages' functions are put into one level and become overloads of each other. Star-imported classifiers and star member imports never conflict: the first hit wins.

**Java statics.** `classTable.load` (`java.go:228`) puts static methods in `cls.Statics` and static fields in `cls.Fields`; instance fields go to `cls.JavaFields`, and enum constants to `Fields` and `Entries`. Interface fields of Java sources are made `public static final` (`javasrc/resolve.go:599`). Kotlin classes from metadata put enum entries and hoisted companion `const`/`@JvmField` fields in `Fields`, and `values`/`valueOf`/`@JvmStatic` methods in `Statics` (`kotlin.go:1517`). Three lookups find them:

- `javaStatics` (`java.go:2010`) walks the `Super` chain only. It skips interfaces, passes through Kotlin classes, and hides by parameter descriptor.
- `javaField` (`java.go:2029`) walks *all* supertypes breadth-first and returns the first hit. It passes through Kotlin classes and never reports a clash.
- `staticFieldOf`, `visibleJavaField` and `hasStatic` (`object.go:351–387`) wrap `javaField`.

These lookups are used in four places:

- `KotlinSub.X`, through `member`'s `staticField` (`expr.go:2122`, `java.go:2609`).
- `KotlinSub.f()`, through `staticCall` (`expr.go:2718`, `java.go:2384`).
- Unqualified fields, through `inheritedStatic`.
- Unqualified calls, through `staticInScope`. This one walks the superclass chain and stops at a program companion that has the name.

The first two both resolve where K2 doesn't.

**Qualified `A.B`** is resolved by `member` (`expr.go:2086`), in this order:
1. `classRef`, for a nested object or companion, unless `hasStatic`.
2. A program package's property.
3. A library package's property.
4. `staticField` on the class.
5. Built-in companion statics.
6. `A` evaluated as a value, so companion members come last.

`classRef` (`java.go:1626`) gives way to a variable named `A` (`isVar`), and to a package. `kc` never reports `NO_COMPANION_OBJECT` or `EXPRESSION_EXPECTED_PACKAGE_FOUND`.

**Most specific.** `resolve1` (`call.go:750`) runs these filters in order:
1. `mapArgs`
2. `applicable`
3. `notPropRefs`/`refsFit`
4. `notLowPriority`
5. `noRefAdaptation`
6. `fewestKotlinJavaSAM`
7. the maximal set by `mostSpecific` (`call.go:2159`)
8. `overriddenGiveWay`
9. `origSAMMaximal`
10. `fewestConversions`
11. `fewestSAM`
12. `nonGenericMaximal`
13. `preferred` (non-vararg, fewer defaults)
14. `byRefShape`/`byLambdaResult`

`resolvePlain` (`call.go:1662`) is the non-generic copy. Both report `ambiguous call` (`call.go:1041`, `1751`). `mostSpecific` has these rules:

- A `boxedPrimitive` rule (`call.go:2428`), under which `Long!` loses to `Long`.
- `numberMoreSpecific` (`call.go:2410`), with no unsigned rule.
- SAM views (`funInterfaceView`, `javaSAMView`).

Candidates are deduplicated in three places:

- `javaMethods` (`java.go:1941`) by parameter key, first wins.
- `dedupe` (`java.go:2064`) by substituted signature.
- Static and top-level candidates are not deduplicated at all.

## The gap against K2

**Static scopes.** In K2, a Java class's static scope is `JavaClassStaticUseSiteScope`. It holds the class's declared static members first. For a name the class doesn't declare, it takes the union of its supertypes' static scopes, deduplicated by symbol, and two distinct symbols are an ambiguity. The supertypes include Java interfaces, for fields. JLS 8.4.8 makes interface static *methods* non-inherited; verify that K2 agrees. A Kotlin class's static scope has only its own: nested classifiers, plus `values`, `valueOf` and `entries` for an enum. Nothing is inherited through a Kotlin class for qualified access. A Java class below a Kotlin class still sees statics above it (`jjkj`'s `D.b`). Groups A and E need this.

**Class-body tower.** For each enclosing class, K2's tower has:
- The implicit receivers `this`, its companion, and the companions of its *superclasses* (`lookupSuperTypes(lookupInterfaces = false)`).
- The static scopes of the class and of its superclasses. Each of those is the scope above, so an interface's field arrives only through a Java superclass that implements it. A directly implemented interface contributes nothing.

Group B needs this, including extension calls on superclass companions (`companionObjectsOrder`).

**Tower levels and ambiguity.** `FirTowerResolver` processes levels in order:
1. Locals.
2. Implicit receivers with their member and extension scopes.
3. Static and classifier scopes.
4. Explicit imports.
5. Package.
6. Star imports.
7. Default imports.

The first level with a *successful* candidate wins, and `ConeOverloadConflictResolver` picks among that level's candidates. Two survivors are `OVERLOAD_RESOLUTION_AMBIGUITY`, whatever import they came from. Classifiers follow the same rule: the star-import scope returns all distinct classifiers of the name, and two of them are an ambiguity. A classifier reached twice is one, and `java.lang` and defaults sit at a lower level. Group D needs this.

**Qualifiers.** K2 resolves the leftmost name of `a.b.c` as a classifier in scope first (local, imported, package), and as a package only if there is none. Once `a` is a classifier, `b` is looked up in its nested classifiers, static scope and companion, with no fallback to a package `a`. The receiver of `?.` is resolved as an expression, so a class without a companion gives `NO_COMPANION_OBJECT` and a package gives `EXPRESSION_EXPECTED_PACKAGE_FOUND`. Names containing `$` don't match nested classes. Group C needs this, and B's `companionObjectScope` needs the rule that a classifier found at a nearer level beats a companion property.

**Most specific.** `ConeOverloadConflictResolver`:
- Compares parameter types (flexible types included), then discriminates generics, then prefers no SAM conversion, no callable-reference adaptation, no vararg and no defaults.
- Before that, `ConeEquivalentCallConflictResolver` drops equivalent candidates, meaning the same signature from different modules. This applies to `varargConflict`, but not to members whose signatures differ (`duplicateMethod/*`).
- Never merges two members declared in one Java class, even when they differ only by return type (`selectMoreSpecific`, `fieldDuplicates`).
- Treats a Java primitive parameter as more specific than its wrapper type (KT-9182, KT-55548), which `primitiveVsBoxedOverloads` checks.

Group E needs this.

**Assignment operators.** K2 resolves `a += b` both as `a.plusAssign(b)` and as `a = a.plus(b)`. The second counts only when `a` is assignable and the result's type is a subtype of `a`'s type. If both succeed, it reports `ASSIGN_OPERATOR_AMBIGUITY`. `a[i] += b` is handled the same way, as `a.get(i).plusAssign(b)` against `a.set(i, a.get(i).plus(b))`. Group F needs this.

## Milestones

Each milestone lands on its own, with `-baseline` reporting 0 regressions on both `bench/results/diagnostics.tsv` and `box.tsv`, and commits the new baselines. They are ordered cheap and high-yield first. For each, *guards* are passing tests in the same area to watch.

### 1. Qualifier forms (C: 5 AI). Small.

- Where `member`/`call` see `x.X` under a safe call (`?.`): if the receiver is a class or object qualifier and the class has no companion, report `NO_COMPANION_OBJECT`. If it is a package, report `expression expected, package found`.
- In `classRef` and `packageRef` resolution, a classifier visible by the simple name wins over a package of that name. Once it wins, a missing member is unresolved, with no fallback to the package.
- Reject `$` in a class name looked up from Kotlin (`findClassifier1` and nested lookup), unless a class is declared with that exact name.

Files: `internal/check/expr.go` (`member`, `call` Member case), `java.go` (`classRef`, `findClassifier1`).

Guards: `tests/qualifiedExpression/PackageVsClass*.kt`, `tests/imports/PackageVsClass.kt`, `javac/qualifiedExpression/*`.

Risk: real code with a class and a package of one name; this is K2's rule, so it is correct, and the box suite will show it.

**Done.** All 5 flipped, with 0 regressions in diagnostics and box. Notes on what landed:
- `classRef` resolves a head name that is a classifier in scope through that classifier first. `packageRef` needed no change.
- A classifier found only by a default import (`java.lang.Package`) still gives way to a root package of the same name. Box's `innerNested/nestedInPackage.kt` (`Package.Outer.Nested()`) needs this.
- `$` names are rejected in `classRef` (`binaryNestedName`) when the part before `$` is a class on the class path.
- Unit test: `testdata/errors/qualifierForms/`.

### 2. Java static scopes, qualified (A: 8 AI). Medium.

Add `javaStaticScope(cls, name)`, modeled on `JavaClassStaticUseSiteScope`:
- A Kotlin class gives only its own `Fields` and `Statics`: enum entries, `values`, `valueOf`, hoisted companion fields.
- A Java class gives its declared statics. If it declares none of that name, it gives the union over its direct supertypes' static scopes, deduplicated by symbol.
  - For a Kotlin supertype, the union recurses through it to the Java classes above it. This is what `jjkj`'s `D.b` needs; confirm the exact K2 recursion on that test.
  - Interfaces contribute fields only.
- It returns every distinct field, so the caller can report ambiguity.

Use it in `staticField`, `hasStatic`, `staticFieldOf` and `staticCall` in place of `javaField` and `javaStatics` for qualified access. Report `OVERLOAD_RESOLUTION_AMBIGUITY` (`kc` message: "ambiguous call…", or a field-specific one) when a field resolves to more than one symbol.

Files: `internal/check/java.go`, `object.go`, `expr.go`.

Guards: `scopes/inheritance/statics/*` (`overloadStatic`, `staticVsCompanion`, `staticVsMember`, `staticPropertyVsImport`, `kjk`, `hidePrivateByPublic`), `ambiguousTypeReferences/companionVsStaticScope.kt`.

Risk:
- Real code that wrote `KotlinSub.JAVA_STATIC` stops compiling, as it does with `kotlinc`.
- Android (`Activity.RESULT_OK` through Kotlin subclasses) needs checking, and so do `analysis/ka` and KSP callers that resolve statics through `classTable`.
- Memoize the scope per class and name, since this sits on a hot path.

**Done.** All 8 flipped, with 0 regressions in diagnostics and box. Notes on what landed:
- `staticFields` (`object.go`) is the static scope for fields. `staticField` reports two distinct visible fields as an overload resolution ambiguity.
- `javaStatics` keeps its walk up the superclass chain only for a Java qualifier, and skips Kotlin classes on the way. Interface static methods were never collected, so nothing changed there.
- `inheritedStatic` (unqualified fields) still uses `javaField`, until milestone 4.
- `staticFields` isn't memoized. The diagnostics suite's time didn't change, so it was left out.
- Unit test: `testdata/errors/javaStaticScopes/`.

### 3. Imports at one level (D: 11 AI, 2 RV). Medium.

- Explicit member imports: resolve every import of the name (`memberImports` plus `moreMemberImports`) as candidates of one level. For values, two distinct survivors are an ambiguity. For calls, they go through `resolve1` together.
- Star member imports: `importMembers` keeps all owners per name rather than the first. A name supplied by two distinct symbols is ambiguous on use.
- Star classifier imports: `findClassifier1` collects the distinct classifiers from all user star imports at that level, and two of them are an ambiguity, reported at the type or qualifier. The library's `c.stars` (the file's own package on the class path) must stay at package level, not star level, or same-package classes from the class path will falsely clash with star imports. Today `resolveImports` adds them to `c.stars` (`java.go:649`), which this step must separate.
- Equivalent candidates: before `mostSpecific`, drop top-level candidates with the same owner FQ name and signature from different modules (`varargConflict`).
- Invoke convention: only `operator` functions (after renaming) are candidates (`kt26303`).

Files: `internal/check/imports.go`, `java.go` (`resolveImports`, `findClassifier1`, `kotlinFunLevels`), `check.go` (`topVar1`, `topFunLevels`), `call.go`.

Guards: `tests/imports/*` (37 tests pass today), `ambiguousTypeReferences/*Case.kt` that pass, `testsWithStdLib/RenameOnImportHidesDefaultImport.kt`.

Risk:
- Real projects star-import packages with clashing class names (`java.util.*` with `java.awt.*` and `List`). Under K2 these fail only on use, and the same must hold here.
- Performance of collecting from all stars: this happens only on a miss in the explicit and package levels, so it should be cheap.

**Done.** All 13 flipped, plus 2 more *accepts invalid* tests (`protectedGetterWithPublicSetter`, `protectedWithGenericsInDifferentPackage`) that had failed on the same Java parse error as `EnumEntryVsStaticAmbiguity4`. 0 regressions in diagnostics and box. Notes on what landed:
- `importAmbiguity` (`imports.go`) counts the distinct values an imported name stands for: explicit imports first, then star member imports only if the explicit level gave none. Two are an overload resolution ambiguity. `importMembers` now keeps star imports of the same name from other owners.
- `starimports.go` collects the visible classifiers that star imports give a name, after the explicit imports, the file's package (program and class path), local and nested classes. Two are an ambiguity in a type, and as a qualifier when more than one is a value, or none is and no default import has the name. Class literals and unbound callable references don't report it (`starFallback`), as K2 resolves them past the ambiguity.
- `equivalentFun` drops top-level functions with the same name, descriptor and package from different modules (`varargConflict`).
- For the invoke convention, non-operator candidates give way to operator ones (`kt26303`).
- Two imported extension properties with the same receiver type are ambiguous.
- `internal/javasrc` accepts a missing `;` after the package declaration, as Kotlin's Java parser does.
- Unit test: `testdata/errors/importAmbiguity/`.

### 4. Unqualified statics and supertype companions (B: 3 AI, 3 RV). Medium.

- `inheritedStatic` and `staticInScope` take, for each enclosing class, its own statics and those of its superclass chain. For each Java superclass they use `javaStaticScope` from milestone 2, and never a directly implemented interface.
- `scopeObjects`' superclass companions also serve as extension receivers in `recvCall`/`thisExtension`, innermost first (`companionObjectsOrder`).
- Visibility: a public static reached through a public subclass is accessible even when its declaring class is package-private (`packagePrivateStatic*`). This needs the accessibility check in `staticField` and `staticCall` to use the qualifier's class, not the owner's.
- A nested classifier found at the class's static level beats a companion property of the same name (`companionObjectScope`).

Files: `internal/check/object.go`, `receiver.go`, `expr.go`.

Depends on milestone 2.

Guards: `staticsFromjava` (unqualified parts), `scopes/inheritance/statics/localVsStatic.kt`, `staticVsOuter.kt`, `visibility/packagePrivateStatic*` that pass.

Risk:
- Kotlin code that uses an implemented Java interface's constant unqualified is rejected, as with `kotlinc`. This is common in old Android code, but K2 rejects it too.

**Done.** All 6 flipped, with 0 regressions in diagnostics and box. Notes on what landed:
- `inheritedStatic` walks the superclass chain of each enclosing class, and stops at the first Java class, whose static scope (`staticFields`) has all those above it. A directly implemented Java interface adds nothing. The field is checked as if qualified by that class, so a field reached through two supertypes is ambiguous here too.
- `staticInScope` returns the first Java superclass rather than the class that declares the method, so a public static of a package-private class is accessible through a public subclass.
- `objectExtension` makes the objects in scope, superclass companions included, receivers of extension calls, innermost first, after their own methods. As for `this`'s extensions, a top-level function of an extension's package comes first (`sameScopeTopFun`), so `run { }` in a class with a companion stays `kotlin.run`. The box suite's `comanionObjectFieldVsClassField` caught that.
- In a delegation call's arguments, a nested object comes before a companion property of the same name. Elsewhere in a class body, the companion is an implicit receiver and its property still wins, so the rule is limited to delegation calls.
- Unit test: `testdata/errors/classStaticScope/`.

### 5. Duplicate Java members and most-specific rules (E: 5 AI). Medium.

- Keep both of two members of one Java class that share parameters, whether methods or fields. `JavaFields` becomes per-name slices; this is mostly `java.go` load code. Restrict `javaMethods`' `paramKey` dedup and `dedupe` to members from *different* classes where one overrides the other.
- `dedupe` merges only candidates whose substituted signatures are equal (`duplicateMethod/*`).
- Callable-reference arguments: when more than one outer candidate stays applicable, report the ambiguity instead of choosing (`javaStatic.kt`). This needs `refsFit` and `byRefShape` to compare the reference's own resolution results. If it reduces to postponed-argument inference, it moves to the inference plan.

Files: `internal/check/java.go`, `call.go`, `class.go` (`memberCallees`).

Guards: `j+k/*` overload tests (`primitiveVsBoxedOverloadAndSecondParameter.kt`), `callableReference/resolve/*`.

Risk: dropping dedup can create false ambiguities between a Java method and its override seen twice through diamond interfaces. The box suite is the main check.

**Done.** All 5 flipped, plus the *rejects valid* `duplicateMethod/substitutedGenericInParams.kt`, with 0 regressions in diagnostics and box. Notes on what landed:
- `javaMethods` and `javaStatics` keep two methods of one Java class with the same parameters. A subclass's method still hides its superclass's. `resolve1` finds the two equally specific and reports the call ambiguous.
- A Java class's second field of a name sets `Ambiguous` on the first (`types.VarSym`), and a use of it is an overload resolution ambiguity, qualified or not. This is simpler than making `JavaFields` slices.
- `memberRefShapes` adds the shapes of a Java class's static methods, so `foo3(JavaClass::bar)` has two candidates that fit.
- `notHidden` keeps a supertype's method, compiled by Kotlin, that a subclass's method hides by erasure but doesn't override, because their substituted parameter types differ (`sameParams`). `overriddenGiveWay` lets it stand too (`notOverridden`). Only a class of one name from two modules gives this.
- The two `duplicateMethod` tests also needed a harness fix. `bench/conformance/compile.go` put every earlier module's output on the class path in file order, so a module never saw the copy of a class from the module it names first. It now puts the declared dependencies first, in the order the `// MODULE:` line names them, then the rest as before (`depsFirst`). That also fixed `substitutedGenericInParams`.
- Unit test: `testdata/errors/javaDuplicates/`. The two-module cases are covered only by the diagnostics suite, since `TestErrors` compiles one module.

### 6. Java primitive versus boxed (fixes `primitiveVsBoxedOverloads` properly). Small.

In `mostSpecific`, a Java `int` parameter is more specific than a Java `Integer` one, enhanced or not. A `long` against an `Integer` stays incomparable (`foo4`). Land this with the test's two lines both right, so it moves from "rejects invalid elsewhere" to "on an expected line" and never passes through *accepts invalid*.

Files: `internal/check/call.go` (`boxedPrimitive`, `mostSpecific`).

Guards: `j+k/primitiveOverrides/*`, which must keep *accepting valid* with JavaTypes on.

### 7. Assignment operators (F: 4 AI, 2 RV). Medium.

- `compoundOp`: resolve both forms. The `op` form counts only if the target is mutable and the op's result type is a subtype of the target's type. Report ambiguity only if both succeed.
- Drop the `builtinOperand` shortcut when an extension `plusAssign` is in scope.
- `indexCompound`: the same, with `get(i).plusAssign(v)` against `set(i, get(i).plus(v))`.

Files: `internal/check/operator.go`.

Risk: `exponentialArraySetWithPlusAssign.kt` nests `+=` in lambdas. Resolving both forms naively re-checks each lambda twice per level, which is exponential, so check the probe without analysing the lambda body (as `hasFun` does now) and analyse it once for the chosen form. Box tests with `+=` on collections are the regression guard.

### 8. Java sources naming inherited nested classes (H: 1 RV). Small.

In `internal/javasrc`, simple type names in a class body also search the member types of all supertypes, including Kotlin program classes and their supertypes, following JLS 6.4.1 and 8.5.

Files: `internal/javasrc/resolve.go`.

Guards: `tests/javac/inheritance/*`.

### 9. References to Java fields and to `entries` (G: 2 RV, 1 unsupported, 1 AI). Large.

- `JavaClass::staticField` is a `KProperty0`/`KMutableProperty0`.
- `JavaClass::instanceField` is a `KProperty1`.
- `::field` via a star import, with visibility (`INVISIBLE_REFERENCE` for private).
- `Some::entries`.
- A nested class named `entries` against synthetic `entries` is an ambiguity.

Needs checking in `internal/check/reflect.go` and code generation of the reference classes, as `kotlinc` writes them (`PropertyReference0Impl` over a field).

Risk: box-suite codegen; checking can't land without it, since the box suite would start rejecting.

Expected total: 37 AI and 10 RV flipped, and 1 unsupported supported. The `-marks` hit count rises on the same lines.

## Validation

- **Per milestone**, run the theme list and the affected directories:
  - `bench/kotlin-tests.sh diagnostics -list <theme list> -o "" -tsv ""`
  - `-filter tests/scopes/`, `-filter tests/imports/`, `-filter testsWithStdLib/resolve/`, `-filter tests/j+k/`
  - With `-marks`, so expected lines are checked, not only categories.

  Then run the full suite with `-baseline bench/results/diagnostics.tsv`, and write the new baseline.
- **Unit tests**: add `testdata/errors/` modules (Java and Kotlin, as `testdata/errors/javaCycle/` does) for each rule:
  - statics through Kotlin classes
  - interface field clashes, and one field seen along two paths
  - directly implemented interfaces
  - two star imports
  - explicit import clashes
  - safe call on a qualifier
  - `+=` both ways
  - Java primitive vs boxed

  Each line carries `// ERROR <substring>`, and `TestErrors` (`internal/driver/driver_test.go:530`) runs them. Add valid counterparts to `testdata/` so a rule that over-reports shows up.
- **Box suite**: `bench/kotlin-tests.sh box -baseline bench/results/box.tsv -o "" -tsv ""`, 0 regressions.
- **analysis/ka**: `go test ./analysis/...`, because KSP resolves Java statics and imports through the same `Library`, and `go test ./...`.
- **Performance**: milestones 2 to 4 sit on name lookup. Compare `go test -bench . ./analysis/ka` (`bench_test.go`) and the suite timings in `docs/conformance.md` before and after. Memoize `javaStaticScope` per class and name, and the star-import classifier sets per file.

## docs/unsupported.md

The theme is about checking: `kc` accepts code `kotlinc` rejects, or rejects valid code in a few corners. It is not missing features, except Java field references (milestone 9). Until the milestones land, add these to *Differences from kotlinc*, under **Checking**:

- Java statics resolve through Kotlin classes (`KotlinSub.STATIC`) and through directly implemented Java interfaces.
- Two star imports, or two explicit imports, that supply one name don't make it ambiguous: the first wins.
- A static field inherited from two Java interfaces isn't ambiguous.
- `x op= v` is not ambiguous for built-in types or indexed targets, and doesn't check that `x op v` can be assigned back.

Remove each entry with the milestone that fixes it. The existing language list should name callable references to Java fields, which aren't supported (`javaStaticFieldViaImport.kt`).
