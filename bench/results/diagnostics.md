# Kotlin's diagnostics tests on kc

The diagnostics tests of JetBrains/kotlin v2.4.21 for the JVM (`compiler/testData/diagnostics/`: tests/, testsWithStdLib/, testsWithJvmBackend/, testsWithAnyBackend/), each compiled by kc. A test's markup marks the diagnostics kotlinc reports, and its `RUN_PIPELINE_TILL` says whether kotlinc stops with errors in the frontend (`FRONTEND`, `FIR2IR`) or goes on to the backend (`BACKEND`). `bench/kotlin-tests.sh diagnostics` writes this file, and [docs/conformance.md](../../docs/conformance.md) explains it.

Of 10827 test files, 2295 are skipped and 8532 run.

| Result | Tests | Share |
|:---|---:|---:|
| rejects invalid | 4289 | 50.3% |
| accepts valid | 4115 | 48.2% |
| accepts invalid | 104 | 1.2% |
| rejects valid | 7 | 0.1% |
| parse/unsupported | 16 | 0.2% |
| crash | 0 | 0.0% |

*Rejects invalid* and *accepts valid* agree with kotlinc. *Accepts invalid* is code kotlinc rejects that kc compiles; *rejects valid*, code kotlinc compiles that kc's checker rejects. Both are bugs. A *parse/unsupported* test uses a feature kc doesn't support, which says nothing about its checks.

Of the 4289 tests both reject, kc reports an error on the line of an expected one in 4199.

A test is invalid if it stops before the backend or marks an error: one that runs to it (`BACKEND`) can mark the JVM backend's errors, such as `CONFLICTING_JVM_DECLARATIONS`. Of the tests run, 222 stop at `FRONTEND` but mark no error: most add the `CHECK_TYPE` helpers, whose names (`_`) kotlinc reports outside the test infrastructure, and count as valid; the others leave their errors out with the `DIAGNOSTICS` directive. 114 run to the backend and mark one. 5 marked names have no declared severity (debug information, for one), and count as neither.

The names without a severity: `DEBUG_INFO_EXPRESSION_TYPE` (742), `DEBUG_INFO_CSR_MIGHT_BE_USED` (44), `DEBUG_INFO_CALL` (28), `DEBUG_INFO_CSR_MIGHT_BE_USED_INSTEAD_OF_IMPORT` (16), `DEBUG_INFO_CALLABLE_OWNER` (6).

## Skipped

Tests for other platforms, that need language features off by default or compiler options, or modules other than plain dependencies, or whose FIR diagnostics aren't checked.

| Reason | Tests | Examples |
|:---|---:|:---|
| `LANGUAGE` | 1579 | `tests/BacktickNames.kt`, `tests/BacktickNamesOld.kt` |
| `modules with dependsOn` | 357 | `tests/multiplatform/abstractEnumInIr.kt`, `tests/multiplatform/actualAnnotationsNotMatchExpect/actualInnerClassMissingMember.kt` |
| `LANGUAGE_FEATURE_TOGGLED` | 93 | `tests/annotatedPackageQualifiers.kt`, `tests/annotationUsedAsAnnotationArgument.kt` |
| `return value checker (crv/, crvFull/)` | 83 | `tests/crv/assignments.kt`, `tests/crv/basicJInterop.kt` |
| `API_VERSION` | 53 | `tests/deprecated/deprecatedSinceKotlin/error.kt`, `tests/deprecated/deprecatedSinceKotlin/hidden.kt` |
| `ALLOW_KOTLIN_PACKAGE` | 47 | `tests/annotations/requireKotlin.kt`, `tests/crvDisabled/oldMustUse.kt` |
| `JDK_KIND` | 27 | `tests/errorProneAtomicArrayPrimitivesAllJavaMethods.kt`, `tests/identityComparisonOnJavaValueBasedClassesWarning.kt` |
| `explicit API mode` | 21 | `tests/testsWithExplicitApi/annotations.kt`, `tests/testsWithExplicitApi/classes.kt` |
| `IGNORE_FIR_DIAGNOSTICS` | 18 | `tests/jvm/contextParameters/kt59590WithContextParameters.kt`, `tests/jvm/duplicateJvmSignature/accidentalOverrides/overridesNothing.kt` |
| `header mode` | 9 | `tests/headerMode/anonymousObjects.kt`, `tests/headerMode/classDeclaration.kt` |
| `multiplatform` | 3 | `tests/multiplatform/errorProneAtomicArrayArgumentsInKMP.kt`, `tests/multiplatform/errorProneAtomicReferenceArgumentsInKMP.kt` |
| `TARGET_BACKEND: NATIVE, JS_IR, WASM` | 2 | `testsWithAnyBackend/inlineOverrideDefaultWarning.kt`, `testsWithAnyBackend/inlineOverrideDefaultWarningWithInlinedFunInKlib.kt` |
| `strict equality` | 2 | `tests/strictEquals/enabled/simple.kt`, `tests/strictEquals/enabled/wrongElementsAnnotated.kt` |
| `TARGET_BACKEND: NATIVE, JS_IR` | 1 | `testsWithAnyBackend/crossFileInlineCycle.kt` |

## Accepts invalid

Tests kotlinc rejects that kc compiles, by the errors their markup expects. A test counts for each; *Only error* counts the tests it is the only expected error of, which implementing it alone would fix.

| Expected error | Tests | Only error | Examples |
|:---|---:|---:|:---|
| `ARGUMENT_TYPE_MISMATCH` | 15 | 12 | `tests/annotations/annotationArgumentScopeForNestedClass.kt`, `tests/collectionLiterals/kt82964.kt` |
| `UNRESOLVED_REFERENCE` | 14 | 11 | `tests/callResolution/eagerLambdaAnalysis/samVsDefault.kt`, `tests/imports/ImportFromCurrentWithDifferentName.kt` |
| `UNSUPPORTED` | 7 | 5 | `tests/NamedFunctionTypeParameterInSupertype.kt`, `tests/callableReference/property/backingField.kt` |
| `(no error marked)` | 5 | 5 | `tests/deprecated/javaDeprecated.kt`, `tests/j+k/missingTypeOfParameterInFakeOverride_1.kt` |
| `CANNOT_INFER_PARAMETER_TYPE` | 3 | 2 | `tests/callableReference/function/ambiguityTopLevelVsTopLevelGeneric.kt`, `tests/j+k/sam/referenceToSamFunctionAgainstExpectedType.kt` |
| `INAPPLICABLE_CANDIDATE` | 3 | 2 | `tests/callableReference/generic/dependOnArgumentType.kt`, `tests/j+k/sam/referenceToSamFunctionAgainstExpectedType.kt` |
| `INITIALIZER_TYPE_MISMATCH` | 3 | 1 | `tests/callableReference/adapted/noKFunctionForAdaptation.kt`, `tests/j+k/sam/referenceToSamFunctionAgainstExpectedType.kt` |
| `NOT_FUNCTION_AS_OPERATOR` | 2 | 2 | `tests/kt34857.kt`, `tests/problems/kt12610.kt` |
| `NOT_YET_SUPPORTED_IN_INLINE` | 2 | 2 | `tests/inline/defaultLambdaInlineSuspend.kt`, `tests/inlineFunctionContextBug.kt` |
| `REPEATED_ANNOTATION` | 2 | 2 | `tests/typealias/typeAliasExpansionRepeatedAnnotations.kt`, `tests/typealias/typealiasRhsRepeatedAnnotationInArguments.kt` |
| `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` | 2 | 2 | `tests/j+k/recursionWithJavaSyntheticProperty.kt`, `tests/j+k/syntheticPropertyOverridden2.kt` |
| `UNINITIALIZED_ENUM_ENTRY` | 2 | 2 | `tests/controlFlowAnalysis/enumInterdependence.kt`, `tests/controlFlowAnalysis/uninitializedQualifiedEnumEntry.kt` |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 2 | 2 | `tests/platformTypes/commonSupertype/withNothing.kt`, `tests/typealias/innerClassTypeAliasConstructor.kt` |
| `UNSAFE_CALL` | 2 | 2 | `tests/smartCasts/alwaysNullWithJava.kt`, `tests/smartCasts/noSmartcastForNonFinalMembersFromJava.kt` |
| `UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH` | 2 | 2 | `tests/inlineClasses/unsignedLiteralsWithoutArtifactOnClasspath.kt`, `tests/valueClasses/unsignedLiteralsWithoutArtifactOnClasspath.kt` |
| `WRONG_ANNOTATION_TARGET` | 2 | 1 | `tests/annotations/options/targets/java.kt`, `tests/annotations/options/targets/mixedTargeting.kt` |
| `ABSTRACT_CLASS_MEMBER_NOT_IMPLEMENTED` | 1 | 1 | `tests/j+k/missingTypeOfParameterInFakeOverride_2.kt` |
| `ACCIDENTAL_OVERRIDE` | 1 | 1 | `tests/j+k/kjkimplicitTypesCrash.kt` |
| `ADAPTED_CALLABLE_REFERENCE_AGAINST_REFLECTION_TYPE` | 1 | 0 | `tests/callableReference/adapted/noKFunctionForAdaptation.kt` |
| `AMBIGUOUS_SUPER` | 1 | 1 | `tests/thisAndSuper/unqualifiedSuper/unqualifiedSuperWithCallableProperty.kt` |
| `ANNOTATION_IN_CONTRACT_ERROR` | 1 | 1 | `tests/contracts/suppressContractWarning.kt` |
| `ASSIGNING_SINGLE_ELEMENT_TO_VARARG_IN_NAMED_FORM_ANNOTATION_ERROR` | 1 | 0 | `tests/varargs/assigningSingleElementsInNamedFormAnnDeprecation.kt` |
| `ASSIGNMENT_TYPE_MISMATCH` | 1 | 0 | `tests/controlFlowAnalysis/smartcastOnThisInInitializationSection.kt` |
| `CONDITION_TYPE_MISMATCH` | 1 | 0 | `tests/syntax/complicatedLTGT.kt` |
| `CONFLICTING_JVM_DECLARATIONS` | 1 | 1 | `tests/jvm/duplicateJvmSignature/specialNames/classObjectCopiedField.kt` |
| `CONSTRUCTOR_OR_SUPERTYPE_ON_TYPEALIAS_WITH_TYPE_PROJECTION_ERROR` | 1 | 0 | `tests/typealias/typeAliasForProjectionInSuperInterfaces.kt` |
| `CONST_VAL_WITH_NON_CONST_INITIALIZER` | 1 | 1 | `tests/modifiers/const/kotlinJavaCycle.kt` |
| `DATA_CLASS_INVISIBLE_COPY_USAGE_ERROR` | 1 | 1 | `tests/jvm/dataClassNonPublicConstructor/dataClassInternalConstructorUsageWillBecomeInaccessible.kt` |
| `DEFINITELY_NON_NULLABLE_AS_REIFIED` | 1 | 1 | `tests/explicitDefinitelyNotNullableViaIntersection/reifiedArguments.kt` |
| `DELEGATE_SPECIAL_FUNCTION_MISSING` | 1 | 1 | `tests/delegatedProperty/inference/provideDelegateReturningOuterTypeVariable.kt` |
| `DELEGATE_USES_EXTENSION_PROPERTY_TYPE_PARAMETER_ERROR` | 1 | 1 | `tests/delegatedProperty/useTypeParameterOfExtensionProperty2.kt` |
| `DEPRECATION_ERROR` | 1 | 0 | `tests/j+k/deprecations/javaWithHiddenDeprecation.kt` |
| `EXPLICIT_TYPE_ARGUMENTS_IN_PROPERTY_ACCESS` | 1 | 0 | `tests/syntax/complicatedLTGT.kt` |
| `FUNCTION_DELEGATE_MEMBER_NAME_CLASH` | 1 | 1 | `tests/funInterface/functionDelegateClashOnJvm.kt` |
| `FUNCTION_EXPECTED` | 1 | 1 | `tests/properties/syntheticPropertiesForJavaAnnotations.kt` |
| `ILLEGAL_SUSPEND_FUNCTION_CALL` | 1 | 1 | `tests/callResolution/eagerLambdaAnalysis/suspendFunctionTypeVsRunnable.kt` |
| `INCONSISTENT_TYPE_PARAMETER_BOUNDS` | 1 | 1 | `tests/generics/conflictingStarAndTypeParameterBounds.kt` |
| `INLINE_FROM_HIGHER_PLATFORM` | 1 | 1 | `tests/jvm/inline/wrongInlineTarget.kt` |
| `INVALID_VERSIONING_ON_NON_OPTIONAL` | 1 | 1 | `tests/versionOverload/propertySetterParameter.kt` |
| `INVISIBLE_REFERENCE` | 1 | 1 | `tests/arguments/ambiguityOnJavaOverride.kt` |
| `JAVA_MODULE_DOES_NOT_EXPORT_PACKAGE` | 1 | 1 | `tests/testsWithJava17/nullSourceInJvmModuleChecker.kt` |
| `JVM_EXPOSE_BOXED_CANNOT_BE_THE_SAME` | 1 | 1 | `tests/inlineClasses/jvmExposeBoxed/kt85086.kt` |
| `JVM_EXPOSE_BOXED_CANNOT_EXPOSE_PRIVATE` | 1 | 1 | `tests/inlineClasses/jvmExposeBoxed/private.kt` |
| `MANY_IMPL_MEMBER_NOT_IMPLEMENTED` | 1 | 1 | `testsWithStdLib/purelyImplementedCollection/intersectionOfPrimitiveBasedFunctions.kt` |
| `MEMBER_PROJECTED_OUT` | 1 | 1 | `tests/typealias/starProjection.kt` |
| `MISSING_DEPENDENCY_CLASS` | 1 | 0 | `tests/j+k/differentFilename.kt` |
| `MUST_BE_INITIALIZED_OR_BE_ABSTRACT` | 1 | 0 | `tests/controlFlowAnalysis/smartcastOnThisInInitializationSection.kt` |
| `NONE_APPLICABLE` | 1 | 1 | `tests/secondaryConstructors/generics3.kt` |
| `NON_CONST_VAL_USED_IN_CONSTANT_EXPRESSION` | 1 | 0 | `tests/annotations/annotationArgumentScopeForNestedClass.kt` |
| `NOTHING_TO_OVERRIDE` | 1 | 1 | `tests/multimodule/duplicateSuper/sameSuperTraitDifferentBounds.kt` |
| `NOT_ALL_MULTIFILE_CLASS_PARTS_ARE_JVM_SYNTHETIC` | 1 | 1 | `tests/jvm/multifileClasses/jvmSynthetic.kt` |
| `NO_ELSE_IN_WHEN` | 1 | 1 | `tests/testsWithJava17/sealedClasses/flexibleSealedInSubject.kt` |
| `NO_VALUE_FOR_PARAMETER` | 1 | 1 | `tests/resolve/invoke/kt9805.kt` |
| `OPT_IN_TO_INHERITANCE_ERROR` | 1 | 1 | `testsWithStdLib/annotations/subclassOptInRequired/JavaKotlinInterop.kt` |
| `PROJECTION_IN_IMMEDIATE_ARGUMENT_TO_SUPERTYPE` | 1 | 0 | `tests/typealias/typeAliasForProjectionInSuperInterfaces.kt` |
| `REIFIED_TYPE_FORBIDDEN_SUBSTITUTION` | 1 | 0 | `testsWithStdLib/ArrayOfNothing.kt` |
| `RETURN_TYPE_MISMATCH` | 1 | 1 | `testsWithStdLib/functionLiterals/nullableUnitLiteral.kt` |
| `RETURN_TYPE_MISMATCH_ON_INHERITANCE` | 1 | 1 | `tests/override/clashesOnInheritance/kt13355viaJava.kt` |
| `SEALED_INHERITOR_IN_DIFFERENT_MODULE` | 1 | 1 | `tests/sealed/interfaces/inheritorInDifferentModule.kt` |
| `SMARTCAST_IMPOSSIBLE` | 1 | 1 | `tests/smartCasts/inference/syntheticPropertyOnUnstableSmartcast.kt` |
| *11 more* | 11 | |

## Rejects valid

Tests kotlinc compiles that kc's checker rejects, by kc's first error, with names replaced by _.

| First error | Tests | Examples |
|:---|---:|:---|
| `unresolved reference: _` | 3 | `tests/classpathSubstitution/localClasspathSubstitution.kt`, `tests/multimodule/duplicateClass/inTheSameModuleWithUsageNoTypeAnnotation.kt` |
| `type mismatch: inferred type is _ but _ was expected` | 2 | `tests/multimodule/duplicateClass/inTheSameModuleWithUsage.kt`, `testsWithStdLib/properties/backingField/explicitBackingFieldPrivateToThis.kt` |
| `expression '_' of type _ cannot be invoked as a function` | 1 | `tests/nullabilityAndSmartCasts/kt1778.kt` |
| `unresolved import: _` | 1 | `tests/platformTypes/notNullTypeParameter/enhancementFromKotlin.kt` |

## Errors by diagnostic

For each expected error of the tests kc checked (not unsupported), whether kc reported an error on its line. Sorted by the errors kc missed.

kc reports 18304 of 21536 expected errors (85.0%) on their lines.

| Diagnostic | Expected | Reported | Missed |
|:---|---:|---:|---:|
| `UNRESOLVED_REFERENCE` | 2500 | 2120 | 380 |
| `ARGUMENT_TYPE_MISMATCH` | 1484 | 1266 | 218 |
| `UNSUPPORTED` | 231 | 61 | 170 |
| `CONFLICTING_OVERLOADS` | 726 | 629 | 97 |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 250 | 160 | 90 |
| `RETURN_TYPE_MISMATCH` | 417 | 346 | 71 |
| `CANNOT_INFER_PARAMETER_TYPE` | 631 | 566 | 65 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS` | 163 | 105 | 58 |
| `INVISIBLE_REFERENCE` | 523 | 467 | 56 |
| `MIXING_SUSPEND_AND_NON_SUSPEND_SUPERTYPES` | 83 | 28 | 55 |
| `CONST_VAL_WITH_NON_CONST_INITIALIZER` | 251 | 198 | 53 |
| `ANNOTATION_ARGUMENT_MUST_BE_CONST` | 224 | 176 | 48 |
| `WRONG_ANNOTATION_TARGET` | 380 | 333 | 47 |
| `NON_PUBLIC_CALL_FROM_PUBLIC_INLINE` | 65 | 19 | 46 |
| `UNINITIALIZED_VARIABLE` | 211 | 165 | 46 |
| `INITIALIZER_TYPE_MISMATCH` | 428 | 387 | 41 |
| `REDECLARATION` | 192 | 157 | 35 |
| `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` | 87 | 52 | 35 |
| `INAPPLICABLE_OPERATOR_MODIFIER` | 143 | 109 | 34 |
| `TYPE_VARIANCE_CONFLICT_ERROR` | 123 | 89 | 34 |
| `NON_PUBLIC_INLINE_CALL_FROM_PUBLIC_INLINE` | 60 | 28 | 32 |
| `NULL_FOR_NONNULL_TYPE` | 155 | 123 | 32 |
| `INAPPLICABLE_JVM_FIELD` | 33 | 2 | 31 |
| `MUST_BE_INITIALIZED` | 102 | 72 | 30 |
| `JVM_EXPOSE_BOXED_CANNOT_EXPOSE_PRIVATE` | 28 | 0 | 28 |
| `EXPLICIT_TYPE_ARGUMENTS_IN_PROPERTY_ACCESS` | 37 | 10 | 27 |
| `ASSIGNMENT_TYPE_MISMATCH` | 114 | 89 | 25 |
| `NO_VALUE_FOR_PARAMETER` | 175 | 150 | 25 |
| `SUBCLASS_OPT_IN_INAPPLICABLE` | 25 | 0 | 25 |
| `CONCURRENT_HASH_MAP_CONTAINS_OPERATOR_ERROR` | 24 | 0 | 24 |
| `SMARTCAST_IMPOSSIBLE` | 183 | 159 | 24 |
| `TYPE_ARGUMENTS_FOR_OUTER_CLASS_WHEN_NESTED_REFERENCED` | 41 | 17 | 24 |
| `UNINITIALIZED_ENUM_COMPANION` | 65 | 42 | 23 |
| `INLINE_FROM_HIGHER_PLATFORM` | 21 | 0 | 21 |
| `OPT_IN_USAGE_ERROR` | 96 | 76 | 20 |
| `TOO_MANY_ARGUMENTS` | 118 | 98 | 20 |
| `UPPER_BOUND_VIOLATED` | 125 | 105 | 20 |
| `INAPPLICABLE_CANDIDATE` | 127 | 110 | 17 |
| `OPT_IN_MARKER_CAN_ONLY_BE_USED_AS_ANNOTATION_OR_ARGUMENT_IN_OPT_IN` | 19 | 2 | 17 |
| `DSL_SCOPE_VIOLATION` | 108 | 92 | 16 |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 159 | 143 | 16 |
| `ASSIGN_OPERATOR_AMBIGUITY` | 35 | 20 | 15 |
| `NOT_YET_SUPPORTED_IN_INLINE` | 23 | 8 | 15 |
| `NO_EXPLICIT_VISIBILITY_IN_API_MODE` | 16 | 1 | 15 |
| `REPEATED_ANNOTATION` | 69 | 54 | 15 |
| `UNINITIALIZED_ENUM_ENTRY` | 15 | 0 | 15 |
| `WRONG_ANNOTATION_TARGET_WITH_USE_SITE_TARGET` | 80 | 65 | 15 |
| `CONTRACT_NOT_ALLOWED` | 19 | 5 | 14 |
| `NON_CONST_VAL_USED_IN_CONSTANT_EXPRESSION` | 96 | 82 | 14 |
| `NOTHING_TO_OVERRIDE` | 93 | 79 | 14 |
| `UNEXPECTED_TRAILING_LAMBDA_ON_A_NEW_LINE` | 14 | 0 | 14 |
| `VAL_REASSIGNMENT` | 231 | 217 | 14 |
| `CAPTURED_VAL_INITIALIZATION` | 34 | 21 | 13 |
| `INFIX_MODIFIER_REQUIRED` | 46 | 33 | 13 |
| `INAPPLICABLE_JVM_NAME` | 12 | 0 | 12 |
| `MULTIPLE_LABELS_ARE_FORBIDDEN` | 17 | 5 | 12 |
| `NAME_FOR_AMBIGUOUS_PARAMETER` | 31 | 20 | 11 |
| `TYPE_PARAMETERS_NOT_ALLOWED` | 13 | 2 | 11 |
| `UNRESOLVED_IMPORT` | 50 | 39 | 11 |
| `CLASSIFIER_REDECLARATION` | 41 | 31 | 10 |
| *534 more* | | | |

### Missed errors by cause

Why kc reports no error on the line of an expected one: *accepted*, it compiled the test; *syntax-error*, its parser rejected the code, and kc checked the declarations it recovered, without those with a syntax error; *module-not-compiled*, the error is in a module after one that failed, which isn't compiled; *nearby*, kc reported an error within 3 lines of it on a line the markup expects none on (most likely the error, at another position); *missing-check*, none of those.

| Cause | Missed | Share | Most missed |
|:---|---:|---:|:---|
| accepted | 361 | 11.2% | `UNSUPPORTED` 45, `UNRESOLVED_REFERENCE` 41, `WRONG_ANNOTATION_TARGET` 34, `ARGUMENT_TYPE_MISMATCH` 33, `JVM_EXPOSE_BOXED_CANNOT_EXPOSE_PRIVATE` 28 |
| syntax-error | 944 | 29.2% | `UNRESOLVED_REFERENCE` 181, `UNSUPPORTED` 122, `WRONG_NUMBER_OF_TYPE_ARGUMENTS` 50, `ARGUMENT_TYPE_MISMATCH` 48, `ANNOTATION_ARGUMENT_MUST_BE_CONST` 32 |
| module-not-compiled | 0 | 0.0% |  |
| nearby | 231 | 7.1% | `MIXING_SUSPEND_AND_NON_SUSPEND_SUPERTYPES` 36, `ARGUMENT_TYPE_MISMATCH` 29, `RETURN_TYPE_MISMATCH` 29, `UNRESOLVED_REFERENCE` 28, `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` 10 |
| missing-check | 1696 | 52.5% | `UNRESOLVED_REFERENCE` 130, `ARGUMENT_TYPE_MISMATCH` 108, `CONFLICTING_OVERLOADS` 95, `UNRESOLVED_REFERENCE_WRONG_RECEIVER` 78, `INVISIBLE_REFERENCE` 52 |

## Errors kotlinc doesn't report

kc's errors on lines where the markup expects no diagnostic, in the tests it checked, with names replaced by _. In a test kotlinc compiles, one is a false error; in one it rejects, it can also be the same error on another line, or one kotlinc doesn't report after a first.

| Error | Count |
|:---|---:|
| `unresolved reference: _` | 233 |
| `only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type _` | 185 |
| `cannot infer type for type parameter '_'. Specify it explicitly.` | 180 |
| `type mismatch: inferred type is _ but _ was expected` | 177 |
| `class '_' is not abstract and does not implement abstract member '_'` | 57 |
| `suspend function '_' cannot override non-suspend function '_' defined in '_'` | 50 |
| `unresolved or unsupported type: _` | 33 |
| `variable '_' must be initialized` | 17 |
| `'_' expression must be exhaustive, add necessary '_' branch` | 16 |
| `cannot use '_' as reified type parameter` | 15 |
| `no function '_' takes n arguments` | 15 |
| `not enough information to infer type arguments of ::foon: specify the expected function type` | 14 |
| `operator '_' cannot be applied to Any?` | 14 |
| `private setters are not allowed for open properties` | 13 |
| `unresolved import: _` | 13 |
| `this lambda must return a value of type _` | 12 |
| `type checking has run into a recursive problem; specify the return type of '_'` | 12 |
| `not enough information to infer type arguments of ::barn: specify the expected function type` | 11 |
| `type checking has run into a recursive problem; specify the type of '_'` | 11 |
| `'_' modifier is required on '_'` | 10 |
| `ambiguous call: n candidates for '_' take (_)` | 10 |
| `expected parameter of type _` | 10 |
| `not enough information to infer type arguments of ::foo: specify the expected function type` | 10 |
| `operator '_' cannot be applied to A and Boolean` | 10 |
| `overload resolution ambiguity for ::foo: specify the expected function type` | 10 |
| `cannot access '_': it is private to this in '_'` | 9 |
| `not enough information to infer type arguments of ::bar: specify the expected function type` | 9 |
| `'_' is not allowed outside a loop` | 8 |
| `expected n parameter of type _ but n were declared` | 8 |
| `'_' overrides nothing` | 7 |
| *169 more* | |

## Unsupported features

By the features the parser (with error recovery) and the checker found. A test counts for each.

| Feature | Tests | Only blocker | Examples |
|:---|---:|---:|:---|
| `destructuring declaration` | 4 | 4 | `tests/destructuring/topLevelWithLambda.kt`, `tests/diagnostics/conflictingOverloads.kt` |
| `callable reference` | 3 | 3 | `tests/callableReference/compatibilityResolveWithVarargAndOperatorCall.kt`, `tests/callableReference/resolve/resolveReferenceAgainstKFunctionAndKPrpoerty.kt` |
| `checker: '_' is not allowed here: local interfaces are not supported` | 2 | 2 | `tests/localInterfaces.kt`, `tests/thisAndSuper/unqualifiedSuper/unqualifiedSuperWithLocalClass.kt` |
| `property reference` | 2 | 2 | `tests/callableReference/unsupported/localVariableWithSubstitution.kt`, `tests/callableReference/unsupported/parameterWithSubstitution.kt` |
| `checker: lambdas with more than n parameters are not supported` | 1 | 1 | `tests/tooLargeFunctionNThroughACallableReference.kt` |
| `package` | 1 | 1 | `tests/modifiers/IllegalModifiers.kt` |
| `parse error: expected '_', found '_'` | 1 | 1 | `tests/innerClasses/inCallableReferenceLHS/withParenthesizes.kt` |
| `parse error: expected expression, found '_'` | 1 | 1 | `tests/innerClasses/inCallableReferenceLHS/misplacedArgumentsInChain.kt` |
| `unsupported types in signature` | 1 | 1 | `tests/syntheticExtensions/samAdapters/InnerClassInGeneric.kt` |

### Greedy order

At each step, the feature that unblocks the most tests given the ones before it.

| Step | Feature | Tests unblocked | Total |
|---:|:---|---:|---:|
| 1 | `destructuring declaration` | 4 | 4 |
| 2 | `callable reference` | 3 | 7 |
| 3 | `checker: '_' is not allowed here: local interfaces are not supported` | 2 | 9 |
| 4 | `property reference` | 2 | 11 |
| 5 | `checker: lambdas with more than n parameters are not supported` | 1 | 12 |
| 6 | `package` | 1 | 13 |
| 7 | `parse error: expected '_', found '_'` | 1 | 14 |
| 8 | `parse error: expected expression, found '_'` | 1 | 15 |
| 9 | `unsupported types in signature` | 1 | 16 |

## Warnings

Of the tests kc compiled, warnings don't decide the category: those are about errors. kc reports 507 of the 3634 warnings the tests expect (14.0%) on their lines, and 8 warnings on lines that don't expect them, but for those a test's `DIAGNOSTICS` directive switches off.

| Warning kc reports | Expected | On their lines | Elsewhere |
|:---|---:|---:|---:|
| `DEPRECATION` | 199 | 115 | 3 |
| `ERROR_SUPPRESSION` | 102 | 6 | 0 |
| `UNNECESSARY_NOT_NULL_ASSERTION` | 93 | 85 | 0 |
| `USELESS_CAST` | 91 | 28 | 2 |
| `SENSELESS_COMPARISON` | 90 | 63 | 1 |
| `UNNECESSARY_SAFE_CALL` | 90 | 78 | 1 |
| `UNCHECKED_CAST` | 87 | 39 | 1 |
| `OPT_IN_USAGE` | 83 | 62 | 0 |
| `USELESS_ELVIS` | 44 | 27 | 0 |
| `TYPEALIAS_EXPANSION_DEPRECATION` | 9 | 4 | 0 |

The warnings the tests expect that kc doesn't report, as a backlog.

| Warning | Count |
|:---|---:|
| `REDUNDANT_INTERPOLATION_PREFIX` | 355 |
| `UNREACHABLE_CODE` | 158 |
| `IMPLICIT_BOXING_IN_IDENTITY_EQUALS` | 152 |
| `UNUSED_EXPRESSION` | 136 |
| `USELESS_IS_CHECK` | 127 |
| `NOTHING_TO_INLINE` | 95 |
| `CAST_NEVER_SUCCEEDS` | 92 |
| `CONTEXTUAL_OVERLOAD_SHADOWED` | 75 |
| `ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY` | 70 |
| `EXTENSION_SHADOWED_BY_MEMBER` | 67 |
| `FINAL_UPPER_BOUND` | 64 |
| `PLATFORM_CLASS_MAPPED_TO_KOTLIN` | 62 |
| `NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL` | 59 |
| `IMPOSSIBLE_IS_CHECK_WARNING` | 57 |
| `REDUNDANT_PROJECTION` | 49 |
| `NO_REFLECTION_IN_CLASS_PATH` | 36 |
| `UNUSED_VARIABLE` | 35 |
| `ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY` | 33 |
| `ASSIGNED_VALUE_IS_NEVER_READ` | 32 |
| `OVERRIDE_DEPRECATION` | 30 |
| `NO_TAIL_CALLS_FOUND` | 29 |
| `DEPRECATED_IDENTITY_EQUALS` | 26 |
| `INCOMPATIBLE_ENUM_COMPARISON` | 26 |
| `INLINE_CLASS_DEPRECATED` | 26 |
| `NOT_NULL_ASSERTION_ON_CALLABLE_REFERENCE` | 26 |
| `OVERRIDE_BY_INLINE` | 26 |
| `REDUNDANT_ELSE_IN_WHEN` | 26 |
| `DIVISION_BY_ZERO` | 25 |
| `LEAKED_IN_PLACE_LAMBDA` | 25 |
| `REDUNDANT_CALL_OF_CONVERSION_METHOD` | 23 |
| *94 more* | |

