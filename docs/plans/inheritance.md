# Plan: checking inherited members as K2 does

This plan covers the diagnostics tests that fail because `kc` doesn't
check the members a class inherits as K2 does. That includes Java
supertypes, the JDK classes that Kotlin maps to its own types, and
Kotlin-only rules about intersections, delegation and default values.

**Status.** M0 to M11 have landed, each with no regressions in the
diagnostics and box baselines: *accepts invalid* went from 383 to 286
and *rejects valid* from 131 to 118. Left out, with the reason:

- M8: `kjkimplicitTypesCrash` (a K2 quirk), `classObjectCopiedField`,
  and the stray errors of
  `supertypeDifferentParameterNullability.kt` in `typeEnhancement`.
- M9: `notImplementedMembersAroundJava`, and
  `missingTypeOfParameterInFakeOverride`, which needs several modules.
- M10: `containsAll.kt`, which expects the standard library's
  `containsAll` extension to be missing, as it is without
  `WITH_STDLIB`; the harness always gives `kc` the standard library.
  `specialFunctionInJava_1.kt` stops at its Java source's missing `;`,
  which `kc` reports and K2 doesn't.
- M11: the purely implemented collections' members take their Kotlin
  parameter types, but their supertypes keep Java's flexible arguments.
  `kc` infers a nullable type argument from a nullable argument of a
  flexible parameter even where the expected type has a non-null one,
  an inference difference, and the flexible arguments make up for it. `wrongTypeParametersCount.kt` puts an import before its first
  `// FILE:`, which the harness drops.

The sections below are the plan as written, with its numbers from
before the work.

The numbers come from Kotlin 2.4.21's diagnostics suite, run on this
branch's `kc` with `bench/kotlin-tests.sh diagnostics`. That run
reproduces the committed `bench/results/diagnostics.tsv` exactly:
383 *accepts invalid*, 131 *rejects valid*, 0 regressions. An earlier
note counted 420 and 178; those numbers predate the current baseline.
Line-level data comes from a scratch build of `bench/conformance` that
also writes each expected mark, whether `kc` reported an error on its
line, and every error `kc` reported. Milestone 0 proposes landing that
as a harness flag.

## 1. The failing tests

The bucket was re-derived from the TSV keys, which list every error a
test expects, and from the test sources. A test is in the theme if at
least one of these holds:

- it expects one of the inheritance diagnostics below;
- it is under a directory about mapped JDK classes;
- it is *rejects valid*, and `kc`'s first error is about overrides,
  inheritance or JVM signature clashes.

| | Tests |
|:---|---:|
| *Accepts invalid*, every expected error is an inheritance one | 70 |
| *Accepts invalid*, mixed with other errors | 2 |
| *Accepts invalid*, under the mapped-JDK directories, expecting other errors | 11 |
| *Rejects valid*, inheritance or clash error first | 15 |
| **Total** | **98** |

Four groups of tests look related but are left out:

- `backingField/FieldAsParam.kt`, `backingField/FieldShadow.kt` and
  `problems/kt29853.kt`: "property must be initialized".
- `suppress/oneWarning/suppressOnDeclarationItself.kt`: `@Suppress` of
  an error.
- `j+k/syntheticPropertyOverridden2.kt`: expects
  `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM`.
- The seven `platformTypes/genericVarianceViolation/*` tests
  (`JAVA_TYPE_MISMATCH`): these are call checks.

When a test is *accepts invalid*, `kc` reported no error at all. Its
"kc output" is therefore "compiles" for every such test below.

### By root cause

**A. No intersection of inherited members.** These tests are Kotlin
only, apart from three that are marked. kc checks each supertype's
members on their own. It never forms the set of members that one
signature inherits from several supertypes, so it can't check that set
as a whole.

- `MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES(_WHEN_NO_EXPLICIT_OVERRIDE)`
  (8 tests):
  - `multipleDefaultsIncorrectlyAllowed1.kt`
  - `multipleDefaultsAnonymousObject.kt`
  - `multipleDefaultsNoSource.kt`
  - `missingMultipleDefaultsOnTransitiveInheritance2.kt`
  - `moreThanOneOverriddenMissingK2.kt`
  - `override/MultipleDefaultParametersInSupertypes.kt`
  - `override/MultipleDefaultsAndNamesInSupertypes.kt`
  - `override/MultipleDefaultsAndSubstitutionOverrides.kt`

  Example: `class Impl : A, B { override fun foo(a: String) = a }`,
  where both `A.foo` and `B.foo` give `a` a default value. kotlinc
  marks `a: String`; kc compiles the file.
- `CANNOT_WEAKEN_ACCESS_PRIVILEGE`: kc reports 1 of the 59 expected
  lines. Four tests fail only on this diagnostic:
  - `scopes/kt1248.kt`: `internal override val` of a public member.
  - `override/kt4785.kt`: `class E : C(), T`, where `C.foo` is protected
    and `T.foo` is public.
  - `intersectedVisibilitiesAccessorDifferences_Small.kt` (12 lines).
  - `intersectedVisibilitiesForNonAbstractPropertyAccessors.kt`.

  A fifth, `varOverriddenByValThroughIntersection.kt`, is mixed. 36
  further lines are in tests both compilers reject, for example
  `intersectedVisibilitiesAccessorDifferences_Huge.kt` (24 lines) and
  `PrivateSetterForOverridden.kt` (7).
- `MANY_INTERFACES_MEMBER_NOT_IMPLEMENTED`:
  - `override/ParentInheritsManyImplementations.kt`
  - `regressions/kt302.kt`
  - `testsWithStdLib/coroutines/mixingSuspendability.kt` (mixed)
- `MANY_IMPL_MEMBER_NOT_IMPLEMENTED`:
  - `override/FakeOverrideDifferentDeclarationSignatures.kt`
  - `testsWithStdLib/purelyImplementedCollection/intersectionOfPrimitiveBasedFunctions.kt`
  - Java: `j+k/manyImplNotImplementedComplexHierarchy.kt`,
    `j+k/manyImplNotImplementedComplexHierarchy2.kt` and
    `j+k/primitiveOverrides/triangleWithFlexibleTypeAndSubstitution4.kt`.
- `RETURN_TYPE_MISMATCH_ON_INHERITANCE`:
  - `override/kt4763.kt`
  - `override/clashesOnInheritance/returnTypeMismatch.kt`
  - `override/clashesOnInheritance/flexibleReturnTypeList.kt`
  - `override/clashesOnInheritance/kt13355viaJava.kt`, which needs
    `@NotNull` (see C)
- `*_BY_DELEGATION`:
  - `delegation/covariantOverrides/{simple,fromClass,irrelevant}.kt`
    expect `RETURN_TYPE_MISMATCH_BY_DELEGATION` and
    `PROPERTY_TYPE_MISMATCH_BY_DELEGATION`.
  - `delegation/clashes/varOverriddenByVal.kt` expects
    `VAR_OVERRIDDEN_BY_VAL_BY_DELEGATION`.
- `ABSTRACT_SUPER_CALL` through an intersection:
  - `missingAbstractSuperCall.kt`
  - `missingAbstractSuperCallWithClass.kt`
  - `missingAbstractSuperCallWithObject.kt`

  An abstract class member and an interface's default make an abstract
  intersection, so `super.check()` is an error. kc resolves the call to
  the interface's body. The *rejects valid* test
  `override/kt53408.kt` has the opposite problem: kc reports "many
  supertypes available" where K2 resolves `super.f()`.
- `ABSTRACT_(CLASS_)MEMBER_NOT_IMPLEMENTED`:
  - `override/kt12358.kt`: `abstract override fun toString()` in a
    superclass, not re-implemented below it.
  - `override/ProtectedAndPrivateFromSupertypes.kt`: a private property
    of a superclass doesn't implement an interface's.
- Data classes:
  - `dataClassWithInheritedComponent.kt` (`DATA_CLASS_OVERRIDE_CONFLICT`)
  - `dataClasses/dataClassOverridingCopy.kt` (`DATA_CLASS_OVERRIDE_DEFAULT_VALUES`)
- `METHOD_OF_ANY_IMPLEMENTED_IN_INTERFACE`:
  - `java8Overrides/implementingMethodOfAny.kt`
  - `java8Overrides/overridingMethodOfAnyChain.kt`
  - `java8Overrides/overridingMethodOfAnyDiamond.kt`

  This check doesn't need intersections, but it lives in the same
  family of checkers. In `diagnostics/methodOfAnyImplementedInInterface.kt`,
  kc reports four "platform declaration clash" errors on lines that
  expect this diagnostic instead.

**B. Override checks that compare against the whole hierarchy, or that
compare the wrong things.** These are the *rejects valid* tests.

- `extraCheckers/kt69190.kt` and
  `smartCasts/intersectionScope/propertiesConflict.kt`. kc says "type of
  'a' doesn't match the type of the overridden var-property", but the
  overridden property is a `val`.
  - Cause: `checkInheritance` (`internal/check/class.go`) demands an
    identical type whenever the *overriding* property is a `var`,
    whatever the overridden one is.
  - It also compares against every property of that name in every
    supertype (`superProps` walks the whole hierarchy), where K2
    compares only against the members the override directly overrides.
- `genericReceiverPropertyOverride.kt` and
  `generics/finalUpperBoundWithOverride.kt`: kc says "'x' overrides
  nothing" for generic member extension properties
  (`override val <T : M> T.x`). `checkExtPropOverrides` doesn't match
  type parameters by position, as `overrideSubst` does for functions.
- `visibility/notOverridingPackagePrivate.kt`: kc says "hides member of
  supertype". The Java member is package-private and in another
  package, so it isn't visible and can't be overridden.
- `platformTypes/methodTypeParameterDefaultBound.kt`: kc reports
  "accidental override (foo()V)". A Java `<V> void foo()` overrides
  Kotlin's `<V : Any> foo()`, and K2 doesn't compare the bounds of a
  Java member's type parameters.

**C. Java override matching and enhanced nullability.** kc gives Java
declarations platform types, and the kc CLI doesn't read
`@NotNull`/`@Nullable` (`check.Library.JavaTypes`, set only by
`analysis/ka`). These tests need the annotations read, and overrides
matched as K2's `JavaOverrideChecker` matches them:

- `platformTypes/typeEnhancement/supertypeDifferentReturnNullability.kt`
- `platformTypes/typeEnhancement/supertypeDifferentParameterNullability.kt`
- `platformTypes/typeEnhancement/saveAnnotationAfterSubstitution.kt`
- `jvm/duplicateJvmSignature/javaInterop/supertypeDifferentParameterNullability.kt`
- `override/clashesOnInheritance/kt13355viaJava.kt`
- `rawTypes/overrideRawTypedJavaMethodWithGenericTyped.kt`: a raw
  override in the Java class; no annotations needed.

An earlier session found that turning `JavaTypes` on in the CLI, with
`org.jetbrains:annotations:13.0` on the harness class path, fixes 36
tests and breaks 4. Running a scratch build with `JavaTypes` on gives
these errors for the 4 (all compile today):

| Test | kc with `JavaTypes` | Cause |
|:---|:---|:---|
| `j+k/primitiveOverrides/notNullAnnotated.kt` | `ambiguous call: 2 candidates for 'foo' take (Int)`, and the same for `bar` | `B.foo(@NotNull Integer)` and `A.foo(int)` both become `foo(Int)`; kc keeps both as overloads |
| `j+k/primitiveOverrides/triangleWithNotNullType.kt` | `ambiguous call: 2 candidates for 'foo' take (Int)` | as above |
| `j+k/signatureAnnotations/notNullVarargOverride.kt` | `'loadCache' overrides nothing`; `type mismatch: inferred type is Array<out Any?> but Array<out Any> was expected` | `enhance` makes the vararg's elements non-null. The annotation, which has no `TYPE_USE` target, applies to the array only. Override matching also compares enhanced nullability. |
| `j+k/signatureAnnotations/nullableVarargOverride.kt` | `'loadCache' overrides nothing` | override matching compares enhanced nullability |

**D. JVM signature clashes with Java and library supertypes.**
`inheritedMembers` (`internal/check/jvmclash.go`) skips every
`External` supertype. Clashes with Java members, including Java
statics, are therefore found only for property getters
(`javaGetterClashes`) and for `Object`'s final methods.

- `ACCIDENTAL_OVERRIDE` (11 tests):
  - Java statics:
    - `jvm/duplicateJvmSignature/statics/kotlinClassExtendsJavaClass.kt`
    - `jvm/duplicateJvmSignature/statics/kotlinClassExtendsJavaClassExtendsJavaClass.kt`
    - `jvm/duplicateJvmSignature/statics/jkjk.kt`
    - `jvm/duplicateJvmSignature/jvmStatic/jk.kt`
    - `jvm/duplicateJvmSignature/jvmStatic/jjk.kt`
    - `jvm/duplicateJvmSignature/jvmStatic/jkjk.kt`
  - Context parameters against Java parameters:
    - `jvm/contextParameters/valueAndContextParametersWithJava.kt`
    - `jvm/contextParameters/intersectionContextValueJava.kt`
    - `jvm/contextParameters/intersectionExtensionValueContextJava.kt`
    - The last two also expect `CONFLICTING_INHERITED_JVM_DECLARATIONS`.
  - `j+k/kjkimplicitTypesCrash.kt`: a Kotlin property over a Java
    getter that itself overrides a Kotlin property.
- `jvm/duplicateJvmSignature/specialNames/classObjectCopiedField.kt`
  (`CONFLICTING_JVM_DECLARATIONS`): a companion's property field is
  copied to the outer class, where it meets ``val `X$1` ``.
- Rejects valid: `contextParameters/overrides/KJKHierarchyWithOverride.kt`.
  kc reports "inherited platform declarations clash" and "is not
  abstract and does not implement 'foo'". A Java class in the middle of
  the hierarchy overrides both inherited members, so the clash and the
  missing members don't exist.

**E. Java class structure.**

- `testsWithJava17/sealedClasses/kotlinInheritsJavaClass.kt` and
  `kotlinInheritsJavaInterface.kt` (`CLASS_INHERITS_JAVA_SEALED_CLASS`):
  `internal/javasrc` parses `sealed` and `permits`, then drops them.
- `j+k/invisibleFake.kt` and `invisibleFake2.kt`
  (`JAVA_CLASS_INHERITS_KT_PRIVATE_CLASS`): this is reported at a call
  on a Java class that extends a private Kotlin class.
- `j+k/missingTypeOfParameterInFakeOverride_2.kt` and
  `annotations/rendering/notImplementedMembersAroundJava.kt`
  (`ABSTRACT_CLASS_MEMBER_NOT_IMPLEMENTED`). In the second, a Java class
  re-declares `getMessage()` abstract over `Throwable.message`.
- `multimodule/duplicateSuper/sameSuperTraitDifferentBounds.kt`
  (`NOTHING_TO_OVERRIDE` across two versions of a supertype).

**F. JDK classes that Kotlin maps to its own types** (K2's
`JvmMappedScope`, renamed members, predefined enhancement and purely
implemented collections):

- Hidden or renamed JDK members:
  - `scopes/specialFunctionInJava_1.kt`: `some.byteValue()` must be
    `UNRESOLVED_REFERENCE`, because `Number.byteValue` is `toByte`.
  - `targetedBuiltIns/backwardCompatibility/fillInStackTrace.kt`,
    *rejects valid*. kc says "hides member of supertype 'Throwable'";
    K2 doesn't show `fillInStackTrace` on `kotlin.Throwable`.
  - `testsWithStdLib/targetedBuiltIns/blackListed.kt` and
    `j+k/collectionOverrides/nonDirectHiddenOverride.kt`:
    `MutableList.sort` is `DEPRECATION_ERROR`.
  - `j+k/collectionOverrides/irrelevantImplCharSequenceKotlin.kt`:
    `charAt` and `length()` inherited from an unrelated class don't
    implement `CharSequence.get` and `length`.
  - `j+k/collectionOverrides/containsAll.kt` (`ARGUMENT_TYPE_MISMATCH`
    through the mapped `containsAll`).
  - `j+k/collectionOverrides/mapMethodsImplementedInJava.kt`, *rejects
    valid*. kc says "does not implement abstract member 'remove'/'get'";
    the Java class's `remove(Object)` and `get(Object)` implement them.
  - `testsWithStdLib/purelyImplementedCollection/wrongTypeParametersCount.kt`,
    *rejects valid*: "unresolved reference: add".
- Final Java getters overridden as Kotlin properties
  (`OVERRIDING_FINAL_MEMBER`):
  - `j+k/finalCollectionSize.kt`: a final `size()` over
    `ArrayList.size`.
  - `j+k/properties/val.kt`, `var.kt` and `isName.kt`: a final Java
    getter that overrides a Kotlin property.
- Predefined enhancement of JDK members:
  - `j+k/types/newListMethodsNullabilityOverridesLinkedListLatest.kt`
  - `testsWithJava21/newListMethodsNullabilityOverridesLatest.kt`
  - `testsWithJava21/newListMethodsNullabilityOverridesLinkedListLatest.kt`
  - These three expect `NOTHING_TO_OVERRIDE` and
    `RETURN_TYPE_MISMATCH_ON_OVERRIDE` on `addFirst`/`removeFirst`
    overridden with `E?`.
  - `j+k/types/newListMethodsNullabilityLinkedListLatest.kt` and
    `testsWithJava21/newListMethodsNullabilityLinkedListLatest.kt`
    expect `NULL_FOR_NONNULL_TYPE`.
  - `targetedBuiltIns/concurrentMapRemove.kt`
- Purely implemented collections: in
  `testsWithStdLib/purelyImplementedCollection/arrayList.kt` and
  `sets.kt`, `ArrayList<String>()` is a `MutableList<String>` whose
  element type isn't flexible, so `x.add(null)` is
  `NULL_FOR_NONNULL_TYPE`.

Tests under these directories that both compilers reject also miss
lines, for example `targetedBuiltIns/forEachRemainingNullability.kt`
(3 `NOTHING_TO_OVERRIDE`).

### Line-level gaps in tests both compilers reject

| Diagnostic | Expected lines | kc hits | Missed |
|:---|---:|---:|---:|
| `CONFLICTING_JVM_DECLARATIONS` | 996 | 518 | 478 |
| `NOTHING_TO_OVERRIDE` | 93 | 64 | 29 |
| `CANNOT_WEAKEN_ACCESS_PRIVILEGE` | 59 | 1 | 58 |
| `ACCIDENTAL_OVERRIDE` | 56 | 39 | 17 |
| `RETURN_TYPE_MISMATCH_ON_OVERRIDE` | 41 | 20 | 21 |
| `MANY_IMPL_MEMBER_NOT_IMPLEMENTED` | 35 | 20 | 15 |
| `RETURN_TYPE_MISMATCH_ON_INHERITANCE` | 20 | 4 | 16 |
| `METHOD_OF_ANY_IMPLEMENTED_IN_INTERFACE` | 16 | 5 | 11 |

These counts include the tests that kc accepts. Two findings matter for
the planning:

- **The 478 missed `CONFLICTING_JVM_DECLARATIONS` lines are mostly the
  first member of a clashing pair.**
  - `sameContainer` (`internal/check/jvmclash.go`) reports a clash only
    at the second member of a group, unless one member is synthetic.
    The backend's `JvmSignatureClashDetector` reports every member of
    the group.
  - 426 of the misses are in the three
    `overload/noConflictingOverloadsWithDeprecatedHidden/Functions*.kt`
    tests, where kc hits every second line.
  - About 45 more are in `jvm/duplicateJvmSignature/**`. In
    `functionAndProperty/*`, `erasure/*` and `vararg.kt`, kc hits the
    second line of each pair and misses the first.
  - Only `specialNames/classObjectCopiedField.kt` (2 lines, root cause
    D) and a few context-parameter lines (`jvm/contextParameters/`, 6)
    need more than reporting at every member.
- **The 150 "platform declaration clash" errors on lines without a mark
  aren't missed checks.**
  - All 150 are in
    `overload/noConflictingOverloadsWithDeprecatedHidden/Constructors{InMemberScope,InPackage,OnTopLevel}.kt`
    (50 each), which are also 3 of the *rejects valid* tests.
  - Those tests switch the diagnostic off
    (`// DIAGNOSTICS: -CONFLICTING_JVM_DECLARATIONS`) and run to
    `BACKEND`. A `HIDDEN` constructor is still written, so the clash is
    real, and `kotlinc` outside the test infrastructure would reject
    the file. kc reports it from `resolveSecondaries`
    (`internal/check/ctor.go`).
  - This is a harness rule, not a check (milestone 1).
  - The other 53 such errors are in tests both reject (12 in
    `contextParameters/overloads/OverloadingByTypeAlias.kt`, 6 each in
    `IdenticalDeclarations.kt`, `OverloadingByName.kt` and
    `ConflictingOlverloadsGenericFunctions.kt`). That is kc reporting a
    clash where K2 reports `CONFLICTING_OVERLOADS`; the same lines are
    usually hit.

**Where kc finds JVM signature clashes.** Everything is in
`internal/check`, in the frontend:

- `checkClashes` (`internal/check/jvmsig.go`) runs for every checked
  file in `CheckOptions` (`internal/check/check.go`), after `conflicts`.
- Local and anonymous classes go through `classClashes` from
  `internal/check/anon.go`.
- No guard skips these checks after other errors, and
  `internal/codegen` detects no clashes.

In kotlinc 2.4, `CONFLICTING_JVM_DECLARATIONS`,
`CONFLICTING_INHERITED_JVM_DECLARATIONS` and `ACCIDENTAL_OVERRIDE` are
JVM backend errors (`JvmBackendErrors.kt`), which kotlinc reports only
when the frontend found none. kc reports more of them than kotlinc
would in files with other errors. That never changes a test's category,
and it is harmless. What kc lacks is the backend's view of the class:
its fake overrides from Java supertypes, Java statics, and copied
companion fields.

## 2. How kc does this today

**Member scopes.** A `types.ClassSym` holds only the members it
declares:

- `Methods`, `ExtMethods`, `ExtProps` and `Props` for program classes;
- also `KProps` for Kotlin classes read from metadata;
- also `Statics`, `Fields` and `JavaFields` for Java classes.

Nothing inherited is copied in. Every lookup walks
`ClassSym.Supertypes` (`internal/types/types.go`), breadth first. There
are no fake-override or intersection-override objects. Metadata members
of the fake-override kind are skipped on load (`loadKotlin`,
`internal/check/kotlin.go`). The partial substitutes are:

- `intersectionDefaults` (`internal/check/java.go`), which copies
  methods that carry inherited default values;
- `delegations`, `delegatedFun`, `delegateProp` and `defaultImpls`
  (`internal/check/delegation.go`), which create real `FunSym`s for
  `by` and for DefaultImpls.

Where members come from:

- **Kotlin sources:** `resolveFun`, `resolveProp`, `dataMembers` and
  `enumMembers` (`internal/check/check.go`), and `resolveSecondaries`
  (`internal/check/ctor.go`).
- **Kotlin class files:** `loadKotlin`, `kfun` and `kprop`
  (`internal/check/kotlin.go`).
- **Java class files:** `classTable.load` (`internal/check/java.go`):
  - It skips synthetic and bridge methods and drops private members,
    which go to `JavaHidden` only with `KeepHidden` or for sources.
  - It gives reference types platform types (`fieldType`).
  - It applies `enhance`, but only under `JavaTypes`, and `enhanceJDK`.
- **Java sources:** `internal/javasrc` parses declarations into
  `classfile.Info`, which `driver.AddJavaSources` registers. They then
  load through the same `classTable.load` (`sourceClass`,
  `withSources`).

**Override matching.**

- `overridden` (`internal/check/class.go`) walks all supertypes for
  non-static, non-private methods of the name. It accepts a method when
  `sameParams` holds: equal arity, contexts, type-parameter count and
  receiver kind, and `sameType` on each parameter after `memberSubst`
  and `overrideSubst`.
- Raw types get their own bounds test (`rawLower`/`rawUpper`).
- `sameBounds` compares type-parameter bounds only when both functions
  are program declarations.
- `sameType` is mutual subtyping, and platform types are flexible
  (`subtype1`, `internal/check/subtype.go`). Without `JavaTypes`, Java
  types therefore match anything of the same class. With it, an
  enhanced `@NotNull` type is an ordinary non-null type and must match
  exactly.
- Visibility isn't part of matching, apart from `private`. A
  package-private Java member in another package is matched.

**Checks over inherited members.**

- `checkInheritance` (`internal/check/class.go`, called from
  `checkClass`) checks, for each declared function:
  - "overrides nothing" or a missing `override`;
  - final overridden members;
  - suspend and `@Composable` mismatches;
  - the return type (`overrideRetFits`).

  As side effects it records `Bridges`, `BridgeOf`, `BoxingBridges`,
  `BoxedRet`, `BoxedParams` and `Infix`.
- Properties go through `overriddenProps` and `superProps`. Java
  getters count only for `builtinJava` classes and
  `CharSequence.length`.
- `inheritedBridges` and `checkImplemented` follow.
  - `checkImplemented` walks all supertypes' abstract members and asks
    whether each is `implemented` (`implementation`, `concreteInSuper`,
    `numberImplemented`, `kotlinAccessorImplements`, `implementingProp`,
    `javaPropImplements`, `inheritedPropImpl`).
  - It reports one generic message, "is not abstract and does not
    implement abstract member".
  - It has no notion of an intersection, so `MANY_IMPL_*`,
    `MANY_INTERFACES_*` and `RETURN_TYPE_MISMATCH_ON_INHERITANCE` can't
    be told apart.
- `inheritDefaults` (`internal/check/check.go`, signature phase)
  forbids defaults in overrides and propagates operator, infix,
  inherited visibility, JVM name mangling and `DefaultsOf`.
- Nothing reports these:
  - `CANNOT_WEAKEN_ACCESS_PRIVILEGE`: visibility is only inherited, in
    `inheritDefaults` and `internal/check/visibility.go`;
  - `MULTIPLE_DEFAULTS_*`: `intersectionDefaults` silently picks one;
  - `METHOD_OF_ANY_IMPLEMENTED_IN_INTERFACE`;
  - the `*_BY_DELEGATION` mismatches: `checkDelegates` only checks the
    delegate's type;
  - Java sealed classes;
  - `JAVA_CLASS_INHERITS_KT_PRIVATE_CLASS`.
- `ABSTRACT_SUPER_CALL` exists (`superCall`, `superMember`), but
  `superTarget` resolves `super.f()` to the first supertype with a
  body. It doesn't use the intersection's modality.

**JDK classes mapped to Kotlin types.**

- **Collections:**
  - `collectionJVM`, `mutableOf` and `readOnlyOf`
    (`internal/check/kotlin.go`) relate Kotlin collection types and
    `java.util` interfaces. `classTable.class` maps the Java interfaces
    to the Kotlin `Mutable*` types.
  - `collection` and `loadCollection` build each Kotlin collection type
    from builtins metadata. Java-only members are copied in with
    `Platform = true`; duplicates of Kotlin signatures are dropped, as
    are mutators on read-only types.
  - Renamed members are listed in `builtinJVMNames`.
- **`java.lang` types:** `javaLangOf` maps Comparable, CharSequence,
  Number, Throwable, Enum, Cloneable and Annotation. These Kotlin types
  *are* the Java classes (`builtinJava`). `Number` gets `toInt` and the
  other conversions as abstract copies with `JVMName = intValue`
  (`classTable.load`).
- **Java classes that implement collections:** `mapCollectionMembers`
  (`internal/check/jdkenhance.go`) retypes the `Object` parameters of
  `contains`, `remove`, `get` and similar members.
- **What is hidden:**
  - `Object`'s members other than `Any`'s, and `List.remove(int)`
    (`javaRemoveAt`), are hidden from calls.
  - `Number.intValue()` and its siblings are hidden only from
    `checkImplemented` (`numberJavaName`); calls still see them.
  - There is no visible/hidden/not-considered member list like
    `JvmMappedScope`'s, and no purely implemented collections.
- **Predefined enhancement:** `enhanceJDK` covers only
  `java.util.Optional`.

**Where codegen depends on override analysis** (the regression surface):

- `FunSym.Bridges`, `BridgeOf` and `BoxingBridges`: `g.bridge` in
  `internal/codegen/gen.go`, and `internal/codegen/value.go`.
- `VarSym.Bridges` and `SetterBridges`.
- `ClassDecl.InheritedBridges`.
- `BoxedParams` and `BoxedRet`.
- `DefaultsOf`, used by `internal/codegen/expr.go` and `compose.go`.
- The `Inherited` and `PublicOnJVM` access flags.
- The `Platform` flag, which decides delegation and DefaultImpls
  members (`delegable`).
- `internal/codegen/collections.go`, which reads `Bridges` again to
  decide collection stubs.
- `analysis` and `analysis/ka`, which export `overridden` through
  `OverrideRecorder`.

Any change to what `overridden` returns can change bridges or KSP's
override answers. Diagnostics alone can't change them.

## 3. The gap against K2

| K2 design | What it does | kc today | Test groups that need it |
|:---|:---|:---|:---|
| Use-site member scopes: `FirClassUseSiteMemberScope`, substitution scopes, `FirTypeIntersectionScope` | Each class gets a scope of its declared members plus its supertypes' substituted members, grouped by signature. A group with several members becomes an intersection override, whose modality, visibility and return type are chosen from the group. Checkers look at the members an override *directly* overrides. | No groups. Every check walks the whole hierarchy on its own, so "directly overridden" and "intersection" don't exist. | A, B (`kt69190`, `propertiesConflict`), part of D, `kt53408` |
| `FirOverrideChecker`, `FirImplementationMismatchChecker`, `FirNotImplementedOverrideChecker`, `FirMultipleDefaultsInheritedFromSupertypesChecker`, the method-of-Any and abstract-super-call checkers | Override checks: visibility (`CANNOT_WEAKEN_ACCESS_PRIVILEGE`), finality, var/val, return and property types. On a class: return type and var/val mismatches on inheritance and by delegation, missing implementations with their three distinct diagnostics, and multiple inherited defaults. | Override checks without visibility. Missing implementations with one message. No inheritance or delegation mismatches. | A, B |
| `JavaClassUseSiteMemberScope` with `JavaOverrideChecker` | Java members override by erased classifiers, ignoring nullability, so enhanced types still match. A Java primitive and its wrapper are different. Package-private members are invisible outside their package. Members whose JVM names Kotlin renames (`size()` and `size`, `charAt` and `get`, `intValue` and `toInt`, `remove(int)` and `removeAt`) are matched under the Kotlin name, and the Java names are hidden. | `sameParams` on Kotlin types. Platform types are lenient; enhanced types are strict. Visibility is ignored. Renaming is special-cased per member (`numberJavaName`, `charSequence`, `javaRemoveAt`). | C, E, F; the 4 `JavaTypes` regressions |
| `JvmMappedScope` with `JvmBuiltInsSignatures` | For each mapped class, the JDK members not in Kotlin's builtins are visible, hidden (deprecated hidden or error), or not considered, by signature. `MutableList.sort` is `DEPRECATION_ERROR`; `Throwable.fillInStackTrace` isn't a member. | Only for the Kotlin collection types, by "not already declared". No table. | F |
| Predefined enhancement (`predefinedEnhancementInfo.kt`) and purely implemented collections (`FakePureImplementationsProvider`) | JDK members get fixed nullability: `Iterator.forEachRemaining`, `Map.getOrDefault`, `ConcurrentHashMap.remove`, the Java 21 `List.addFirst`/`getFirst`, and others. `java.util.ArrayList<E>` and its siblings are seen as `MutableList<E>`, and so on, with `E` not flexible. | `Optional` only. No purely implemented classes. | F (`newListMethodsNullability*`, `concurrentMapRemove`, `purelyImplementedCollection/*`, `forEachRemainingNullability`) |
| JVM clash detection: the IR backend's `JvmSignatureClashDetector`; `CLASS_INHERITS_JAVA_SEALED_CLASS` and `JAVA_CLASS_INHERITS_KT_PRIVATE_CLASS` from FIR JVM checkers | Clashes over all methods of the generated class, including fake overrides from Java supertypes, and reported at every member of a group. Java sealed hierarchies; calls on Java classes that extend private Kotlin classes. | Frontend checks over program supertypes only, reported at one member of a group. No Java sealed classes. | D, E, and the 478 missed lines |
| Delegation (`by`) | The delegated member is an implicit override in the class's scope. It must fit every member it overrides there, by return type, property type and var/val. A member the class also inherits through another path is checked against it. | Delegated members are created and bridged; only the delegate's type is checked. | A (delegation) |
| Default-value inheritance | Defaults come from the overridden members. An override that inherits defaults for a parameter from more than one member is an error, as is an implicit intersection that does. | `inheritDefaults` takes the first; `intersectionDefaults` picks one. | A (multiple defaults) |

kc doesn't need to copy K2's scope objects. It needs one computed
answer per class: for each inherited signature, the set of supertype
members that no other member of the set overrides. That is K2's
intersection group. From it, kc can tell which members a declared
member directly overrides and which it implements. Milestone 4 adds
this as a cached helper. It doesn't touch the per-supertype lookups
that bridges and calls use.

## 4. Milestones

Every milestone has to land on its own with
`bench/kotlin-tests.sh diagnostics -baseline bench/results/diagnostics.tsv`
and `box -baseline bench/results/box.tsv` showing 0 regressions, as CI's
*Conformance* job demands. It then commits the new baselines. The
order puts cheap checks that can't change code generation first, the
shared helper next, and the JDK-mapping work, which has the widest
blast radius, last.

Sizes: **S** is under 200 lines with tests, **M** is up to about 600,
**L** is more.

### M0. Line-level harness output (S)

Add `-marks FILE` to `bench/conformance`. It writes each expected mark
with whether kc hit its line, and each error kc reported, as the
scratch build used for this plan did (a few lines in `runDiagnostics`).
Later milestones state their gains in lines, which the summary doesn't
show.

- Files: `bench/conformance/diagnostics.go`, `main.go` and
  `docs/conformance.md`.
- Risk: none.

### M1. Report JVM clashes at every member, and honour switched-off backend errors (S)

- In `sameContainer`, report every member of a group of equal
  signatures, not only the later ones. Keep the `conflicted` skip.
- In the harness, if a test's `DIAGNOSTICS` directive switches off
  `CONFLICTING_JVM_DECLARATIONS`, `ACCIDENTAL_OVERRIDE` or
  `CONFLICTING_INHERITED_JVM_DECLARATIONS`, drop kc's errors that start
  with that diagnostic's message. kc's messages are the backend's:
  "platform declaration clash", "accidental override", "inherited
  platform declarations clash". `kotlinc` outside the infrastructure
  rejects these files too, so the rule belongs with the harness's other
  infrastructure differences in `docs/conformance.md`.
- Flips: the 3 *rejects valid*
  `noConflictingOverloadsWithDeprecatedHidden/Constructors*.kt`.
- Lines: about 470 of the 478 missed `CONFLICTING_JVM_DECLARATIONS`
  lines, and all 150 unmarked clash errors.
- Files: `internal/check/jvmclash.go`,
  `bench/conformance/diagnostics.go`, and `testdata/errors/*.kt` where
  the `// ERROR` lines of clashing pairs change.
- Risk: new errors on lines without a mark, where a clash pair's first
  member is synthetic or elsewhere. Check with `-errors`. No codegen
  effect.

### M2. Property overrides against directly overridden members (S)

- A `var` must have the same type only as the `var`s it overrides; a
  `val` it overrides only needs a subtype.
- Generic member extension properties must match type parameters by
  position, as `overrideSubst` does for functions
  (`checkExtPropOverrides`).
- Flips (*rejects valid*):
  - `extraCheckers/kt69190.kt`
  - `smartCasts/intersectionScope/propertiesConflict.kt`
  - `genericReceiverPropertyOverride.kt`
  - `generics/finalUpperBoundWithOverride.kt`
- Files: `internal/check/class.go` and `internal/check/extension.go`.
- Risk: low. Bridges for property overrides come from the same loop, so
  check that bridges are unchanged with the box suite.

### M3. Cheap Kotlin checks on explicit overrides (S each)

These need only `overridden`, not intersections.

- **`CANNOT_WEAKEN_ACCESS_PRIVILEGE` on a declared override:** the
  override's explicit visibility, or its setter's, is weaker than an
  overridden member's.
  - Flips: `scopes/kt1248.kt`.
  - Lines: `PrivateSetterForOverridden.kt` (7),
    `VisibilityInheritModifier.kt` (2), `kt151.kt`,
    `OverridenSetterVisibility.kt` and others.
- **`MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES` on a declared
  override:** two or more members it overrides give a parameter a
  default value. Do this in `inheritDefaults`.
  - Flips: `multipleDefaultsIncorrectlyAllowed1.kt`,
    `multipleDefaultsAnonymousObject.kt`,
    `override/MultipleDefaultParametersInSupertypes.kt` and
    `missingMultipleDefaultsOnTransitiveInheritance2.kt`. Confirm the
    last against "directly overridden" once M4 lands.
- **`METHOD_OF_ANY_IMPLEMENTED_IN_INTERFACE`:** an interface member
  that overrides `equals`, `hashCode` or `toString` with a body.
  - Flips the three `java8Overrides/*MethodOfAny*.kt` tests.
  - Lines: removes the four clash errors in
    `diagnostics/methodOfAnyImplementedInInterface.kt` and hits its
    four lines.

Shared details:

- Files: `internal/check/class.go`, `check.go` and `visibility.go`,
  plus `testdata/errors/inheritance.kt` (new).
- Risk: the visibility rule has K2 subtleties. Protected against
  internal is incomparable (`CANNOT_INFER_VISIBILITY`); compare with
  `chooseIntersectionVisibility`. Keep to explicit modifiers first.
- Expected: about 8 tests, and about 25 lines.

### M4. Inherited-member groups (M, no diagnostics change by itself)

Add `(*Checker).inheritedGroups(cls)`. For each signature a class
inherits, it gives the supertype members that no other member of the
set overrides, with the substitution each one is seen through. It is
computed once per program class and cached in the `Universe` local
results.

Rebase `checkImplemented` onto it, keeping today's message. Expose
`directlyOverridden(cls, f)` for `checkInheritance`. Leave
`overridden`, the bridge code and the `OverrideRecorder` export as they
are, so codegen and `analysis` see no change.

- Flips: none. The milestone is right if both ratchets show no change
  at all.
- Files: `internal/check/class.go` and a new
  `internal/check/inherited.go`.
- Risk: performance. Classes with deep hierarchies, such as Android
  views and the kotlinpoet sources, would compute groups for many
  inherited names. Compute groups lazily per name, or only for classes
  that `checkImplemented` visits. Measure with `bench/run.sh` and the
  kotlinpoet and ktor timings (`bench/kotlinpoet-time.sh`,
  `bench/ktor-time.sh`), and with the `analysis/ka` benchmarks
  (`go test ./analysis/ka -bench .`).

### M5. Intersection checks (M)

Built on M4:

- `MANY_INTERFACES_MEMBER_NOT_IMPLEMENTED`: two or more interface
  bodies and no class member.
- `MANY_IMPL_MEMBER_NOT_IMPLEMENTED`: a class member and an interface
  body that the class member doesn't override, or one that doesn't fit.
- `ABSTRACT_MEMBER_NOT_IMPLEMENTED` against
  `ABSTRACT_CLASS_MEMBER_NOT_IMPLEMENTED`: whether the abstract member
  comes from a class.
- `RETURN_TYPE_MISMATCH_ON_INHERITANCE` and
  `PROPERTY_TYPE_MISMATCH_ON_INHERITANCE`: no member of the group has a
  return type that is a subtype of all the others.
- `MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES_WHEN_NO_EXPLICIT_OVERRIDE`
- The class-level `CANNOT_WEAKEN_ACCESS_PRIVILEGE`: a class member that
  implements an interface member with weaker visibility.
- `super.f()` resolution by the group: an abstract class member beats
  an interface body, giving `ABSTRACT_SUPER_CALL`. A group with exactly
  one body resolves, as in `kt53408.kt`.
- `kt12358`: `abstract override` of an `Any` member. The superclass's
  abstract re-declaration hides `Object`'s implementation.
- `ProtectedAndPrivateFromSupertypes`: a private member doesn't
  implement.

Flips:

| Diagnostic or cause | Tests |
|:---|:---|
| Many interface bodies | `override/ParentInheritsManyImplementations.kt`, `regressions/kt302.kt` |
| Many implementations | `override/FakeOverrideDifferentDeclarationSignatures.kt`, `purelyImplementedCollection/intersectionOfPrimitiveBasedFunctions.kt` |
| Return type on inheritance | `override/kt4763.kt`, `override/clashesOnInheritance/returnTypeMismatch.kt`, `flexibleReturnTypeList.kt` |
| Abstract super call | `missingAbstractSuperCall*.kt` (3), `override/kt53408.kt` (*rejects valid*) |
| Missing members | `override/kt12358.kt`, `override/ProtectedAndPrivateFromSupertypes.kt` |
| Multiple defaults, no explicit override | `moreThanOneOverriddenMissingK2.kt`, `multipleDefaultsNoSource.kt`, `override/MultipleDefaultsAndNamesInSupertypes.kt`, `MultipleDefaultsAndSubstitutionOverrides.kt` |
| Visibility on inheritance | `override/kt4785.kt`, `intersectedVisibilities*_Small.kt`, `intersectedVisibilitiesForNonAbstractPropertyAccessors.kt` |
| Data classes | `dataClassWithInheritedComponent.kt`, `dataClasses/dataClassOverridingCopy.kt` |

That is about 22 tests, and the 24 lines of
`intersectedVisibilitiesAccessorDifferences_Huge.kt`.

- Files: `internal/check/class.go`, `inherited.go`, and `superTarget`
  and `superCall` in `class.go`.
- Risk: `ABSTRACT_SUPER_CALL` and `super` resolution change which
  function a `super` call targets, which codegen emits. Any box test
  using `super.f()` with an interface default and an abstract class
  member could change. The box suite covers this (`thisAndSuper`,
  `super/` and `traits/` directories). `superTarget` should change only
  where it now reports an error or where kc reported one.

### M6. Delegation mismatches (S–M)

For each member that a `by` clause delegates, compare it with the other
members of its group (M4):

- `RETURN_TYPE_MISMATCH_BY_DELEGATION`
- `PROPERTY_TYPE_MISMATCH_BY_DELEGATION`
- `VAR_OVERRIDDEN_BY_VAL_BY_DELEGATION`

Flips:

- `delegation/covariantOverrides/{simple,fromClass,irrelevant}.kt`
- `delegation/clashes/varOverriddenByVal.kt`
- Lines in `delegation/clashes/returnTypeMismatch.kt` (5).

Details:

- Files: `internal/check/delegation.go`.
- Risk: `delegable` decides which delegated methods are generated.
  Don't change it here; only report.

### M7. Java override matching as `JavaOverrideChecker` does, then `JavaTypes` in the CLI (M–L)

1. **Matching.** When either side is a Java member, compare parameter
   types ignoring nullability (enhanced or not), but tell a Java
   primitive from its wrapper. A wrapper parameter in a Java subclass
   doesn't override a primitive one, and the call picks the subclass's
   member, as in `notNullAnnotated.kt`. Confirm the exact rule against
   K2's `JavaOverrideChecker` and its overload resolution before
   coding.
2. **Return types.** Check them with the enhanced types:
   `RETURN_TYPE_MISMATCH_ON_OVERRIDE` when a Kotlin override's return
   type is less specific than the enhanced one. Check parameters with
   the rule for Kotlin overriding Java: an enhanced non-null parameter
   overridden with a nullable type is `NOTHING_TO_OVERRIDE`.
3. **Varargs.** `enhance` applies a declaration annotation to the array
   of a vararg, not its elements (`classTable.load`).
4. **Visibility.** A package-private Java member is invisible to a
   class in another package. It is neither overridden nor hidden
   (`visibility/notOverridingPackagePrivate.kt`).
5. **Type parameters.** Ignore type-parameter bounds when the
   overridden or overriding member is Java's
   (`platformTypes/methodTypeParameterDefaultBound.kt`), and let raw
   types override as K2 does
   (`rawTypes/overrideRawTypedJavaMethodWithGenericTyped.kt`).
6. **CLI and harness.** Turn on `check.Library.JavaTypes()` in
   `driver.newLibrary` for the CLI, as `analysis/ka` already does. Put
   `org.jetbrains:annotations:13.0` on the harness's class path, as the
   test infrastructure does.

Flips:

- The 36 tests that `JavaTypes` fixed in the earlier session, without
  its 4 regressions.
- The typeEnhancement tests in C.
- `kt13355viaJava.kt`.
- `supertypeDifferentParameterNullability.kt` (both copies; the
  `CONFLICTING_INHERITED_JVM_DECLARATIONS` one needs M8).
- *Rejects valid*: `notOverridingPackagePrivate.kt` and
  `methodTypeParameterDefaultBound.kt`.
- About 45 tests in all.

Risks:

- Box tests with Java sources are skipped, so the box suite says little
  here. KSP via `analysis/ka` already runs with `JavaTypes`; its tests
  (`go test ./analysis/...`) must stay green, and its override answers
  must not change.
- Real code compiled against annotated jars (kotlinpoet, ktor,
  coroutines in `bench/`) gets stricter types. Recompile those benches
  and compare `bench/apidump` and `bench/sigdump` output for changes.
- Performance: reading annotations from every Java method. Measure
  JDK-heavy compiles with `bench/run.sh`.
- Files: `internal/check/class.go`, `java.go`, `call.go`/`overloads.go`
  for the primitive rule, `internal/driver/driver.go`,
  `bench/conformance/compile.go` and `bench/kotlin-tests.sh`.

### M8. JVM clashes with Java and library supertypes (M)

Extend `inheritedMembers` to `External` supertypes: Java and class-file
instance methods, and Java statics, which the JVM sees as inherited for
accidental overrides. Use `JVMMethods` and the member descriptors kc
already reads. A member that a class in the middle of the hierarchy
overrides isn't inherited from above it.

Then, in `classClashes`:

- `ACCIDENTAL_OVERRIDE` against Java statics (`statics/*`,
  `jvmStatic/*`, 6 tests);
- context parameters against Java parameters
  (`jvm/contextParameters/*Java*.kt`, 3 tests);
- `kjkimplicitTypesCrash.kt`;
- `CONFLICTING_INHERITED_JVM_DECLARATIONS` from Java members, with
  M7's nullability (`supertypeDifferentParameterNullability.kt`);
- the companion field copied to the outer class
  (`classObjectCopiedField.kt`).

Fix the *rejects valid* `KJKHierarchyWithOverride.kt`: neither the
clash nor `checkImplemented` should look past the Java class that
overrides both members.

- Flips: about 12 tests. Lines: the remaining `ACCIDENTAL_OVERRIDE` and
  `CONFLICTING_JVM_DECLARATIONS` misses.
- Files: `internal/check/jvmclash.go` and `jvmsig.go`.
- Risk: false clashes against JDK classes in real code, the main one.
  Run `bench/kotlinpoet.sh` and the KSP benches, and compile
  `testdata/lib` and `testdata/javasrc`. Performance: walking
  class-file supertypes for every program class. Cache per external
  class, keyed by descriptor.

### M9. Java class structure (S)

- `CLASS_INHERITS_JAVA_SEALED_CLASS`: keep `sealed` and `permits` in
  `internal/javasrc` (into `classfile.Info`'s `PermittedSubclasses`,
  which class files carry too). Report a Kotlin class or interface that
  extends one without being permitted.
- `JAVA_CLASS_INHERITS_KT_PRIVATE_CLASS`: at a constructor call or
  member call on a Java class whose superclass chain has a private
  Kotlin class from this module.
- `ABSTRACT_CLASS_MEMBER_NOT_IMPLEMENTED` when a Java class re-declares
  an inherited member abstract (`notImplementedMembersAroundJava.kt`),
  or a Java member's parameter type is missing
  (`missingTypeOfParameterInFakeOverride_2.kt`).
- Flips: 6 tests.
- Files: `internal/javasrc/parse.go`, `internal/check/class.go` and
  `call.go`.
- Risk: low.

### M10. Mapped JDK members: a `JvmMappedScope` table (L)

Port `JvmBuiltInsSignatures`'s lists as a Go table keyed by class and
JVM signature: visible, hidden, deprecated-list and not-considered
members, and hidden constructors. Apply it when a mapped class's
members are read (`loadCollection`, the `javaLangOf` classes in
`classTable.load`):

- **Hidden** Java names for renamed members disappear from calls:
  `Number.byteValue`, `CharSequence.charAt`, `Collection.size()`,
  `Map.keySet` and others. Replace the per-member special cases
  (`numberJavaName`, `charSequence`, `javaRemoveAt`) one by one.
- **Not-considered** members aren't members: `Throwable.fillInStackTrace`.
- **`DEPRECATED_LIST_METHODS`** (`sort`) are `DEPRECATION_ERROR`.

Java getters that implement or override a mapped Kotlin property are
that property:

- a final `size()` is a final `size`, so `OVERRIDING_FINAL_MEMBER`;
- `j+k/properties/*`: a final Java getter over a Kotlin property;
- `get(Object)` and `remove(Object)` in a Java class implement
  `Map.get` and `remove` (`mapMethodsImplementedInJava.kt`);
- `charAt` and `length()` from a class that doesn't implement
  `CharSequence` don't implement it
  (`irrelevantImplCharSequenceKotlin.kt`).

Flips:

- `scopes/specialFunctionInJava_1.kt`
- `targetedBuiltIns/backwardCompatibility/fillInStackTrace.kt` and
  `j+k/collectionOverrides/mapMethodsImplementedInJava.kt` (*rejects
  valid*)
- `testsWithStdLib/targetedBuiltIns/blackListed.kt`
- `j+k/collectionOverrides/nonDirectHiddenOverride.kt`
- `j+k/finalCollectionSize.kt`
- `j+k/properties/{val,var,isName}.kt`
- `j+k/collectionOverrides/irrelevantImplCharSequenceKotlin.kt`
- `j+k/collectionOverrides/containsAll.kt`
- `purelyImplementedCollection/wrongTypeParametersCount.kt` (*rejects
  valid*; confirm the cause first)
- About 12 tests in all.

Details:

- Files: a new `internal/check/jvmmapped.go`, plus
  `internal/check/kotlin.go`, `java.go`, `class.go` and
  `jdkenhance.go`.
- Risk: highest of the plan.
  - `internal/codegen/collections.go` generates collection stubs and
    bridges from what the checker leaves, so hidden and renamed members
    must keep their JVM identity for codegen. Change visibility to
    Kotlin, not membership.
  - The box suite's `collections/`, `builtinStubMethods/` and
    `specialBuiltins/` directories are the guard.
  - `analysis/ka` lists Java members (`KeepHidden`); hidden mapped
    members must stay listable there, as K2's Analysis API shows them.

### M11. Predefined JDK enhancement and purely implemented collections (M–L)

Port the predefined enhancement table into `enhanceJDK`, replacing its
`Optional`-only table, and generate it from K2's
`predefinedEnhancementInfo.kt`. Make `java.util.ArrayList`,
`LinkedList`, `HashSet`, `LinkedHashSet`, `HashMap`, `LinkedHashMap`
and the rest of `FakePureImplementationsProvider`'s classes act as
their Kotlin interface with non-flexible type arguments, at use sites.

Flips:

- `purelyImplementedCollection/{arrayList,sets}.kt`
- `targetedBuiltIns/concurrentMapRemove.kt`
- The five `newListMethodsNullability*Latest.kt` tests that kc accepts
- Lines in `forEachRemainingNullability.kt`,
  `purelyImplementedCollection/maps*.kt` and similar tests.
- About 8 tests in all.

Details:

- Files: `internal/check/jdkenhance.go`, `java.go` and `kotlin.go`, and
  a generator under `tools/`.
- Risk: inference everywhere `ArrayList<T>()` is used. Element types
  that were flexible become strict, which can turn valid code into
  *rejects valid*, including in box tests and real code. Land the
  enhancement table and the purely implemented collections as separate
  changes, each with full box and diagnostics runs and a `bench/survey`
  pass over the survey corpus.

### Totals

| Milestone | Tests flipped (approx.) | Lines | Size |
|:---|---:|---:|:---|
| M0 | 0 | (measurement) | S |
| M1 | 3 | ~470 hit, 150 unmarked errors gone | S |
| M2 | 4 | | S |
| M3 | ~8 | ~25 | S |
| M4 | 0 | | M |
| M5 | ~22 | ~30 | M |
| M6 | 4 | ~5 | S–M |
| M7 | ~45 | | M–L |
| M8 | ~12 | ~20 | M |
| M9 | 6 | | S |
| M10 | ~12 | | L |
| M11 | ~8 | ~15 | M–L |

The theme's 98 tests are mostly covered: M2–M11 address about 85 of
them. M7's count includes tests outside the theme that `JavaTypes`
fixes. The rest need the out-of-scope fixes listed in section 1, or
they expect other errors too.

## 5. Validation

For every milestone:

- **Diagnostics ratchet.** First run the tests the milestone targets:
  `bench/kotlin-tests.sh diagnostics -filter tests/override/ -baseline
  bench/results/diagnostics.tsv -o "" -tsv ""`, and the same with
  `-filter tests/j+k/`, `tests/jvm/duplicateJvmSignature/`,
  `tests/delegation/` and `testsWithStdLib/purelyImplementedCollection/`.
  The list of flipped tests in each milestone above can go in a `-list`
  file. Then run the full suite and commit the rewritten
  `bench/results/diagnostics.{md,tsv}`.
- **Line-level data.** Compare `-errors` output before and after:
  - New errors on unmarked lines must be explained or fixed.
  - With M0, compare `-marks` output for the lines each milestone
    claims.
- **Box suite.** Run `bench/kotlin-tests.sh box -baseline
  bench/results/box.tsv -o "" -tsv ""` in full for any milestone that
  touches `class.go`'s override loop, `superTarget`, delegation or the
  mapped members (M2, M5, M6, M7, M10, M11). The bridge and stub
  directories (`bridges/`, `collections/`, `builtinStubMethods/`,
  `specialBuiltins/`, `fakeOverride/`, `traits/`, `super/`) are the
  sensitive ones; run them first with `-filter`.
- **Unit tests.**
  - Add `testdata/errors/inheritance.kt`, and a directory
    `testdata/errors/inheritanceJava/` for Java and Kotlin modules. Use
    the `// ERROR` convention that `TestErrors`
    (`internal/driver/driver_test.go`) reads.
  - Add a case per new diagnostic, and per rule that is easy to get
    wrong: package-private visibility, primitive against wrapper, and
    the target of a vararg annotation.
  - M4's helper gets a table test in `internal/check`, like
    `lookups_test.go`, over small hierarchies: diamond, a class against
    an interface, substitution, and a Java class in the middle.
- **Analysis API and KSP.** Run `go test ./analysis/... ./psi/...`.
  `analysis/ka` reports overrides through `OverrideRecorder`, and its
  tests pin them. Overrides must not change before M7, and for M7
  every change must match K2's. Also run `bench/ksp` for any milestone
  that changes Java member visibility.
- **Performance.**
  - Run `bench/run.sh` (the hyperfine numbers in
    `bench/results/summary-*.md`), `bench/kotlinpoet-time.sh` and
    `bench/ktor-time.sh` before and after M4, M7, M8 and M10.
  - Use `go test ./analysis/ka -bench .` for the warm Analysis API path.
  - A regression over 2% in a cold compile of the 1000-file benchmark
    should block the milestone until it is cached away.
- **The harness self-check.** For M1's harness rule, run `-kotlinc
  kotlinc -sample 300` where `kotlinc` is available. The rule must
  leave every sampled test *rejects invalid* or *accepts valid*.

## 6. `docs/unsupported.md` changes

The current bullets in **Differences from kotlinc → Checking** that
these milestones touch:

- **"Java module exports aren't checked, and the Java members Kotlin
  replaces (`Number.intValue()`) aren't hidden."**
  - After M10, rewrite it as "Java module exports aren't checked."
    Delete the clause about replaced members.
  - Nothing in this plan checks module exports
    (`JAVA_MODULE_DOES_NOT_EXPORT_PACKAGE`).
- **"Of the Java collection members K2 gives Kotlin's signatures, kc
  maps `contains`, ...; others, such as `getOrDefault`, keep Java's."**
  - After M10 and M11 it can be deleted, if the mapped table covers the
    remaining members.
  - If M10 lands without M11, shorten it to the members whose
    nullability still differs.
- **"Of K2's predefined nullability of JDK members, only
  `java.util.Optional`'s ... is applied."**
  - After M11, delete its first sentence.
  - Keep the second, about a nullable argument for `T!`, which is an
    inference difference.

New bullets to add while the work is partial, and to remove as it
completes:

- **Until M7:** "`kc` gives Java declarations platform types even where
  they are annotated `@NotNull` or `@Nullable`; the Analysis API reads
  the annotations." This is said today only in `docs/conformance.md`'s
  results; it belongs in `docs/unsupported.md`.
- **Until M5 and M6:** "The members a class inherits from several
  supertypes aren't checked as one intersection: inherited default
  values, visibilities and return types that conflict, and members of
  delegated interfaces whose types don't fit, aren't reported."
- **Until M8:** "JVM signature clashes with members inherited from Java
  and library classes, including Java statics, are reported only for
  property getters."

`docs/conformance.md`:

- **M1:** add the switched-off-backend-error rule to the list of
  differences from the test infrastructure.
- **Each milestone:** update the counts in **Results**.
- **M7:** remove the paragraph saying that kc doesn't read Java
  sources' `@NotNull` and `@Nullable`.
