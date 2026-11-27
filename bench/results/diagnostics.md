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

kc reports 21873 of 22025 expected errors (99.3%) on their lines.

| Diagnostic | Expected | Reported | Missed |
|:---|---:|---:|---:|
| `UNRESOLVED_REFERENCE` | 2553 | 2516 | 37 |
| `ARGUMENT_TYPE_MISMATCH` | 1491 | 1470 | 21 |
| `CANNOT_INFER_PARAMETER_TYPE` | 635 | 624 | 11 |
| `UNRESOLVED_REFERENCE_WRONG_RECEIVER` | 251 | 244 | 7 |
| `UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH` | 10 | 3 | 7 |
| `RETURN_TYPE_MISMATCH` | 418 | 412 | 6 |
| `TYPE_INFERENCE_ONLY_INPUT_TYPES_ERROR` | 61 | 58 | 3 |
| `UNRESOLVED_IMPORT` | 57 | 54 | 3 |
| `WRONG_MODIFIER_CONTAINING_DECLARATION` | 56 | 53 | 3 |
| `ANNOTATION_ARGUMENT_MUST_BE_CONST` | 224 | 222 | 2 |
| `CANNOT_CHECK_FOR_ERASED` | 27 | 25 | 2 |
| `IMPOSSIBLE_IS_CHECK_ERROR` | 84 | 82 | 2 |
| `INITIALIZER_TYPE_MISMATCH` | 428 | 426 | 2 |
| `INVISIBLE_REFERENCE` | 525 | 523 | 2 |
| `MEMBER_PROJECTED_OUT` | 50 | 48 | 2 |
| `MISSING_DEPENDENCY_CLASS` | 75 | 73 | 2 |
| `OTHER_ERROR_WITH_REASON` | 2 | 0 | 2 |
| `OVERLOAD_RESOLUTION_AMBIGUITY` | 159 | 157 | 2 |
| `REPEATED_ANNOTATION` | 69 | 67 | 2 |
| `RETURN_NOT_ALLOWED` | 63 | 61 | 2 |
| `TYPEOF_SUSPEND_TYPE` | 4 | 2 | 2 |
| `TYPE_ARGUMENTS_NOT_ALLOWED` | 11 | 9 | 2 |
| `TYPE_MISMATCH` | 33 | 31 | 2 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS` | 163 | 161 | 2 |
| `ACCIDENTAL_OVERRIDE_CLASH_BY_JVM_SIGNATURE` | 1 | 0 | 1 |
| `ARGUMENT_PASSED_TWICE` | 16 | 15 | 1 |
| `ASSIGNMENT_TYPE_MISMATCH` | 114 | 113 | 1 |
| `CYCLIC_INHERITANCE_HIERARCHY` | 45 | 44 | 1 |
| `DATA_CLASS_INVISIBLE_COPY_USAGE_ERROR` | 1 | 0 | 1 |
| `FUNCTION_CALL_EXPECTED` | 29 | 28 | 1 |
| `INAPPLICABLE_TARGET_PROPERTY_HAS_NO_DELEGATE` | 7 | 6 | 1 |
| `INCORRECT_LEFT_COMPONENT_OF_INTERSECTION` | 14 | 13 | 1 |
| `ITERATOR_MISSING` | 10 | 9 | 1 |
| `MISSING_CONSTRUCTOR_KEYWORD` | 4 | 3 | 1 |
| `NOTHING_TO_OVERRIDE` | 93 | 92 | 1 |
| `NOT_YET_SUPPORTED_IN_INLINE` | 25 | 24 | 1 |
| `NO_ELSE_IN_WHEN` | 217 | 216 | 1 |
| `NO_SET_METHOD` | 11 | 10 | 1 |
| `NO_VALUE_FOR_PARAMETER` | 175 | 174 | 1 |
| `NULL_FOR_NONNULL_TYPE` | 155 | 154 | 1 |
| `PRIVATE_CLASS_MEMBER_FROM_INLINE` | 9 | 8 | 1 |
| `TYPEALIAS_EXPANSION_CAPTURES_OUTER_TYPE_PARAMETERS` | 6 | 5 | 1 |
| `UNINITIALIZED_VARIABLE` | 213 | 212 | 1 |
| `UNSAFE_CALL` | 525 | 524 | 1 |
| `UNSUPPORTED` | 233 | 232 | 1 |
| `VAL_REASSIGNMENT` | 231 | 230 | 1 |
| `WRONG_ANNOTATION_TARGET` | 403 | 402 | 1 |
| `WRONG_ANNOTATION_TARGET_WITH_USE_SITE_TARGET` | 97 | 96 | 1 |
| `ABBREVIATED_NOTHING_PROPERTY_TYPE` | 2 | 2 | 0 |
| `ABBREVIATED_NOTHING_RETURN_TYPE` | 1 | 1 | 0 |
| `ABSENCE_OF_PRIMARY_CONSTRUCTOR_FOR_VALUE_CLASS` | 2 | 2 | 0 |
| `ABSTRACT_CLASS_MEMBER_NOT_IMPLEMENTED` | 23 | 23 | 0 |
| `ABSTRACT_DELEGATED_PROPERTY` | 2 | 2 | 0 |
| `ABSTRACT_FUNCTION_IN_NON_ABSTRACT_CLASS` | 3 | 3 | 0 |
| `ABSTRACT_FUNCTION_WITH_BODY` | 4 | 4 | 0 |
| `ABSTRACT_MEMBER_NOT_IMPLEMENTED` | 69 | 69 | 0 |
| `ABSTRACT_MEMBER_NOT_IMPLEMENTED_BY_ENUM_ENTRY` | 6 | 6 | 0 |
| `ABSTRACT_PROPERTY_IN_NON_ABSTRACT_CLASS` | 8 | 8 | 0 |
| `ABSTRACT_PROPERTY_IN_PRIMARY_CONSTRUCTOR_PARAMETERS` | 1 | 1 | 0 |
| `ABSTRACT_PROPERTY_WITHOUT_TYPE` | 2 | 2 | 0 |
| *539 more* | | | |

### Missed errors by cause

Why kc reports no error on the line of an expected one: *accepted*, it compiled the test; *syntax-error*, its parser rejected the code, and kc checked it as recovered from kotlinc's grammar, without the statements and declarations it couldn't lower; *module-not-compiled*, the error is in a module after one that failed, which isn't compiled; *environment*, the test has no `WITH_STDLIB`, which gives kotlinc the test infrastructure's minimal standard library, and the error is an unsigned literal or a name that only the full one, which kc gets, declares at the top level (`println`, `listOf`); *nearby*, kc reported an error within 3 lines of it on a line the markup expects none on (most likely the error, at another position); *missing-check*, none of those.

| Cause | Missed | Share | Most missed |
|:---|---:|---:|:---|
| accepted | 26 | 17.1% | `ARGUMENT_TYPE_MISMATCH` 8, `UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH` 6, `UNRESOLVED_REFERENCE` 5, `MEMBER_PROJECTED_OUT` 2, `MISSING_DEPENDENCY_CLASS` 2 |
| syntax-error | 41 | 27.0% | `UNRESOLVED_REFERENCE` 8, `CANNOT_INFER_PARAMETER_TYPE` 5, `RETURN_TYPE_MISMATCH` 4, `UNRESOLVED_IMPORT` 3, `WRONG_MODIFIER_CONTAINING_DECLARATION` 3 |
| module-not-compiled | 0 | 0.0% |  |
| environment | 13 | 8.6% | `UNRESOLVED_REFERENCE` 12, `UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH` 1 |
| nearby | 18 | 11.8% | `UNRESOLVED_REFERENCE` 4, `ARGUMENT_TYPE_MISMATCH` 2, `INVISIBLE_REFERENCE` 2, `WRONG_NUMBER_OF_TYPE_ARGUMENTS` 2, `ARGUMENT_PASSED_TWICE` 1 |
| missing-check | 54 | 35.5% | `ARGUMENT_TYPE_MISMATCH` 11, `UNRESOLVED_REFERENCE` 8, `CANNOT_INFER_PARAMETER_TYPE` 6, `UNRESOLVED_REFERENCE_WRONG_RECEIVER` 5, `TYPE_INFERENCE_ONLY_INPUT_TYPES_ERROR` 3 |

## Errors kotlinc doesn't report

kc's errors on lines where the markup expects no diagnostic, in the tests it checked, with names replaced by _. Not those of an error the test's `DIAGNOSTICS` directive switches off, which kotlinc reports too, as far as kc's messages name it. In a test kotlinc compiles, one is a false error; in one it rejects, it can also be the same error on another line, or one kotlinc doesn't report after a first.

| Error | Count |
|:---|---:|
| `unresolved reference: _` | 90 |
| `type mismatch: inferred type is _ but _ was expected` | 84 |
| `cannot infer type for type parameter '_'. Specify it explicitly.` | 46 |
| `class '_' is not abstract and does not implement abstract member '_'` | 20 |
| `only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type _` | 18 |
| `unresolved or unsupported type: _` | 14 |
| `no function '_' takes n arguments` | 11 |
| `unresolved import: _` | 11 |
| `'_' overrides nothing` | 8 |
| `missing return statement in function '_'` | 7 |
| `cannot use '_' as reified type parameter` | 6 |
| `captured values cannot be initialized because of possible reassignments` | 6 |
| `val cannot be reassigned` | 6 |
| `'_' modifier is required on '_'` | 5 |
| `ambiguous call: n candidates for '_' take (_)` | 5 |
| `cannot infer a type for this parameter. Please specify it explicitly.` | 5 |
| `'_' expression must be exhaustive, add necessary '_' branch` | 4 |
| `'_' is not allowed outside a loop` | 4 |
| `illegal usage of inline-parameter '_' in '_': add '_' modifier to the parameter declaration` | 4 |
| `inheriting an annotation class is prohibited` | 4 |
| `no context argument for '_' found` | 4 |
| `named arguments are not allowed for non-Kotlin functions` | 3 |
| `names _, __, ___, ..., can be used only in back-ticks ('_', '__', '___', ...)` | 3 |
| `reference has a nullable type '_', use explicit '_' to make a function-like call instead` | 3 |
| `there'_'B.Bn'` | 3 |
| `'_' annotation is applicable only to value classes` | 2 |
| `conflicting overloads: _` | 2 |
| `expected a name` | 2 |
| `expected parameter of type _` | 2 |
| `incompatible types Int and Nothing` | 2 |
| *57 more* | |

## Unsupported features

By the features the parser (with error recovery) and the checker found. A test counts for each.

### Greedy order

At each step, the feature that unblocks the most tests given the ones before it.

## Warnings

Of the tests kc compiled, warnings don't decide the category: those are about errors. kc reports 3580 of the 3667 warnings the tests expect (97.6%) on their lines, and 13 warnings on lines that don't expect them, but for those a test's `DIAGNOSTICS` directive switches off.

| Warning kc reports | Expected | On their lines | Elsewhere |
|:---|---:|---:|---:|
| `REDUNDANT_INTERPOLATION_PREFIX` | 355 | 355 | 0 |
| `DEPRECATION` | 203 | 202 | 3 |
| `UNREACHABLE_CODE` | 158 | 157 | 0 |
| `IMPLICIT_BOXING_IN_IDENTITY_EQUALS` | 152 | 152 | 0 |
| `UNUSED_EXPRESSION` | 136 | 136 | 0 |
| `USELESS_IS_CHECK` | 127 | 120 | 0 |
| `ERROR_SUPPRESSION` | 102 | 102 | 0 |
| `NOTHING_TO_INLINE` | 100 | 100 | 0 |
| `UNNECESSARY_NOT_NULL_ASSERTION` | 95 | 94 | 0 |
| `CAST_NEVER_SUCCEEDS` | 92 | 91 | 0 |
| `SENSELESS_COMPARISON` | 91 | 88 | 1 |
| `USELESS_CAST` | 91 | 89 | 1 |
| `UNNECESSARY_SAFE_CALL` | 90 | 89 | 1 |
| `UNCHECKED_CAST` | 88 | 84 | 1 |
| `OPT_IN_USAGE` | 83 | 78 | 0 |
| `CONTEXTUAL_OVERLOAD_SHADOWED` | 75 | 71 | 0 |
| `ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY` | 70 | 70 | 0 |
| `FINAL_UPPER_BOUND` | 69 | 67 | 0 |
| `EXTENSION_SHADOWED_BY_MEMBER` | 67 | 65 | 0 |
| `PLATFORM_CLASS_MAPPED_TO_KOTLIN` | 62 | 60 | 0 |
| `NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL` | 59 | 59 | 0 |
| `IMPOSSIBLE_IS_CHECK_WARNING` | 57 | 57 | 0 |
| `REDUNDANT_PROJECTION` | 49 | 49 | 0 |
| `USELESS_ELVIS` | 44 | 43 | 0 |
| `NO_REFLECTION_IN_CLASS_PATH` | 36 | 36 | 1 |
| `UNUSED_VARIABLE` | 35 | 35 | 3 |
| `ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY` | 33 | 33 | 0 |
| `ASSIGNED_VALUE_IS_NEVER_READ` | 32 | 30 | 0 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING` | 31 | 30 | 0 |
| `OVERRIDE_DEPRECATION` | 30 | 30 | 0 |
| `NO_TAIL_CALLS_FOUND` | 29 | 27 | 0 |
| `OVERRIDE_BY_INLINE` | 28 | 28 | 0 |
| `DEPRECATED_IDENTITY_EQUALS` | 26 | 26 | 0 |
| `INCOMPATIBLE_ENUM_COMPARISON` | 26 | 26 | 0 |
| `INLINE_CLASS_DEPRECATED` | 26 | 26 | 0 |
| `NOT_NULL_ASSERTION_ON_CALLABLE_REFERENCE` | 26 | 26 | 0 |
| `REDUNDANT_ELSE_IN_WHEN` | 26 | 24 | 0 |
| `DIVISION_BY_ZERO` | 25 | 25 | 0 |
| `LEAKED_IN_PLACE_LAMBDA` | 25 | 25 | 0 |
| `REDUNDANT_CALL_OF_CONVERSION_METHOD` | 23 | 22 | 2 |
| `MISPLACED_TYPE_PARAMETER_CONSTRAINTS` | 22 | 22 | 0 |
| `NON_TAIL_RECURSIVE_CALL` | 22 | 22 | 0 |
| `PROPERTY_HIDES_JAVA_FIELD` | 22 | 22 | 0 |
| `REDUNDANT_VISIBILITY_MODIFIER` | 22 | 20 | 0 |
| `INFERRED_INVISIBLE_RETURN_TYPE_WARNING` | 20 | 20 | 0 |
| `EMPTY_RANGE` | 19 | 19 | 0 |
| `DELEGATED_MEMBER_HIDES_SUPERTYPE_OVERRIDE` | 18 | 18 | 0 |
| `DEPRECATED_JAVA_ANNOTATION` | 16 | 16 | 0 |
| `DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES` | 16 | 15 | 0 |
| `CANNOT_WEAKEN_ACCESS_PRIVILEGE_WARNING` | 15 | 15 | 0 |
| `EQUALITY_NOT_APPLICABLE_WARNING` | 15 | 15 | 0 |
| `INFERRED_TYPE_VARIABLE_INTO_EMPTY_INTERSECTION_WARNING` | 15 | 8 | 0 |
| `PARAMETER_NAME_CHANGED_ON_OVERRIDE` | 15 | 15 | 0 |
| `USELESS_ELVIS_LEFT_IS_NULL` | 15 | 13 | 0 |
| `ARRAY_EQUALITY_OPERATOR_CAN_BE_REPLACED_WITH_CONTENT_EQUALS` | 13 | 12 | 0 |
| `RETURN_IN_FUNCTION_WITH_EXPRESSION_BODY_WARNING` | 13 | 13 | 0 |
| `VARIABLE_INITIALIZER_IS_REDUNDANT` | 13 | 13 | 0 |
| `IMPLICIT_PROPERTY_TYPE_MAKES_BEHAVIOR_ORDER_DEPENDANT` | 12 | 12 | 0 |
| `INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL` | 12 | 12 | 0 |
| `TAIL_RECURSION_IN_TRY_IS_NOT_SUPPORTED` | 12 | 12 | 0 |
| `WRONG_INVOCATION_KIND` | 12 | 10 | 0 |
| `NOT_NULL_ASSERTION_ON_LAMBDA_EXPRESSION` | 11 | 11 | 0 |
| `REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_FUNCTION` | 11 | 11 | 0 |
| `INFERRED_INVISIBLE_REIFIED_TYPE_ARGUMENT_WARNING` | 10 | 10 | 0 |
| `INFERRED_INVISIBLE_WHEN_TYPE_WARNING` | 10 | 10 | 0 |
| `INFERRED_TYPE_VARIABLE_INTO_POSSIBLE_EMPTY_INTERSECTION` | 10 | 10 | 0 |
| `UNUSED_LAMBDA_EXPRESSION` | 10 | 10 | 0 |
| `UPPER_BOUND_VIOLATED_DEPRECATION_WARNING` | 10 | 2 | 0 |
| `USELESS_VARARG_ON_PARAMETER` | 10 | 10 | 0 |
| `DUPLICATE_BRANCH_CONDITION_IN_WHEN` | 9 | 9 | 0 |
| `ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING` | 9 | 9 | 0 |
| `REDUNDANT_NULLABLE` | 9 | 9 | 0 |
| `REDUNDANT_OPEN_IN_INTERFACE` | 9 | 9 | 0 |
| `TYPEALIAS_EXPANSION_DEPRECATION` | 9 | 9 | 0 |
| `USELESS_ELVIS_RIGHT_IS_NULL` | 9 | 9 | 0 |
| `DANGEROUS_CHARACTERS` | 8 | 8 | 0 |
| `FORBIDDEN_IDENTITY_EQUALS_WARNING` | 8 | 7 | 0 |
| `REDUNDANT_MODIFIER` | 8 | 8 | 0 |
| `SENSELESS_NULL_IN_WHEN` | 8 | 8 | 0 |
| `USELESS_CALL_ON_NOT_NULL` | 8 | 8 | 0 |
| `JAVA_CLASS_ON_COMPANION` | 7 | 7 | 0 |
| `NON_FINAL_MEMBER_IN_FINAL_CLASS` | 7 | 7 | 0 |
| `NON_FINAL_MEMBER_IN_OBJECT` | 7 | 7 | 0 |
| `REDUNDANT_EXPLICIT_BACKING_FIELD` | 7 | 6 | 0 |
| `REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_ANNOTATION` | 7 | 6 | 0 |
| `UNNECESSARY_LATEINIT` | 7 | 5 | 0 |
| `ANNOTATION_TARGETS_ONLY_IN_JAVA` | 6 | 6 | 0 |
| `DSL_MARKER_APPLIED_TO_WRONG_TARGET` | 6 | 6 | 0 |
| `INFERRED_INVISIBLE_VARARG_TYPE_ARGUMENT_WARNING` | 6 | 6 | 0 |
| `UNUSED_ANONYMOUS_PARAMETER` | 6 | 6 | 0 |
| `MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES_DEPRECATION_WARNING` | 5 | 5 | 0 |
| `OPT_IN_OVERRIDE` | 5 | 5 | 0 |
| `POTENTIALLY_NON_REPORTED_ANNOTATION` | 5 | 5 | 0 |
| `REDUNDANT_REPEATABLE_ANNOTATION` | 5 | 5 | 0 |
| `SYNCHRONIZED_ON_INLINE` | 5 | 5 | 0 |
| `DSL_MARKER_PROPAGATES_TO_MANY` | 4 | 4 | 0 |
| `EXTENSION_FUNCTION_SHADOWED_BY_MEMBER_PROPERTY_WITH_INVOKE` | 4 | 4 | 0 |
| `TRIM_MARGIN_BLANK_PREFIX` | 4 | 4 | 0 |
| `UPPER_BOUND_VIOLATED_IN_TYPEALIAS_EXPANSION_DEPRECATION_WARNING` | 4 | 3 | 0 |
| `WRONG_NUMBER_OF_TYPE_ARGUMENTS_IN_LOCAL_CLASS_IN_LHS_WARNING` | 4 | 4 | 0 |
| `CAN_BE_VAL` | 3 | 3 | 0 |
| `DEPRECATED_MODIFIER_FOR_TARGET` | 3 | 3 | 0 |
| `OPERATOR_RENAMED_ON_IMPORT` | 3 | 3 | 0 |
| `OPT_IN_ARGUMENT_IS_NOT_MARKER` | 3 | 3 | 0 |
| `OPT_IN_TO_INHERITANCE` | 3 | 3 | 0 |
| `REDUNDANT_ANNOTATION_TARGET` | 3 | 3 | 0 |
| `REDUNDANT_MODIFIER_FOR_TARGET` | 3 | 3 | 0 |
| `REDUNDANT_RETURN_UNIT_TYPE` | 3 | 3 | 0 |
| `TYPE_ARGUMENTS_REDUNDANT_IN_SUPER_QUALIFIER` | 3 | 2 | 0 |
| `UPPER_BOUND_VIOLATED_IN_LHS_OF_CLASS_LITERAL_WARNING` | 3 | 3 | 0 |
| `USELESS_JVM_EXPOSE_BOXED` | 3 | 2 | 0 |
| `VARIABLE_NEVER_READ` | 3 | 3 | 0 |
| `ANNOTATIONS_ON_BLOCK_LEVEL_EXPRESSION_ON_THE_SAME_LINE` | 2 | 2 | 0 |
| `INCOMPATIBLE_ANNOTATION_TARGETS` | 2 | 2 | 0 |
| `INCOMPATIBLE_TYPES_WARNING` | 2 | 2 | 0 |
| `OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS` | 2 | 2 | 0 |
| `REDUNDANT_SETTER_PARAMETER_TYPE` | 2 | 2 | 0 |
| `REDUNDANT_SINGLE_EXPRESSION_STRING_TEMPLATE` | 2 | 2 | 0 |
| `SAFE_CAST_RELYING_ON_NULL` | 2 | 2 | 0 |
| `CAN_BE_VAL_DELAYED_INITIALIZATION` | 1 | 1 | 0 |
| `CONFLICT_VERSION_AND_JVM_OVERLOADS_ANNOTATION` | 1 | 1 | 0 |
| `LABEL_NAME_CLASH` | 1 | 1 | 0 |
| `NEWER_VERSION_IN_SINCE_KOTLIN` | 1 | 1 | 0 |
| `OPT_IN_WITHOUT_ARGUMENTS` | 1 | 1 | 0 |
| `OVERLOADS_PRIVATE` | 1 | 1 | 0 |
| `TYPE_ARGUMENTS_NOT_ALLOWED_IN_PACKAGE_QUALIFIER_WARNING` | 1 | 1 | 0 |
| `UNSAFE_CAST_RELYING_ON_NULL` | 1 | 1 | 0 |

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

