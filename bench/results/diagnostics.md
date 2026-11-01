# Kotlin's diagnostics tests on kc

The diagnostics tests of JetBrains/kotlin v2.4.21 for the JVM (`compiler/testData/diagnostics/`: tests/, testsWithStdLib/, testsWithJvmBackend/, testsWithAnyBackend/), each compiled by kc. A test's markup marks the diagnostics kotlinc reports, and its `RUN_PIPELINE_TILL` says whether kotlinc stops with errors in the frontend (`FRONTEND`, `FIR2IR`) or goes on to the backend (`BACKEND`). `bench/kotlin-tests.sh diagnostics` writes this file, and [docs/conformance.md](../../docs/conformance.md) explains it.

Of 10827 test files, 3627 are skipped and 7200 run.

| Result | Tests | Share |
|:---|---:|---:|
| rejects invalid | 3024 | 42.0% |
| accepts valid | 2805 | 39.0% |
| accepts invalid | 794 | 11.0% |
| rejects valid | 497 | 6.9% |
| parse/unsupported | 79 | 1.1% |
| crash | 1 | 0.0% |

*Rejects invalid* and *accepts valid* agree with kotlinc. *Accepts invalid* is code kotlinc rejects that kc compiles; *rejects valid*, code kotlinc compiles that kc's checker rejects. Both are bugs. A *parse/unsupported* test uses a feature kc doesn't support, which says nothing about its checks.

Of the 3024 tests both reject, kc reports an error on the line of an expected one in 2749.

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
| `WRONG_ANNOTATION_TARGET` | 47 | 36 | `tests/Casts.kt`, `tests/annotationOnGetClassCall.kt` |
| `ARGUMENT_TYPE_MISMATCH` | 38 | 22 | `tests/annotations/AnnotationForClassTypeParameter.kt`, `tests/annotations/AnnotationForFunctionTypeParameter.kt` |
| `ANNOTATION_ARGUMENT_MUST_BE_CONST` | 34 | 22 | `tests/annotations/MutuallyRecursivelyAnnotatedGlobalFunction.kt`, `tests/annotations/RecursivelyAnnotatedFunctionParameter.kt` |
| `UNRESOLVED_REFERENCE` | 31 | 19 | `tests/annotations/AnnotationForObject.kt`, `tests/annotations/UnresolvedAnnotationOnObject.kt` |
| `USAGE_IS_NOT_INLINABLE` | 27 | 25 | `tests/inline/assignment.kt`, `tests/inline/binaryExpressions/arrayAccess.kt` |
| `OPT_IN_USAGE_ERROR` | 26 | 14 | `tests/errorSuppressionsWithoutWarning.kt`, `tests/jvm/jvmDefault/annotationsFromBinariesWithNonTrivialJvmDefaultConfiguration.kt` |
| `ACCIDENTAL_OVERRIDE` | 24 | 21 | `tests/jvm/duplicateJvmSignature/accidentalOverrides/accidentalOverrideFromGrandparent.kt`, `tests/jvm/duplicateJvmSignature/accidentalOverrides/classFunctionOverriddenByProperty.kt` |
| `CONFLICTING_JVM_DECLARATIONS` | 24 | 22 | `tests/jvm/duplicateJvmSignature/caseInProperties.kt`, `tests/jvm/duplicateJvmSignature/erasure/delegationAndOwnMethod.kt` |
| `UNSAFE_CALL` | 19 | 17 | `tests/dataFlow/assignment/assignToNewVal.kt`, `tests/dataFlow/assignment/uninitializedValNullability.kt` |
| `CONFLICTING_OVERLOADS` | 18 | 15 | `tests/annotations/rendering/conflictingOverloads.kt`, `tests/dataClasses/conflictingCopyOverloads.kt` |
| `WRONG_ANNOTATION_TARGET_WITH_USE_SITE_TARGET` | 16 | 2 | `tests/annotations/options/targets/file.kt`, `tests/annotations/withUseSiteTarget/DelegateAnnotations.kt` |
| `DSL_SCOPE_VIOLATION` | 15 | 15 | `tests/resolve/dslMarker/annotatedFunctionType.kt`, `tests/resolve/dslMarker/annotatedTypeArgument.kt` |
| `IMPOSSIBLE_IS_CHECK_ERROR` | 15 | 12 | `tests/cast/classVsClassIsCheck.kt`, `tests/cast/classVsClassIsCheckAndIntersectionType.kt` |
| `CONST_VAL_WITH_NON_CONST_INITIALIZER` | 14 | 10 | `tests/constantEvaluator/constant/typeAliasToUnsigned.kt`, `tests/constantEvaluator/constant/unsignedOperations.kt` |
| `ERROR_IN_CONTRACT_DESCRIPTION` | 14 | 9 | `tests/kt56877.kt`, `testsWithStdLib/contracts/dsl/errors/accessToOuterThis.kt` |
| `NON_CONST_VAL_USED_IN_CONSTANT_EXPRESSION` | 13 | 6 | `tests/annotations/annotationArgumentScopeForNestedClass.kt`, `tests/annotations/initializerWithNestedAnnotationInLocalClass.kt` |
| `OPT_IN_TO_INHERITANCE_ERROR` | 12 | 6 | `testsWithStdLib/annotations/subclassOptInRequired/DiagnosticCustomMessage_error.kt`, `testsWithStdLib/annotations/subclassOptInRequired/DiagnosticDefaultMessage_error.kt` |
| `DEPRECATION_ERROR` | 11 | 7 | `tests/annotations/deprecatedResolutionOfAnonymousObject.kt`, `tests/deprecated/deprecatedError.kt` |
| `ILLEGAL_RESTRICTED_SUSPENDING_FUNCTION_CALL` | 11 | 11 | `tests/restrictsSuspensionDifference.kt`, `testsWithStdLib/coroutines/restrictSuspension/complicatedTypes.kt` |
| `INVISIBLE_REFERENCE` | 11 | 8 | `tests/callableReference/bound/privateToThis.kt`, `tests/callingProtectedFromInline.kt` |
| `REPEATED_ANNOTATION` | 11 | 8 | `tests/annotations/AnnotationForClassTypeParameter.kt`, `tests/annotations/AnnotationForFunctionTypeParameter.kt` |
| `CANNOT_INFER_PARAMETER_TYPE` | 10 | 4 | `tests/callableReference/function/ambiguityTopLevelVsTopLevelGeneric.kt`, `tests/generics/kt30590.kt` |
| `TYPE_INFERENCE_ONLY_INPUT_TYPES_ERROR` | 10 | 9 | `testsWithStdLib/cast/AsInsideIn.kt`, `testsWithStdLib/commonCollections.kt` |
| `UNINITIALIZED_VARIABLE` | 10 | 10 | `tests/controlFlowAnalysis/capturingUninitializedVariableInNonInPlaceLambda.kt`, `tests/controlFlowAnalysis/constructorPropertyInterdependence.kt` |
| `CYCLIC_INHERITANCE_HIERARCHY` | 9 | 8 | `tests/breakLoopsBug3.kt`, `tests/classpathSubstitution/cyclicSupertype.kt` |
| `EXPOSED_FUNCTION_RETURN_TYPE` | 8 | 0 | `tests/enum/classObjectInEnumPrivate.kt`, `tests/exposed/functional.kt` |
| `INITIALIZER_TYPE_MISMATCH` | 8 | 5 | `tests/callableReference/adapted/noKFunctionForAdaptation.kt`, `tests/functionAsExpression/NonSuspendAnonFuncAsInitializerOfSuspendFunctionType.kt` |
| `NESTED_CLASS_NOT_ALLOWED` | 8 | 7 | `tests/classObjects/InnerClassClassObject.kt`, `tests/enum/inner/insideInnerClassNotAllowed.kt` |
| `NON_LOCAL_RETURN_NOT_ALLOWED` | 8 | 5 | `tests/inline/inlineLambdaInDefaultInlineParameter.kt`, `tests/inline/nonLocalReturns/anonymousObjects.kt` |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 8 | 6 | `tests/callResolution/kt50386.kt`, `tests/imports/NestedClassClash.kt` |
| `WRONG_MODIFIER_CONTAINING_DECLARATION` | 8 | 3 | `tests/dataClasses/implementTraitWhichHasFinalComponent1.kt`, `tests/inner/innerClassInEnumEntryClass.kt` |
| `CONFLICTING_INHERITED_JVM_DECLARATIONS` | 7 | 6 | `tests/jvm/contextParameters/intersectionExtensionValueContext.kt`, `tests/jvm/duplicateJvmSignature/erasure/clashFromInterfaceAndSuperClass.kt` |
| `EXPOSED_PARAMETER_TYPE` | 7 | 3 | `tests/contextParameters/declarationAndUsages/visibilityOfContextTypeOnProperty.kt`, `tests/exposed/functional.kt` |
| `PRIVATE_TYPE_USED_IN_NON_PRIVATE_INLINE_FUNCTION_ERROR` | 7 | 7 | `tests/jvm/exposedPrivate/contextParameter.kt`, `tests/jvm/exposedPrivate/extensionReceiver.kt` |
| `SMARTCAST_IMPOSSIBLE` | 7 | 4 | `tests/smartCasts/lambdaAndArgumentFun.kt`, `tests/smartCasts/localClassChanges.kt` |
| `WRONG_MODIFIER_TARGET` | 7 | 4 | `tests/constVal/constValNotTopLevelOrObject.kt`, `tests/dataClasses/companionDataObject.kt` |
| `ANNOTATION_PARAMETER_DEFAULT_VALUE_MUST_BE_CONSTANT` | 6 | 5 | `tests/annotations/annotationParameterMustBeConstant/annotationConstructorDefaultParameter.kt`, `tests/annotations/nonConstArrayAsSpreadArgumentInDefaults.kt` |
| `CANNOT_WEAKEN_ACCESS_PRIVILEGE` | 6 | 4 | `tests/intersectedVisibilitiesAccessorDifferences_Small.kt`, `tests/intersectedVisibilitiesForNonAbstractPropertyAccessors.kt` |
| `DECLARATION_CANT_BE_INLINED` | 6 | 5 | `tests/inline/nonVirtualMembersWithInline.kt`, `tests/inline/override.kt` |
| `MISSING_DEPENDENCY_CLASS` | 6 | 3 | `tests/multimodule/AnonymousFunctionParametersOfInaccessibleTypes.kt`, `tests/multimodule/BoxedInaccessibleGenericTypeUsedByDependencies.kt` |
| `NULL_FOR_NONNULL_TYPE` | 6 | 2 | `tests/j+k/types/newListMethodsNullabilityLinkedListLatest.kt`, `tests/targetedBuiltIns/concurrentMapRemove.kt` |
| `RETURN_TYPE_MISMATCH` | 6 | 5 | `tests/functionLiterals/return/LocalReturnNoCoercionToUnit.kt`, `tests/functionLiterals/return/LocalReturnSecondUnit.kt` |
| `(no error marked)` | 5 | 5 | `tests/inline/anonymousObjects.kt`, `tests/nullableTypes/nullAssertOnTypeWithNullableUpperBound.kt` |
| `CONFLICTING_IMPORT` | 5 | 3 | `tests/exceptions/uninitializedTypeArgumentsMappingIfErrorCandidates.kt`, `tests/imports/ClassImportsConflicting.kt` |
| `EXPOSED_PROPERTY_TYPE` | 5 | 0 | `tests/exposed/internal.kt`, `tests/exposed/object.kt` |
| `EXPOSED_SUPER_CLASS` | 5 | 1 | `tests/exposed/internal.kt`, `tests/exposed/object.kt` |
| `INCOMPATIBLE_MODIFIERS` | 5 | 3 | `tests/dataClasses/innerDataClass.kt`, `tests/modifiers/inlineParameters.kt` |
| `NOTHING_TO_OVERRIDE` | 5 | 2 | `tests/j+k/types/newListMethodsNullabilityOverridesLinkedListLatest.kt`, `tests/multimodule/duplicateSuper/sameSuperTraitDifferentBounds.kt` |
| `NO_TYPE_ARGUMENTS_ON_RHS` | 5 | 5 | `tests/cast/bare/AsNullableNotEnough.kt`, `tests/cast/bare/NullableAsNotEnough.kt` |
| `REDECLARATION` | 5 | 4 | `tests/expresssions/objectVsProperty.kt`, `tests/redeclarations/EnumName.kt` |
| `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` | 5 | 0 | `tests/annotations/cycleAnnotationOnFunctionParameterTypeInsideLocalClass.kt`, `tests/annotations/cycleAnnotationOnFunctionReceiverTypeInsideLocalClass.kt` |
| `UNINITIALIZED_ENUM_COMPANION` | 5 | 5 | `tests/companionObjectInEnumAccess.kt`, `tests/controlFlowAnalysis/UninitializedEnumCompanionVals.kt` |
| `UNSUPPORTED` | 5 | 5 | `tests/NamedFunctionTypeParameterInSupertype.kt`, `tests/callableReference/property/backingField.kt` |
| `UPPER_BOUND_VIOLATED` | 5 | 2 | `tests/generics/RecursiveUpperBoundWithTwoArguments.kt`, `tests/inference/capturedTypes/kt49101.kt` |
| `ASSIGN_OPERATOR_AMBIGUITY` | 4 | 4 | `tests/callResolution/exponentialArraySetWithPlusAssign.kt`, `tests/operatorsOverloading/AssignOperatorAmbiguity.kt` |
| `EXPOSED_SUPER_INTERFACE` | 4 | 2 | `tests/exposed/nestedInheritance.kt`, `tests/exposed/protectedInProtected.kt` |
| `ILLEGAL_JVM_NAME` | 4 | 3 | `tests/inlineClasses/jvmExposeBoxed/simple.kt`, `tests/jvmNameInvalidCharacters.kt` |
| `INAPPLICABLE_TARGET_ON_PROPERTY` | 4 | 0 | `tests/annotations/withUseSiteTarget/GetterAnnotations.kt`, `tests/annotations/withUseSiteTarget/PropertyAnnotations.kt` |
| `INVALID_VERSIONING_ON_NONFINAL_FUNCTION` | 4 | 3 | `tests/versionOverload/abstractMember.kt`, `tests/versionOverload/interfaceMember.kt` |
| `MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES` | 4 | 4 | `tests/missingMultipleDefaultsOnTransitiveInheritance2.kt`, `tests/multipleDefaultsAnonymousObject.kt` |
| *201 more* | 317 | |

## Rejects valid

Tests kotlinc compiles that kc's checker rejects, by kc's first error, with names replaced by _.

| First error | Tests | Examples |
|:---|---:|:---|
| `type mismatch: inferred type is _ but _ was expected` | 92 | `tests/annotations/parameters/expressions/unaryPlus.kt`, `tests/callableReference/function/localConstructor.kt` |
| `cannot infer type for type parameter '_'. Specify it explicitly.` | 80 | `tests/callableReference/generic/kt35896.kt`, `tests/callableReference/genericCallWithReferenceAgainstVararg.kt` |
| `only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type _` | 48 | `tests/cast/kt56629.kt`, `tests/cfg/safeCalls.kt` |
| `unresolved reference: _` | 43 | `tests/annotations/nestedClassAsAnnotationParameter.kt`, `tests/annotations/nestedObjectAsAnnotationParameter.kt` |
| `unresolved import: _` | 30 | `tests/annotations/requiredAnnotationWithStarTypealiasedImportedArgument.kt`, `tests/callableReference/function/differentPackageClass.kt` |
| `ambiguous call: n candidates for '_' take (_)` | 28 | `tests/arguments/overloadByReceiver.kt`, `tests/callResolution/eagerLambdaAnalysis/samVsKProperty.kt` |
| `cannot infer a type for this parameter. Please specify it explicitly.` | 10 | `tests/inference/completion/postponedArgumentsAnalysis/lackOfDeepIncorporation.kt`, `tests/inference/kt36819.kt` |
| `'_' expression must be exhaustive, add necessary '_' branch` | 8 | `tests/controlStructures/whenWithNothingTypedSubject.kt`, `tests/exhaustiveness/positive/smartcastToExhaustiveInterface.kt` |
| `expression '_' of type _ cannot be invoked as a function` | 6 | `tests/companionObjectCall.kt`, `tests/expresssions/invoke/incorrectInvokeReceiver.kt` |
| `no function '_' takes n arguments` | 6 | `tests/cast/kt15161.kt`, `tests/errorProneAtomicArrayArguments.kt` |
| `type argument is not within its bounds: inferred type K of type _ '_' is not a subtype of Out<B>` | 6 | `tests/inference/emptyIntersectionTypes/kt45461_11.kt`, `tests/inference/emptyIntersectionTypes/kt45461_14.kt` |
| `conflicting overloads: _` | 5 | `tests/callableReference/function/localConstructorFromLocalExtension.kt`, `tests/localFunctionsHiding.kt` |
| `'_' is not defined in this context` | 4 | `tests/functionLiterals/kt51317.kt`, `tests/inference/completion/postponedArgumentsAnalysis/takingExtensibilityFromDeclarationOfAnonymousFunction.kt` |
| `annotation class cannot have type parameters` | 4 | `tests/annotations/ConstructorCallAllowed.kt`, `tests/annotations/kt55286.kt` |
| `cannot check for instance of erased type: _` | 4 | `tests/cast/IsErasedAllowForDerivedWithOneSubstitutedAndOneSameGeneric.kt`, `tests/cast/IsErasedAllowForTypeWithTwoSameTypeSubstitutions.kt` |
| `none of the n candidates for '_' take (_)` | 4 | `tests/callableReference/noAmbiguityWhenAllReferencesAreInapplicable.kt`, `tests/problems/kt52219.kt` |
| `property must be initialized or be abstract` | 4 | `tests/backingField/FieldAsParam.kt`, `tests/backingField/FieldShadow.kt` |
| `type argument is not within its bounds: inferred type DoesNotImplementBase of type _ '_' is not a subtype of Base` | 4 | `tests/inference/emptyIntersectionTypes/kt48935.kt`, `tests/inference/emptyIntersectionTypes/kt48935_2.kt` |
| `'_' overrides nothing` | 3 | `tests/genericReceiverPropertyOverride.kt`, `tests/generics/finalUpperBoundWithOverride.kt` |
| `expression of type _ cannot be invoked as a function` | 3 | `tests/controlFlowAnalysis/deadCode/deadCallInInvokeCall.kt`, `tests/coroutines/suspendInvokeWithReceiver.kt` |
| `operator '_' cannot be applied to Int? and Int` | 3 | `tests/smartCasts/objectLiterals/exclexcl.kt`, `tests/smartCasts/objectLiterals/exclexclArgument.kt` |
| `overload resolution ambiguity for B::foo: specify the expected function type` | 3 | `tests/callableReference/callableReferenceWithMostSpecificGenericTypeParameter.kt`, `tests/callableReference/resolve/kt9601.kt` |
| `type argument is not within its bounds: type parameter '_' must be subtype of '_', but actual: _` | 3 | `tests/inference/capturedTypes/capturedTypeWithInnerTypealias.kt`, `tests/inference/unsoundness2.kt` |
| `type checking has run into a recursive problem; specify the type of '_'` | 3 | `tests/checkers/uselessCastLeadsToRecursiveProblem.kt`, `tests/problems/kt27790.kt` |
| `unresolved label: _` | 3 | `tests/labelAndReceiverForInfix.kt`, `tests/labeledReturnFromNotLabeledUnnamedFunction.kt` |
| `conflicting declarations: _` | 2 | `tests/overload/noConflictingOverloadsWithDeprecatedHidden/ConflictingOverloadsValsDifferentTypeInClass.kt`, `testsWithStdLib/duplicateJvmSignature/delegatedProperty.kt` |
| `function invocation '_' expected; use ::foo for a reference` | 2 | `tests/callableReference/resolve/adaptedReferenceAgainstKCallable.kt`, `tests/syntax/incorrectLTGTFallback.kt` |
| `illegal usage of inline-parameter '_' in '_': add '_' modifier to the parameter declaration` | 2 | `tests/contracts/pos/1.kt`, `testsWithStdLib/contracts/dsl/fqnContractFunction.kt` |
| `n type arguments expected for '_'` | 2 | `tests/delegatingConstructorsAndTypeAliases.kt`, `tests/generics/innerClasses/implicitArguments/secondLevelDepth.kt` |
| `operator '_' cannot be applied to String` | 2 | `tests/Builders.kt`, `tests/operatorsOverloading/kt3450.kt` |
| `overload resolution ambiguity for A::foo: specify the expected function type` | 2 | `tests/callableReference/function/noAmbiguityMemberVsExtension.kt`, `tests/callableReference/referenceToCompanionObjectMemberViaClassNameCompatibility.kt` |
| `return type of '_' is not a subtype of the return type of the overridden member '_' defined in '_'` | 2 | `tests/callableReference/suspendCallableReference.kt`, `tests/imports/InaccessiblePrivateInFile.kt` |
| `tailrec is not allowed on open members` | 2 | `tests/tailRecOverridden.kt`, `tests/tailRecSingleton.kt` |
| `too many arguments for annotation java/lang/annotation/Target` | 2 | `tests/annotations/JavaAnnotationConstructors.kt`, `testsWithStdLib/annotations/annotationIncompatibleTargets.kt` |
| `type argument is not within its bounds: inferred type K of type _ '_' is not a subtype of B<String>` | 2 | `tests/inference/emptyIntersectionTypes/substitutingSuperTypes.kt`, `tests/inference/emptyIntersectionTypes/substitutingSuperTypes2.kt` |
| `type argument is not within its bounds: inferred type K of type _ '_' is not a subtype of In<Inv<Int>>` | 2 | `tests/inference/emptyIntersectionTypes/kt45461_20.kt`, `tests/inference/emptyIntersectionTypes/kt45461_21.kt` |
| `type argument is not within its bounds: inferred type K of type _ '_' is not a subtype of Inv<in Inv<in Int>>` | 2 | `tests/inference/emptyIntersectionTypes/kt45461_23.kt`, `tests/inference/emptyIntersectionTypes/kt45461_27.kt` |
| `type argument is not within its bounds: inferred type K of type _ '_' is not a subtype of Out<Int>` | 2 | `tests/inference/emptyIntersectionTypes/kt45461_10.kt`, `tests/inference/emptyIntersectionTypes/kt45461_18.kt` |
| `type of '_' doesn't match the type of the overridden var-property` | 2 | `tests/extraCheckers/kt69190.kt`, `tests/smartCasts/intersectionScope/propertiesConflict.kt` |
| `val cannot be reassigned` | 2 | `tests/controlFlowAnalysis/scopeOfAnonymousInitializer.kt`, `testsWithStdLib/reinitializations/constructorVarWrite.kt` |
| *60 more* | 60 | |

## Errors by diagnostic

For each expected error of the tests kc checked (not unsupported), whether kc reported an error on its line. Sorted by the errors kc missed.

kc reports 10321 of 18906 expected errors (54.6%) on their lines.

| Diagnostic | Expected | Reported | Missed |
|:---|---:|---:|---:|
| `CONFLICTING_JVM_DECLARATIONS` | 996 | 472 | 524 |
| `ARGUMENT_TYPE_MISMATCH` | 1257 | 781 | 476 |
| `UNRESOLVED_REFERENCE` | 2026 | 1558 | 468 |
| `SYNTAX` | 638 | 197 | 441 |
| `CONFLICTING_OVERLOADS` | 719 | 364 | 355 |
| `WRONG_ANNOTATION_TARGET` | 337 | 12 | 325 |
| `WRONG_MODIFIER_TARGET` | 298 | 116 | 182 |
| `RETURN_TYPE_MISMATCH` | 395 | 231 | 164 |
| `CONST_VAL_WITH_NON_CONST_INITIALIZER` | 183 | 24 | 159 |
| `UNSUPPORTED` | 192 | 34 | 158 |
| `USAGE_IS_NOT_INLINABLE` | 177 | 31 | 146 |
| `REDECLARATION` | 192 | 72 | 120 |
| `ANNOTATION_ARGUMENT_MUST_BE_CONST` | 157 | 41 | 116 |
| `ILLEGAL_RESTRICTED_SUSPENDING_FUNCTION_CALL` | 100 | 2 | 98 |
| `OPT_IN_USAGE_ERROR` | 96 | 1 | 95 |
| `INVISIBLE_REFERENCE` | 361 | 270 | 91 |
| `UNINITIALIZED_VARIABLE` | 196 | 108 | 88 |
| `CANNOT_INFER_PARAMETER_TYPE` | 559 | 472 | 87 |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 211 | 127 | 84 |
| `MUST_BE_INITIALIZED` | 101 | 18 | 83 |
| `DSL_SCOPE_VIOLATION` | 108 | 34 | 74 |
| `INITIALIZER_TYPE_MISMATCH` | 381 | 307 | 74 |
| `UNSAFE_CALL` | 471 | 399 | 72 |
| `IMPOSSIBLE_IS_CHECK_ERROR` | 71 | 3 | 68 |
| `TYPE_VARIANCE_CONFLICT_ERROR` | 122 | 54 | 68 |
| `WRONG_ANNOTATION_TARGET_WITH_USE_SITE_TARGET` | 79 | 13 | 66 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS` | 158 | 92 | 66 |
| `NULL_FOR_NONNULL_TYPE` | 102 | 38 | 64 |
| `UNINITIALIZED_ENUM_COMPANION` | 65 | 1 | 64 |
| `REPEATED_ANNOTATION` | 62 | 0 | 62 |
| `MISSING_DEPENDENCY_CLASS` | 61 | 0 | 61 |
| `CANNOT_WEAKEN_ACCESS_PRIVILEGE` | 59 | 1 | 58 |
| `NON_CONST_VAL_USED_IN_CONSTANT_EXPRESSION` | 92 | 36 | 56 |
| `MIXING_SUSPEND_AND_NON_SUSPEND_SUPERTYPES` | 83 | 28 | 55 |
| `NO_RETURN_IN_FUNCTION_WITH_BLOCK_BODY` | 145 | 90 | 55 |
| `NO_VALUE_FOR_PARAMETER` | 167 | 113 | 54 |
| `TYPE_INFERENCE_ONLY_INPUT_TYPES_ERROR` | 61 | 8 | 53 |
| `SMARTCAST_IMPOSSIBLE` | 176 | 131 | 45 |
| `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` | 85 | 40 | 45 |
| `UPPER_BOUND_VIOLATED` | 110 | 68 | 42 |
| `NON_PUBLIC_CALL_FROM_PUBLIC_INLINE` | 59 | 18 | 41 |
| `WRONG_MODIFIER_CONTAINING_DECLARATION` | 56 | 16 | 40 |
| `INAPPLICABLE_OPERATOR_MODIFIER` | 143 | 104 | 39 |
| `MUST_BE_INITIALIZED_OR_BE_ABSTRACT` | 78 | 39 | 39 |
| `TOO_MANY_ARGUMENTS` | 102 | 63 | 39 |
| `AMBIGUOUS_ANONYMOUS_TYPE_INFERRED` | 38 | 0 | 38 |
| `ERROR_IN_CONTRACT_DESCRIPTION` | 38 | 0 | 38 |
| `REPEATED_MODIFIER` | 39 | 1 | 38 |
| `ACCIDENTAL_OVERRIDE` | 36 | 1 | 35 |
| `EXPOSED_PARAMETER_TYPE` | 40 | 6 | 34 |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 120 | 86 | 34 |
| `DEPRECATION_ERROR` | 42 | 9 | 33 |
| `CONFUSING_BRANCH_CONDITION_ERROR` | 35 | 3 | 32 |
| `INCOMPATIBLE_MODIFIERS` | 44 | 12 | 32 |
| `EXPLICIT_TYPE_ARGUMENTS_IN_PROPERTY_ACCESS` | 36 | 5 | 31 |
| `INAPPLICABLE_JVM_FIELD` | 33 | 2 | 31 |
| `ASSIGNMENT_IN_EXPRESSION_CONTEXT` | 32 | 3 | 29 |
| `ILLEGAL_KOTLIN_VERSION_STRING_VALUE` | 28 | 0 | 28 |
| `JVM_EXPOSE_BOXED_CANNOT_EXPOSE_PRIVATE` | 28 | 0 | 28 |
| `UNDERSCORE_USAGE_WITHOUT_BACKTICKS` | 28 | 0 | 28 |
| *511 more* | | | |

## Errors kotlinc doesn't report

kc's errors on lines where the markup expects no diagnostic, in the tests it checked, with names replaced by _. In a test kotlinc compiles, one is a false error; in one it rejects, it can also be the same error on another line, or one kotlinc doesn't report after a first.

| Error | Count |
|:---|---:|
| `cannot infer a type for this parameter. Please specify it explicitly.` | 1059 |
| `cannot infer type for type parameter '_'. Specify it explicitly.` | 633 |
| `only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type _` | 484 |
| `unresolved reference: _` | 432 |
| `type mismatch: inferred type is _ but _ was expected` | 384 |
| `conflicting overloads: _` | 305 |
| `unresolved import: _` | 76 |
| `class '_' is not abstract and does not implement abstract member '_'` | 56 |
| `suspend function '_' cannot override non-suspend function '_' defined in '_'` | 50 |
| `ambiguous call: n candidates for '_' take (_)` | 48 |
| `missing return statement in function '_'` | 35 |
| `expected member declaration, found '_'` | 31 |
| `no function '_' takes n arguments` | 31 |
| `unresolved or unsupported type: _` | 27 |
| `expected expression, found '_'` | 23 |
| `'_' expression must be exhaustive, add necessary '_' branch` | 21 |
| `variable '_' must be initialized` | 20 |
| `val cannot be reassigned` | 17 |
| `operator '_' cannot be applied to Any?` | 16 |
| `tailrec is not allowed on open members` | 16 |
| `type checking has run into a recursive problem; specify the type of '_'` | 16 |
| `'_' overrides nothing` | 15 |
| `expected declaration, found '_'` | 15 |
| `not enough information to infer type arguments of ::foon: specify the expected function type` | 14 |
| `operator '_' cannot be applied to String` | 14 |
| `expected n parameter of type _ but n were declared` | 13 |
| `overload resolution ambiguity for ::foo: specify the expected function type` | 13 |
| `annotation class cannot have type parameters` | 12 |
| `expected no parameters, but n were declared` | 12 |
| `expression '_' of type _ cannot be invoked as a function` | 12 |
| *200 more* | |

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
| `safe-call assignment` | 2 | 2 | `tests/inference/pcla/fixationOnDemand/syntaxForms/PostfixIncrementOperatorCall.kt`, `tests/inference/pcla/fixationOnDemand/syntaxForms/PrefixIncrementOperatorCall.kt` |
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
| 14 | `safe-call assignment` | 2 | 76 |
| 15 | `checker: lambdas with more than n parameters are not supported` | 1 | 77 |
| 16 | `package` | 1 | 78 |
| 17 | `parse error: invalid float literal` | 1 | 79 |

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

