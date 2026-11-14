# Kotlin's diagnostics tests on kc

The diagnostics tests of JetBrains/kotlin v2.4.21 for the JVM (`compiler/testData/diagnostics/`: tests/, testsWithStdLib/, testsWithJvmBackend/, testsWithAnyBackend/), each compiled by kc. A test's markup marks the diagnostics kotlinc reports, and its `RUN_PIPELINE_TILL` says whether kotlinc stops with errors in the frontend (`FRONTEND`, `FIR2IR`) or goes on to the backend (`BACKEND`). `bench/kotlin-tests.sh diagnostics` writes this file, and [docs/conformance.md](../../docs/conformance.md) explains it.

Of 10827 test files, 2312 are skipped and 8515 run.

| Result | Tests | Share |
|:---|---:|---:|
| rejects invalid | 3962 | 46.5% |
| accepts valid | 3877 | 45.5% |
| accepts invalid | 421 | 4.9% |
| rejects valid | 235 | 2.8% |
| parse/unsupported | 18 | 0.2% |
| crash | 1 | 0.0% |

*Rejects invalid* and *accepts valid* agree with kotlinc. *Accepts invalid* is code kotlinc rejects that kc compiles; *rejects valid*, code kotlinc compiles that kc's checker rejects. Both are bugs. A *parse/unsupported* test uses a feature kc doesn't support, which says nothing about its checks.

Of the 3962 tests both reject, kc reports an error on the line of an expected one in 3735.

A test is invalid if it stops before the backend or marks an error: one that runs to it (`BACKEND`) can mark the JVM backend's errors, such as `CONFLICTING_JVM_DECLARATIONS`. Of the tests run, 222 stop at `FRONTEND` but mark no error: most add the `CHECK_TYPE` helpers, whose names (`_`) kotlinc reports outside the test infrastructure, and count as valid; the others leave their errors out with the `DIAGNOSTICS` directive. 114 run to the backend and mark one. 5 marked names have no declared severity (debug information, for one), and count as neither.

The names without a severity: `DEBUG_INFO_EXPRESSION_TYPE` (742), `DEBUG_INFO_CSR_MIGHT_BE_USED` (44), `DEBUG_INFO_CALL` (28), `DEBUG_INFO_CSR_MIGHT_BE_USED_INSTEAD_OF_IMPORT` (16), `DEBUG_INFO_CALLABLE_OWNER` (6).

## Skipped

Tests for other platforms, that need language features off by default or compiler options, or modules other than plain dependencies, or whose FIR diagnostics aren't checked.

| Reason | Tests | Examples |
|:---|---:|:---|
| `LANGUAGE` | 1579 | `tests/BacktickNames.kt`, `tests/BacktickNamesOld.kt` |
| `modules with friends or dependsOn` | 363 | `tests/jvm/internal/internalFromFriendModule.kt`, `tests/multimodule/friendModule.kt` |
| `LANGUAGE_FEATURE_TOGGLED` | 93 | `tests/annotatedPackageQualifiers.kt`, `tests/annotationUsedAsAnnotationArgument.kt` |
| `return value checker (crv/, crvFull/)` | 83 | `tests/crv/assignments.kt`, `tests/crv/basicJInterop.kt` |
| `API_VERSION` | 53 | `tests/deprecated/deprecatedSinceKotlin/error.kt`, `tests/deprecated/deprecatedSinceKotlin/hidden.kt` |
| `ALLOW_KOTLIN_PACKAGE` | 47 | `tests/annotations/requireKotlin.kt`, `tests/crvDisabled/oldMustUse.kt` |
| `JDK_KIND` | 27 | `tests/errorProneAtomicArrayPrimitivesAllJavaMethods.kt`, `tests/identityComparisonOnJavaValueBasedClassesWarning.kt` |
| `explicit API mode` | 21 | `tests/testsWithExplicitApi/annotations.kt`, `tests/testsWithExplicitApi/classes.kt` |
| `IGNORE_FIR_DIAGNOSTICS` | 18 | `tests/jvm/contextParameters/kt59590WithContextParameters.kt`, `tests/jvm/duplicateJvmSignature/accidentalOverrides/overridesNothing.kt` |
| `header mode` | 9 | `tests/headerMode/anonymousObjects.kt`, `tests/headerMode/classDeclaration.kt` |
| `-jvm-default modes` | 7 | `testsWithStdLib/annotations/jvmDefault/generic.kt`, `testsWithStdLib/annotations/jvmDefault/jvmDefaultWithCompatibility/disable.kt` |
| `JVM_DEFAULT_MODE: enable` | 4 | `tests/jvm/jvmDefault/compatibilityAgainstJava.kt`, `testsWithStdLib/annotations/jvmDefault/allCompatibility/specialization.kt` |
| `multiplatform` | 3 | `tests/multiplatform/errorProneAtomicArrayArgumentsInKMP.kt`, `tests/multiplatform/errorProneAtomicReferenceArgumentsInKMP.kt` |
| `TARGET_BACKEND: NATIVE, JS_IR, WASM` | 2 | `testsWithAnyBackend/inlineOverrideDefaultWarning.kt`, `testsWithAnyBackend/inlineOverrideDefaultWarningWithInlinedFunInKlib.kt` |
| `strict equality` | 2 | `tests/strictEquals/enabled/simple.kt`, `tests/strictEquals/enabled/wrongElementsAnnotated.kt` |
| `TARGET_BACKEND: NATIVE, JS_IR` | 1 | `testsWithAnyBackend/crossFileInlineCycle.kt` |

## Accepts invalid

Tests kotlinc rejects that kc compiles, by the errors their markup expects. A test counts for each; *Only error* counts the tests it is the only expected error of, which implementing it alone would fix.

| Expected error | Tests | Only error | Examples |
|:---|---:|---:|:---|
| `ARGUMENT_TYPE_MISMATCH` | 43 | 28 | `tests/annotations/annotationArgumentScopeForNestedClass.kt`, `tests/collectionLiterals/kt82964.kt` |
| `UNRESOLVED_REFERENCE` | 37 | 25 | `tests/SafeCallOnFakePackage.kt`, `tests/callResolution/eagerLambdaAnalysis/samVsDefault.kt` |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 24 | 19 | `tests/callResolution/kt50386.kt`, `tests/imports/EnumEntryVsStaticAmbiguity.kt` |
| `MISSING_DEPENDENCY_CLASS` | 16 | 10 | `tests/j+k/differentFilename.kt`, `tests/javac/Lambda.kt` |
| `NULL_FOR_NONNULL_TYPE` | 15 | 10 | `tests/j+k/SupertypeArgumentsNullability-NotNull-SpecialTypes.kt`, `tests/j+k/SupertypeArgumentsNullability-NotNull-UserTypes.kt` |
| `UNSAFE_CALL` | 12 | 9 | `tests/j+k/sam/enhancedSamConstructor.kt`, `tests/platformTypes/nullabilityWarnings/arithmetic.kt` |
| `CANNOT_INFER_PARAMETER_TYPE` | 11 | 3 | `tests/callableReference/function/ambiguityTopLevelVsTopLevelGeneric.kt`, `tests/generics/kt30590.kt` |
| `INITIALIZER_TYPE_MISMATCH` | 11 | 6 | `tests/callableReference/adapted/noKFunctionForAdaptation.kt`, `tests/inference/flexibleConstraints2.kt` |
| `ACCIDENTAL_OVERRIDE` | 10 | 8 | `tests/j+k/kjkimplicitTypesCrash.kt`, `tests/jvm/contextParameters/intersectionContextValueJava.kt` |
| `INVISIBLE_REFERENCE` | 9 | 9 | `tests/arguments/ambiguityOnJavaOverride.kt`, `tests/classObjects/companionObjectOfPrivateClassVisibility.kt` |
| `(no error marked)` | 8 | 8 | `tests/cyclicHierarchy/javaJavaCycle.kt`, `tests/deprecated/javaDeprecated.kt` |
| `UNSUPPORTED` | 8 | 5 | `tests/NamedFunctionTypeParameterInSupertype.kt`, `tests/callableReference/property/backingField.kt` |
| `CLASSIFIER_REDECLARATION` | 7 | 7 | `tests/j+k/brokenCode/classDuplicates.kt`, `tests/redeclarations/kotlinClassJavaClassNestedRedeclaration.kt` |
| `JAVA_TYPE_MISMATCH` | 7 | 7 | `tests/platformTypes/genericVarianceViolation/deepTypeHierarchy.kt`, `tests/platformTypes/genericVarianceViolation/javaOutProjection.kt` |
| `MANY_IMPL_MEMBER_NOT_IMPLEMENTED` | 6 | 5 | `tests/j+k/manyImplNotImplementedComplexHierarchy.kt`, `tests/j+k/manyImplNotImplementedComplexHierarchy2.kt` |
| `NOTHING_TO_OVERRIDE` | 6 | 3 | `tests/j+k/types/newListMethodsNullabilityOverridesLinkedListLatest.kt`, `tests/multimodule/duplicateSuper/sameSuperTraitDifferentBounds.kt` |
| `TYPE_INFERENCE_ONLY_INPUT_TYPES_ERROR` | 6 | 5 | `testsWithStdLib/commonCollections.kt`, `testsWithStdLib/inference/annotationsForResolve/kt29307.kt` |
| `ASSIGNMENT_TYPE_MISMATCH` | 5 | 2 | `tests/controlFlowAnalysis/smartcastOnThisInInitializationSection.kt`, `tests/generics/varProjection/setterNotProjectedOutAssign.kt` |
| `CANNOT_WEAKEN_ACCESS_PRIVILEGE` | 5 | 4 | `tests/intersectedVisibilitiesAccessorDifferences_Small.kt`, `tests/intersectedVisibilitiesForNonAbstractPropertyAccessors.kt` |
| `DEPRECATION_ERROR` | 5 | 2 | `tests/deprecated/javaDeprecatedHidden.kt`, `tests/j+k/collectionOverrides/nonDirectHiddenOverride.kt` |
| `REIFIED_TYPE_FORBIDDEN_SUBSTITUTION` | 5 | 3 | `tests/generics/tpAsReified/CapturedAsReified.kt`, `tests/inference/constraints/ignoreConstraintFromImplicitInNothing.kt` |
| `RETURN_TYPE_MISMATCH` | 5 | 3 | `tests/inference/noInferUnit.kt`, `tests/j+k/rawTypesFromCaptured.kt` |
| `RETURN_TYPE_MISMATCH_ON_OVERRIDE` | 5 | 1 | `tests/inference/pcla/issues/kt47989b.kt`, `tests/j+k/types/newListMethodsNullabilityOverridesLinkedListLatest.kt` |
| `UPPER_BOUND_VIOLATED` | 5 | 2 | `tests/generics/RecursiveUpperBoundWithTwoArguments.kt`, `tests/inference/capturedTypes/kt49101.kt` |
| `ASSIGN_OPERATOR_AMBIGUITY` | 4 | 4 | `tests/callResolution/exponentialArraySetWithPlusAssign.kt`, `tests/operatorsOverloading/AssignOperatorAmbiguity.kt` |
| `CYCLIC_INHERITANCE_HIERARCHY` | 4 | 3 | `tests/classpathSubstitution/cyclicSupertype.kt`, `tests/classpathSubstitution/cyclicSupertype2.kt` |
| `INAPPLICABLE_CANDIDATE` | 4 | 3 | `tests/callableReference/generic/dependOnArgumentType.kt`, `tests/j+k/sam/referenceToSamFunctionAgainstExpectedType.kt` |
| `INCONSISTENT_TYPE_PARAMETER_BOUNDS` | 4 | 2 | `tests/enum/equalityOfEnumAndParameter.kt`, `tests/enum/equalityOfFlexibleTypeParameters.kt` |
| `MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES` | 4 | 4 | `tests/missingMultipleDefaultsOnTransitiveInheritance2.kt`, `tests/multipleDefaultsAnonymousObject.kt` |
| `MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES_WHEN_NO_EXPLICIT_OVERRIDE` | 4 | 4 | `tests/moreThanOneOverriddenMissingK2.kt`, `tests/multipleDefaultsNoSource.kt` |
| `OPT_IN_USAGE_ERROR` | 4 | 3 | `tests/jvm/jvmDefault/annotationsFromBinariesWithNonTrivialJvmDefaultConfiguration.kt`, `testsWithStdLib/experimental/diffrentModules.kt` |
| `OVERRIDING_FINAL_MEMBER` | 4 | 4 | `tests/j+k/finalCollectionSize.kt`, `tests/j+k/properties/isName.kt` |
| `POSITIONED_VALUE_ARGUMENT_FOR_JAVA_ANNOTATION` | 4 | 3 | `testsWithStdLib/annotations/annotationParameters/valueArrayAndOtherDefault.kt`, `testsWithStdLib/annotations/prohibitPositionedArgument/withValue.kt` |
| `RETURN_TYPE_MISMATCH_ON_INHERITANCE` | 4 | 4 | `tests/override/clashesOnInheritance/flexibleReturnTypeList.kt`, `tests/override/clashesOnInheritance/kt13355viaJava.kt` |
| `TYPE_MISMATCH` | 4 | 4 | `tests/inference/noInferUpperLower.kt`, `tests/platformTypes/nullabilityWarnings/delegation.kt` |
| `UNDERSCORE_USAGE_WITHOUT_BACKTICKS` | 4 | 4 | `tests/UnderscoreUsageInCall.kt`, `tests/UnderscoreUsageInCallableRefTypeLHS.kt` |
| `ABSTRACT_CLASS_MEMBER_NOT_IMPLEMENTED` | 3 | 3 | `tests/annotations/rendering/notImplementedMembersAroundJava.kt`, `tests/j+k/missingTypeOfParameterInFakeOverride_2.kt` |
| `ABSTRACT_MEMBER_NOT_IMPLEMENTED` | 3 | 3 | `tests/j+k/collectionOverrides/irrelevantImplCharSequenceKotlin.kt`, `tests/override/ProtectedAndPrivateFromSupertypes.kt` |
| `ABSTRACT_SUPER_CALL` | 3 | 3 | `tests/missingAbstractSuperCall.kt`, `tests/missingAbstractSuperCallWithClass.kt` |
| `CONFLICTING_INHERITED_JVM_DECLARATIONS` | 3 | 1 | `tests/jvm/contextParameters/intersectionContextValueJava.kt`, `tests/jvm/contextParameters/intersectionExtensionValueContextJava.kt` |
| `CONST_VAL_WITH_NON_CONST_INITIALIZER` | 3 | 3 | `tests/modifiers/const/fromJavaWithNonConstInitializer.kt`, `tests/modifiers/const/kotlinJavaCycle.kt` |
| `MANY_INTERFACES_MEMBER_NOT_IMPLEMENTED` | 3 | 2 | `tests/override/ParentInheritsManyImplementations.kt`, `tests/regressions/kt302.kt` |
| `METHOD_OF_ANY_IMPLEMENTED_IN_INTERFACE` | 3 | 3 | `tests/java8Overrides/implementingMethodOfAny.kt`, `tests/java8Overrides/overridingMethodOfAnyChain.kt` |
| `NONE_APPLICABLE` | 3 | 2 | `tests/secondaryConstructors/companionObjectScope.kt`, `tests/secondaryConstructors/generics3.kt` |
| `NO_ELSE_IN_WHEN` | 3 | 3 | `tests/sealed/WhenOnEmptySealed.kt`, `tests/testsWithJava17/sealedClasses/flexibleSealedInSubject.kt` |
| `PROPERTY_TYPE_MISMATCH_BY_DELEGATION` | 3 | 0 | `tests/delegation/covariantOverrides/fromClass.kt`, `tests/delegation/covariantOverrides/irrelevant.kt` |
| `PROTECTED_CALL_FROM_PUBLIC_INLINE_ERROR` | 3 | 3 | `tests/inline/inlineCallsStaticMethodFromOtherPackage.kt`, `tests/inline/protectedJavaFieldInInline.kt` |
| `REPEATED_ANNOTATION` | 3 | 3 | `tests/annotations/repeatable/javaUnrepeatable.kt`, `tests/typealias/typeAliasExpansionRepeatedAnnotations.kt` |
| `RETURN_TYPE_MISMATCH_BY_DELEGATION` | 3 | 0 | `tests/delegation/covariantOverrides/fromClass.kt`, `tests/delegation/covariantOverrides/irrelevant.kt` |
| `SYNTAX` | 3 | 2 | `tests/explicitDefinitelyNotNullableViaIntersection/isAsOperators.kt`, `tests/modifiers/primaryConstructorMissingBrackets.kt` |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 3 | 2 | `tests/j+k/selectMoreSpecific.kt`, `tests/platformTypes/commonSupertype/withNothing.kt` |
| `UPPER_BOUND_VIOLATED_IN_TYPEALIAS_EXPANSION` | 3 | 1 | `tests/typealias/boundsViolationInTypeAliasRHS.kt`, `tests/typealias/kt19601.kt` |
| `AMBIGUOUS_ANONYMOUS_TYPE_INFERRED` | 2 | 2 | `tests/declarationChecks/localObjectInInnerClass.kt`, `tests/exposed/ambiguousAnonymousExposed.kt` |
| `AMBIGUOUS_LABEL` | 2 | 2 | `tests/ambiguousLabelInExtensionFunctionWithOuterClass.kt`, `tests/ambiguousLabelInSuper.kt` |
| `CLASS_INHERITS_JAVA_SEALED_CLASS` | 2 | 2 | `tests/testsWithJava17/sealedClasses/kotlinInheritsJavaClass.kt`, `tests/testsWithJava17/sealedClasses/kotlinInheritsJavaInterface.kt` |
| `CONDITION_TYPE_MISMATCH` | 2 | 0 | `tests/platformTypes/nullabilityWarnings/conditions.kt`, `tests/syntax/complicatedLTGT.kt` |
| `CONFLICTING_UPPER_BOUNDS` | 2 | 0 | `tests/enum/equalityOfEnumAndParameter.kt`, `tests/enum/equalityOfFlexibleTypeParameters.kt` |
| `EXPANSIVE_INHERITANCE` | 2 | 2 | `tests/declarationChecks/nonExpansiveInheritanceRestriction/JavaWithKotlin.kt`, `tests/declarationChecks/nonExpansiveInheritanceRestriction/PureKotlin.kt` |
| `FINITE_BOUNDS_VIOLATION` | 2 | 2 | `tests/declarationChecks/finiteBoundRestriction/CasesWithTwoTypeParameters.kt`, `tests/regressions/kt9633.kt` |
| `INVISIBLE_SETTER` | 2 | 2 | `tests/extraCheckers/RedundantVisibilityModifierCheckerForInnerComponents.kt`, `tests/properties/kt47621.kt` |
| *101 more* | 120 | |

## Rejects valid

Tests kotlinc compiles that kc's checker rejects, by kc's first error, with names replaced by _.

| First error | Tests | Examples |
|:---|---:|:---|
| `type mismatch: inferred type is _ but _ was expected` | 57 | `tests/annotations/javaAnnotationWithClassArray.kt`, `tests/annotations/parameters/expressions/unaryPlus.kt` |
| `cannot infer type for type parameter '_'. Specify it explicitly.` | 39 | `tests/delegatedProperty/provideDelegate/commonSupertype.kt`, `tests/delegates/delegateWithLambda.kt` |
| `unresolved reference: _` | 31 | `tests/callResolution/incorrectPlusOperatorFromJava.kt`, `tests/callResolution/suspendCOnversionForReceiver.kt` |
| `'_' expression must be exhaustive, add necessary '_' branch` | 8 | `tests/controlStructures/whenWithNothingTypedSubject.kt`, `tests/exhaustiveness/positive/smartcastToExhaustiveInterface.kt` |
| `ambiguous call: n candidates for '_' take (_)` | 8 | `tests/callableReference/resolve/resolveReferenceAgainstKFunctionAndKPrpoerty.kt`, `tests/inference/emptyIntersectionTypes/kt51016.kt` |
| `no function '_' takes n arguments` | 6 | `tests/annotations/javaAnnotationWithParameter.kt`, `tests/cast/kt15161.kt` |
| `only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type _` | 6 | `tests/j+k/withTestConnection3.kt`, `tests/platformTypes/methodCall/multipleExactBoundsNullable.kt` |
| `unresolved import: _` | 6 | `tests/errorProneAtomicArraySuggestions.kt`, `tests/errorProneAtomicReferenceArguments.kt` |
| `expression '_' of type _ cannot be invoked as a function` | 5 | `tests/companionObjectCall.kt`, `tests/expresssions/invoke/incorrectInvokeReceiver.kt` |
| `'_' overrides nothing` | 3 | `tests/genericReceiverPropertyOverride.kt`, `tests/generics/finalUpperBoundWithOverride.kt` |
| `cannot infer a type for this parameter. Please specify it explicitly.` | 3 | `tests/inference/completion/postponedArgumentsAnalysis/lackOfDeepIncorporation.kt`, `tests/inference/kt36819.kt` |
| `none of the n candidates for '_' take (_)` | 3 | `tests/problems/kt52219.kt`, `testsWithStdLib/contracts/smartcasts/when/kt36818.kt` |
| `platform declaration clash: _` | 3 | `tests/overload/noConflictingOverloadsWithDeprecatedHidden/ConstructorsInMemberScope.kt`, `tests/overload/noConflictingOverloadsWithDeprecatedHidden/ConstructorsInPackage.kt` |
| `property must be initialized or be abstract` | 3 | `tests/backingField/FieldAsParam.kt`, `tests/backingField/FieldShadow.kt` |
| `type argument is not within its bounds: type parameter '_' must be subtype of '_', but actual: _` | 3 | `tests/inference/capturedTypes/capturedTypeWithInnerTypealias.kt`, `tests/inference/unsoundness2.kt` |
| `type checking has run into a recursive problem; specify the type of '_'` | 3 | `tests/checkers/uselessCastLeadsToRecursiveProblem.kt`, `tests/problems/kt27790.kt` |
| `'_' hides member of supertype '_' and needs '_' modifier` | 2 | `tests/targetedBuiltIns/backwardCompatibility/fillInStackTrace.kt`, `tests/visibility/notOverridingPackagePrivate.kt` |
| `'_' is not defined in this context` | 2 | `tests/functionLiterals/kt51317.kt`, `testsWithStdLib/mapList.kt` |
| `assignment operators ambiguity: '_' and '_' both apply` | 2 | `tests/operatorsOverloading/plusAssignOnVarAndCollections.kt`, `testsWithStdLib/java/intersectFlexibleAndMutable.kt` |
| `cannot create an instance of Function` | 2 | `tests/arguments/lambdaInLambda2.kt`, `tests/functionLiterals/lambdaInLambda2.kt` |
| `expression of type _ cannot be invoked as a function` | 2 | `tests/controlFlowAnalysis/deadCode/deadCallInInvokeCall.kt`, `tests/expresssions/invoke/safeCallsWithImplicitInvoke.kt` |
| `n type arguments expected for '_'` | 2 | `tests/delegatingConstructorsAndTypeAliases.kt`, `tests/generics/innerClasses/implicitArguments/secondLevelDepth.kt` |
| `type of '_' doesn't match the type of the overridden var-property` | 2 | `tests/extraCheckers/kt69190.kt`, `tests/smartCasts/intersectionScope/propertiesConflict.kt` |
| `unresolved label: _` | 2 | `tests/labelAndReceiverForInfix.kt`, `tests/labeledReturnFromNotLabeledUnnamedFunction.kt` |
| `'_' is deprecated with level HIDDEN` | 1 | `tests/problems/kt14059.kt` |
| `'_' modifier is required on '_'` | 1 | `tests/problems/kt36815.kt` |
| `'_' needs an explicit return type on '_'` | 1 | `tests/inference/completion/inferLabeledElementTypeForLambdaInReturnInCatch.kt` |
| `accidental override: the following declarations have the same JVM signature (foo()V)` | 1 | `tests/platformTypes/methodTypeParameterDefaultBound.kt` |
| `class '_' is not abstract and does not implement abstract member '_'` | 1 | `tests/j+k/collectionOverrides/mapMethodsImplementedInJava.kt` |
| `class Base is not public` | 1 | `tests/visibility/packagePrivateStatic.kt` |
| `class PackagePrivateGrandparentAbstractClass is not public` | 1 | `tests/visibility/packagePrivateStaticViaTypeAlias.kt` |
| `expected no parameters, but n were declared` | 1 | `tests/inference/extensionLambdasAndArrow.kt` |
| `expected parameter of type _` | 1 | `tests/resolve/starProjectionAndFunctionType.kt` |
| `function invocation '_' expected; use ::bar for a reference` | 1 | `tests/resolve/resolveAnnotatedLambdaArgument.kt` |
| `function invocation '_' expected; use ::foo for a reference` | 1 | `tests/syntax/incorrectLTGTFallback.kt` |
| `inherited platform declarations clash: the following declarations have the same JVM signature (foo(Ljava/lang/String;I)Ljava/lang/String;)` | 1 | `tests/contextParameters/overrides/KJKHierarchyWithOverride.kt` |
| `many supertypes available, please specify the one you mean in angle brackets, e.g. '_'` | 1 | `tests/override/kt53408.kt` |
| `missing return statement in function '_'` | 1 | `tests/inline/inlinedReturnInBranch.kt` |
| `named arguments are not allowed for non-Kotlin functions` | 1 | `testsWithStdLib/problems/NamedParameterInConcurrentMap.kt` |
| `no type arguments expected for class OuterClass` | 1 | `tests/signatureEnhancementOuterClassBoundsToInner.kt` |
| *16 more* | 16 | |

## Errors by diagnostic

For each expected error of the tests kc checked (not unsupported), whether kc reported an error on its line. Sorted by the errors kc missed.

kc reports 14516 of 21476 expected errors (67.6%) on their lines.

| Diagnostic | Expected | Reported | Missed |
|:---|---:|---:|---:|
| `SYNTAX` | 875 | 222 | 653 |
| `UNRESOLVED_REFERENCE` | 2493 | 1905 | 588 |
| `ARGUMENT_TYPE_MISMATCH` | 1484 | 990 | 494 |
| `CONFLICTING_JVM_DECLARATIONS` | 996 | 518 | 478 |
| `UNSUPPORTED` | 231 | 35 | 196 |
| `WRONG_MODIFIER_TARGET` | 300 | 170 | 130 |
| `CONFLICTING_OVERLOADS` | 726 | 602 | 124 |
| `RETURN_TYPE_MISMATCH` | 417 | 305 | 112 |
| `INVISIBLE_REFERENCE` | 513 | 404 | 109 |
| `NULL_FOR_NONNULL_TYPE` | 155 | 53 | 102 |
| `CANNOT_INFER_PARAMETER_TYPE` | 620 | 521 | 99 |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 250 | 157 | 93 |
| `CONST_VAL_WITH_NON_CONST_INITIALIZER` | 251 | 166 | 85 |
| `MUST_BE_INITIALIZED` | 102 | 21 | 81 |
| `ANNOTATION_ARGUMENT_MUST_BE_CONST` | 224 | 144 | 80 |
| `REDECLARATION` | 192 | 119 | 73 |
| `MISSING_DEPENDENCY_CLASS` | 71 | 0 | 71 |
| `TYPE_VARIANCE_CONFLICT_ERROR` | 123 | 54 | 69 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS` | 163 | 96 | 67 |
| `WRONG_ANNOTATION_TARGET` | 380 | 316 | 64 |
| `CANNOT_WEAKEN_ACCESS_PRIVILEGE` | 59 | 1 | 58 |
| `INITIALIZER_TYPE_MISMATCH` | 428 | 370 | 58 |
| `NO_RETURN_IN_FUNCTION_WITH_BLOCK_BODY` | 148 | 91 | 57 |
| `MIXING_SUSPEND_AND_NON_SUSPEND_SUPERTYPES` | 83 | 28 | 55 |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 153 | 98 | 55 |
| `NO_VALUE_FOR_PARAMETER` | 175 | 127 | 48 |
| `UPPER_BOUND_VIOLATED` | 125 | 78 | 47 |
| `NON_PUBLIC_CALL_FROM_PUBLIC_INLINE` | 65 | 19 | 46 |
| `UNINITIALIZED_VARIABLE` | 211 | 165 | 46 |
| `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` | 87 | 45 | 42 |
| `UNSAFE_CALL` | 523 | 481 | 42 |
| `TYPE_INFERENCE_ONLY_INPUT_TYPES_ERROR` | 61 | 21 | 40 |
| `INAPPLICABLE_OPERATOR_MODIFIER` | 143 | 104 | 39 |
| `TOO_MANY_ARGUMENTS` | 118 | 81 | 37 |
| `AMBIGUOUS_ANONYMOUS_TYPE_INFERRED` | 38 | 2 | 36 |
| `ASSIGNMENT_TYPE_MISMATCH` | 114 | 78 | 36 |
| `MUST_BE_INITIALIZED_OR_BE_ABSTRACT` | 79 | 44 | 35 |
| `CONFUSING_BRANCH_CONDITION_ERROR` | 37 | 3 | 34 |
| `EXPLICIT_TYPE_ARGUMENTS_IN_PROPERTY_ACCESS` | 37 | 5 | 32 |
| `NON_PUBLIC_INLINE_CALL_FROM_PUBLIC_INLINE` | 60 | 28 | 32 |
| `INAPPLICABLE_JVM_FIELD` | 33 | 2 | 31 |
| `OPT_IN_USAGE_ERROR` | 96 | 66 | 30 |
| `ASSIGNMENT_IN_EXPRESSION_CONTEXT` | 32 | 3 | 29 |
| `NOTHING_TO_OVERRIDE` | 93 | 64 | 29 |
| `REPEATED_MODIFIER` | 39 | 10 | 29 |
| `JVM_EXPOSE_BOXED_CANNOT_EXPOSE_PRIVATE` | 28 | 0 | 28 |
| `WRONG_ANNOTATION_TARGET_WITH_USE_SITE_TARGET` | 80 | 52 | 28 |
| `UNDERSCORE_USAGE_WITHOUT_BACKTICKS` | 28 | 1 | 27 |
| `TYPE_ARGUMENTS_FOR_OUTER_CLASS_WHEN_NESTED_REFERENCED` | 41 | 15 | 26 |
| `INAPPLICABLE_CANDIDATE` | 123 | 98 | 25 |
| `SUBCLASS_OPT_IN_INAPPLICABLE` | 25 | 0 | 25 |
| `CONCURRENT_HASH_MAP_CONTAINS_OPERATOR_ERROR` | 24 | 0 | 24 |
| `SMARTCAST_IMPOSSIBLE` | 183 | 159 | 24 |
| `ASSIGN_OPERATOR_AMBIGUITY` | 35 | 12 | 23 |
| `PROPERTY_INITIALIZER_NO_BACKING_FIELD` | 32 | 9 | 23 |
| `UNINITIALIZED_ENUM_COMPANION` | 65 | 42 | 23 |
| `CLASSIFIER_REDECLARATION` | 41 | 20 | 21 |
| `INLINE_FROM_HIGHER_PLATFORM` | 21 | 0 | 21 |
| `REPEATED_ANNOTATION` | 69 | 48 | 21 |
| `RETURN_TYPE_MISMATCH_ON_OVERRIDE` | 41 | 20 | 21 |
| *532 more* | | | |

## Errors kotlinc doesn't report

kc's errors on lines where the markup expects no diagnostic, in the tests it checked, with names replaced by _. In a test kotlinc compiles, one is a false error; in one it rejects, it can also be the same error on another line, or one kotlinc doesn't report after a first.

| Error | Count |
|:---|---:|
| `cannot infer type for type parameter '_'. Specify it explicitly.` | 590 |
| `type mismatch: inferred type is _ but _ was expected` | 329 |
| `unresolved reference: _` | 315 |
| `only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type _` | 213 |
| `platform declaration clash: _` | 150 |
| `class '_' is not abstract and does not implement abstract member '_'` | 68 |
| `'_' expression must be exhaustive, add necessary '_' branch` | 62 |
| `unresolved or unsupported type: _` | 59 |
| `cannot infer a type for this parameter. Please specify it explicitly.` | 55 |
| `suspend function '_' cannot override non-suspend function '_' defined in '_'` | 50 |
| `missing return statement in function '_'` | 38 |
| `unresolved import: _` | 35 |
| `no function '_' takes n arguments` | 27 |
| `ambiguous call: n candidates for '_' take (_)` | 18 |
| `variable '_' must be initialized` | 17 |
| `type checking has run into a recursive problem; specify the type of '_'` | 16 |
| `'_' overrides nothing` | 15 |
| `expected expression, found '_'` | 15 |
| `expected n parameter of type _ but n were declared` | 14 |
| `not enough information to infer type arguments of ::foon: specify the expected function type` | 14 |
| `operator '_' cannot be applied to Any?` | 14 |
| `expected parameter of type _` | 13 |
| `assignment operators ambiguity: '_' and '_' both apply` | 12 |
| `expected no parameters, but n were declared` | 12 |
| `private setters are not allowed for open properties` | 12 |
| `this lambda must return a value of type _` | 12 |
| `'_' modifier is required on '_'` | 11 |
| `not enough information to infer type arguments of ::barn: specify the expected function type` | 11 |
| `overload resolution ambiguity for ::foo: specify the expected function type` | 11 |
| `val cannot be reassigned` | 11 |
| *191 more* | |

## Unsupported features

By the features the parser (with error recovery) and the checker found. A test counts for each.

| Feature | Tests | Only blocker | Examples |
|:---|---:|---:|:---|
| `property reference` | 5 | 5 | `tests/callableReference/property/javaStaticFieldViaImport.kt`, `tests/callableReference/unsupported/localVariableWithSubstitution.kt` |
| `destructuring declaration` | 4 | 4 | `tests/destructuring/topLevelWithLambda.kt`, `tests/diagnostics/conflictingOverloads.kt` |
| `checker: '_' is not allowed here: local interfaces are not supported` | 2 | 2 | `tests/localInterfaces.kt`, `tests/thisAndSuper/unqualifiedSuper/unqualifiedSuperWithLocalClass.kt` |
| `unsupported types in signature` | 2 | 2 | `tests/javac/qualifiedExpression/PackageVsClass2.kt`, `tests/syntheticExtensions/samAdapters/InnerClassInGeneric.kt` |
| `callable reference` | 1 | 1 | `tests/callableReference/compatibilityResolveWithVarargAndOperatorCall.kt` |
| `checker: lambdas with more than n parameters are not supported` | 1 | 1 | `tests/tooLargeFunctionNThroughACallableReference.kt` |
| `package` | 1 | 1 | `tests/modifiers/IllegalModifiers.kt` |
| `parse error: expected '_', found '_'` | 1 | 1 | `tests/innerClasses/inCallableReferenceLHS/withParenthesizes.kt` |
| `parse error: expected expression, found '_'` | 1 | 1 | `tests/innerClasses/inCallableReferenceLHS/misplacedArgumentsInChain.kt` |

### Greedy order

At each step, the feature that unblocks the most tests given the ones before it.

| Step | Feature | Tests unblocked | Total |
|---:|:---|---:|---:|
| 1 | `property reference` | 5 | 5 |
| 2 | `destructuring declaration` | 4 | 9 |
| 3 | `checker: '_' is not allowed here: local interfaces are not supported` | 2 | 11 |
| 4 | `unsupported types in signature` | 2 | 13 |
| 5 | `callable reference` | 1 | 14 |
| 6 | `checker: lambdas with more than n parameters are not supported` | 1 | 15 |
| 7 | `package` | 1 | 16 |
| 8 | `parse error: expected '_', found '_'` | 1 | 17 |
| 9 | `parse error: expected expression, found '_'` | 1 | 18 |

## Crashes

| Panic | Tests | Examples |
|:---|---:|:---|
| `stack overflow in types.Subst.applyA (generic.go:445)` | 1 | `tests/j+k/samByProjectedType/javaSamTypeWithRecursionInBounds.kt` |

## Warnings

kc reports no warnings. The warnings the tests run expect, as a backlog.

| Warning | Count |
|:---|---:|
| `REDUNDANT_INTERPOLATION_PREFIX` | 355 |
| `DEPRECATION` | 199 |
| `UNREACHABLE_CODE` | 158 |
| `IMPLICIT_BOXING_IN_IDENTITY_EQUALS` | 152 |
| `UNUSED_EXPRESSION` | 136 |
| `USELESS_IS_CHECK` | 127 |
| `ERROR_SUPPRESSION` | 102 |
| `NOTHING_TO_INLINE` | 95 |
| `UNNECESSARY_NOT_NULL_ASSERTION` | 93 |
| `CAST_NEVER_SUCCEEDS` | 92 |
| `USELESS_CAST` | 91 |
| `SENSELESS_COMPARISON` | 90 |
| `UNNECESSARY_SAFE_CALL` | 90 |
| `UNCHECKED_CAST` | 87 |
| `OPT_IN_USAGE` | 83 |
| `CONTEXTUAL_OVERLOAD_SHADOWED` | 75 |
| `ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY` | 70 |
| `FINAL_UPPER_BOUND` | 69 |
| `EXTENSION_SHADOWED_BY_MEMBER` | 67 |
| `PLATFORM_CLASS_MAPPED_TO_KOTLIN` | 62 |
| `NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL` | 59 |
| `IMPOSSIBLE_IS_CHECK_WARNING` | 57 |
| `REDUNDANT_PROJECTION` | 49 |
| `USELESS_ELVIS` | 44 |
| `NO_REFLECTION_IN_CLASS_PATH` | 36 |
| `UNUSED_VARIABLE` | 35 |
| `ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY` | 33 |
| `ASSIGNED_VALUE_IS_NEVER_READ` | 32 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING` | 31 |
| `OVERRIDE_DEPRECATION` | 30 |
| *105 more* | |

