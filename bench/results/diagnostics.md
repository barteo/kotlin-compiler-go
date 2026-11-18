# Kotlin's diagnostics tests on kc

The diagnostics tests of JetBrains/kotlin v2.4.21 for the JVM (`compiler/testData/diagnostics/`: tests/, testsWithStdLib/, testsWithJvmBackend/, testsWithAnyBackend/), each compiled by kc. A test's markup marks the diagnostics kotlinc reports, and its `RUN_PIPELINE_TILL` says whether kotlinc stops with errors in the frontend (`FRONTEND`, `FIR2IR`) or goes on to the backend (`BACKEND`). `bench/kotlin-tests.sh diagnostics` writes this file, and [docs/conformance.md](../../docs/conformance.md) explains it.

Of 10827 test files, 2295 are skipped and 8532 run.

| Result | Tests | Share |
|:---|---:|---:|
| rejects invalid | 4366 | 51.2% |
| accepts valid | 4127 | 48.4% |
| accepts invalid | 38 | 0.4% |
| rejects valid | 0 | 0.0% |
| parse/unsupported | 0 | 0.0% |
| crash | 0 | 0.0% |

*Rejects invalid* and *accepts valid* agree with kotlinc. *Accepts invalid* is code kotlinc rejects that kc compiles; *rejects valid*, code kotlinc compiles that kc's checker rejects. Both are bugs. A *parse/unsupported* test uses a feature kc doesn't support, which says nothing about its checks.

Of the 4366 tests both reject, kc reports an error on the line of an expected one in 4284.

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
| `ARGUMENT_TYPE_MISMATCH` | 9 | 8 | `tests/collectionLiterals/kt82964.kt`, `tests/j+k/collectionOverrides/containsAll.kt` |
| `UNRESOLVED_REFERENCE` | 6 | 4 | `tests/callResolution/eagerLambdaAnalysis/samVsDefault.kt`, `tests/inference/kt68570.kt` |
| `(no error marked)` | 3 | 3 | `tests/j+k/missingTypeOfParameterInFakeOverride_1.kt`, `tests/nullableTypes/nullAssertOnTypeWithNullableUpperBound.kt` |
| `CANNOT_INFER_PARAMETER_TYPE` | 2 | 2 | `tests/callableReference/function/ambiguityTopLevelVsTopLevelGeneric.kt`, `tests/j+k/samByProjectedType/javaSamTypeWithRecursionInBounds.kt` |
| `INITIALIZER_TYPE_MISMATCH` | 2 | 1 | `tests/callableReference/adapted/noKFunctionForAdaptation.kt`, `testsWithStdLib/functionX.kt` |
| `UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH` | 2 | 2 | `tests/inlineClasses/unsignedLiteralsWithoutArtifactOnClasspath.kt`, `tests/valueClasses/unsignedLiteralsWithoutArtifactOnClasspath.kt` |
| `ABSTRACT_CLASS_MEMBER_NOT_IMPLEMENTED` | 1 | 1 | `tests/j+k/missingTypeOfParameterInFakeOverride_2.kt` |
| `ACCIDENTAL_OVERRIDE` | 1 | 1 | `tests/j+k/kjkimplicitTypesCrash.kt` |
| `ADAPTED_CALLABLE_REFERENCE_AGAINST_REFLECTION_TYPE` | 1 | 0 | `tests/callableReference/adapted/noKFunctionForAdaptation.kt` |
| `ASSIGNMENT_TYPE_MISMATCH` | 1 | 0 | `tests/controlFlowAnalysis/smartcastOnThisInInitializationSection.kt` |
| `CONDITION_TYPE_MISMATCH` | 1 | 0 | `tests/syntax/complicatedLTGT.kt` |
| `CONST_VAL_WITH_NON_CONST_INITIALIZER` | 1 | 1 | `tests/modifiers/const/kotlinJavaCycle.kt` |
| `DATA_CLASS_INVISIBLE_COPY_USAGE_ERROR` | 1 | 1 | `tests/jvm/dataClassNonPublicConstructor/dataClassInternalConstructorUsageWillBecomeInaccessible.kt` |
| `DELEGATE_SPECIAL_FUNCTION_MISSING` | 1 | 1 | `tests/delegatedProperty/inference/provideDelegateReturningOuterTypeVariable.kt` |
| `EXPLICIT_TYPE_ARGUMENTS_IN_PROPERTY_ACCESS` | 1 | 0 | `tests/syntax/complicatedLTGT.kt` |
| `ILLEGAL_SUSPEND_FUNCTION_CALL` | 1 | 1 | `tests/callResolution/eagerLambdaAnalysis/suspendFunctionTypeVsRunnable.kt` |
| `INLINE_FROM_HIGHER_PLATFORM` | 1 | 1 | `tests/jvm/inline/wrongInlineTarget.kt` |
| `MEMBER_PROJECTED_OUT` | 1 | 1 | `tests/typealias/starProjection.kt` |
| `MISSING_DEPENDENCY_CLASS` | 1 | 0 | `tests/j+k/differentFilename.kt` |
| `MUST_BE_INITIALIZED_OR_BE_ABSTRACT` | 1 | 0 | `tests/controlFlowAnalysis/smartcastOnThisInInitializationSection.kt` |
| `NOTHING_TO_OVERRIDE` | 1 | 1 | `tests/multimodule/duplicateSuper/sameSuperTraitDifferentBounds.kt` |
| `RETURN_TYPE_MISMATCH_ON_INHERITANCE` | 1 | 1 | `tests/override/clashesOnInheritance/kt13355viaJava.kt` |
| `SUSPENSION_POINT_INSIDE_MONITOR` | 1 | 1 | `tests/jvm/suspensionPointInMonitor.kt` |
| `SYNTAX` | 1 | 0 | `tests/syntax/complicatedLTGT.kt` |
| `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` | 1 | 1 | `tests/j+k/syntheticPropertyOverridden2.kt` |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 1 | 1 | `tests/platformTypes/commonSupertype/withNothing.kt` |
| `UPPER_BOUND_VIOLATED` | 1 | 0 | `tests/platformTypes/comparingWithWrongNullabilityWithoutWrtHack.kt` |
| `VAL_REASSIGNMENT` | 1 | 0 | `tests/controlFlowAnalysis/smartcastOnThisInInitializationSection.kt` |

## Rejects valid

Tests kotlinc compiles that kc's checker rejects, by kc's first error, with names replaced by _.

## Errors by diagnostic

For each expected error of the tests kc checked (not unsupported), whether kc reported an error on its line. Sorted by the errors kc missed.

kc reports 20205 of 21714 expected errors (93.1%) on their lines.

| Diagnostic | Expected | Reported | Missed |
|:---|---:|---:|---:|
| `UNRESOLVED_REFERENCE` | 2511 | 2321 | 190 |
| `ARGUMENT_TYPE_MISMATCH` | 1486 | 1356 | 130 |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 250 | 167 | 83 |
| `RETURN_TYPE_MISMATCH` | 418 | 362 | 56 |
| `CANNOT_INFER_PARAMETER_TYPE` | 633 | 582 | 51 |
| `WRONG_MODIFIER_TARGET` | 375 | 330 | 45 |
| `INVISIBLE_REFERENCE` | 523 | 484 | 39 |
| `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` | 87 | 53 | 34 |
| `ASSIGNMENT_TYPE_MISMATCH` | 114 | 89 | 25 |
| `CONST_VAL_WITH_NON_CONST_INITIALIZER` | 251 | 228 | 23 |
| `SMARTCAST_IMPOSSIBLE` | 183 | 160 | 23 |
| `REDECLARATION` | 196 | 174 | 22 |
| `INLINE_FROM_HIGHER_PLATFORM` | 21 | 0 | 21 |
| `OPT_IN_USAGE_ERROR` | 96 | 77 | 19 |
| `INITIALIZER_TYPE_MISMATCH` | 428 | 411 | 17 |
| `DSL_SCOPE_VIOLATION` | 108 | 92 | 16 |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 159 | 143 | 16 |
| `ASSIGN_OPERATOR_AMBIGUITY` | 35 | 20 | 15 |
| `NO_EXPLICIT_VISIBILITY_IN_API_MODE` | 16 | 1 | 15 |
| `ANNOTATION_ARGUMENT_MUST_BE_CONST` | 224 | 210 | 14 |
| `VAL_REASSIGNMENT` | 231 | 217 | 14 |
| `CAPTURED_VAL_INITIALIZATION` | 34 | 21 | 13 |
| `INAPPLICABLE_CANDIDATE` | 127 | 114 | 13 |
| `INFIX_MODIFIER_REQUIRED` | 46 | 34 | 12 |
| `UPPER_BOUND_VIOLATED` | 125 | 113 | 12 |
| `WRONG_ANNOTATION_TARGET_WITH_USE_SITE_TARGET` | 80 | 68 | 12 |
| `NAME_FOR_AMBIGUOUS_PARAMETER` | 31 | 20 | 11 |
| `NULL_FOR_NONNULL_TYPE` | 155 | 144 | 11 |
| `TOO_MANY_ARGUMENTS` | 118 | 108 | 10 |
| `CONFLICTING_OVERLOADS` | 734 | 725 | 9 |
| `INAPPLICABLE_FILE_TARGET` | 11 | 2 | 9 |
| `INCOMPATIBLE_MODIFIERS` | 74 | 65 | 9 |
| `TYPE_MISMATCH` | 33 | 24 | 9 |
| `NON_INLINE_MEMBER_VAL_INITIALIZATION` | 9 | 1 | 8 |
| `NOT_A_MULTIPLATFORM_COMPILATION` | 16 | 8 | 8 |
| `NO_ELSE_IN_WHEN` | 217 | 209 | 8 |
| `OPT_IN_MARKER_CAN_ONLY_BE_USED_AS_ANNOTATION_OR_ARGUMENT_IN_OPT_IN` | 19 | 11 | 8 |
| `SUBCLASS_CANT_CALL_COMPANION_PROTECTED_NON_STATIC` | 12 | 4 | 8 |
| `TYPE_CANT_BE_USED_FOR_CONST_VAL` | 19 | 11 | 8 |
| `WRONG_ANNOTATION_TARGET` | 382 | 374 | 8 |
| `CLASS_LITERAL_LHS_NOT_A_CLASS` | 16 | 9 | 7 |
| `INCONSISTENT_BACKING_FIELD_TYPE` | 18 | 11 | 7 |
| `NOT_YET_SUPPORTED_IN_INLINE` | 23 | 16 | 7 |
| `UNINITIALIZED_VARIABLE` | 213 | 206 | 7 |
| `UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH` | 10 | 3 | 7 |
| `EQUALITY_NOT_APPLICABLE` | 27 | 21 | 6 |
| `EXTENSION_IN_CLASS_REFERENCE_NOT_ALLOWED` | 12 | 6 | 6 |
| `FORBIDDEN_IDENTITY_EQUALS` | 45 | 39 | 6 |
| `INAPPLICABLE_TARGET_ON_PROPERTY` | 18 | 12 | 6 |
| `INVISIBLE_SETTER` | 45 | 39 | 6 |
| `NON_CONST_VAL_USED_IN_CONSTANT_EXPRESSION` | 96 | 90 | 6 |
| `NULLABLE_TYPE_IN_CLASS_LITERAL_LHS` | 9 | 3 | 6 |
| `SYNTAX` | 886 | 880 | 6 |
| `UPPER_BOUND_IS_EXTENSION_OR_CONTEXT_FUNCTION_TYPE` | 8 | 2 | 6 |
| `WRONG_EXTENSION_FUNCTION_TYPE` | 6 | 0 | 6 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS` | 163 | 157 | 6 |
| `CONFLICTING_JVM_DECLARATIONS` | 996 | 991 | 5 |
| `CYCLIC_CONSTRUCTOR_DELEGATION_CALL` | 28 | 23 | 5 |
| `FUN_INTERFACE_CANNOT_HAVE_ABSTRACT_PROPERTIES` | 6 | 1 | 5 |
| `ILLEGAL_JVM_NAME` | 14 | 9 | 5 |
| *536 more* | | | |

### Missed errors by cause

Why kc reports no error on the line of an expected one: *accepted*, it compiled the test; *syntax-error*, its parser rejected the code, and kc checked it as recovered from kotlinc's grammar, without the statements and declarations it couldn't lower; *module-not-compiled*, the error is in a module after one that failed, which isn't compiled; *nearby*, kc reported an error within 3 lines of it on a line the markup expects none on (most likely the error, at another position); *missing-check*, none of those.

| Cause | Missed | Share | Most missed |
|:---|---:|---:|:---|
| accepted | 97 | 6.4% | `INLINE_FROM_HIGHER_PLATFORM` 21, `ARGUMENT_TYPE_MISMATCH` 16, `UNRESOLVED_REFERENCE` 10, `SYNTAX` 6, `UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH` 6 |
| syntax-error | 205 | 13.6% | `WRONG_MODIFIER_TARGET` 42, `UNRESOLVED_REFERENCE` 22, `INCOMPATIBLE_MODIFIERS` 9, `RETURN_TYPE_MISMATCH` 8, `NULLABLE_TYPE_IN_CLASS_LITERAL_LHS` 6 |
| module-not-compiled | 0 | 0.0% |  |
| nearby | 177 | 11.7% | `RETURN_TYPE_MISMATCH` 29, `UNRESOLVED_REFERENCE` 27, `ARGUMENT_TYPE_MISMATCH` 26, `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` 10, `INITIALIZER_TYPE_MISMATCH` 8 |
| missing-check | 1030 | 68.3% | `UNRESOLVED_REFERENCE` 131, `ARGUMENT_TYPE_MISMATCH` 87, `UNRESOLVED_REFERENCE_WRONG_RECEIVER` 78, `CANNOT_INFER_PARAMETER_TYPE` 37, `INVISIBLE_REFERENCE` 36 |

## Errors kotlinc doesn't report

kc's errors on lines where the markup expects no diagnostic, in the tests it checked, with names replaced by _. In a test kotlinc compiles, one is a false error; in one it rejects, it can also be the same error on another line, or one kotlinc doesn't report after a first.

| Error | Count |
|:---|---:|
| `unresolved reference: _` | 209 |
| `only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type _` | 185 |
| `cannot infer type for type parameter '_'. Specify it explicitly.` | 181 |
| `type mismatch: inferred type is _ but _ was expected` | 178 |
| `class '_' is not abstract and does not implement abstract member '_'` | 57 |
| `suspend function '_' cannot override non-suspend function '_' defined in '_'` | 50 |
| `unresolved or unsupported type: _` | 29 |
| `no function '_' takes n arguments` | 22 |
| `cannot use '_' as reified type parameter` | 15 |
| `variable '_' must be initialized` | 15 |
| `not enough information to infer type arguments of ::foon: specify the expected function type` | 14 |
| `operator '_' cannot be applied to Any?` | 14 |
| `'_' expression must be exhaustive, add necessary '_' branch` | 12 |
| `this lambda must return a value of type _` | 12 |
| `type checking has run into a recursive problem; specify the return type of '_'` | 12 |
| `not enough information to infer type arguments of ::barn: specify the expected function type` | 11 |
| `type checking has run into a recursive problem; specify the type of '_'` | 11 |
| `unresolved import: _` | 11 |
| `ambiguous call: n candidates for '_' take (_)` | 10 |
| `expected parameter of type _` | 10 |
| `not enough information to infer type arguments of ::foo: specify the expected function type` | 10 |
| `operator '_' cannot be applied to A and Boolean` | 10 |
| `overload resolution ambiguity for ::foo: specify the expected function type` | 10 |
| `'_' overrides nothing` | 9 |
| `cannot access '_': it is private to this in '_'` | 9 |
| `not enough information to infer type arguments of ::bar: specify the expected function type` | 9 |
| `'_' modifier is required on '_'` | 8 |
| `expected n parameter of type _ but n were declared` | 8 |
| `'_' sub-interface exposes its '_' supertype '_'` | 7 |
| `cannot infer a type for this parameter. Please specify it explicitly.` | 7 |
| *176 more* | |

## Unsupported features

By the features the parser (with error recovery) and the checker found. A test counts for each.

### Greedy order

At each step, the feature that unblocks the most tests given the ones before it.

## Warnings

Of the tests kc compiled, warnings don't decide the category: those are about errors. kc reports 799 of the 3652 warnings the tests expect (21.9%) on their lines, and 8 warnings on lines that don't expect them, but for those a test's `DIAGNOSTICS` directive switches off.

| Warning kc reports | Expected | On their lines | Elsewhere |
|:---|---:|---:|---:|
| `DEPRECATION` | 199 | 115 | 3 |
| `ERROR_SUPPRESSION` | 102 | 7 | 0 |
| `NOTHING_TO_INLINE` | 95 | 94 | 0 |
| `UNNECESSARY_NOT_NULL_ASSERTION` | 93 | 85 | 0 |
| `USELESS_CAST` | 91 | 28 | 2 |
| `SENSELESS_COMPARISON` | 90 | 63 | 1 |
| `UNNECESSARY_SAFE_CALL` | 90 | 78 | 1 |
| `UNCHECKED_CAST` | 87 | 39 | 1 |
| `OPT_IN_USAGE` | 83 | 62 | 0 |
| `FINAL_UPPER_BOUND` | 69 | 56 | 0 |
| `PLATFORM_CLASS_MAPPED_TO_KOTLIN` | 62 | 44 | 0 |
| `REDUNDANT_PROJECTION` | 49 | 47 | 0 |
| `USELESS_ELVIS` | 44 | 27 | 0 |
| `NO_TAIL_CALLS_FOUND` | 29 | 24 | 0 |
| `OVERRIDE_BY_INLINE` | 26 | 26 | 0 |
| `TYPEALIAS_EXPANSION_DEPRECATION` | 9 | 4 | 0 |

The warnings the tests expect that kc doesn't report, as a backlog.

| Warning | Count |
|:---|---:|
| `REDUNDANT_INTERPOLATION_PREFIX` | 355 |
| `UNREACHABLE_CODE` | 158 |
| `IMPLICIT_BOXING_IN_IDENTITY_EQUALS` | 152 |
| `UNUSED_EXPRESSION` | 136 |
| `USELESS_IS_CHECK` | 127 |
| `CAST_NEVER_SUCCEEDS` | 92 |
| `CONTEXTUAL_OVERLOAD_SHADOWED` | 75 |
| `ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY` | 70 |
| `EXTENSION_SHADOWED_BY_MEMBER` | 67 |
| `NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL` | 59 |
| `IMPOSSIBLE_IS_CHECK_WARNING` | 57 |
| `NO_REFLECTION_IN_CLASS_PATH` | 36 |
| `UNUSED_VARIABLE` | 35 |
| `ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY` | 33 |
| `ASSIGNED_VALUE_IS_NEVER_READ` | 32 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING` | 31 |
| `OVERRIDE_DEPRECATION` | 30 |
| `DEPRECATED_IDENTITY_EQUALS` | 26 |
| `INCOMPATIBLE_ENUM_COMPARISON` | 26 |
| `INLINE_CLASS_DEPRECATED` | 26 |
| `NOT_NULL_ASSERTION_ON_CALLABLE_REFERENCE` | 26 |
| `REDUNDANT_ELSE_IN_WHEN` | 26 |
| `DIVISION_BY_ZERO` | 25 |
| `LEAKED_IN_PLACE_LAMBDA` | 25 |
| `REDUNDANT_CALL_OF_CONVERSION_METHOD` | 23 |
| `MISPLACED_TYPE_PARAMETER_CONSTRAINTS` | 22 |
| `NON_TAIL_RECURSIVE_CALL` | 22 |
| `PROPERTY_HIDES_JAVA_FIELD` | 22 |
| `REDUNDANT_VISIBILITY_MODIFIER` | 22 |
| `INFERRED_INVISIBLE_RETURN_TYPE_WARNING` | 20 |
| *89 more* | |

