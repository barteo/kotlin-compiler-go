# Kotlin's diagnostics tests on kc

The diagnostics tests of JetBrains/kotlin v2.4.21 for the JVM (`compiler/testData/diagnostics/`: tests/, testsWithStdLib/, testsWithJvmBackend/, testsWithAnyBackend/), each compiled by kc. A test's markup marks the diagnostics kotlinc reports, and its `RUN_PIPELINE_TILL` says whether kotlinc stops with errors in the frontend (`FRONTEND`, `FIR2IR`) or goes on to the backend (`BACKEND`). `bench/kotlin-tests.sh diagnostics` writes this file, and [docs/conformance.md](../../docs/conformance.md) explains it.

Of 10827 test files, 2295 are skipped and 8532 run.

| Result | Tests | Share |
|:---|---:|---:|
| rejects invalid | 4233 | 49.6% |
| accepts valid | 4104 | 48.1% |
| accepts invalid | 159 | 1.9% |
| rejects valid | 20 | 0.2% |
| parse/unsupported | 14 | 0.2% |
| crash | 1 | 0.0% |

*Rejects invalid* and *accepts valid* agree with kotlinc. *Accepts invalid* is code kotlinc rejects that kc compiles; *rejects valid*, code kotlinc compiles that kc's checker rejects. Both are bugs. A *parse/unsupported* test uses a feature kc doesn't support, which says nothing about its checks.

Of the 4233 tests both reject, kc reports an error on the line of an expected one in 4083.

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
| `ARGUMENT_TYPE_MISMATCH` | 17 | 14 | `tests/annotations/annotationArgumentScopeForNestedClass.kt`, `tests/collectionLiterals/kt82964.kt` |
| `UNRESOLVED_REFERENCE` | 15 | 12 | `tests/callResolution/eagerLambdaAnalysis/samVsDefault.kt`, `tests/imports/ImportFromCurrentWithDifferentName.kt` |
| `(no error marked)` | 8 | 8 | `tests/cyclicHierarchy/javaJavaCycle.kt`, `tests/deprecated/javaDeprecated.kt` |
| `INVISIBLE_REFERENCE` | 7 | 7 | `tests/arguments/ambiguityOnJavaOverride.kt`, `tests/classObjects/companionObjectOfPrivateClassVisibility.kt` |
| `UNSUPPORTED` | 7 | 5 | `tests/NamedFunctionTypeParameterInSupertype.kt`, `tests/callableReference/property/backingField.kt` |
| `INAPPLICABLE_CANDIDATE` | 4 | 3 | `tests/callableReference/generic/dependOnArgumentType.kt`, `tests/j+k/sam/referenceToSamFunctionAgainstExpectedType.kt` |
| `INITIALIZER_TYPE_MISMATCH` | 4 | 1 | `tests/callableReference/adapted/noKFunctionForAdaptation.kt`, `tests/j+k/sam/referenceToSamFunctionAgainstExpectedType.kt` |
| `MANY_IMPL_MEMBER_NOT_IMPLEMENTED` | 4 | 4 | `tests/j+k/manyImplNotImplementedComplexHierarchy.kt`, `tests/j+k/manyImplNotImplementedComplexHierarchy2.kt` |
| `NULL_FOR_NONNULL_TYPE` | 3 | 2 | `tests/j+k/jSpecifyWithVarargs.kt`, `tests/jSpecifySimple.kt` |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 3 | 3 | `tests/callResolution/kt50386.kt`, `tests/resolve/overloadConflicts/emptyLambdaUnitNoDiscrimination.kt` |
| `SYNTAX` | 3 | 2 | `tests/explicitDefinitelyNotNullableViaIntersection/isAsOperators.kt`, `tests/modifiers/primaryConstructorMissingBrackets.kt` |
| `AMBIGUOUS_LABEL` | 2 | 2 | `tests/ambiguousLabelInExtensionFunctionWithOuterClass.kt`, `tests/ambiguousLabelInSuper.kt` |
| `ASSIGNMENT_TYPE_MISMATCH` | 2 | 1 | `tests/controlFlowAnalysis/smartcastOnThisInInitializationSection.kt`, `tests/generics/varProjection/setterNotProjectedOutAssign.kt` |
| `CANNOT_INFER_PARAMETER_TYPE` | 2 | 1 | `tests/callableReference/function/ambiguityTopLevelVsTopLevelGeneric.kt`, `tests/j+k/sam/referenceToSamFunctionAgainstExpectedType.kt` |
| `INVISIBLE_SETTER` | 2 | 2 | `tests/extraCheckers/RedundantVisibilityModifierCheckerForInnerComponents.kt`, `tests/properties/kt47621.kt` |
| `JAVA_FIELD_SHADOWED_BY_KOTLIN_PROPERTY` | 2 | 1 | `tests/fieldRename/javaFieldKotlinPropertyJavaFieldInPackagePrivate.kt`, `tests/propertyVsField/propertyFieldShadow.kt` |
| `NOTHING_TO_OVERRIDE` | 2 | 2 | `tests/multimodule/duplicateSuper/sameSuperTraitDifferentBounds.kt`, `tests/rawTypes/overrideRawTypedJavaMethodWithGenericTyped.kt` |
| `NOT_FUNCTION_AS_OPERATOR` | 2 | 2 | `tests/kt34857.kt`, `tests/problems/kt12610.kt` |
| `NOT_YET_SUPPORTED_IN_INLINE` | 2 | 2 | `tests/inline/defaultLambdaInlineSuspend.kt`, `tests/inlineFunctionContextBug.kt` |
| `NO_ELSE_IN_WHEN` | 2 | 2 | `tests/sealed/WhenOnEmptySealed.kt`, `tests/testsWithJava17/sealedClasses/flexibleSealedInSubject.kt` |
| `REPEATED_ANNOTATION` | 2 | 2 | `tests/typealias/typeAliasExpansionRepeatedAnnotations.kt`, `tests/typealias/typealiasRhsRepeatedAnnotationInArguments.kt` |
| `RETURN_TYPE_MISMATCH` | 2 | 2 | `tests/smartCasts/safecalls/receiverAndChainFalse.kt`, `testsWithStdLib/functionLiterals/nullableUnitLiteral.kt` |
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
| `CLASS_CANNOT_BE_EXTENDED_DIRECTLY` | 1 | 1 | `tests/enum/extendingEnumDirectly.kt` |
| `CONDITION_TYPE_MISMATCH` | 1 | 0 | `tests/syntax/complicatedLTGT.kt` |
| `CONFLICTING_JVM_DECLARATIONS` | 1 | 1 | `tests/jvm/duplicateJvmSignature/specialNames/classObjectCopiedField.kt` |
| `CONSTRUCTOR_OR_SUPERTYPE_ON_TYPEALIAS_WITH_TYPE_PROJECTION_ERROR` | 1 | 0 | `tests/typealias/typeAliasForProjectionInSuperInterfaces.kt` |
| `CONST_VAL_WITH_NON_CONST_INITIALIZER` | 1 | 1 | `tests/modifiers/const/kotlinJavaCycle.kt` |
| `CYCLIC_INHERITANCE_HIERARCHY` | 1 | 0 | `tests/cyclicHierarchy/kotlinJavaNestedCycle.kt` |
| `DATA_CLASS_INVISIBLE_COPY_USAGE_ERROR` | 1 | 1 | `tests/jvm/dataClassNonPublicConstructor/dataClassInternalConstructorUsageWillBecomeInaccessible.kt` |
| `DATA_OBJECT_CUSTOM_EQUALS_OR_HASH_CODE` | 1 | 1 | `tests/dataObjects/overrideEqualsAndHashCode.kt` |
| `DEFINITELY_NON_NULLABLE_AS_REIFIED` | 1 | 1 | `tests/explicitDefinitelyNotNullableViaIntersection/reifiedArguments.kt` |
| `DELEGATE_SPECIAL_FUNCTION_MISSING` | 1 | 1 | `tests/delegatedProperty/inference/provideDelegateReturningOuterTypeVariable.kt` |
| `DELEGATE_USES_EXTENSION_PROPERTY_TYPE_PARAMETER_ERROR` | 1 | 1 | `tests/delegatedProperty/useTypeParameterOfExtensionProperty2.kt` |
| `DEPRECATION_ERROR` | 1 | 0 | `tests/j+k/deprecations/javaWithHiddenDeprecation.kt` |
| `EXPLICIT_TYPE_ARGUMENTS_IN_PROPERTY_ACCESS` | 1 | 0 | `tests/syntax/complicatedLTGT.kt` |
| `EXPOSED_FUNCTION_RETURN_TYPE` | 1 | 1 | `tests/exposed/packagePrivate.kt` |
| `EXPOSED_PARAMETER_TYPE` | 1 | 1 | `testsWithStdLib/j+k/MyException.kt` |
| `EXPOSED_SUPER_CLASS` | 1 | 0 | `tests/fieldRename/javaFieldKotlinPropertyJavaFieldInPackagePrivate.kt` |
| `EXPOSED_SUPER_INTERFACE` | 1 | 0 | `tests/cyclicHierarchy/kotlinJavaNestedCycle.kt` |
| `EXTENSION_PROPERTY_WITH_BACKING_FIELD` | 1 | 1 | `tests/expresssions/genericDescriptor.kt` |
| `FORBIDDEN_VARARG_PARAMETER_TYPE` | 1 | 1 | `tests/varargs/varargOfNothing.kt` |
| `FUNCTION_DELEGATE_MEMBER_NAME_CLASH` | 1 | 1 | `tests/funInterface/functionDelegateClashOnJvm.kt` |
| `FUNCTION_EXPECTED` | 1 | 1 | `tests/properties/syntheticPropertiesForJavaAnnotations.kt` |
| `GENERIC_THROWABLE_SUBCLASS` | 1 | 0 | `tests/controlStructures/catchGenerics.kt` |
| `ILLEGAL_SUSPEND_FUNCTION_CALL` | 1 | 1 | `tests/callResolution/eagerLambdaAnalysis/suspendFunctionTypeVsRunnable.kt` |
| `IMPLICIT_NOTHING_PROPERTY_TYPE` | 1 | 1 | `tests/properties/inferenceFromGetters/nullAsNothing.kt` |
| `INCONSISTENT_TYPE_PARAMETER_BOUNDS` | 1 | 1 | `tests/generics/conflictingStarAndTypeParameterBounds.kt` |
| `INCORRECT_TYPE_PARAMETER_OF_PROPERTY` | 1 | 1 | `tests/typeParameters/propertyTypeParametersWithUpperBounds.kt` |
| *41 more* | 41 | |

## Rejects valid

Tests kotlinc compiles that kc's checker rejects, by kc's first error, with names replaced by _.

| First error | Tests | Examples |
|:---|---:|:---|
| `unresolved reference: _` | 6 | `tests/callResolution/suspendCOnversionForReceiver.kt`, `tests/classpathSubstitution/localClasspathSubstitution.kt` |
| `type mismatch: inferred type is _ but _ was expected` | 5 | `tests/multimodule/duplicateClass/inTheSameModuleWithUsage.kt`, `testsWithStdLib/inference/annotationsForResolve/noInferMismatchingClosestReceiver.kt` |
| `ambiguous call: n candidates for '_' take (_)` | 3 | `tests/callableReference/resolve/resolveReferenceAgainstKFunctionAndKPrpoerty.kt`, `tests/problems/kt24230.kt` |
| `'_' modifier is required on '_'` | 1 | `tests/problems/kt36815.kt` |
| `expression '_' of type _ cannot be invoked as a function` | 1 | `tests/nullabilityAndSmartCasts/kt1778.kt` |
| `no function '_' takes n arguments` | 1 | `tests/defaultParametersInheritedToJava.kt` |
| `one type argument expected for class X.Y` | 1 | `tests/innerClasses/inCallableReferenceLHS/misplacedArgumentsInInnerOnly.kt` |
| `type arguments of the outer class of BaseIntrospector.AbstractRetriever are needed here: write BaseIntrospector<...>.AbstractRetriever` | 1 | `testsWithStdLib/intellij/baseIntrospector.kt` |
| `unresolved import: _` | 1 | `tests/platformTypes/notNullTypeParameter/enhancementFromKotlin.kt` |

## Errors by diagnostic

For each expected error of the tests kc checked (not unsupported), whether kc reported an error on its line. Sorted by the errors kc missed.

kc reports 16633 of 21532 expected errors (77.2%) on their lines.

| Diagnostic | Expected | Reported | Missed |
|:---|---:|---:|---:|
| `SYNTAX` | 875 | 224 | 651 |
| `UNRESOLVED_REFERENCE` | 2500 | 1971 | 529 |
| `ARGUMENT_TYPE_MISMATCH` | 1484 | 1254 | 230 |
| `UNSUPPORTED` | 231 | 35 | 196 |
| `RETURN_TYPE_MISMATCH` | 417 | 303 | 114 |
| `CONFLICTING_OVERLOADS` | 726 | 629 | 97 |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 250 | 161 | 89 |
| `INVISIBLE_REFERENCE` | 523 | 437 | 86 |
| `CONST_VAL_WITH_NON_CONST_INITIALIZER` | 251 | 168 | 83 |
| `ANNOTATION_ARGUMENT_MUST_BE_CONST` | 224 | 146 | 78 |
| `CANNOT_INFER_PARAMETER_TYPE` | 627 | 552 | 75 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS` | 163 | 96 | 67 |
| `WRONG_ANNOTATION_TARGET` | 380 | 316 | 64 |
| `NO_RETURN_IN_FUNCTION_WITH_BLOCK_BODY` | 148 | 91 | 57 |
| `TYPE_VARIANCE_CONFLICT_ERROR` | 123 | 66 | 57 |
| `MIXING_SUSPEND_AND_NON_SUSPEND_SUPERTYPES` | 83 | 28 | 55 |
| `INITIALIZER_TYPE_MISMATCH` | 428 | 377 | 51 |
| `NO_VALUE_FOR_PARAMETER` | 175 | 128 | 47 |
| `NON_PUBLIC_CALL_FROM_PUBLIC_INLINE` | 65 | 19 | 46 |
| `UNINITIALIZED_VARIABLE` | 211 | 165 | 46 |
| `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` | 87 | 45 | 42 |
| `REDECLARATION` | 192 | 152 | 40 |
| `NULL_FOR_NONNULL_TYPE` | 155 | 116 | 39 |
| `UPPER_BOUND_VIOLATED` | 125 | 90 | 35 |
| `CONFUSING_BRANCH_CONDITION_ERROR` | 37 | 3 | 34 |
| `INAPPLICABLE_OPERATOR_MODIFIER` | 143 | 109 | 34 |
| `EXPLICIT_TYPE_ARGUMENTS_IN_PROPERTY_ACCESS` | 37 | 5 | 32 |
| `NON_PUBLIC_INLINE_CALL_FROM_PUBLIC_INLINE` | 60 | 28 | 32 |
| `INAPPLICABLE_JVM_FIELD` | 33 | 2 | 31 |
| `MUST_BE_INITIALIZED` | 102 | 72 | 30 |
| `ASSIGNMENT_IN_EXPRESSION_CONTEXT` | 32 | 3 | 29 |
| `JVM_EXPOSE_BOXED_CANNOT_EXPOSE_PRIVATE` | 28 | 0 | 28 |
| `WRONG_ANNOTATION_TARGET_WITH_USE_SITE_TARGET` | 80 | 52 | 28 |
| `ASSIGNMENT_TYPE_MISMATCH` | 114 | 87 | 27 |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 159 | 132 | 27 |
| `TYPE_ARGUMENTS_FOR_OUTER_CLASS_WHEN_NESTED_REFERENCED` | 41 | 15 | 26 |
| `SUBCLASS_OPT_IN_INAPPLICABLE` | 25 | 0 | 25 |
| `TOO_MANY_ARGUMENTS` | 118 | 93 | 25 |
| `CONCURRENT_HASH_MAP_CONTAINS_OPERATOR_ERROR` | 24 | 0 | 24 |
| `SMARTCAST_IMPOSSIBLE` | 183 | 159 | 24 |
| `UNINITIALIZED_ENUM_COMPANION` | 65 | 42 | 23 |
| `INAPPLICABLE_CANDIDATE` | 127 | 105 | 22 |
| `INLINE_FROM_HIGHER_PLATFORM` | 21 | 0 | 21 |
| `OPT_IN_USAGE_ERROR` | 96 | 76 | 20 |
| `INCORRECT_TYPE_PARAMETER_OF_PROPERTY` | 30 | 11 | 19 |
| `OPT_IN_MARKER_CAN_ONLY_BE_USED_AS_ANNOTATION_OR_ARGUMENT_IN_OPT_IN` | 19 | 2 | 17 |
| `REPEATED_ANNOTATION` | 69 | 52 | 17 |
| `DSL_SCOPE_VIOLATION` | 108 | 92 | 16 |
| `NOTHING_TO_OVERRIDE` | 93 | 77 | 16 |
| `WRONG_LONG_SUFFIX` | 19 | 3 | 16 |
| `ASSIGN_OPERATOR_AMBIGUITY` | 35 | 20 | 15 |
| `EXPRESSION_EXPECTED` | 19 | 4 | 15 |
| `NON_CONST_VAL_USED_IN_CONSTANT_EXPRESSION` | 96 | 81 | 15 |
| `NOT_YET_SUPPORTED_IN_INLINE` | 23 | 8 | 15 |
| `NO_ELSE_IN_WHEN` | 217 | 202 | 15 |
| `NO_EXPLICIT_VISIBILITY_IN_API_MODE` | 16 | 1 | 15 |
| `UNINITIALIZED_ENUM_ENTRY` | 15 | 0 | 15 |
| `CONTRACT_NOT_ALLOWED` | 19 | 5 | 14 |
| `INFIX_MODIFIER_REQUIRED` | 46 | 32 | 14 |
| `NON_INTERNAL_PUBLISHED_API` | 14 | 0 | 14 |
| *534 more* | | | |

## Errors kotlinc doesn't report

kc's errors on lines where the markup expects no diagnostic, in the tests it checked, with names replaced by _. In a test kotlinc compiles, one is a false error; in one it rejects, it can also be the same error on another line, or one kotlinc doesn't report after a first.

| Error | Count |
|:---|---:|
| `unresolved reference: _` | 210 |
| `type mismatch: inferred type is _ but _ was expected` | 186 |
| `only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type _` | 185 |
| `cannot infer type for type parameter '_'. Specify it explicitly.` | 172 |
| `class '_' is not abstract and does not implement abstract member '_'` | 56 |
| `suspend function '_' cannot override non-suspend function '_' defined in '_'` | 50 |
| `cannot infer a type for this parameter. Please specify it explicitly.` | 39 |
| `missing return statement in function '_'` | 37 |
| `unresolved or unsupported type: _` | 29 |
| `variable '_' must be initialized` | 17 |
| `'_' expression must be exhaustive, add necessary '_' branch` | 16 |
| `no function '_' takes n arguments` | 16 |
| `expected expression, found '_'` | 15 |
| `not enough information to infer type arguments of ::foon: specify the expected function type` | 14 |
| `operator '_' cannot be applied to Any?` | 14 |
| `ambiguous call: n candidates for '_' take (_)` | 13 |
| `private setters are not allowed for open properties` | 13 |
| `this lambda must return a value of type _` | 12 |
| `unresolved import: _` | 12 |
| `'_' modifier is required on '_'` | 11 |
| `not enough information to infer type arguments of ::barn: specify the expected function type` | 11 |
| `type checking has run into a recursive problem; specify the type of '_'` | 11 |
| `expected parameter of type _` | 10 |
| `not enough information to infer type arguments of ::foo: specify the expected function type` | 10 |
| `overload resolution ambiguity for ::foo: specify the expected function type` | 10 |
| `type checking has run into a recursive problem; specify the return type of '_'` | 10 |
| `cannot access '_': it is private to this in '_'` | 9 |
| `expected '_', found '_'` | 9 |
| `names _, __, ___, ... are reserved in Kotlin` | 9 |
| `not enough information to infer type arguments of ::bar: specify the expected function type` | 9 |
| *159 more* | |

## Unsupported features

By the features the parser (with error recovery) and the checker found. A test counts for each.

| Feature | Tests | Only blocker | Examples |
|:---|---:|---:|:---|
| `destructuring declaration` | 4 | 4 | `tests/destructuring/topLevelWithLambda.kt`, `tests/diagnostics/conflictingOverloads.kt` |
| `checker: '_' is not allowed here: local interfaces are not supported` | 2 | 2 | `tests/localInterfaces.kt`, `tests/thisAndSuper/unqualifiedSuper/unqualifiedSuperWithLocalClass.kt` |
| `property reference` | 2 | 2 | `tests/callableReference/unsupported/localVariableWithSubstitution.kt`, `tests/callableReference/unsupported/parameterWithSubstitution.kt` |
| `callable reference` | 1 | 1 | `tests/callableReference/compatibilityResolveWithVarargAndOperatorCall.kt` |
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
| 2 | `checker: '_' is not allowed here: local interfaces are not supported` | 2 | 6 |
| 3 | `property reference` | 2 | 8 |
| 4 | `callable reference` | 1 | 9 |
| 5 | `checker: lambdas with more than n parameters are not supported` | 1 | 10 |
| 6 | `package` | 1 | 11 |
| 7 | `parse error: expected '_', found '_'` | 1 | 12 |
| 8 | `parse error: expected expression, found '_'` | 1 | 13 |
| 9 | `unsupported types in signature` | 1 | 14 |

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

