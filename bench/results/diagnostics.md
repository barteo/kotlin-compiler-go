# Kotlin's diagnostics tests on kc

The diagnostics tests of JetBrains/kotlin v2.4.21 for the JVM (`compiler/testData/diagnostics/`: tests/, testsWithStdLib/, testsWithJvmBackend/, testsWithAnyBackend/), each compiled by kc. A test's markup marks the diagnostics kotlinc reports, and its `RUN_PIPELINE_TILL` says whether kotlinc stops with errors in the frontend (`FRONTEND`, `FIR2IR`) or goes on to the backend (`BACKEND`). `bench/kotlin-tests.sh diagnostics` writes this file, and [docs/conformance.md](../../docs/conformance.md) explains it.

Of 10827 test files, 2213 are skipped and 8614 run.

| Result | Tests | Share |
|:---|---:|---:|
| rejects invalid | 4435 | 51.5% |
| accepts valid | 4150 | 48.2% |
| accepts invalid | 28 | 0.3% |
| rejects valid | 0 | 0.0% |
| parse/unsupported | 0 | 0.0% |
| crash | 0 | 0.0% |

*Rejects invalid* and *accepts valid* agree with kotlinc. *Accepts invalid* is code kotlinc rejects that kc compiles; *rejects valid*, code kotlinc compiles that kc's checker rejects. Both are bugs. A *parse/unsupported* test uses a feature kc doesn't support, which says nothing about its checks.

Of the 4435 tests both reject, kc reports an error on the line of an expected one in 4370.

A test is invalid if it stops before the backend or marks an error: one that runs to it (`BACKEND`) can mark the JVM backend's errors, such as `CONFLICTING_JVM_DECLARATIONS`. Of the tests run, 222 stop at `FRONTEND` but mark no error: most add the `CHECK_TYPE` helpers, whose names (`_`) kotlinc reports outside the test infrastructure, and count as valid; the others leave their errors out with the `DIAGNOSTICS` directive. 115 run to the backend and mark one. 5 marked names have no declared severity (debug information, for one), and count as neither.

The names without a severity: `DEBUG_INFO_EXPRESSION_TYPE` (746), `DEBUG_INFO_CSR_MIGHT_BE_USED` (44), `DEBUG_INFO_CALL` (28), `DEBUG_INFO_CSR_MIGHT_BE_USED_INSTEAD_OF_IMPORT` (16), `DEBUG_INFO_CALLABLE_OWNER` (6).

## Skipped

Tests for other platforms, that need language features off by default or compiler options, or modules other than plain dependencies, or whose FIR diagnostics aren't checked.

| Reason | Tests | Examples |
|:---|---:|:---|
| `LANGUAGE` | 1517 | `tests/BacktickNames.kt`, `tests/BacktickNamesOld.kt` |
| `modules with dependsOn` | 358 | `tests/multiplatform/abstractEnumInIr.kt`, `tests/multiplatform/actualAnnotationsNotMatchExpect/actualInnerClassMissingMember.kt` |
| `LANGUAGE_FEATURE_TOGGLED` | 93 | `tests/annotatedPackageQualifiers.kt`, `tests/annotationUsedAsAnnotationArgument.kt` |
| `return value checker (crv/, crvFull/)` | 83 | `tests/crv/assignments.kt`, `tests/crv/basicJInterop.kt` |
| `API_VERSION` | 53 | `tests/deprecated/deprecatedSinceKotlin/error.kt`, `tests/deprecated/deprecatedSinceKotlin/hidden.kt` |
| `ALLOW_KOTLIN_PACKAGE` | 47 | `tests/annotations/requireKotlin.kt`, `tests/crvDisabled/oldMustUse.kt` |
| `JDK_KIND` | 27 | `tests/errorProneAtomicArrayPrimitivesAllJavaMethods.kt`, `tests/identityComparisonOnJavaValueBasedClassesWarning.kt` |
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
| `UNRESOLVED_REFERENCE` | 4 | 2 | `tests/inference/kt68570.kt`, `tests/j+k/differentFilename.kt` |
| `(no error marked)` | 3 | 3 | `tests/j+k/missingTypeOfParameterInFakeOverride_1.kt`, `tests/nullableTypes/nullAssertOnTypeWithNullableUpperBound.kt` |
| `UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH` | 2 | 2 | `tests/inlineClasses/unsignedLiteralsWithoutArtifactOnClasspath.kt`, `tests/valueClasses/unsignedLiteralsWithoutArtifactOnClasspath.kt` |
| `ABSTRACT_CLASS_MEMBER_NOT_IMPLEMENTED` | 1 | 1 | `tests/j+k/missingTypeOfParameterInFakeOverride_2.kt` |
| `ACCIDENTAL_OVERRIDE` | 1 | 1 | `tests/j+k/kjkimplicitTypesCrash.kt` |
| `ASSIGNMENT_TYPE_MISMATCH` | 1 | 0 | `tests/controlFlowAnalysis/smartcastOnThisInInitializationSection.kt` |
| `CANNOT_INFER_PARAMETER_TYPE` | 1 | 1 | `tests/callableReference/function/ambiguityTopLevelVsTopLevelGeneric.kt` |
| `CONDITION_TYPE_MISMATCH` | 1 | 0 | `tests/syntax/complicatedLTGT.kt` |
| `CONST_VAL_WITH_NON_CONST_INITIALIZER` | 1 | 1 | `tests/modifiers/const/kotlinJavaCycle.kt` |
| `DATA_CLASS_INVISIBLE_COPY_USAGE_ERROR` | 1 | 1 | `tests/jvm/dataClassNonPublicConstructor/dataClassInternalConstructorUsageWillBecomeInaccessible.kt` |
| `EXPLICIT_TYPE_ARGUMENTS_IN_PROPERTY_ACCESS` | 1 | 0 | `tests/syntax/complicatedLTGT.kt` |
| `MEMBER_PROJECTED_OUT` | 1 | 1 | `tests/typealias/starProjection.kt` |
| `MISSING_DEPENDENCY_CLASS` | 1 | 0 | `tests/j+k/differentFilename.kt` |
| `MUST_BE_INITIALIZED_OR_BE_ABSTRACT` | 1 | 0 | `tests/controlFlowAnalysis/smartcastOnThisInInitializationSection.kt` |
| `NOTHING_TO_OVERRIDE` | 1 | 1 | `tests/multimodule/duplicateSuper/sameSuperTraitDifferentBounds.kt` |
| `SYNTAX` | 1 | 0 | `tests/syntax/complicatedLTGT.kt` |
| `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` | 1 | 1 | `tests/j+k/syntheticPropertyOverridden2.kt` |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 1 | 1 | `tests/platformTypes/commonSupertype/withNothing.kt` |
| `UPPER_BOUND_VIOLATED` | 1 | 0 | `tests/platformTypes/comparingWithWrongNullabilityWithoutWrtHack.kt` |
| `VAL_REASSIGNMENT` | 1 | 0 | `tests/controlFlowAnalysis/smartcastOnThisInInitializationSection.kt` |

## Rejects valid

Tests kotlinc compiles that kc's checker rejects, by kc's first error, with names replaced by _.

## Errors by diagnostic

For each expected error of the tests kc checked (not unsupported), whether kc reported an error on its line. Sorted by the errors kc missed.

kc reports 21107 of 22025 expected errors (95.8%) on their lines.

| Diagnostic | Expected | Reported | Missed |
|:---|---:|---:|---:|
| `UNRESOLVED_REFERENCE` | 2552 | 2426 | 126 |
| `ARGUMENT_TYPE_MISMATCH` | 1491 | 1370 | 121 |
| `CANNOT_INFER_PARAMETER_TYPE` | 635 | 588 | 47 |
| `RETURN_TYPE_MISMATCH` | 418 | 372 | 46 |
| `INVISIBLE_REFERENCE` | 525 | 502 | 23 |
| `REDECLARATION` | 196 | 176 | 20 |
| `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` | 87 | 68 | 19 |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 159 | 143 | 16 |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 251 | 236 | 15 |
| `VAL_REASSIGNMENT` | 231 | 217 | 14 |
| `SMARTCAST_IMPOSSIBLE` | 183 | 170 | 13 |
| `INAPPLICABLE_CANDIDATE` | 127 | 115 | 12 |
| `INITIALIZER_TYPE_MISMATCH` | 428 | 416 | 12 |
| `NULL_FOR_NONNULL_TYPE` | 155 | 144 | 11 |
| `WRONG_ANNOTATION_TARGET` | 403 | 392 | 11 |
| `ASSIGNMENT_TYPE_MISMATCH` | 114 | 104 | 10 |
| `TOO_MANY_ARGUMENTS` | 118 | 108 | 10 |
| `CONFLICTING_OVERLOADS` | 734 | 725 | 9 |
| `TYPE_MISMATCH` | 33 | 24 | 9 |
| `CONST_VAL_WITH_NON_CONST_INITIALIZER` | 251 | 243 | 8 |
| `NO_ELSE_IN_WHEN` | 217 | 209 | 8 |
| `UPPER_BOUND_VIOLATED` | 125 | 117 | 8 |
| `NOT_YET_SUPPORTED_IN_INLINE` | 25 | 18 | 7 |
| `UNINITIALIZED_VARIABLE` | 213 | 206 | 7 |
| `UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH` | 10 | 3 | 7 |
| `ANNOTATION_ARGUMENT_MUST_BE_CONST` | 224 | 218 | 6 |
| `INVISIBLE_SETTER` | 45 | 39 | 6 |
| `NON_CONST_VAL_USED_IN_CONSTANT_EXPRESSION` | 96 | 90 | 6 |
| `SYNTAX` | 886 | 880 | 6 |
| `UPPER_BOUND_IS_EXTENSION_OR_CONTEXT_FUNCTION_TYPE` | 8 | 2 | 6 |
| `WRONG_EXTENSION_FUNCTION_TYPE` | 6 | 0 | 6 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS` | 163 | 157 | 6 |
| `CONFLICTING_JVM_DECLARATIONS` | 996 | 991 | 5 |
| `CYCLIC_CONSTRUCTOR_DELEGATION_CALL` | 28 | 23 | 5 |
| `OVERRIDING_FINAL_MEMBER_BY_DELEGATION` | 8 | 3 | 5 |
| `TYPE_INFERENCE_ONLY_INPUT_TYPES_ERROR` | 61 | 56 | 5 |
| `TYPE_PARAMETERS_NOT_ALLOWED` | 13 | 8 | 5 |
| `TYPE_PARAMETER_AS_REIFIED_ARRAY_ERROR` | 5 | 0 | 5 |
| `UNDERSCORE_USAGE_WITHOUT_BACKTICKS` | 28 | 23 | 5 |
| `UNEXPECTED_TRAILING_LAMBDA_ON_A_NEW_LINE` | 14 | 9 | 5 |
| `UNRESOLVED_IMPORT` | 57 | 52 | 5 |
| `UNSAFE_CALL` | 525 | 520 | 5 |
| `BUILDER_INFERENCE_STUB_RECEIVER` | 5 | 1 | 4 |
| `CANNOT_CHECK_FOR_ERASED` | 27 | 23 | 4 |
| `CANNOT_INFER_VALUE_PARAMETER_TYPE` | 91 | 87 | 4 |
| `CAPTURED_VAL_INITIALIZATION` | 34 | 30 | 4 |
| `CONFLICTING_PROJECTION` | 29 | 25 | 4 |
| `IMPOSSIBLE_IS_CHECK_ERROR` | 84 | 80 | 4 |
| `INAPPLICABLE_LATEINIT_MODIFIER` | 61 | 57 | 4 |
| `JVM_STATIC_ON_CONST_OR_JVM_FIELD` | 6 | 2 | 4 |
| `MULTIPLE_LABELS_ARE_FORBIDDEN` | 17 | 13 | 4 |
| `NO_TYPE_ARGUMENTS_ON_RHS` | 26 | 22 | 4 |
| `OPT_IN_USAGE_ERROR` | 102 | 98 | 4 |
| `TYPE_PARAMETER_AS_REIFIED` | 19 | 15 | 4 |
| `TYPE_PARAMETER_ON_LHS_OF_DOT` | 5 | 1 | 4 |
| `VARARG_OUTSIDE_PARENTHESES` | 10 | 6 | 4 |
| `ANNOTATION_ARGUMENT_KCLASS_LITERAL_OF_TYPE_PARAMETER_ERROR` | 6 | 3 | 3 |
| `COMMA_IN_WHEN_CONDITION_WITH_WHEN_GUARD` | 4 | 1 | 3 |
| `CONFLICTING_INHERITED_MEMBERS` | 15 | 12 | 3 |
| `CONSTRUCTOR_IN_INTERFACE` | 13 | 10 | 3 |
| *540 more* | | | |

### Missed errors by cause

Why kc reports no error on the line of an expected one: *accepted*, it compiled the test; *syntax-error*, its parser rejected the code, and kc checked it as recovered from kotlinc's grammar, without the statements and declarations it couldn't lower; *module-not-compiled*, the error is in a module after one that failed, which isn't compiled; *nearby*, kc reported an error within 3 lines of it on a line the markup expects none on (most likely the error, at another position); *missing-check*, none of those.

| Cause | Missed | Share | Most missed |
|:---|---:|---:|:---|
| accepted | 54 | 5.9% | `ARGUMENT_TYPE_MISMATCH` 16, `UNRESOLVED_REFERENCE` 7, `SYNTAX` 6, `UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH` 6, `CONST_VAL_WITH_NON_CONST_INITIALIZER` 2 |
| syntax-error | 141 | 15.4% | `UNRESOLVED_REFERENCE` 22, `RETURN_TYPE_MISMATCH` 6, `TOO_MANY_ARGUMENTS` 6, `UPPER_BOUND_IS_EXTENSION_OR_CONTEXT_FUNCTION_TYPE` 6, `CANNOT_INFER_PARAMETER_TYPE` 5 |
| module-not-compiled | 0 | 0.0% |  |
| nearby | 131 | 14.3% | `ARGUMENT_TYPE_MISMATCH` 25, `UNRESOLVED_REFERENCE` 24, `RETURN_TYPE_MISMATCH` 21, `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` 8, `INITIALIZER_TYPE_MISMATCH` 6 |
| missing-check | 592 | 64.5% | `ARGUMENT_TYPE_MISMATCH` 79, `UNRESOLVED_REFERENCE` 73, `CANNOT_INFER_PARAMETER_TYPE` 37, `INVISIBLE_REFERENCE` 20, `RETURN_TYPE_MISMATCH` 19 |

## Errors kotlinc doesn't report

kc's errors on lines where the markup expects no diagnostic, in the tests it checked, with names replaced by _. In a test kotlinc compiles, one is a false error; in one it rejects, it can also be the same error on another line, or one kotlinc doesn't report after a first.

| Error | Count |
|:---|---:|
| `unresolved reference: _` | 202 |
| `only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type _` | 185 |
| `type mismatch: inferred type is _ but _ was expected` | 174 |
| `cannot infer type for type parameter '_'. Specify it explicitly.` | 171 |
| `class '_' is not abstract and does not implement abstract member '_'` | 58 |
| `suspend function '_' cannot override non-suspend function '_' defined in '_'` | 50 |
| `unresolved or unsupported type: _` | 29 |
| `no function '_' takes n arguments` | 22 |
| `cannot use '_' as reified type parameter` | 15 |
| `variable '_' must be initialized` | 15 |
| `not enough information to infer type arguments of ::foon: specify the expected function type` | 14 |
| `operator '_' cannot be applied to Any?` | 14 |
| `'_' expression must be exhaustive, add necessary '_' branch` | 12 |
| `expected parameter of type _` | 12 |
| `this lambda must return a value of type _` | 12 |
| `expected n parameter of type _ but n were declared` | 11 |
| `not enough information to infer type arguments of ::barn: specify the expected function type` | 11 |
| `type checking has run into a recursive problem; specify the type of '_'` | 11 |
| `unresolved import: _` | 11 |
| `ambiguous call: n candidates for '_' take (_)` | 10 |
| `expected no parameters, but n were declared` | 10 |
| `not enough information to infer type arguments of ::foo: specify the expected function type` | 10 |
| `operator '_' cannot be applied to A and Boolean` | 10 |
| `overload resolution ambiguity for ::foo: specify the expected function type` | 10 |
| `'_' overrides nothing` | 9 |
| `cannot access '_': it is private to this in '_'` | 9 |
| `not enough information to infer type arguments of ::bar: specify the expected function type` | 9 |
| `'_' modifier is required on '_'` | 8 |
| `'_' sub-interface exposes its '_' supertype '_'` | 7 |
| `cannot infer a type for this parameter. Please specify it explicitly.` | 7 |
| *171 more* | |

## Unsupported features

By the features the parser (with error recovery) and the checker found. A test counts for each.

### Greedy order

At each step, the feature that unblocks the most tests given the ones before it.

## Warnings

Of the tests kc compiled, warnings don't decide the category: those are about errors. kc reports 2697 of the 3667 warnings the tests expect (73.5%) on their lines, and 18 warnings on lines that don't expect them, but for those a test's `DIAGNOSTICS` directive switches off.

| Warning kc reports | Expected | On their lines | Elsewhere |
|:---|---:|---:|---:|
| `REDUNDANT_INTERPOLATION_PREFIX` | 355 | 287 | 0 |
| `DEPRECATION` | 203 | 197 | 7 |
| `UNREACHABLE_CODE` | 158 | 131 | 0 |
| `IMPLICIT_BOXING_IN_IDENTITY_EQUALS` | 152 | 152 | 0 |
| `UNUSED_EXPRESSION` | 136 | 122 | 0 |
| `USELESS_IS_CHECK` | 127 | 114 | 0 |
| `ERROR_SUPPRESSION` | 102 | 98 | 0 |
| `NOTHING_TO_INLINE` | 100 | 99 | 0 |
| `UNNECESSARY_NOT_NULL_ASSERTION` | 95 | 87 | 0 |
| `CAST_NEVER_SUCCEEDS` | 92 | 87 | 0 |
| `SENSELESS_COMPARISON` | 91 | 72 | 1 |
| `USELESS_CAST` | 91 | 68 | 2 |
| `UNNECESSARY_SAFE_CALL` | 90 | 79 | 1 |
| `UNCHECKED_CAST` | 88 | 45 | 1 |
| `OPT_IN_USAGE` | 83 | 63 | 0 |
| `ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY` | 70 | 70 | 0 |
| `FINAL_UPPER_BOUND` | 69 | 56 | 0 |
| `EXTENSION_SHADOWED_BY_MEMBER` | 67 | 65 | 0 |
| `PLATFORM_CLASS_MAPPED_TO_KOTLIN` | 62 | 44 | 0 |
| `NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL` | 59 | 59 | 0 |
| `IMPOSSIBLE_IS_CHECK_WARNING` | 57 | 45 | 0 |
| `REDUNDANT_PROJECTION` | 49 | 47 | 0 |
| `USELESS_ELVIS` | 44 | 40 | 0 |
| `NO_REFLECTION_IN_CLASS_PATH` | 36 | 36 | 1 |
| `UNUSED_VARIABLE` | 35 | 35 | 3 |
| `ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY` | 33 | 33 | 0 |
| `ASSIGNED_VALUE_IS_NEVER_READ` | 32 | 8 | 0 |
| `OVERRIDE_DEPRECATION` | 30 | 30 | 0 |
| `NO_TAIL_CALLS_FOUND` | 29 | 27 | 0 |
| `OVERRIDE_BY_INLINE` | 28 | 28 | 0 |
| `DEPRECATED_IDENTITY_EQUALS` | 26 | 26 | 0 |
| `INCOMPATIBLE_ENUM_COMPARISON` | 26 | 1 | 0 |
| `INLINE_CLASS_DEPRECATED` | 26 | 26 | 0 |
| `NOT_NULL_ASSERTION_ON_CALLABLE_REFERENCE` | 26 | 26 | 0 |
| `REDUNDANT_ELSE_IN_WHEN` | 26 | 19 | 0 |
| `DIVISION_BY_ZERO` | 25 | 22 | 0 |
| `REDUNDANT_CALL_OF_CONVERSION_METHOD` | 23 | 22 | 2 |
| `MISPLACED_TYPE_PARAMETER_CONSTRAINTS` | 22 | 22 | 0 |
| `NON_TAIL_RECURSIVE_CALL` | 22 | 22 | 0 |
| `PROPERTY_HIDES_JAVA_FIELD` | 22 | 22 | 0 |
| `EMPTY_RANGE` | 19 | 19 | 0 |
| `DELEGATED_MEMBER_HIDES_SUPERTYPE_OVERRIDE` | 18 | 18 | 0 |
| `DEPRECATED_JAVA_ANNOTATION` | 16 | 16 | 0 |
| `DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES` | 16 | 12 | 0 |
| `PARAMETER_NAME_CHANGED_ON_OVERRIDE` | 15 | 9 | 0 |
| `USELESS_ELVIS_LEFT_IS_NULL` | 15 | 13 | 0 |
| `ARRAY_EQUALITY_OPERATOR_CAN_BE_REPLACED_WITH_CONTENT_EQUALS` | 13 | 12 | 0 |
| `VARIABLE_INITIALIZER_IS_REDUNDANT` | 13 | 5 | 0 |
| `TAIL_RECURSION_IN_TRY_IS_NOT_SUPPORTED` | 12 | 12 | 0 |
| `NOT_NULL_ASSERTION_ON_LAMBDA_EXPRESSION` | 11 | 11 | 0 |
| `UNUSED_LAMBDA_EXPRESSION` | 10 | 9 | 0 |
| `TYPEALIAS_EXPANSION_DEPRECATION` | 9 | 4 | 0 |
| `USELESS_ELVIS_RIGHT_IS_NULL` | 9 | 9 | 0 |
| `SENSELESS_NULL_IN_WHEN` | 8 | 6 | 0 |
| `EXTENSION_FUNCTION_SHADOWED_BY_MEMBER_PROPERTY_WITH_INVOKE` | 4 | 4 | 0 |
| `VARIABLE_NEVER_READ` | 3 | 3 | 0 |
| `SAFE_CAST_RELYING_ON_NULL` | 2 | 2 | 0 |
| `UNSAFE_CAST_RELYING_ON_NULL` | 1 | 1 | 0 |

The warnings the tests expect that kc doesn't report, as a backlog.

| Warning | Count |
|:---|---:|
| `CONTEXTUAL_OVERLOAD_SHADOWED` | 75 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING` | 31 |
| `LEAKED_IN_PLACE_LAMBDA` | 25 |
| `REDUNDANT_VISIBILITY_MODIFIER` | 22 |
| `INFERRED_INVISIBLE_RETURN_TYPE_WARNING` | 20 |
| `CANNOT_WEAKEN_ACCESS_PRIVILEGE_WARNING` | 15 |
| `EQUALITY_NOT_APPLICABLE_WARNING` | 15 |
| `INFERRED_TYPE_VARIABLE_INTO_EMPTY_INTERSECTION_WARNING` | 15 |
| `RETURN_IN_FUNCTION_WITH_EXPRESSION_BODY_WARNING` | 13 |
| `IMPLICIT_PROPERTY_TYPE_MAKES_BEHAVIOR_ORDER_DEPENDANT` | 12 |
| `INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL` | 12 |
| `WRONG_INVOCATION_KIND` | 12 |
| `REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_FUNCTION` | 11 |
| `INFERRED_INVISIBLE_REIFIED_TYPE_ARGUMENT_WARNING` | 10 |
| `INFERRED_INVISIBLE_WHEN_TYPE_WARNING` | 10 |
| `INFERRED_TYPE_VARIABLE_INTO_POSSIBLE_EMPTY_INTERSECTION` | 10 |
| `UPPER_BOUND_VIOLATED_DEPRECATION_WARNING` | 10 |
| `USELESS_VARARG_ON_PARAMETER` | 10 |
| `DUPLICATE_BRANCH_CONDITION_IN_WHEN` | 9 |
| `ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING` | 9 |
| `REDUNDANT_NULLABLE` | 9 |
| `REDUNDANT_OPEN_IN_INTERFACE` | 9 |
| `DANGEROUS_CHARACTERS` | 8 |
| `FORBIDDEN_IDENTITY_EQUALS_WARNING` | 8 |
| `REDUNDANT_MODIFIER` | 8 |
| `USELESS_CALL_ON_NOT_NULL` | 8 |
| `JAVA_CLASS_ON_COMPANION` | 7 |
| `NON_FINAL_MEMBER_IN_FINAL_CLASS` | 7 |
| `NON_FINAL_MEMBER_IN_OBJECT` | 7 |
| `REDUNDANT_EXPLICIT_BACKING_FIELD` | 7 |
| *47 more* | |

