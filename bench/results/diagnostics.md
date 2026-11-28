# Kotlin's diagnostics tests on kc

The diagnostics tests of JetBrains/kotlin v2.4.21 for the JVM (`compiler/testData/diagnostics/`: tests/, testsWithStdLib/, testsWithJvmBackend/, testsWithAnyBackend/), each compiled by kc. A test's markup marks the diagnostics kotlinc reports, and its `RUN_PIPELINE_TILL` says whether kotlinc stops with errors in the frontend (`FRONTEND`, `FIR2IR`) or goes on to the backend (`BACKEND`). `bench/kotlin-tests.sh diagnostics` writes this file, and [docs/conformance.md](../../docs/conformance.md) explains it.

Of 10827 test files, 2214 are skipped and 8613 run.

| Result | Tests | Share |
|:---|---:|---:|
| rejects invalid | 4448 | 51.6% |
| accepts valid | 4150 | 48.2% |
| accepts invalid | 15 | 0.2% |
| rejects valid | 0 | 0.0% |
| parse/unsupported | 0 | 0.0% |
| crash | 0 | 0.0% |

*Rejects invalid* and *accepts valid* agree with kotlinc. *Accepts invalid* is code kotlinc rejects that kc compiles; *rejects valid*, code kotlinc compiles that kc's checker rejects. Both are bugs. A *parse/unsupported* test uses a feature kc doesn't support, which says nothing about its checks.

Of the 4448 tests both reject, kc reports an error on the line of an expected one in 4424.

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
| `IDE_MODE error` | 1 | `tests/resolve/implicitPropertyTypeThatMakesBehaviorOrderDependantIdeMode.kt` |
| `TARGET_BACKEND: NATIVE, JS_IR` | 1 | `testsWithAnyBackend/crossFileInlineCycle.kt` |

## Accepts invalid

Tests kotlinc rejects that kc compiles, by the errors their markup expects. A test counts for each; *Only error* counts the tests it is the only expected error of, which implementing it alone would fix.

| Expected error | Tests | Only error | Examples |
|:---|---:|---:|:---|
| `(no error marked)` | 3 | 3 | `tests/j+k/missingTypeOfParameterInFakeOverride_1.kt`, `tests/nullableTypes/nullAssertOnTypeWithNullableUpperBound.kt` |
| `ARGUMENT_TYPE_MISMATCH` | 3 | 3 | `tests/j+k/collectionOverrides/containsAll.kt`, `tests/nullableTypes/nullableArgumentForIn.kt` |
| `UNRESOLVED_REFERENCE` | 3 | 2 | `tests/j+k/differentFilename.kt`, `tests/jvm/classpath/classFromJdkInLibrary.kt` |
| `OTHER_ERROR_WITH_REASON` | 2 | 2 | `testsWithStdLib/typealias/upperBoundViolated.kt`, `testsWithStdLib/typealias/upperBoundViolated2.kt` |
| `UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH` | 2 | 2 | `tests/inlineClasses/unsignedLiteralsWithoutArtifactOnClasspath.kt`, `tests/valueClasses/unsignedLiteralsWithoutArtifactOnClasspath.kt` |
| `DATA_CLASS_INVISIBLE_COPY_USAGE_ERROR` | 1 | 1 | `tests/jvm/dataClassNonPublicConstructor/dataClassInternalConstructorUsageWillBecomeInaccessible.kt` |
| `MEMBER_PROJECTED_OUT` | 1 | 1 | `tests/typealias/starProjection.kt` |
| `MISSING_DEPENDENCY_CLASS` | 1 | 0 | `tests/j+k/differentFilename.kt` |

## Rejects valid

Tests kotlinc compiles that kc's checker rejects, by kc's first error, with names replaced by _.

## Errors by diagnostic

For each expected error of the tests kc checked (not unsupported), whether kc reported an error on its line. Sorted by the errors kc missed.

kc reports 21910 of 22025 expected errors (99.5%) on their lines. Of the 599 diagnostics expected, kc reports every one of 562 on its line; the table lists the other 37.

| Diagnostic | Expected | Reported | Missed |
|:---|---:|---:|---:|
| `UNRESOLVED_REFERENCE` | 2553 | 2525 | 28 |
| `ARGUMENT_TYPE_MISMATCH` | 1491 | 1472 | 19 |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 251 | 244 | 7 |
| `UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH` | 10 | 3 | 7 |
| `CANNOT_INFER_PARAMETER_TYPE` | 635 | 630 | 5 |
| `RETURN_TYPE_MISMATCH` | 418 | 413 | 5 |
| `UNRESOLVED_IMPORT` | 57 | 54 | 3 |
| `ANNOTATION_ARGUMENT_MUST_BE_CONST` | 224 | 222 | 2 |
| `INITIALIZER_TYPE_MISMATCH` | 428 | 426 | 2 |
| `INVISIBLE_REFERENCE` | 525 | 523 | 2 |
| `MEMBER_PROJECTED_OUT` | 50 | 48 | 2 |
| `MISSING_DEPENDENCY_CLASS` | 75 | 73 | 2 |
| `OTHER_ERROR_WITH_REASON` | 2 | 0 | 2 |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 159 | 157 | 2 |
| `TYPE_ARGUMENTS_NOT_ALLOWED` | 11 | 9 | 2 |
| `TYPE_MISMATCH` | 33 | 31 | 2 |
| `WRONG_MODIFIER_CONTAINING_DECLARATION` | 56 | 54 | 2 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS` | 163 | 161 | 2 |
| `ARGUMENT_PASSED_TWICE` | 16 | 15 | 1 |
| `ASSIGNMENT_TYPE_MISMATCH` | 114 | 113 | 1 |
| `CANNOT_CHECK_FOR_ERASED` | 27 | 26 | 1 |
| `DATA_CLASS_INVISIBLE_COPY_USAGE_ERROR` | 1 | 0 | 1 |
| `INCORRECT_LEFT_COMPONENT_OF_INTERSECTION` | 14 | 13 | 1 |
| `NOTHING_TO_OVERRIDE` | 93 | 92 | 1 |
| `NOT_YET_SUPPORTED_IN_INLINE` | 25 | 24 | 1 |
| `NO_ELSE_IN_WHEN` | 217 | 216 | 1 |
| `NO_SET_METHOD` | 11 | 10 | 1 |
| `NO_VALUE_FOR_PARAMETER` | 175 | 174 | 1 |
| `NULL_FOR_NONNULL_TYPE` | 155 | 154 | 1 |
| `REPEATED_ANNOTATION` | 69 | 68 | 1 |
| `RETURN_NOT_ALLOWED` | 63 | 62 | 1 |
| `TYPEALIAS_EXPANSION_CAPTURES_OUTER_TYPE_PARAMETERS` | 6 | 5 | 1 |
| `UNINITIALIZED_VARIABLE` | 213 | 212 | 1 |
| `UNSAFE_CALL` | 525 | 524 | 1 |
| `UNSUPPORTED` | 233 | 232 | 1 |
| `VAL_REASSIGNMENT` | 231 | 230 | 1 |
| `WRONG_ANNOTATION_TARGET_WITH_USE_SITE_TARGET` | 97 | 96 | 1 |

### Missed errors by cause

Why kc reports no error on the line of an expected one, the first that applies: *accepted*, it compiled the test; *environment*, the test has no `WITH_STDLIB`, which gives kotlinc the test infrastructure's minimal standard library, and the error is an unsigned literal, a name, index operator or package that only the full one, which kc gets, declares (`println`, `listOf`, `MutableMap.set`, `kotlin.math`), or the test has no `FULL_JDK`, which gives kotlinc the mock JDK, and the error is an import of a package that only the real one has (`java.util.function`); *syntax-error*, kc's parser rejected the code, and kc checked it as recovered from kotlinc's grammar, and the error is in a statement or declaration that recovery left out; *module-not-compiled*, the error is in a module after one that failed, which isn't compiled; *nearby*, kc reported an error within 3 lines of it on a line the markup expects none on (most likely the error, at another position); *missing-check*, none of those.

| Cause | Missed | Share | Most missed |
|:---|---:|---:|:---|
| accepted | 26 | 22.6% | `ARGUMENT_TYPE_MISMATCH` 8, `UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH` 6, `UNRESOLVED_REFERENCE` 5, `MEMBER_PROJECTED_OUT` 2, `MISSING_DEPENDENCY_CLASS` 2 |
| environment | 19 | 16.5% | `UNRESOLVED_REFERENCE` 14, `UNRESOLVED_IMPORT` 3, `NO_SET_METHOD` 1, `UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH` 1 |
| syntax-error | 5 | 4.3% | `UNRESOLVED_REFERENCE` 2, `RETURN_NOT_ALLOWED` 1, `RETURN_TYPE_MISMATCH` 1, `UNINITIALIZED_VARIABLE` 1 |
| module-not-compiled | 0 | 0.0% |  |
| nearby | 19 | 16.5% | `UNRESOLVED_REFERENCE` 4, `ARGUMENT_TYPE_MISMATCH` 2, `INVISIBLE_REFERENCE` 2, `WRONG_NUMBER_OF_TYPE_ARGUMENTS` 2, `ARGUMENT_PASSED_TWICE` 1 |
| missing-check | 46 | 40.0% | `ARGUMENT_TYPE_MISMATCH` 9, `UNRESOLVED_REFERENCE_WRONG_RECEIVER` 7, `CANNOT_INFER_PARAMETER_TYPE` 5, `RETURN_TYPE_MISMATCH` 4, `UNRESOLVED_REFERENCE` 3 |

### Errors by name

kc reports 19007 of the 22025 expected errors (86.3%) on their lines under kotlinc's names, and 2903 (13.2% of those it reports on their lines) under other names: the error is found, but called something else, or kc reports another error first. [diagnostics-misnamed.tsv](diagnostics-misnamed.tsv) lists each expected error kc doesn't report under its name, with the names kc reports on its line.

The diagnostics kc reports most often under other names:

| Diagnostic | Expected | Reported | Named | Other name |
|:---|---:|---:|---:|---:|
| `ARGUMENT_TYPE_MISMATCH` | 1491 | 1472 | 1207 | 265 |
| `UNRESOLVED_REFERENCE` | 2553 | 2525 | 2294 | 231 |
| `INVISIBLE_REFERENCE` | 525 | 523 | 391 | 132 |
| `CANNOT_INFER_PARAMETER_TYPE` | 635 | 630 | 510 | 120 |
| `INAPPLICABLE_CANDIDATE` | 127 | 127 | 8 | 119 |
| `SMARTCAST_IMPOSSIBLE` | 183 | 183 | 68 | 115 |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 251 | 244 | 140 | 104 |
| `WRONG_MODIFIER_TARGET` | 375 | 375 | 281 | 94 |
| `NONE_APPLICABLE` | 150 | 150 | 62 | 88 |
| `NO_VALUE_FOR_PARAMETER` | 175 | 174 | 102 | 72 |
| `DELEGATE_SPECIAL_FUNCTION_NONE_APPLICABLE` | 45 | 45 | 0 | 45 |
| `ANNOTATION_ARGUMENT_MUST_BE_CONST` | 224 | 222 | 182 | 40 |
| `RETURN_TYPE_MISMATCH` | 418 | 413 | 373 | 40 |
| `NON_CONST_VAL_USED_IN_CONSTANT_EXPRESSION` | 96 | 96 | 58 | 38 |
| `EXPLICIT_TYPE_ARGUMENTS_IN_PROPERTY_ACCESS` | 37 | 37 | 0 | 37 |
| `INITIALIZER_TYPE_MISMATCH` | 428 | 426 | 389 | 37 |
| `INCOMPATIBLE_MODIFIERS` | 74 | 74 | 43 | 31 |
| `FUNCTION_CALL_EXPECTED` | 29 | 29 | 0 | 29 |
| `UNSUPPORTED` | 233 | 232 | 204 | 28 |
| `ASSIGNING_SINGLE_ELEMENT_TO_VARARG_IN_NAMED_FORM_FUNCTION_ERROR` | 27 | 27 | 0 | 27 |
| `NO_COMPANION_OBJECT` | 53 | 53 | 30 | 23 |
| `INACCESSIBLE_OUTER_CLASS_RECEIVER` | 40 | 40 | 18 | 22 |
| `TOO_MANY_ARGUMENTS` | 118 | 118 | 96 | 22 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS` | 163 | 161 | 140 | 21 |
| `INSTANCE_ACCESS_BEFORE_SUPER_CALL` | 116 | 116 | 96 | 20 |
| `TYPE_MISMATCH` | 33 | 31 | 11 | 20 |
| `SUPERTYPE_INITIALIZED_IN_INTERFACE` | 18 | 18 | 0 | 18 |
| `CANNOT_INFER_VALUE_PARAMETER_TYPE` | 91 | 91 | 74 | 17 |
| `FUNCTION_DECLARATION_WITH_NO_NAME` | 17 | 17 | 0 | 17 |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 159 | 157 | 140 | 17 |
| `ILLEGAL_SELECTOR` | 15 | 15 | 0 | 15 |
| `MEMBER_PROJECTED_OUT` | 50 | 48 | 33 | 15 |
| `TYPECHECKER_HAS_RUN_INTO_RECURSIVE_PROBLEM` | 87 | 87 | 72 | 15 |
| `ASSIGNMENT_TYPE_MISMATCH` | 114 | 113 | 99 | 14 |
| `KOTLIN_ACTUAL_ANNOTATION_HAS_NO_EFFECT_IN_KOTLIN` | 14 | 14 | 0 | 14 |
| `NAMED_PARAMETER_NOT_FOUND` | 29 | 29 | 15 | 14 |
| `NULL_FOR_NONNULL_TYPE` | 155 | 154 | 140 | 14 |
| `DELEGATE_SPECIAL_FUNCTION_MISSING` | 15 | 15 | 2 | 13 |
| `PROPERTY_INITIALIZER_NO_BACKING_FIELD` | 32 | 32 | 19 | 13 |
| `PRIVATE_TYPE_USED_IN_NON_PRIVATE_INLINE_FUNCTION_ERROR` | 12 | 12 | 0 | 12 |

What kc reports instead, by the expected error and kc's names of the errors on its line (`-` for an error kc doesn't name):

| Expected | kc | Count |
|:---|:---|---:|
| `UNRESOLVED_REFERENCE` | `SYNTAX` | 110 |
| `INVISIBLE_REFERENCE` | `UNRESOLVED_REFERENCE` | 97 |
| `SMARTCAST_IMPOSSIBLE` | `UNSAFE_CALL` | 69 |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | `UNRESOLVED_REFERENCE` | 55 |
| `CANNOT_INFER_PARAMETER_TYPE` | `UPPER_BOUND_VIOLATED` | 39 |
| `INAPPLICABLE_CANDIDATE` | `ARGUMENT_TYPE_MISMATCH` | 39 |
| `SMARTCAST_IMPOSSIBLE` | `UNRESOLVED_REFERENCE` | 39 |
| `INAPPLICABLE_CANDIDATE` | `UPPER_BOUND_VIOLATED` | 37 |
| `ARGUMENT_TYPE_MISMATCH` | `UPPER_BOUND_VIOLATED` | 35 |
| `ARGUMENT_TYPE_MISMATCH` | `TYPE_INFERENCE_ONLY_INPUT_TYPES_ERROR` | 31 |
| `NONE_APPLICABLE` | `UNRESOLVED_REFERENCE` | 31 |
| `EXPLICIT_TYPE_ARGUMENTS_IN_PROPERTY_ACCESS` | `SYNTAX` | 30 |
| `CANNOT_INFER_PARAMETER_TYPE` | `UNRESOLVED_REFERENCE` | 25 |
| `ARGUMENT_TYPE_MISMATCH` | `CANNOT_INFER_PARAMETER_TYPE` | 23 |
| `UNRESOLVED_REFERENCE` | `FUNCTION_EXPECTED` | 22 |
| `ARGUMENT_TYPE_MISMATCH` | `UNRESOLVED_REFERENCE` | 21 |
| `ARGUMENT_TYPE_MISMATCH` | `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 21 |
| `ARGUMENT_TYPE_MISMATCH` | `NONE_APPLICABLE` | 20 |
| `ASSIGNING_SINGLE_ELEMENT_TO_VARARG_IN_NAMED_FORM_FUNCTION_ERROR` | `ARGUMENT_TYPE_MISMATCH` | 19 |
| `FUNCTION_CALL_EXPECTED` | `UNRESOLVED_REFERENCE` | 19 |
| `UNSUPPORTED` | `UNRESOLVED_REFERENCE` | 19 |
| `WRONG_MODIFIER_TARGET` | `MUST_BE_INITIALIZED_OR_BE_ABSTRACT` | 19 |
| `WRONG_MODIFIER_TARGET` | `UNRESOLVED_REFERENCE` | 19 |
| `ARGUMENT_TYPE_MISMATCH` | `RETURN_TYPE_MISMATCH` | 18 |
| `NO_VALUE_FOR_PARAMETER` | `SYNTAX` | 18 |
| `FUNCTION_DECLARATION_WITH_NO_NAME` | `SYNTAX` | 17 |
| `INCOMPATIBLE_MODIFIERS` | `WRONG_MODIFIER_TARGET` | 16 |
| `NO_COMPANION_OBJECT` | `UNRESOLVED_REFERENCE` | 16 |
| `ILLEGAL_SELECTOR` | `SYNTAX` | 15 |
| `ANNOTATION_ARGUMENT_MUST_BE_CONST` | `ARGUMENT_TYPE_MISMATCH` | 14 |
| `ANNOTATION_ARGUMENT_MUST_BE_CONST` | `UNRESOLVED_REFERENCE` | 14 |
| `ARGUMENT_TYPE_MISMATCH` | `NULL_FOR_NONNULL_TYPE` | 14 |
| `MEMBER_PROJECTED_OUT` | `ARGUMENT_TYPE_MISMATCH` | 14 |
| `NON_CONST_VAL_USED_IN_CONSTANT_EXPRESSION` | `ARGUMENT_TYPE_MISMATCH` | 14 |
| `WRONG_MODIFIER_TARGET` | `MUST_BE_INITIALIZED` | 14 |
| `ARGUMENT_TYPE_MISMATCH` | `NO_VALUE_FOR_PARAMETER` | 13 |
| `INITIALIZER_TYPE_MISMATCH` | `ARGUMENT_TYPE_MISMATCH` | 12 |
| `PRIVATE_TYPE_USED_IN_NON_PRIVATE_INLINE_FUNCTION_ERROR` | `-` | 12 |
| `ANONYMOUS_FUNCTION_WITH_NAME` | `SYNTAX` | 11 |
| `INAPPLICABLE_TARGET_ON_PROPERTY` | `WRONG_ANNOTATION_TARGET_WITH_USE_SITE_TARGET` | 11 |
| *731 more* | | |

## Errors kotlinc doesn't report

kc's errors on lines where the markup expects no diagnostic, in the tests it checked, by their names, or by their messages with names replaced by _ for those kc doesn't name. Not those of an error the test's `DIAGNOSTICS` directive switches off, which kotlinc reports too, as far as kc's messages name it. In a test kotlinc compiles, one is a false error; in one it rejects, it can also be the same error on another line, or one kotlinc doesn't report after a first.

| Error | Count |
|:---|---:|
| `UNRESOLVED_REFERENCE` | 103 |
| `ARGUMENT_TYPE_MISMATCH` | 37 |
| `CANNOT_INFER_PARAMETER_TYPE` | 36 |
| `ABSTRACT_MEMBER_NOT_IMPLEMENTED` | 19 |
| `INITIALIZER_TYPE_MISMATCH` | 14 |
| `UNRESOLVED_IMPORT` | 11 |
| `UNSAFE_CALL` | 11 |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 9 |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 9 |
| `ASSIGNMENT_TYPE_MISMATCH` | 8 |
| `VAL_REASSIGNMENT` | 7 |
| `CONDITION_TYPE_MISMATCH` | 6 |
| `RETURN_TYPE_MISMATCH` | 6 |
| `TOO_MANY_ARGUMENTS` | 6 |
| `TYPE_PARAMETER_AS_REIFIED` | 6 |
| `CANNOT_INFER_VALUE_PARAMETER_TYPE` | 5 |
| `MEMBER_PROJECTED_OUT` | 5 |
| `SYNTAX` | 5 |
| `UPPER_BOUND_VIOLATED` | 5 |
| `BREAK_OR_CONTINUE_JUMPS_ACROSS_FUNCTION_BOUNDARY` | 4 |
| `CAPTURED_VAL_INITIALIZATION` | 4 |
| `EXTENDING_AN_ANNOTATION_CLASS_ERROR` | 4 |
| `NONE_APPLICABLE` | 4 |
| `NOTHING_TO_OVERRIDE` | 4 |
| `NO_CONTEXT_ARGUMENT` | 4 |
| `NO_ELSE_IN_WHEN` | 4 |
| `USAGE_IS_NOT_INLINABLE` | 4 |
| `INFIX_MODIFIER_REQUIRED` | 3 |
| `NAMED_ARGUMENTS_NOT_ALLOWED` | 3 |
| `NO_VALUE_FOR_PARAMETER` | 3 |
| *43 more* | |

## Unsupported features

By the features the parser (with error recovery) and the checker found. A test counts for each.

### Greedy order

At each step, the feature that unblocks the most tests given the ones before it.

## Warnings

Of the tests kc compiled, warnings don't decide the category: those are about errors. kc reports 3600 of the 3667 warnings the tests expect (98.2%) on their lines, and 14 warnings on lines that don't expect them, but for those a test's `DIAGNOSTICS` directive switches off.

Of the 127 warnings kc reports, 95 are on every line the tests expect them and nowhere else; the table lists the other 32.

| Warning kc reports | Expected | On their lines | Elsewhere |
|:---|---:|---:|---:|
| `DEPRECATION` | 203 | 202 | 3 |
| `UNREACHABLE_CODE` | 158 | 157 | 0 |
| `USELESS_IS_CHECK` | 127 | 125 | 0 |
| `UNNECESSARY_NOT_NULL_ASSERTION` | 95 | 94 | 1 |
| `CAST_NEVER_SUCCEEDS` | 92 | 91 | 0 |
| `SENSELESS_COMPARISON` | 91 | 88 | 1 |
| `USELESS_CAST` | 91 | 89 | 1 |
| `UNNECESSARY_SAFE_CALL` | 90 | 89 | 1 |
| `UNCHECKED_CAST` | 88 | 87 | 1 |
| `OPT_IN_USAGE` | 83 | 80 | 0 |
| `FINAL_UPPER_BOUND` | 69 | 68 | 0 |
| `EXTENSION_SHADOWED_BY_MEMBER` | 67 | 65 | 0 |
| `PLATFORM_CLASS_MAPPED_TO_KOTLIN` | 62 | 60 | 0 |
| `USELESS_ELVIS` | 44 | 43 | 0 |
| `NO_REFLECTION_IN_CLASS_PATH` | 36 | 36 | 1 |
| `UNUSED_VARIABLE` | 35 | 35 | 3 |
| `ASSIGNED_VALUE_IS_NEVER_READ` | 32 | 30 | 0 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING` | 31 | 30 | 0 |
| `REDUNDANT_ELSE_IN_WHEN` | 26 | 24 | 0 |
| `REDUNDANT_CALL_OF_CONVERSION_METHOD` | 23 | 22 | 2 |
| `REDUNDANT_VISIBILITY_MODIFIER` | 22 | 20 | 0 |
| `DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES` | 16 | 15 | 0 |
| `INFERRED_TYPE_VARIABLE_INTO_EMPTY_INTERSECTION_WARNING` | 15 | 8 | 0 |
| `ARRAY_EQUALITY_OPERATOR_CAN_BE_REPLACED_WITH_CONTENT_EQUALS` | 13 | 12 | 0 |
| `WRONG_INVOCATION_KIND` | 12 | 10 | 0 |
| `UPPER_BOUND_VIOLATED_DEPRECATION_WARNING` | 10 | 2 | 0 |
| `FORBIDDEN_IDENTITY_EQUALS_WARNING` | 8 | 7 | 0 |
| `REDUNDANT_EXPLICIT_BACKING_FIELD` | 7 | 6 | 0 |
| `REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_ANNOTATION` | 7 | 6 | 0 |
| `UNNECESSARY_LATEINIT` | 7 | 5 | 0 |
| `TYPE_ARGUMENTS_REDUNDANT_IN_SUPER_QUALIFIER` | 3 | 2 | 0 |
| `USELESS_JVM_EXPOSE_BOXED` | 3 | 2 | 0 |

The warnings the tests expect that kc doesn't report, as a backlog.

| Warning | Count |
|:---|---:|
| `MISSING_DEPENDENCY_SUPERCLASS_WARNING` | 3 |
| `REDUNDANT_RETURN` | 2 |
| `CANNOT_CHANGE_ACCESS_PRIVILEGE_WARNING` | 1 |
| `CANNOT_INFER_VISIBILITY_WARNING` | 1 |
| `EXPANSIVE_INHERITANCE_IN_JAVA` | 1 |
| `FINITE_BOUNDS_VIOLATION_IN_JAVA` | 1 |
| `K_SUSPEND_FUNCTION_TYPE_OF_DANGEROUSLY_LARGE_ARITY` | 1 |
| `MUTABLE_PROPERTY_WITH_CAPTURED_TYPE` | 1 |

