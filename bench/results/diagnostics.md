# Kotlin's diagnostics tests on kc

The diagnostics tests of JetBrains/kotlin v2.4.21 for the JVM (`compiler/testData/diagnostics/`: tests/, testsWithStdLib/, testsWithJvmBackend/, testsWithAnyBackend/), each compiled by kc. A test's markup marks the diagnostics kotlinc reports, and its `RUN_PIPELINE_TILL` says whether kotlinc stops with errors in the frontend (`FRONTEND`, `FIR2IR`) or goes on to the backend (`BACKEND`). `bench/kotlin-tests.sh diagnostics` writes this file, and [docs/conformance.md](../../docs/conformance.md) explains it.

Of 10827 test files, 3541 are skipped and 7286 run.

| Result | Tests | Share |
|:---|---:|---:|
| rejects invalid | 3616 | 49.6% |
| accepts valid | 3134 | 43.0% |
| accepts invalid | 263 | 3.6% |
| rejects valid | 255 | 3.5% |
| parse/unsupported | 18 | 0.2% |
| crash | 0 | 0.0% |

*Rejects invalid* and *accepts valid* agree with kotlinc. *Accepts invalid* is code kotlinc rejects that kc compiles; *rejects valid*, code kotlinc compiles that kc's checker rejects. Both are bugs. A *parse/unsupported* test uses a feature kc doesn't support, which says nothing about its checks.

Of the 3616 tests both reject, kc reports an error on the line of an expected one in 3401.

A test is invalid if it stops before the backend or marks an error: one that runs to it (`BACKEND`) can mark the JVM backend's errors, such as `CONFLICTING_JVM_DECLARATIONS`. Of the tests run, 177 stop at `FRONTEND` but mark no error: most add the `CHECK_TYPE` helpers, whose names (`_`) kotlinc reports outside the test infrastructure, and count as valid; the others leave their errors out with the `DIAGNOSTICS` directive. 101 run to the backend and mark one. 5 marked names have no declared severity (debug information, for one), and count as neither.

The names without a severity: `DEBUG_INFO_EXPRESSION_TYPE` (680), `DEBUG_INFO_CSR_MIGHT_BE_USED` (44), `DEBUG_INFO_CALL` (26), `DEBUG_INFO_CSR_MIGHT_BE_USED_INSTEAD_OF_IMPORT` (16), `DEBUG_INFO_CALLABLE_OWNER` (4).

## Skipped

Tests for other platforms, that need language features off by default or compiler options, Java sources, or modules other than plain dependencies, or whose FIR diagnostics aren't checked.

| Reason | Tests | Examples |
|:---|---:|:---|
| `LANGUAGE` | 1579 | `tests/BacktickNames.kt`, `tests/BacktickNamesOld.kt` |
| `Java sources` | 1304 | `tests/IsExpressions.kt`, `tests/SafeCallOnFakePackage.kt` |
| `modules with friends or dependsOn` | 322 | `tests/jvm/internal/internalFromFriendModule.kt`, `tests/multimodule/friendModule.kt` |
| `LANGUAGE_FEATURE_TOGGLED` | 87 | `tests/annotatedPackageQualifiers.kt`, `tests/annotationUsedAsAnnotationArgument.kt` |
| `return value checker (crv/, crvFull/)` | 72 | `tests/crv/assignments.kt`, `tests/crv/booleanShortcut.kt` |
| `API_VERSION` | 53 | `tests/deprecated/deprecatedSinceKotlin/error.kt`, `tests/deprecated/deprecatedSinceKotlin/hidden.kt` |
| `ALLOW_KOTLIN_PACKAGE` | 47 | `tests/annotations/requireKotlin.kt`, `tests/crvDisabled/oldMustUse.kt` |
| `explicit API mode` | 21 | `tests/testsWithExplicitApi/annotations.kt`, `tests/testsWithExplicitApi/classes.kt` |
| `JDK_KIND` | 17 | `tests/errorProneAtomicArrayPrimitivesAllJavaMethods.kt`, `tests/identityComparisonOnJavaValueBasedClassesWarning.kt` |
| `IGNORE_FIR_DIAGNOSTICS` | 12 | `tests/jvm/contextParameters/kt59590WithContextParameters.kt`, `tests/jvm/duplicateJvmSignature/accidentalOverrides/overridesNothing.kt` |
| `header mode` | 9 | `tests/headerMode/anonymousObjects.kt`, `tests/headerMode/classDeclaration.kt` |
| `-jvm-default modes` | 6 | `testsWithStdLib/annotations/jvmDefault/generic.kt`, `testsWithStdLib/annotations/jvmDefault/jvmDefaultWithCompatibility/disable.kt` |
| `JVM_DEFAULT_MODE: enable` | 4 | `tests/jvm/jvmDefault/compatibilityAgainstJava.kt`, `testsWithStdLib/annotations/jvmDefault/allCompatibility/specialization.kt` |
| `multiplatform` | 3 | `tests/multiplatform/errorProneAtomicArrayArgumentsInKMP.kt`, `tests/multiplatform/errorProneAtomicReferenceArgumentsInKMP.kt` |
| `TARGET_BACKEND: NATIVE, JS_IR, WASM` | 2 | `testsWithAnyBackend/inlineOverrideDefaultWarning.kt`, `testsWithAnyBackend/inlineOverrideDefaultWarningWithInlinedFunInKlib.kt` |
| `strict equality` | 2 | `tests/strictEquals/enabled/simple.kt`, `tests/strictEquals/enabled/wrongElementsAnnotated.kt` |
| `TARGET_BACKEND: NATIVE, JS_IR` | 1 | `testsWithAnyBackend/crossFileInlineCycle.kt` |

## Accepts invalid

Tests kotlinc rejects that kc compiles, by the errors their markup expects. A test counts for each; *Only error* counts the tests it is the only expected error of, which implementing it alone would fix.

| Expected error | Tests | Only error | Examples |
|:---|---:|---:|:---|
| `ARGUMENT_TYPE_MISMATCH` | 26 | 16 | `tests/annotations/annotationArgumentScopeForNestedClass.kt`, `tests/collectionLiterals/kt82964.kt` |
| `UNSAFE_CALL` | 19 | 17 | `tests/dataFlow/assignment/assignToNewVal.kt`, `tests/dataFlow/assignment/uninitializedValNullability.kt` |
| `UNRESOLVED_REFERENCE` | 14 | 10 | `tests/callResolution/eagerLambdaAnalysis/morePreciseSamVsLessPreciseFunctionType.kt`, `tests/imports/ImportFromCurrentWithDifferentName.kt` |
| `CANNOT_INFER_PARAMETER_TYPE` | 9 | 3 | `tests/callableReference/function/ambiguityTopLevelVsTopLevelGeneric.kt`, `tests/generics/kt30590.kt` |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 8 | 7 | `tests/callResolution/kt50386.kt`, `tests/inference/pcla/overloadResolutionByLambdaReturnType.kt` |
| `UNSUPPORTED` | 8 | 5 | `tests/NamedFunctionTypeParameterInSupertype.kt`, `tests/callableReference/property/backingField.kt` |
| `INITIALIZER_TYPE_MISMATCH` | 7 | 4 | `tests/callableReference/adapted/noKFunctionForAdaptation.kt`, `tests/functionAsExpression/NonSuspendAnonFuncAsInitializerOfSuspendFunctionType.kt` |
| `SMARTCAST_IMPOSSIBLE` | 7 | 4 | `tests/smartCasts/lambdaAndArgumentFun.kt`, `tests/smartCasts/localClassChanges.kt` |
| `MISSING_DEPENDENCY_CLASS` | 6 | 3 | `tests/multimodule/AnonymousFunctionParametersOfInaccessibleTypes.kt`, `tests/multimodule/BoxedInaccessibleGenericTypeUsedByDependencies.kt` |
| `TYPE_INFERENCE_ONLY_INPUT_TYPES_ERROR` | 6 | 5 | `testsWithStdLib/commonCollections.kt`, `testsWithStdLib/inference/annotationsForResolve/kt29307.kt` |
| `CANNOT_WEAKEN_ACCESS_PRIVILEGE` | 5 | 4 | `tests/intersectedVisibilitiesAccessorDifferences_Small.kt`, `tests/intersectedVisibilitiesForNonAbstractPropertyAccessors.kt` |
| `NULL_FOR_NONNULL_TYPE` | 5 | 2 | `tests/j+k/types/newListMethodsNullabilityLinkedListLatest.kt`, `tests/targetedBuiltIns/concurrentMapRemove.kt` |
| `REIFIED_TYPE_FORBIDDEN_SUBSTITUTION` | 5 | 3 | `tests/generics/tpAsReified/CapturedAsReified.kt`, `tests/inference/constraints/ignoreConstraintFromImplicitInNothing.kt` |
| `UPPER_BOUND_VIOLATED` | 5 | 2 | `tests/generics/RecursiveUpperBoundWithTwoArguments.kt`, `tests/inference/capturedTypes/kt49101.kt` |
| `ASSIGN_OPERATOR_AMBIGUITY` | 4 | 4 | `tests/callResolution/exponentialArraySetWithPlusAssign.kt`, `tests/operatorsOverloading/AssignOperatorAmbiguity.kt` |
| `MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES` | 4 | 4 | `tests/missingMultipleDefaultsOnTransitiveInheritance2.kt`, `tests/multipleDefaultsAnonymousObject.kt` |
| `MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES_WHEN_NO_EXPLICIT_OVERRIDE` | 4 | 4 | `tests/moreThanOneOverriddenMissingK2.kt`, `tests/multipleDefaultsNoSource.kt` |
| `NOTHING_TO_OVERRIDE` | 4 | 1 | `tests/j+k/types/newListMethodsNullabilityOverridesLinkedListLatest.kt`, `tests/multimodule/duplicateSuper/sameSuperTraitDifferentBounds.kt` |
| `OPT_IN_USAGE_ERROR` | 4 | 3 | `tests/jvm/jvmDefault/annotationsFromBinariesWithNonTrivialJvmDefaultConfiguration.kt`, `testsWithStdLib/experimental/diffrentModules.kt` |
| `RETURN_TYPE_MISMATCH_ON_OVERRIDE` | 4 | 0 | `tests/inference/pcla/issues/kt47989b.kt`, `tests/j+k/types/newListMethodsNullabilityOverridesLinkedListLatest.kt` |
| `UNDERSCORE_USAGE_WITHOUT_BACKTICKS` | 4 | 4 | `tests/UnderscoreUsageInCall.kt`, `tests/UnderscoreUsageInCallableRefTypeLHS.kt` |
| `(no error marked)` | 3 | 3 | `tests/nullableTypes/nullAssertOnTypeWithNullableUpperBound.kt`, `tests/nullableTypes/safeCallOnTypeWithNullableUpperBound.kt` |
| `ABSTRACT_SUPER_CALL` | 3 | 3 | `tests/missingAbstractSuperCall.kt`, `tests/missingAbstractSuperCallWithClass.kt` |
| `CYCLIC_INHERITANCE_HIERARCHY` | 3 | 3 | `tests/classpathSubstitution/cyclicSupertype.kt`, `tests/classpathSubstitution/cyclicSupertype2.kt` |
| `DEPRECATION_ERROR` | 3 | 2 | `tests/j+k/collectionOverrides/nonDirectHiddenOverride.kt`, `tests/jvm/jvmDefault/annotationsFromBinariesWithNonTrivialJvmDefaultConfiguration.kt` |
| `INCONSISTENT_TYPE_PARAMETER_BOUNDS` | 3 | 2 | `tests/enum/equalityOfEnumAndParameter.kt`, `tests/generics/InconsistentTypeParameterBounds.kt` |
| `INVISIBLE_REFERENCE` | 3 | 3 | `tests/classObjects/companionObjectOfPrivateClassVisibility.kt`, `tests/resolve/invoke/invokeOnInvisiblePropertyOfFunctionType.kt` |
| `MANY_IMPL_MEMBER_NOT_IMPLEMENTED` | 3 | 2 | `tests/override/FakeOverrideDifferentDeclarationSignatures.kt`, `tests/varOverriddenByValThroughIntersection.kt` |
| `MANY_INTERFACES_MEMBER_NOT_IMPLEMENTED` | 3 | 2 | `tests/override/ParentInheritsManyImplementations.kt`, `tests/regressions/kt302.kt` |
| `METHOD_OF_ANY_IMPLEMENTED_IN_INTERFACE` | 3 | 3 | `tests/java8Overrides/implementingMethodOfAny.kt`, `tests/java8Overrides/overridingMethodOfAnyChain.kt` |
| `PROPERTY_TYPE_MISMATCH_BY_DELEGATION` | 3 | 0 | `tests/delegation/covariantOverrides/fromClass.kt`, `tests/delegation/covariantOverrides/irrelevant.kt` |
| `RETURN_TYPE_MISMATCH` | 3 | 2 | `tests/inference/noInferUnit.kt`, `tests/smartCasts/safecalls/receiverAndChainFalse.kt` |
| `RETURN_TYPE_MISMATCH_BY_DELEGATION` | 3 | 0 | `tests/delegation/covariantOverrides/fromClass.kt`, `tests/delegation/covariantOverrides/irrelevant.kt` |
| `SYNTAX` | 3 | 2 | `tests/explicitDefinitelyNotNullableViaIntersection/isAsOperators.kt`, `tests/modifiers/primaryConstructorMissingBrackets.kt` |
| `UPPER_BOUND_VIOLATED_IN_TYPEALIAS_EXPANSION` | 3 | 1 | `tests/typealias/boundsViolationInTypeAliasRHS.kt`, `tests/typealias/kt19601.kt` |
| `AMBIGUOUS_ANONYMOUS_TYPE_INFERRED` | 2 | 2 | `tests/declarationChecks/localObjectInInnerClass.kt`, `tests/exposed/ambiguousAnonymousExposed.kt` |
| `AMBIGUOUS_LABEL` | 2 | 2 | `tests/ambiguousLabelInExtensionFunctionWithOuterClass.kt`, `tests/ambiguousLabelInSuper.kt` |
| `ASSIGNMENT_TYPE_MISMATCH` | 2 | 1 | `tests/controlFlowAnalysis/smartcastOnThisInInitializationSection.kt`, `tests/generics/varProjection/setterNotProjectedOutAssign.kt` |
| `FINITE_BOUNDS_VIOLATION` | 2 | 2 | `tests/declarationChecks/finiteBoundRestriction/CasesWithTwoTypeParameters.kt`, `tests/regressions/kt9633.kt` |
| `MEMBER_PROJECTED_OUT` | 2 | 2 | `tests/scopes/variantProjections/irrelevantUnsafeVariance.kt`, `tests/typealias/starProjection.kt` |
| `MISSING_DEPENDENCY_SUPERCLASS` | 2 | 1 | `tests/dataClasses/missingDependencyInSupertype.kt`, `tests/multimodule/inaccessibleSuperTypeWithRepeatedContext.kt` |
| `NONE_APPLICABLE` | 2 | 2 | `tests/secondaryConstructors/companionObjectScope.kt`, `tests/secondaryConstructors/generics3.kt` |
| `NOT_FUNCTION_AS_OPERATOR` | 2 | 2 | `tests/kt34857.kt`, `tests/problems/kt12610.kt` |
| `NOT_YET_SUPPORTED_IN_INLINE` | 2 | 2 | `tests/inline/defaultLambdaInlineSuspend.kt`, `tests/inlineFunctionContextBug.kt` |
| `REPEATED_ANNOTATION` | 2 | 2 | `tests/typealias/typeAliasExpansionRepeatedAnnotations.kt`, `tests/typealias/typealiasRhsRepeatedAnnotationInArguments.kt` |
| `RETURN_TYPE_MISMATCH_ON_INHERITANCE` | 2 | 2 | `tests/override/clashesOnInheritance/returnTypeMismatch.kt`, `tests/override/kt4763.kt` |
| `SETTER_VISIBILITY_INCONSISTENT_WITH_PROPERTY_VISIBILITY` | 2 | 2 | `tests/SetterVisibility.kt`, `tests/lateinit/setter.kt` |
| `TYPE_MISMATCH` | 2 | 2 | `tests/inference/noInferUpperLower.kt`, `testsWithStdLib/inference/annotationsForResolve/noInferAndLowPriority.kt` |
| `TYPE_PARAMETERS_IN_ENUM` | 2 | 2 | `tests/diagnostics/typeParametersInEnum.kt`, `tests/enum/typeParametersInEnum.kt` |
| `TYPE_VARIANCE_CONFLICT_ERROR` | 2 | 2 | `tests/typealias/kt14498a.kt`, `tests/variance/Class.kt` |
| `UNDERSCORE_IS_RESERVED` | 2 | 2 | `tests/labeledLambda.kt`, `tests/unnamedParameterOnOverride.kt` |
| `UNINITIALIZED_ENUM_ENTRY` | 2 | 2 | `tests/controlFlowAnalysis/enumInterdependence.kt`, `tests/controlFlowAnalysis/uninitializedQualifiedEnumEntry.kt` |
| `UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH` | 2 | 2 | `tests/inlineClasses/unsignedLiteralsWithoutArtifactOnClasspath.kt`, `tests/valueClasses/unsignedLiteralsWithoutArtifactOnClasspath.kt` |
| `ABBREVIATED_NOTHING_PROPERTY_TYPE` | 1 | 0 | `tests/typealias/returnTypeNothingShouldBeSpecifiedExplicitly.kt` |
| `ABBREVIATED_NOTHING_RETURN_TYPE` | 1 | 0 | `tests/typealias/returnTypeNothingShouldBeSpecifiedExplicitly.kt` |
| `ABSTRACT_CLASS_MEMBER_NOT_IMPLEMENTED` | 1 | 1 | `tests/override/kt12358.kt` |
| `ABSTRACT_MEMBER_NOT_IMPLEMENTED` | 1 | 1 | `tests/override/ProtectedAndPrivateFromSupertypes.kt` |
| `ADAPTED_CALLABLE_REFERENCE_AGAINST_REFLECTION_TYPE` | 1 | 0 | `tests/callableReference/adapted/noKFunctionForAdaptation.kt` |
| `AMBIGUOUS_SUPER` | 1 | 1 | `tests/thisAndSuper/unqualifiedSuper/unqualifiedSuperWithCallableProperty.kt` |
| `ANNOTATION_IN_CONTRACT_ERROR` | 1 | 1 | `tests/contracts/suppressContractWarning.kt` |
| *71 more* | 71 | |

## Rejects valid

Tests kotlinc compiles that kc's checker rejects, by kc's first error, with names replaced by _.

| First error | Tests | Examples |
|:---|---:|:---|
| `type mismatch: inferred type is _ but _ was expected` | 66 | `tests/annotations/parameters/expressions/unaryPlus.kt`, `tests/callableReference/function/localConstructor.kt` |
| `cannot infer type for type parameter '_'. Specify it explicitly.` | 51 | `tests/callableReference/generic/kt35896.kt`, `tests/callableReference/genericCallWithReferenceAgainstVararg.kt` |
| `unresolved reference: _` | 27 | `tests/callResolution/suspendCOnversionForReceiver.kt`, `tests/callableReference/bound/controlFlow.kt` |
| `ambiguous call: n candidates for '_' take (_)` | 14 | `tests/callableReference/resolve/kt35887.kt`, `tests/contextSensitiveResolutionUsingExpectedType/ideHint/forExpressions/multipleOverloads.kt` |
| `'_' expression must be exhaustive, add necessary '_' branch` | 7 | `tests/controlStructures/whenWithNothingTypedSubject.kt`, `tests/exhaustiveness/positive/smartcastToExhaustiveInterface.kt` |
| `no function '_' takes n arguments` | 6 | `tests/cast/kt15161.kt`, `tests/errorProneAtomicArrayArguments.kt` |
| `expression '_' of type _ cannot be invoked as a function` | 5 | `tests/companionObjectCall.kt`, `tests/expresssions/invoke/incorrectInvokeReceiver.kt` |
| `none of the n candidates for '_' take (_)` | 4 | `tests/callableReference/noAmbiguityWhenAllReferencesAreInapplicable.kt`, `tests/problems/kt52219.kt` |
| `only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type _` | 4 | `tests/regressions/Jet169.kt`, `tests/secondaryConstructors/dataFlowInDelegationCall.kt` |
| `unresolved import: _` | 4 | `tests/errorProneAtomicArraySuggestions.kt`, `tests/errorProneAtomicReferenceArguments.kt` |
| `'_' overrides nothing` | 3 | `tests/genericReceiverPropertyOverride.kt`, `tests/generics/finalUpperBoundWithOverride.kt` |
| `overload resolution ambiguity for B::foo: specify the expected function type` | 3 | `tests/callableReference/callableReferenceWithMostSpecificGenericTypeParameter.kt`, `tests/callableReference/resolve/kt9601.kt` |
| `platform declaration clash: _` | 3 | `tests/overload/noConflictingOverloadsWithDeprecatedHidden/ConstructorsInMemberScope.kt`, `tests/overload/noConflictingOverloadsWithDeprecatedHidden/ConstructorsInPackage.kt` |
| `property must be initialized or be abstract` | 3 | `tests/backingField/FieldAsParam.kt`, `tests/backingField/FieldShadow.kt` |
| `type argument is not within its bounds: type parameter '_' must be subtype of '_', but actual: _` | 3 | `tests/inference/capturedTypes/capturedTypeWithInnerTypealias.kt`, `tests/inference/unsoundness2.kt` |
| `type checking has run into a recursive problem; specify the type of '_'` | 3 | `tests/checkers/uselessCastLeadsToRecursiveProblem.kt`, `tests/problems/kt27790.kt` |
| `'_' is not defined in this context` | 2 | `tests/functionLiterals/kt51317.kt`, `testsWithStdLib/mapList.kt` |
| `expression of type _ cannot be invoked as a function` | 2 | `tests/controlFlowAnalysis/deadCode/deadCallInInvokeCall.kt`, `tests/expresssions/invoke/safeCallsWithImplicitInvoke.kt` |
| `n type arguments expected for '_'` | 2 | `tests/delegatingConstructorsAndTypeAliases.kt`, `tests/generics/innerClasses/implicitArguments/secondLevelDepth.kt` |
| `too many arguments for annotation java/lang/annotation/Target` | 2 | `tests/annotations/JavaAnnotationConstructors.kt`, `testsWithStdLib/annotations/annotationIncompatibleTargets.kt` |
| `type of '_' doesn't match the type of the overridden var-property` | 2 | `tests/extraCheckers/kt69190.kt`, `tests/smartCasts/intersectionScope/propertiesConflict.kt` |
| `unresolved label: _` | 2 | `tests/labelAndReceiverForInfix.kt`, `tests/labeledReturnFromNotLabeledUnnamedFunction.kt` |
| `'_' hides member of supertype '_' and needs '_' modifier` | 1 | `tests/targetedBuiltIns/backwardCompatibility/fillInStackTrace.kt` |
| `'_' is deprecated with level HIDDEN` | 1 | `tests/problems/kt14059.kt` |
| `'_' modifier is required on '_'` | 1 | `tests/problems/kt36815.kt` |
| `'_' needs an explicit return type on '_'` | 1 | `tests/inference/completion/inferLabeledElementTypeForLambdaInReturnInCatch.kt` |
| `assignment operators ambiguity: '_' and '_' both apply` | 1 | `tests/operatorsOverloading/plusAssignOnVarAndCollections.kt` |
| `cannot infer a type for this parameter. Please specify it explicitly.` | 1 | `tests/inference/completion/postponedArgumentsAnalysis/lackOfDeepIncorporation.kt` |
| `expected no parameters, but n were declared` | 1 | `tests/inference/extensionLambdasAndArrow.kt` |
| `expected parameter of type _` | 1 | `tests/resolve/starProjectionAndFunctionType.kt` |
| `function invocation '_' expected; use ::bar for a reference` | 1 | `tests/resolve/resolveAnnotatedLambdaArgument.kt` |
| `function invocation '_' expected; use ::foo for a reference` | 1 | `tests/syntax/incorrectLTGTFallback.kt` |
| `many supertypes available, please specify the one you mean in angle brackets, e.g. '_'` | 1 | `tests/override/kt53408.kt` |
| `missing return statement in function '_'` | 1 | `tests/inline/inlinedReturnInBranch.kt` |
| `named arguments are not allowed for non-Kotlin functions` | 1 | `testsWithStdLib/problems/NamedParameterInConcurrentMap.kt` |
| `one type argument expected for class B<T>` | 1 | `tests/multimodule/duplicateMethod/substitutedGenericInParams.kt` |
| `one type argument expected for class X.Y` | 1 | `tests/innerClasses/inCallableReferenceLHS/misplacedArgumentsInInnerOnly.kt` |
| `one type argument expected for typealias TA<T>` | 1 | `tests/upperBoundsViolatedInLhsOfGetClass.kt` |
| `operator '_' cannot be applied to A and B` | 1 | `tests/inference/emptyIntersectionTypes/emptyIntersectionOnIf.kt` |
| `operator '_' cannot be applied to A<*> and B<*>` | 1 | `tests/comparisonOfGenericInterfaceWithGenericClass.kt` |
| *19 more* | 19 | |

## Errors by diagnostic

For each expected error of the tests kc checked (not unsupported), whether kc reported an error on its line. Sorted by the errors kc missed.

kc reports 13379 of 19696 expected errors (67.9%) on their lines.

| Diagnostic | Expected | Reported | Missed |
|:---|---:|---:|---:|
| `SYNTAX` | 872 | 222 | 650 |
| `UNRESOLVED_REFERENCE` | 2154 | 1658 | 496 |
| `CONFLICTING_JVM_DECLARATIONS` | 996 | 518 | 478 |
| `ARGUMENT_TYPE_MISMATCH` | 1263 | 837 | 426 |
| `UNSUPPORTED` | 230 | 34 | 196 |
| `WRONG_MODIFIER_TARGET` | 299 | 169 | 130 |
| `CONFLICTING_OVERLOADS` | 719 | 596 | 123 |
| `RETURN_TYPE_MISMATCH` | 395 | 280 | 115 |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 211 | 127 | 84 |
| `CANNOT_INFER_PARAMETER_TYPE` | 560 | 477 | 83 |
| `CONST_VAL_WITH_NON_CONST_INITIALIZER` | 243 | 162 | 81 |
| `MUST_BE_INITIALIZED` | 101 | 20 | 81 |
| `ANNOTATION_ARGUMENT_MUST_BE_CONST` | 217 | 139 | 78 |
| `REDECLARATION` | 192 | 119 | 73 |
| `TYPE_VARIANCE_CONFLICT_ERROR` | 122 | 54 | 68 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS` | 158 | 93 | 65 |
| `NULL_FOR_NONNULL_TYPE` | 102 | 39 | 63 |
| `MISSING_DEPENDENCY_CLASS` | 61 | 0 | 61 |
| `CANNOT_WEAKEN_ACCESS_PRIVILEGE` | 59 | 1 | 58 |
| `UNSAFE_CALL` | 473 | 415 | 58 |
| `NO_RETURN_IN_FUNCTION_WITH_BLOCK_BODY` | 147 | 90 | 57 |
| `MIXING_SUSPEND_AND_NON_SUSPEND_SUPERTYPES` | 83 | 28 | 55 |
| `INITIALIZER_TYPE_MISMATCH` | 388 | 338 | 50 |
| `NO_VALUE_FOR_PARAMETER` | 167 | 120 | 47 |
| `INVISIBLE_REFERENCE` | 361 | 315 | 46 |
| `SMARTCAST_IMPOSSIBLE` | 176 | 130 | 46 |
| `UNINITIALIZED_VARIABLE` | 208 | 162 | 46 |
| `UPPER_BOUND_VIOLATED` | 112 | 70 | 42 |
| `NON_PUBLIC_CALL_FROM_PUBLIC_INLINE` | 59 | 19 | 40 |
| `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` | 85 | 45 | 40 |
| `TYPE_INFERENCE_ONLY_INPUT_TYPES_ERROR` | 61 | 21 | 40 |
| `INAPPLICABLE_OPERATOR_MODIFIER` | 143 | 104 | 39 |
| `AMBIGUOUS_ANONYMOUS_TYPE_INFERRED` | 38 | 2 | 36 |
| `MUST_BE_INITIALIZED_OR_BE_ABSTRACT` | 79 | 44 | 35 |
| `CONFUSING_BRANCH_CONDITION_ERROR` | 37 | 3 | 34 |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 121 | 89 | 32 |
| `EXPLICIT_TYPE_ARGUMENTS_IN_PROPERTY_ACCESS` | 36 | 5 | 31 |
| `INAPPLICABLE_JVM_FIELD` | 33 | 2 | 31 |
| `OPT_IN_USAGE_ERROR` | 96 | 66 | 30 |
| `WRONG_ANNOTATION_TARGET` | 337 | 307 | 30 |
| `ASSIGNMENT_IN_EXPRESSION_CONTEXT` | 32 | 3 | 29 |
| `REPEATED_MODIFIER` | 39 | 10 | 29 |
| `JVM_EXPOSE_BOXED_CANNOT_EXPOSE_PRIVATE` | 28 | 0 | 28 |
| `UNDERSCORE_USAGE_WITHOUT_BACKTICKS` | 28 | 1 | 27 |
| `WRONG_ANNOTATION_TARGET_WITH_USE_SITE_TARGET` | 79 | 52 | 27 |
| `TYPE_ARGUMENTS_FOR_OUTER_CLASS_WHEN_NESTED_REFERENCED` | 41 | 15 | 26 |
| `SUBCLASS_OPT_IN_INAPPLICABLE` | 25 | 0 | 25 |
| `CONCURRENT_HASH_MAP_CONTAINS_OPERATOR_ERROR` | 24 | 0 | 24 |
| `TOO_MANY_ARGUMENTS` | 102 | 78 | 24 |
| `ASSIGN_OPERATOR_AMBIGUITY` | 35 | 12 | 23 |
| `PROPERTY_INITIALIZER_NO_BACKING_FIELD` | 32 | 9 | 23 |
| `UNINITIALIZED_ENUM_COMPANION` | 65 | 42 | 23 |
| `INLINE_FROM_HIGHER_PLATFORM` | 21 | 0 | 21 |
| `NOTHING_TO_OVERRIDE` | 65 | 44 | 21 |
| `INAPPLICABLE_CANDIDATE` | 115 | 96 | 19 |
| `INCORRECT_TYPE_PARAMETER_OF_PROPERTY` | 30 | 11 | 19 |
| `RETURN_TYPE_MISMATCH_ON_OVERRIDE` | 36 | 18 | 18 |
| `UPPER_BOUND_VIOLATED_IN_TYPEALIAS_EXPANSION` | 44 | 26 | 18 |
| `OPT_IN_MARKER_CAN_ONLY_BE_USED_AS_ANNOTATION_OR_ARGUMENT_IN_OPT_IN` | 19 | 2 | 17 |
| `DSL_SCOPE_VIOLATION` | 108 | 92 | 16 |
| *520 more* | | | |

## Errors kotlinc doesn't report

kc's errors on lines where the markup expects no diagnostic, in the tests it checked, with names replaced by _. In a test kotlinc compiles, one is a false error; in one it rejects, it can also be the same error on another line, or one kotlinc doesn't report after a first.

| Error | Count |
|:---|---:|
| `cannot infer type for type parameter '_'. Specify it explicitly.` | 527 |
| `only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type _` | 313 |
| `type mismatch: inferred type is _ but _ was expected` | 303 |
| `unresolved reference: _` | 252 |
| `platform declaration clash: _` | 150 |
| `class '_' is not abstract and does not implement abstract member '_'` | 55 |
| `suspend function '_' cannot override non-suspend function '_' defined in '_'` | 50 |
| `missing return statement in function '_'` | 38 |
| `'_' expression must be exhaustive, add necessary '_' branch` | 34 |
| `no function '_' takes n arguments` | 32 |
| `cannot infer a type for this parameter. Please specify it explicitly.` | 28 |
| `ambiguous call: n candidates for '_' take (_)` | 26 |
| `unresolved or unsupported type: _` | 26 |
| `operator '_' cannot be applied to Any?` | 16 |
| `type checking has run into a recursive problem; specify the type of '_'` | 16 |
| `variable '_' must be initialized` | 16 |
| `'_' overrides nothing` | 15 |
| `expected expression, found '_'` | 14 |
| `expected n parameter of type _ but n were declared` | 14 |
| `not enough information to infer type arguments of ::foon: specify the expected function type` | 14 |
| `expected no parameters, but n were declared` | 12 |
| `overload resolution ambiguity for ::foo: specify the expected function type` | 12 |
| `private setters are not allowed for open properties` | 12 |
| `unresolved import: _` | 12 |
| `'_' modifier is required on '_'` | 11 |
| `assignment operators ambiguity: '_' and '_' both apply` | 11 |
| `not enough information to infer type arguments of ::barn: specify the expected function type` | 11 |
| `val cannot be reassigned` | 11 |
| `expected parameter of type _` | 10 |
| `expression '_' of type _ cannot be invoked as a function` | 10 |
| *190 more* | |

## Unsupported features

By the features the parser (with error recovery) and the checker found. A test counts for each.

| Feature | Tests | Only blocker | Examples |
|:---|---:|---:|:---|
| `destructuring declaration` | 4 | 4 | `tests/destructuring/topLevelWithLambda.kt`, `tests/diagnostics/conflictingOverloads.kt` |
| `property reference` | 4 | 4 | `tests/callableReference/unsupported/localVariableWithSubstitution.kt`, `tests/callableReference/unsupported/parameterWithSubstitution.kt` |
| `callable reference` | 2 | 2 | `tests/callableReference/compatibilityResolveWithVarargAndOperatorCall.kt`, `tests/callableReference/genericCallWithReferenceAgainstVarargAndKFunction.kt` |
| `checker: '_' is not allowed here: local interfaces are not supported` | 2 | 2 | `tests/localInterfaces.kt`, `tests/thisAndSuper/unqualifiedSuper/unqualifiedSuperWithLocalClass.kt` |
| `inline-only call` | 2 | 2 | `testsWithStdLib/cast/IsArray.kt`, `testsWithStdLib/cast/IsReified.kt` |
| `checker: lambdas with more than n parameters are not supported` | 1 | 1 | `tests/tooLargeFunctionNThroughACallableReference.kt` |
| `package` | 1 | 1 | `tests/modifiers/IllegalModifiers.kt` |
| `parse error: expected '_', found '_'` | 1 | 1 | `tests/innerClasses/inCallableReferenceLHS/withParenthesizes.kt` |
| `parse error: expected expression, found '_'` | 1 | 1 | `tests/innerClasses/inCallableReferenceLHS/misplacedArgumentsInChain.kt` |

### Greedy order

At each step, the feature that unblocks the most tests given the ones before it.

| Step | Feature | Tests unblocked | Total |
|---:|:---|---:|---:|
| 1 | `destructuring declaration` | 4 | 4 |
| 2 | `property reference` | 4 | 8 |
| 3 | `callable reference` | 2 | 10 |
| 4 | `checker: '_' is not allowed here: local interfaces are not supported` | 2 | 12 |
| 5 | `inline-only call` | 2 | 14 |
| 6 | `checker: lambdas with more than n parameters are not supported` | 1 | 15 |
| 7 | `package` | 1 | 16 |
| 8 | `parse error: expected '_', found '_'` | 1 | 17 |
| 9 | `parse error: expected expression, found '_'` | 1 | 18 |

## Warnings

kc reports no warnings. The warnings the tests run expect, as a backlog.

| Warning | Count |
|:---|---:|
| `REDUNDANT_INTERPOLATION_PREFIX` | 355 |
| `UNREACHABLE_CODE` | 158 |
| `IMPLICIT_BOXING_IN_IDENTITY_EQUALS` | 152 |
| `DEPRECATION` | 151 |
| `UNUSED_EXPRESSION` | 136 |
| `USELESS_IS_CHECK` | 111 |
| `CAST_NEVER_SUCCEEDS` | 90 |
| `ERROR_SUPPRESSION` | 87 |
| `UNNECESSARY_NOT_NULL_ASSERTION` | 86 |
| `OPT_IN_USAGE` | 83 |
| `USELESS_CAST` | 83 |
| `CONTEXTUAL_OVERLOAD_SHADOWED` | 75 |
| `NOTHING_TO_INLINE` | 75 |
| `SENSELESS_COMPARISON` | 75 |
| `ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY` | 70 |
| `UNCHECKED_CAST` | 70 |
| `UNNECESSARY_SAFE_CALL` | 68 |
| `FINAL_UPPER_BOUND` | 66 |
| `EXTENSION_SHADOWED_BY_MEMBER` | 63 |
| `NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL` | 59 |
| `PLATFORM_CLASS_MAPPED_TO_KOTLIN` | 58 |
| `IMPOSSIBLE_IS_CHECK_WARNING` | 57 |
| `REDUNDANT_PROJECTION` | 49 |
| `NO_REFLECTION_IN_CLASS_PATH` | 36 |
| `UNUSED_VARIABLE` | 35 |
| `USELESS_ELVIS` | 34 |
| `ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY` | 33 |
| `ASSIGNED_VALUE_IS_NEVER_READ` | 32 |
| `NO_TAIL_CALLS_FOUND` | 29 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING` | 27 |
| *102 more* | |

