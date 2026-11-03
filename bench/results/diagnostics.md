# Kotlin's diagnostics tests on kc

The diagnostics tests of JetBrains/kotlin v2.4.21 for the JVM (`compiler/testData/diagnostics/`: tests/, testsWithStdLib/, testsWithJvmBackend/, testsWithAnyBackend/), each compiled by kc. A test's markup marks the diagnostics kotlinc reports, and its `RUN_PIPELINE_TILL` says whether kotlinc stops with errors in the frontend (`FRONTEND`, `FIR2IR`) or goes on to the backend (`BACKEND`). `bench/kotlin-tests.sh diagnostics` writes this file, and [docs/conformance.md](../../docs/conformance.md) explains it.

Of 10827 test files, 3627 are skipped and 7200 run.

| Result | Tests | Share |
|:---|---:|---:|
| rejects invalid | 3443 | 47.8% |
| accepts valid | 2985 | 41.5% |
| accepts invalid | 377 | 5.2% |
| rejects valid | 317 | 4.4% |
| parse/unsupported | 77 | 1.1% |
| crash | 1 | 0.0% |

*Rejects invalid* and *accepts valid* agree with kotlinc. *Accepts invalid* is code kotlinc rejects that kc compiles; *rejects valid*, code kotlinc compiles that kc's checker rejects. Both are bugs. A *parse/unsupported* test uses a feature kc doesn't support, which says nothing about its checks.

Of the 3443 tests both reject, kc reports an error on the line of an expected one in 3177.

A test is invalid if it stops before the backend or marks an error: one that runs to it (`BACKEND`) can mark the JVM backend's errors, such as `CONFLICTING_JVM_DECLARATIONS`. Of the tests run, 177 stop at `FRONTEND` but mark no error: most add the `CHECK_TYPE` helpers, whose names (`_`) kotlinc reports outside the test infrastructure, and count as valid; the others leave their errors out with the `DIAGNOSTICS` directive. 101 run to the backend and mark one. 5 marked names have no declared severity (debug information, for one), and count as neither.

The names without a severity: `DEBUG_INFO_EXPRESSION_TYPE` (680), `DEBUG_INFO_CSR_MIGHT_BE_USED` (44), `DEBUG_INFO_CALL` (26), `DEBUG_INFO_CSR_MIGHT_BE_USED_INSTEAD_OF_IMPORT` (16), `DEBUG_INFO_CALLABLE_OWNER` (4).

## Skipped

Tests for other platforms, that need language features off by default or compiler options, Java sources, or modules other than plain dependencies, or whose FIR diagnostics aren't checked.

| Reason | Tests | Examples |
|:---|---:|:---|
| `LANGUAGE` | 1674 | `tests/BacktickNames.kt`, `tests/BacktickNamesOld.kt` |
| `Java sources` | 1302 | `tests/IsExpressions.kt`, `tests/SafeCallOnFakePackage.kt` |
| `modules with friends or dependsOn` | 321 | `tests/jvm/internal/internalFromFriendModule.kt`, `tests/multimodule/friendModule.kt` |
| `LANGUAGE_FEATURE_TOGGLED` | 83 | `tests/annotatedPackageQualifiers.kt`, `tests/annotationUsedAsAnnotationArgument.kt` |
| `return value checker (crv/, crvFull/)` | 70 | `tests/crv/assignments.kt`, `tests/crv/booleanShortcut.kt` |
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
| `ARGUMENT_TYPE_MISMATCH` | 28 | 18 | `tests/annotations/annotationArgumentScopeForNestedClass.kt`, `tests/collectionLiterals/kt82964.kt` |
| `UNRESOLVED_REFERENCE` | 19 | 12 | `tests/annotations/annotationArgumentOnCompanionResolvesToInner.kt`, `tests/callResolution/eagerLambdaAnalysis/morePreciseSamVsLessPreciseFunctionType.kt` |
| `UNSAFE_CALL` | 19 | 17 | `tests/dataFlow/assignment/assignToNewVal.kt`, `tests/dataFlow/assignment/uninitializedValNullability.kt` |
| `CONFLICTING_OVERLOADS` | 16 | 14 | `tests/annotations/rendering/conflictingOverloads.kt`, `tests/dataClasses/conflictingCopyOverloads.kt` |
| `DSL_SCOPE_VIOLATION` | 15 | 15 | `tests/resolve/dslMarker/annotatedFunctionType.kt`, `tests/resolve/dslMarker/annotatedTypeArgument.kt` |
| `ERROR_IN_CONTRACT_DESCRIPTION` | 14 | 9 | `tests/kt56877.kt`, `testsWithStdLib/contracts/dsl/errors/accessToOuterThis.kt` |
| `ILLEGAL_RESTRICTED_SUSPENDING_FUNCTION_CALL` | 11 | 11 | `tests/restrictsSuspensionDifference.kt`, `testsWithStdLib/coroutines/restrictSuspension/complicatedTypes.kt` |
| `CANNOT_INFER_PARAMETER_TYPE` | 10 | 4 | `tests/callableReference/function/ambiguityTopLevelVsTopLevelGeneric.kt`, `tests/generics/kt30590.kt` |
| `UNINITIALIZED_VARIABLE` | 10 | 10 | `tests/controlFlowAnalysis/capturingUninitializedVariableInNonInPlaceLambda.kt`, `tests/controlFlowAnalysis/constructorPropertyInterdependence.kt` |
| `INITIALIZER_TYPE_MISMATCH` | 8 | 5 | `tests/callableReference/adapted/noKFunctionForAdaptation.kt`, `tests/functionAsExpression/NonSuspendAnonFuncAsInitializerOfSuspendFunctionType.kt` |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 7 | 6 | `tests/callResolution/kt50386.kt`, `tests/inference/pcla/overloadResolutionByLambdaReturnType.kt` |
| `SMARTCAST_IMPOSSIBLE` | 7 | 4 | `tests/smartCasts/lambdaAndArgumentFun.kt`, `tests/smartCasts/localClassChanges.kt` |
| `MISSING_DEPENDENCY_CLASS` | 6 | 3 | `tests/multimodule/AnonymousFunctionParametersOfInaccessibleTypes.kt`, `tests/multimodule/BoxedInaccessibleGenericTypeUsedByDependencies.kt` |
| `RETURN_TYPE_MISMATCH` | 6 | 5 | `tests/functionLiterals/return/LocalReturnNoCoercionToUnit.kt`, `tests/functionLiterals/return/LocalReturnSecondUnit.kt` |
| `TYPE_INFERENCE_ONLY_INPUT_TYPES_ERROR` | 6 | 5 | `testsWithStdLib/commonCollections.kt`, `testsWithStdLib/inference/annotationsForResolve/kt29307.kt` |
| `CANNOT_WEAKEN_ACCESS_PRIVILEGE` | 5 | 4 | `tests/intersectedVisibilitiesAccessorDifferences_Small.kt`, `tests/intersectedVisibilitiesForNonAbstractPropertyAccessors.kt` |
| `NULL_FOR_NONNULL_TYPE` | 5 | 2 | `tests/j+k/types/newListMethodsNullabilityLinkedListLatest.kt`, `tests/targetedBuiltIns/concurrentMapRemove.kt` |
| `OPT_IN_USAGE_ERROR` | 5 | 3 | `tests/jvm/jvmDefault/annotationsFromBinariesWithNonTrivialJvmDefaultConfiguration.kt`, `testsWithStdLib/contracts/dsl/errors/nestedConditionalEffects.kt` |
| `REDECLARATION` | 5 | 4 | `tests/expresssions/objectVsProperty.kt`, `tests/redeclarations/EnumName.kt` |
| `UNINITIALIZED_ENUM_COMPANION` | 5 | 5 | `tests/companionObjectInEnumAccess.kt`, `tests/controlFlowAnalysis/UninitializedEnumCompanionVals.kt` |
| `UNSUPPORTED` | 5 | 5 | `tests/NamedFunctionTypeParameterInSupertype.kt`, `tests/callableReference/property/backingField.kt` |
| `UPPER_BOUND_VIOLATED` | 5 | 2 | `tests/generics/RecursiveUpperBoundWithTwoArguments.kt`, `tests/inference/capturedTypes/kt49101.kt` |
| `ASSIGN_OPERATOR_AMBIGUITY` | 4 | 4 | `tests/callResolution/exponentialArraySetWithPlusAssign.kt`, `tests/operatorsOverloading/AssignOperatorAmbiguity.kt` |
| `INVALID_VERSIONING_ON_NONFINAL_FUNCTION` | 4 | 3 | `tests/versionOverload/abstractMember.kt`, `tests/versionOverload/interfaceMember.kt` |
| `MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES` | 4 | 4 | `tests/missingMultipleDefaultsOnTransitiveInheritance2.kt`, `tests/multipleDefaultsAnonymousObject.kt` |
| `MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES_WHEN_NO_EXPLICIT_OVERRIDE` | 4 | 4 | `tests/moreThanOneOverriddenMissingK2.kt`, `tests/multipleDefaultsNoSource.kt` |
| `MUST_BE_INITIALIZED_OR_BE_ABSTRACT` | 4 | 3 | `tests/controlFlowAnalysis/deadCode/initializationInUnreachableCode.kt`, `tests/controlFlowAnalysis/smartcastOnThisInInitializationSection.kt` |
| `NOTHING_TO_OVERRIDE` | 4 | 1 | `tests/j+k/types/newListMethodsNullabilityOverridesLinkedListLatest.kt`, `tests/multimodule/duplicateSuper/sameSuperTraitDifferentBounds.kt` |
| `RETURN_TYPE_MISMATCH_ON_OVERRIDE` | 4 | 0 | `tests/inference/pcla/issues/kt47989b.kt`, `tests/j+k/types/newListMethodsNullabilityOverridesLinkedListLatest.kt` |
| `UNDERSCORE_USAGE_WITHOUT_BACKTICKS` | 4 | 4 | `tests/UnderscoreUsageInCall.kt`, `tests/UnderscoreUsageInCallableRefTypeLHS.kt` |
| `(no error marked)` | 3 | 3 | `tests/nullableTypes/nullAssertOnTypeWithNullableUpperBound.kt`, `tests/nullableTypes/safeCallOnTypeWithNullableUpperBound.kt` |
| `ABSTRACT_SUPER_CALL` | 3 | 3 | `tests/missingAbstractSuperCall.kt`, `tests/missingAbstractSuperCallWithClass.kt` |
| `CYCLIC_INHERITANCE_HIERARCHY` | 3 | 3 | `tests/classpathSubstitution/cyclicSupertype.kt`, `tests/classpathSubstitution/cyclicSupertype2.kt` |
| `DEPRECATION_ERROR` | 3 | 2 | `tests/j+k/collectionOverrides/nonDirectHiddenOverride.kt`, `tests/jvm/jvmDefault/annotationsFromBinariesWithNonTrivialJvmDefaultConfiguration.kt` |
| `INCONSISTENT_TYPE_PARAMETER_BOUNDS` | 3 | 2 | `tests/enum/equalityOfEnumAndParameter.kt`, `tests/generics/InconsistentTypeParameterBounds.kt` |
| `INFERENCE_ERROR` | 3 | 0 | `tests/kt56877.kt`, `testsWithStdLib/contracts/fromSource/bad/callsInPlace/contractAndImplicitTypeInterdependency1.kt` |
| `INSTANCE_ACCESS_BEFORE_SUPER_CALL` | 3 | 2 | `tests/annotations/annotationArgumentOnCompanionResolvesToInner.kt`, `tests/scopes/classHeader/companionObjectSuperConstructorArguments.kt` |
| `INVALID_VERSIONING_ON_NON_OPTIONAL` | 3 | 3 | `tests/versionOverload/nonOptionalParameter.kt`, `tests/versionOverload/propertySetterParameter.kt` |
| `INVISIBLE_REFERENCE` | 3 | 3 | `tests/classObjects/companionObjectOfPrivateClassVisibility.kt`, `tests/resolve/invoke/invokeOnInvisiblePropertyOfFunctionType.kt` |
| `MANY_IMPL_MEMBER_NOT_IMPLEMENTED` | 3 | 2 | `tests/override/FakeOverrideDifferentDeclarationSignatures.kt`, `tests/varOverriddenByValThroughIntersection.kt` |
| `MANY_INTERFACES_MEMBER_NOT_IMPLEMENTED` | 3 | 2 | `tests/override/ParentInheritsManyImplementations.kt`, `tests/regressions/kt302.kt` |
| `METHOD_OF_ANY_IMPLEMENTED_IN_INTERFACE` | 3 | 3 | `tests/java8Overrides/implementingMethodOfAny.kt`, `tests/java8Overrides/overridingMethodOfAnyChain.kt` |
| `NON_ASCENDING_VERSION_ANNOTATION` | 3 | 3 | `tests/versionOverload/inlineDefaultLambdaCapture.kt`, `tests/versionOverload/nonAscendingVersion.kt` |
| `PACKAGE_CONFLICTS_WITH_CLASSIFIER` | 3 | 3 | `tests/redeclarations/MultiFilePackageRedeclaration.kt`, `tests/redeclarations/PackageRedeclaredByNestedClass.kt` |
| `PROPERTY_TYPE_MISMATCH_BY_DELEGATION` | 3 | 0 | `tests/delegation/covariantOverrides/fromClass.kt`, `tests/delegation/covariantOverrides/irrelevant.kt` |
| `REIFIED_TYPE_FORBIDDEN_SUBSTITUTION` | 3 | 3 | `tests/generics/tpAsReified/CapturedAsReified.kt`, `tests/inference/constraints/ignoreConstraintFromImplicitInNothing.kt` |
| `RETURN_TYPE_MISMATCH_BY_DELEGATION` | 3 | 0 | `tests/delegation/covariantOverrides/fromClass.kt`, `tests/delegation/covariantOverrides/irrelevant.kt` |
| `SYNTAX` | 3 | 2 | `tests/explicitDefinitelyNotNullableViaIntersection/isAsOperators.kt`, `tests/modifiers/primaryConstructorMissingBrackets.kt` |
| `UPPER_BOUND_VIOLATED_IN_TYPEALIAS_EXPANSION` | 3 | 1 | `tests/typealias/boundsViolationInTypeAliasRHS.kt`, `tests/typealias/kt19601.kt` |
| `AMBIGUOUS_ANONYMOUS_TYPE_INFERRED` | 2 | 2 | `tests/declarationChecks/localObjectInInnerClass.kt`, `tests/exposed/ambiguousAnonymousExposed.kt` |
| `AMBIGUOUS_LABEL` | 2 | 2 | `tests/ambiguousLabelInExtensionFunctionWithOuterClass.kt`, `tests/ambiguousLabelInSuper.kt` |
| `ASSIGNMENT_TYPE_MISMATCH` | 2 | 1 | `tests/controlFlowAnalysis/smartcastOnThisInInitializationSection.kt`, `tests/generics/varProjection/setterNotProjectedOutAssign.kt` |
| `CONFLICTING_INHERITED_MEMBERS` | 2 | 0 | `tests/override/clashesOnInheritance/conflictingInherited.kt`, `testsWithStdLib/coroutines/mixingSuspendability.kt` |
| `DATA_CLASS_OVERRIDE_DEFAULT_VALUES` | 2 | 1 | `tests/dataClasses/dataClassExplicitlyOverridingCopyNoDefaults.kt`, `tests/dataClasses/dataClassOverridingCopy.kt` |
| `DUPLICATE_CLASS_NAMES` | 2 | 2 | `tests/jvm/duplicateJvmSignature/specialNames/classInitializer.kt`, `tests/jvm/duplicateJvmSignature/specialNames/innerClass.kt` |
| `FINITE_BOUNDS_VIOLATION` | 2 | 2 | `tests/declarationChecks/finiteBoundRestriction/CasesWithTwoTypeParameters.kt`, `tests/regressions/kt9633.kt` |
| `FUNCTION_CALL_EXPECTED` | 2 | 0 | `tests/testsWithJava21/implementationsForSequencedCollection.kt`, `tests/testsWithJava21/newListMethods.kt` |
| `INVALID_NON_OPTIONAL_PARAMETER_POSITION` | 2 | 2 | `tests/versionOverload/optionalBeforeNonOptional.kt`, `testsWithStdLib/versionOverloads/nonOptionalAfterOptional.kt` |
| `MEMBER_PROJECTED_OUT` | 2 | 2 | `tests/scopes/variantProjections/irrelevantUnsafeVariance.kt`, `tests/typealias/starProjection.kt` |
| `MISSING_DEPENDENCY_SUPERCLASS` | 2 | 1 | `tests/dataClasses/missingDependencyInSupertype.kt`, `tests/multimodule/inaccessibleSuperTypeWithRepeatedContext.kt` |
| *95 more* | 109 | |

## Rejects valid

Tests kotlinc compiles that kc's checker rejects, by kc's first error, with names replaced by _.

| First error | Tests | Examples |
|:---|---:|:---|
| `type mismatch: inferred type is _ but _ was expected` | 82 | `tests/annotations/parameters/expressions/unaryPlus.kt`, `tests/callableReference/function/localConstructor.kt` |
| `cannot infer type for type parameter '_'. Specify it explicitly.` | 63 | `tests/callableReference/generic/kt35896.kt`, `tests/callableReference/genericCallWithReferenceAgainstVararg.kt` |
| `unresolved reference: _` | 29 | `tests/callResolution/suspendCOnversionForReceiver.kt`, `tests/callableReference/bound/controlFlow.kt` |
| `ambiguous call: n candidates for '_' take (_)` | 14 | `tests/callableReference/resolve/kt35887.kt`, `tests/contextSensitiveResolutionUsingExpectedType/ideHint/forExpressions/multipleOverloads.kt` |
| `cannot infer a type for this parameter. Please specify it explicitly.` | 10 | `tests/inference/completion/postponedArgumentsAnalysis/lackOfDeepIncorporation.kt`, `tests/inference/kt36819.kt` |
| `'_' expression must be exhaustive, add necessary '_' branch` | 8 | `tests/controlStructures/whenWithNothingTypedSubject.kt`, `tests/exhaustiveness/positive/smartcastToExhaustiveInterface.kt` |
| `no function '_' takes n arguments` | 6 | `tests/cast/kt15161.kt`, `tests/errorProneAtomicArrayArguments.kt` |
| `only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type _` | 6 | `tests/regressions/Jet169.kt`, `tests/secondaryConstructors/dataFlowInDelegationCall.kt` |
| `conflicting overloads: _` | 5 | `tests/callableReference/function/localConstructorFromLocalExtension.kt`, `tests/localFunctionsHiding.kt` |
| `expression '_' of type _ cannot be invoked as a function` | 5 | `tests/companionObjectCall.kt`, `tests/expresssions/invoke/incorrectInvokeReceiver.kt` |
| `'_' is not defined in this context` | 4 | `tests/functionLiterals/kt51317.kt`, `tests/inference/completion/postponedArgumentsAnalysis/takingExtensibilityFromDeclarationOfAnonymousFunction.kt` |
| `none of the n candidates for '_' take (_)` | 4 | `tests/callableReference/noAmbiguityWhenAllReferencesAreInapplicable.kt`, `tests/problems/kt52219.kt` |
| `property must be initialized or be abstract` | 4 | `tests/backingField/FieldAsParam.kt`, `tests/backingField/FieldShadow.kt` |
| `unresolved import: _` | 4 | `tests/errorProneAtomicArraySuggestions.kt`, `tests/errorProneAtomicReferenceArguments.kt` |
| `'_' overrides nothing` | 3 | `tests/genericReceiverPropertyOverride.kt`, `tests/generics/finalUpperBoundWithOverride.kt` |
| `overload resolution ambiguity for B::foo: specify the expected function type` | 3 | `tests/callableReference/callableReferenceWithMostSpecificGenericTypeParameter.kt`, `tests/callableReference/resolve/kt9601.kt` |
| `type argument is not within its bounds: type parameter '_' must be subtype of '_', but actual: _` | 3 | `tests/inference/capturedTypes/capturedTypeWithInnerTypealias.kt`, `tests/inference/unsoundness2.kt` |
| `type checking has run into a recursive problem; specify the type of '_'` | 3 | `tests/checkers/uselessCastLeadsToRecursiveProblem.kt`, `tests/problems/kt27790.kt` |
| `unresolved label: _` | 3 | `tests/labelAndReceiverForInfix.kt`, `tests/labeledReturnFromNotLabeledUnnamedFunction.kt` |
| `conflicting declarations: _` | 2 | `tests/overload/noConflictingOverloadsWithDeprecatedHidden/ConflictingOverloadsValsDifferentTypeInClass.kt`, `testsWithStdLib/duplicateJvmSignature/delegatedProperty.kt` |
| `expression of type _ cannot be invoked as a function` | 2 | `tests/controlFlowAnalysis/deadCode/deadCallInInvokeCall.kt`, `tests/expresssions/invoke/safeCallsWithImplicitInvoke.kt` |
| `illegal usage of inline-parameter '_' in '_': add '_' modifier to the parameter declaration` | 2 | `tests/contracts/pos/1.kt`, `testsWithStdLib/contracts/dsl/fqnContractFunction.kt` |
| `n type arguments expected for '_'` | 2 | `tests/delegatingConstructorsAndTypeAliases.kt`, `tests/generics/innerClasses/implicitArguments/secondLevelDepth.kt` |
| `tailrec is not allowed on open members` | 2 | `tests/tailRecOverridden.kt`, `tests/tailRecSingleton.kt` |
| `too many arguments for annotation java/lang/annotation/Target` | 2 | `tests/annotations/JavaAnnotationConstructors.kt`, `testsWithStdLib/annotations/annotationIncompatibleTargets.kt` |
| `type of '_' doesn't match the type of the overridden var-property` | 2 | `tests/extraCheckers/kt69190.kt`, `tests/smartCasts/intersectionScope/propertiesConflict.kt` |
| `val cannot be reassigned` | 2 | `tests/controlFlowAnalysis/scopeOfAnonymousInitializer.kt`, `testsWithStdLib/reinitializations/constructorVarWrite.kt` |
| `'_' hides member of supertype '_' and needs '_' modifier` | 1 | `tests/targetedBuiltIns/backwardCompatibility/fillInStackTrace.kt` |
| `'_' is deprecated with level HIDDEN` | 1 | `tests/problems/kt14059.kt` |
| `'_' is not an expression, it can only be used on the left-hand side of a dot ('_')` | 1 | `tests/superQualifierTypeArgsInDelegatedConstructorCall.kt` |
| `'_' modifier is required on '_'` | 1 | `tests/problems/kt36815.kt` |
| `'_' needs an explicit return type on '_'` | 1 | `tests/inference/completion/inferLabeledElementTypeForLambdaInReturnInCatch.kt` |
| `assignment operators ambiguity: '_' and '_' both apply` | 1 | `tests/operatorsOverloading/plusAssignOnVarAndCollections.kt` |
| `expected no parameters, but n were declared` | 1 | `tests/inference/extensionLambdasAndArrow.kt` |
| `expected parameter of type _` | 1 | `tests/resolve/starProjectionAndFunctionType.kt` |
| `function invocation '_' expected; use ::bar for a reference` | 1 | `tests/resolve/resolveAnnotatedLambdaArgument.kt` |
| `function invocation '_' expected; use ::foo for a reference` | 1 | `tests/syntax/incorrectLTGTFallback.kt` |
| `invalid double literal` | 1 | `tests/annotations/parameters/double.kt` |
| `many supertypes available, please specify the one you mean in angle brackets, e.g. '_'` | 1 | `tests/override/kt53408.kt` |
| `missing return statement in function '_'` | 1 | `tests/inline/inlinedReturnInBranch.kt` |
| *29 more* | 29 | |

## Errors by diagnostic

For each expected error of the tests kc checked (not unsupported), whether kc reported an error on its line. Sorted by the errors kc missed.

kc reports 12219 of 18918 expected errors (64.6%) on their lines.

| Diagnostic | Expected | Reported | Missed |
|:---|---:|---:|---:|
| `CONFLICTING_JVM_DECLARATIONS` | 996 | 518 | 478 |
| `UNRESOLVED_REFERENCE` | 2038 | 1594 | 444 |
| `SYNTAX` | 638 | 197 | 441 |
| `ARGUMENT_TYPE_MISMATCH` | 1257 | 817 | 440 |
| `CONFLICTING_OVERLOADS` | 719 | 370 | 349 |
| `RETURN_TYPE_MISMATCH` | 395 | 233 | 162 |
| `UNSUPPORTED` | 192 | 34 | 158 |
| `WRONG_MODIFIER_TARGET` | 298 | 154 | 144 |
| `REDECLARATION` | 192 | 86 | 106 |
| `ILLEGAL_RESTRICTED_SUSPENDING_FUNCTION_CALL` | 100 | 2 | 98 |
| `CANNOT_INFER_PARAMETER_TYPE` | 559 | 470 | 89 |
| `UNINITIALIZED_VARIABLE` | 196 | 108 | 88 |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 211 | 125 | 86 |
| `MUST_BE_INITIALIZED` | 101 | 18 | 83 |
| `DSL_SCOPE_VIOLATION` | 108 | 34 | 74 |
| `INITIALIZER_TYPE_MISMATCH` | 381 | 307 | 74 |
| `UNSAFE_CALL` | 471 | 401 | 70 |
| `TYPE_VARIANCE_CONFLICT_ERROR` | 122 | 54 | 68 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS` | 158 | 93 | 65 |
| `UNINITIALIZED_ENUM_COMPANION` | 65 | 1 | 64 |
| `NULL_FOR_NONNULL_TYPE` | 102 | 39 | 63 |
| `MISSING_DEPENDENCY_CLASS` | 61 | 0 | 61 |
| `CANNOT_WEAKEN_ACCESS_PRIVILEGE` | 59 | 1 | 58 |
| `MIXING_SUSPEND_AND_NON_SUSPEND_SUPERTYPES` | 83 | 28 | 55 |
| `NO_RETURN_IN_FUNCTION_WITH_BLOCK_BODY` | 145 | 90 | 55 |
| `NO_VALUE_FOR_PARAMETER` | 167 | 119 | 48 |
| `INVISIBLE_REFERENCE` | 361 | 315 | 46 |
| `SMARTCAST_IMPOSSIBLE` | 176 | 130 | 46 |
| `UPPER_BOUND_VIOLATED` | 110 | 68 | 42 |
| `NON_PUBLIC_CALL_FROM_PUBLIC_INLINE` | 59 | 19 | 40 |
| `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` | 85 | 45 | 40 |
| `TYPE_INFERENCE_ONLY_INPUT_TYPES_ERROR` | 61 | 21 | 40 |
| `INAPPLICABLE_OPERATOR_MODIFIER` | 143 | 104 | 39 |
| `MUST_BE_INITIALIZED_OR_BE_ABSTRACT` | 78 | 39 | 39 |
| `ERROR_IN_CONTRACT_DESCRIPTION` | 38 | 0 | 38 |
| `AMBIGUOUS_ANONYMOUS_TYPE_INFERRED` | 38 | 2 | 36 |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 120 | 86 | 34 |
| `CONFUSING_BRANCH_CONDITION_ERROR` | 35 | 3 | 32 |
| `OPT_IN_USAGE_ERROR` | 96 | 64 | 32 |
| `EXPLICIT_TYPE_ARGUMENTS_IN_PROPERTY_ACCESS` | 36 | 5 | 31 |
| `INAPPLICABLE_JVM_FIELD` | 33 | 2 | 31 |
| `ASSIGNMENT_IN_EXPRESSION_CONTEXT` | 32 | 3 | 29 |
| `REPEATED_MODIFIER` | 39 | 10 | 29 |
| `WRONG_ANNOTATION_TARGET` | 337 | 308 | 29 |
| `JVM_EXPOSE_BOXED_CANNOT_EXPOSE_PRIVATE` | 28 | 0 | 28 |
| `UNDERSCORE_USAGE_WITHOUT_BACKTICKS` | 28 | 0 | 28 |
| `WRONG_ANNOTATION_TARGET_WITH_USE_SITE_TARGET` | 79 | 52 | 27 |
| `TYPE_ARGUMENTS_FOR_OUTER_CLASS_WHEN_NESTED_REFERENCED` | 41 | 15 | 26 |
| `SUBCLASS_OPT_IN_INAPPLICABLE` | 25 | 0 | 25 |
| `CONCURRENT_HASH_MAP_CONTAINS_OPERATOR_ERROR` | 24 | 0 | 24 |
| `INAPPLICABLE_CANDIDATE` | 113 | 89 | 24 |
| `TOO_MANY_ARGUMENTS` | 102 | 78 | 24 |
| `ASSIGN_OPERATOR_AMBIGUITY` | 35 | 12 | 23 |
| `PROPERTY_INITIALIZER_NO_BACKING_FIELD` | 32 | 9 | 23 |
| `CONST_VAL_WITH_NON_CONST_INITIALIZER` | 183 | 162 | 21 |
| `INLINE_FROM_HIGHER_PLATFORM` | 21 | 0 | 21 |
| `NOTHING_TO_OVERRIDE` | 65 | 44 | 21 |
| `INCORRECT_TYPE_PARAMETER_OF_PROPERTY` | 30 | 11 | 19 |
| `RETURN_TYPE_MISMATCH_ON_OVERRIDE` | 36 | 17 | 19 |
| `ANNOTATION_ARGUMENT_MUST_BE_CONST` | 157 | 139 | 18 |
| *511 more* | | | |

## Errors kotlinc doesn't report

kc's errors on lines where the markup expects no diagnostic, in the tests it checked, with names replaced by _. In a test kotlinc compiles, one is a false error; in one it rejects, it can also be the same error on another line, or one kotlinc doesn't report after a first.

| Error | Count |
|:---|---:|
| `cannot infer a type for this parameter. Please specify it explicitly.` | 1059 |
| `cannot infer type for type parameter '_'. Specify it explicitly.` | 561 |
| `type mismatch: inferred type is _ but _ was expected` | 337 |
| `only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type _` | 321 |
| `conflicting overloads: _` | 305 |
| `unresolved reference: _` | 267 |
| `class '_' is not abstract and does not implement abstract member '_'` | 55 |
| `suspend function '_' cannot override non-suspend function '_' defined in '_'` | 50 |
| `missing return statement in function '_'` | 35 |
| `expected member declaration, found '_'` | 31 |
| `no function '_' takes n arguments` | 31 |
| `ambiguous call: n candidates for '_' take (_)` | 26 |
| `unresolved or unsupported type: _` | 26 |
| `'_' expression must be exhaustive, add necessary '_' branch` | 21 |
| `variable '_' must be initialized` | 20 |
| `expected expression, found '_'` | 19 |
| `val cannot be reassigned` | 17 |
| `operator '_' cannot be applied to Any?` | 16 |
| `tailrec is not allowed on open members` | 16 |
| `type checking has run into a recursive problem; specify the type of '_'` | 16 |
| `'_' overrides nothing` | 15 |
| `expected declaration, found '_'` | 15 |
| `assignment operators ambiguity: '_' and '_' both apply` | 14 |
| `not enough information to infer type arguments of ::foon: specify the expected function type` | 14 |
| `expected n parameter of type _ but n were declared` | 13 |
| `expected no parameters, but n were declared` | 12 |
| `overload resolution ambiguity for ::foo: specify the expected function type` | 12 |
| `private setters are not allowed for open properties` | 12 |
| `'_' modifier is required on '_'` | 11 |
| `expected '_', found '_'` | 11 |
| *193 more* | |

## Unsupported features

By the features the parser (with error recovery) and the checker found. A test counts for each.

| Feature | Tests | Only blocker | Examples |
|:---|---:|---:|:---|
| `parse error: expected newline or '_', found '_'` | 15 | 15 | `tests/callableReference/whitespacesInExpression.kt`, `tests/cast/bare/RedundantNullable.kt` |
| `parse error: expected member declaration, found '_'` | 13 | 12 | `tests/nestedTypeAliases/annotations.kt`, `testsWithStdLib/properties/backingField/explicitBackingFieldAndMultipleIntersection.kt` |
| `parse error: expected '_', found '_'` | 11 | 11 | `tests/innerClasses/inCallableReferenceLHS/withParenthesizes.kt`, `tests/kt55666.kt` |
| `parse error: expected function body, found '_'` | 7 | 7 | `tests/suppress/manyWarnings/mixed.kt`, `tests/suppress/manyWarnings/onClass.kt` |
| `parse error: expected declaration, found '_'` | 5 | 4 | `testsWithStdLib/properties/backingField/backingFieldWithSmartTypeParameters.kt`, `testsWithStdLib/properties/backingField/explicitBackingFieldsInInlineExtensionFun.kt` |
| `destructuring declaration` | 4 | 4 | `tests/destructuring/topLevelWithLambda.kt`, `tests/diagnostics/conflictingOverloads.kt` |
| `parse error: expected expression, found '_'` | 4 | 4 | `tests/controlStructures/redundantLabel.kt`, `tests/innerClasses/inCallableReferenceLHS/misplacedArgumentsInChain.kt` |
| `property reference` | 4 | 4 | `tests/callableReference/unsupported/localVariableWithSubstitution.kt`, `tests/callableReference/unsupported/parameterWithSubstitution.kt` |
| `parse error: expected getter body, found '_'` | 3 | 3 | `tests/suppress/allWarnings/suppressWarningsOnPropertyAccessor.kt`, `tests/suppress/manyWarnings/onPropertyAccessor.kt` |
| `parse error: expected identifier, found '_'` | 3 | 3 | `testsWithStdLib/pureReifiable.kt`, `testsWithStdLib/trailingComma/valueParametersWithEnabledFeature.kt` |
| `callable reference` | 2 | 2 | `tests/callableReference/compatibilityResolveWithVarargAndOperatorCall.kt`, `tests/callableReference/genericCallWithReferenceAgainstVarargAndKFunction.kt` |
| `checker: '_' is not allowed here: local interfaces are not supported` | 2 | 2 | `tests/localInterfaces.kt`, `tests/thisAndSuper/unqualifiedSuper/unqualifiedSuperWithLocalClass.kt` |
| `inline-only call` | 2 | 2 | `testsWithStdLib/cast/IsArray.kt`, `testsWithStdLib/cast/IsReified.kt` |
| `checker: lambdas with more than n parameters are not supported` | 1 | 1 | `tests/tooLargeFunctionNThroughACallableReference.kt` |
| `package` | 1 | 1 | `tests/modifiers/IllegalModifiers.kt` |
| `parse error: invalid float literal` | 1 | 1 | `tests/evaluate/floatLiteralOutOfRange.kt` |

### Greedy order

At each step, the feature that unblocks the most tests given the ones before it.

| Step | Feature | Tests unblocked | Total |
|---:|:---|---:|---:|
| 1 | `parse error: expected newline or '_', found '_'` | 15 | 15 |
| 2 | `parse error: expected member declaration, found '_'` | 12 | 27 |
| 3 | `parse error: expected '_', found '_'` | 11 | 38 |
| 4 | `parse error: expected function body, found '_'` | 7 | 45 |
| 5 | `parse error: expected declaration, found '_'` | 5 | 50 |
| 6 | `destructuring declaration` | 4 | 54 |
| 7 | `parse error: expected expression, found '_'` | 4 | 58 |
| 8 | `property reference` | 4 | 62 |
| 9 | `parse error: expected getter body, found '_'` | 3 | 65 |
| 10 | `parse error: expected identifier, found '_'` | 3 | 68 |
| 11 | `callable reference` | 2 | 70 |
| 12 | `checker: '_' is not allowed here: local interfaces are not supported` | 2 | 72 |
| 13 | `inline-only call` | 2 | 74 |
| 14 | `checker: lambdas with more than n parameters are not supported` | 1 | 75 |
| 15 | `package` | 1 | 76 |
| 16 | `parse error: invalid float literal` | 1 | 77 |

## Crashes

| Panic | Tests | Examples |
|:---|---:|:---|
| `stack overflow in types.Unboxed (value.go:23)` | 1 | `tests/inlineClasses/inlineClassConstructor.kt` |

## Warnings

kc reports no warnings. The warnings the tests run expect, as a backlog.

| Warning | Count |
|:---|---:|
| `UNREACHABLE_CODE` | 158 |
| `IMPLICIT_BOXING_IN_IDENTITY_EQUALS` | 152 |
| `DEPRECATION` | 151 |
| `UNUSED_EXPRESSION` | 136 |
| `USELESS_IS_CHECK` | 105 |
| `CAST_NEVER_SUCCEEDS` | 88 |
| `ERROR_SUPPRESSION` | 87 |
| `UNNECESSARY_NOT_NULL_ASSERTION` | 86 |
| `OPT_IN_USAGE` | 83 |
| `USELESS_CAST` | 83 |
| `CONTEXTUAL_OVERLOAD_SHADOWED` | 75 |
| `SENSELESS_COMPARISON` | 74 |
| `NOTHING_TO_INLINE` | 71 |
| `ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY` | 70 |
| `UNCHECKED_CAST` | 70 |
| `UNNECESSARY_SAFE_CALL` | 68 |
| `EXTENSION_SHADOWED_BY_MEMBER` | 63 |
| `FINAL_UPPER_BOUND` | 62 |
| `NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL` | 59 |
| `PLATFORM_CLASS_MAPPED_TO_KOTLIN` | 58 |
| `REDUNDANT_PROJECTION` | 49 |
| `IMPOSSIBLE_IS_CHECK_WARNING` | 38 |
| `NO_REFLECTION_IN_CLASS_PATH` | 36 |
| `UNUSED_VARIABLE` | 35 |
| `ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY` | 33 |
| `ASSIGNED_VALUE_IS_NEVER_READ` | 32 |
| `USELESS_ELVIS` | 30 |
| `NO_TAIL_CALLS_FOUND` | 29 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING` | 27 |
| `DEPRECATED_IDENTITY_EQUALS` | 26 |
| *101 more* | |

