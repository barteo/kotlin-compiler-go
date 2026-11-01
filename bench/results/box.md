# Kotlin's box tests on kc

The box tests of JetBrains/kotlin v2.4.21 (`compiler/testData/codegen/box`), each compiled by kc and run: its `box()` must return `"OK"`. `bench/kotlin-tests.sh box` writes this file, and [docs/conformance.md](../../docs/conformance.md) explains it.

Of 7480 test files, 997 are skipped and 6483 run. 6285 pass (96.9%).

| Result | Tests | Share |
|:---|---:|---:|
| pass | 6285 | 96.9% |
| parse/unsupported | 0 | 0.0% |
| check error | 127 | 2.0% |
| crash | 0 | 0.0% |
| codegen error | 7 | 0.1% |
| VerifyError | 21 | 0.3% |
| linkage error | 8 | 0.1% |
| exception | 14 | 0.2% |
| wrong result | 21 | 0.3% |
| timeout | 0 | 0.0% |

## Skipped

Tests for other backends, that kotlinc's JVM backend ignores, or that need what the harness doesn't do: language features off by default, compiler options, Java sources, or modules other than plain dependencies.

| Reason | Tests | Examples |
|:---|---:|:---|
| `LANGUAGE` | 628 | `annotations/checkReturnValue.kt`, `annotations/instances/multiplatformInstantiation.kt` |
| `TARGET_BACKEND: WASM` | 122 | `casts/kt73931.kt`, `casts/wasmCastMessages.kt` |
| `IGNORE_BACKEND: JVM_IR` | 48 | `binaryOp/call.kt`, `binaryOp/callNullable.kt` |
| `TARGET_BACKEND: JS_IR, JS_IR_ES6` | 36 | `builtins/stringTemplateWithValueOf.kt`, `casts/functions/isFunKSmallJS.kt` |
| `TARGET_BACKEND: JS_IR` | 34 | `bridges/jsName.kt`, `bridges/jsNative.kt` |
| `TARGET_BACKEND: NATIVE` | 27 | `annotations/nativeThrows.kt`, `argumentOrder/varargAndDefaultParameters_ForNative.kt` |
| `DONT_TARGET_EXACT_BACKEND: JVM_IR` | 25 | `associatedObjects/associatedObjectNested.kt`, `associatedObjects/findAssociatedObject.kt` |
| `modules with friends or dependsOn` | 21 | `bridges/internalMethodFakeOverrideInFriendModule.kt`, `bridges/internalMethodOverrideInFriendModule.kt` |
| `FREE_COMPILER_ARGS` | 20 | `basics/kt7972.kt`, `basics/unchecked_cast11.kt` |
| `JVM_DEFAULT_MODE: enable` | 11 | `compileKotlinAgainstKotlin/jvm8/defaults/enable/superCall.kt`, `compileKotlinAgainstKotlin/jvm8/defaults/enable/superCallFromInterface.kt` |
| `API_VERSION` | 10 | `delegatedProperty/beforeDeclarationContainerOptimization.kt`, `volatile/volatileBool.kt` |
| `Java sources` | 6 | `constants/numberLiteralCoercionToInferredType.kt`, `fakeOverride/kt40180.kt` |
| `LAMBDAS: CLASS` | 4 | `functions/functionNtoString.kt`, `functions/functionNtoStringGeneric.kt` |
| `ALLOW_KOTLIN_PACKAGE` | 2 | `annotations/intrinsicConstEvaluationInSources.kt`, `package/packageLocalClassNotImportedWithDefaultImport.kt` |
| `ASSERTIONS_MODE` | 2 | `assert/alwaysDisable.kt`, `assert/alwaysEnable.kt` |
| `TARGET_BACKEND: WASM_JS` | 1 | `casts/wasmCastMessagesWasmJs.kt` |

## By directory

*Bad bytecode* is VerifyError and linkage errors; *wrong* is a wrong result, an exception or a timeout.

| Directory | Run | Pass | Unsupported | Check error | Crash | Bad bytecode | Wrong |
|:---|---:|---:|---:|---:|---:|---:|---:|
| ranges | 805 | 804 | 0 | 1 | 0 | 0 | 0 |
| inlineClasses | 784 | 761 | 0 | 16 | 0 | 7 | 0 |
| coroutines | 497 | 491 | 0 | 1 | 3 | 1 | 1 |
| callableReference | 297 | 283 | 0 | 12 | 0 | 0 | 2 |
| inference | 205 | 194 | 0 | 10 | 0 | 0 | 1 |
| delegatedProperty | 149 | 145 | 0 | 3 | 0 | 0 | 1 |
| controlStructures | 146 | 145 | 0 | 0 | 0 | 0 | 1 |
| bridges | 137 | 136 | 0 | 0 | 0 | 1 | 0 |
| inline | 132 | 128 | 0 | 4 | 0 | 0 | 0 |
| classes | 120 | 119 | 0 | 0 | 0 | 1 | 0 |
| objects | 116 | 114 | 0 | 1 | 0 | 1 | 0 |
| properties | 109 | 106 | 0 | 2 | 0 | 1 | 0 |
| closures | 107 | 104 | 0 | 2 | 0 | 0 | 1 |
| functions | 106 | 102 | 0 | 4 | 0 | 0 | 0 |
| enum | 101 | 97 | 0 | 1 | 0 | 0 | 3 |
| when | 99 | 98 | 0 | 1 | 0 | 0 | 0 |
| primitiveTypes | 98 | 90 | 0 | 7 | 0 | 0 | 1 |
| defaultArguments | 82 | 82 | 0 | 0 | 0 | 0 | 0 |
| arrays | 78 | 77 | 0 | 1 | 0 | 0 | 0 |
| regressions | 75 | 72 | 0 | 2 | 1 | 0 | 0 |
| contextParameters | 70 | 70 | 0 | 0 | 0 | 0 | 0 |
| casts | 68 | 64 | 0 | 1 | 0 | 0 | 3 |
| compileKotlinAgainstKotlin | 65 | 60 | 0 | 4 | 0 | 0 | 1 |
| diagnostics | 60 | 58 | 0 | 2 | 0 | 0 | 0 |
| dataClasses | 53 | 52 | 0 | 0 | 0 | 1 | 0 |
| extensionFunctions | 53 | 43 | 0 | 10 | 0 | 0 | 0 |
| inlineEvaluationOrder | 52 | 52 | 0 | 0 | 0 | 0 | 0 |
| innerNested | 52 | 50 | 0 | 0 | 0 | 2 | 0 |
| annotations | 51 | 49 | 0 | 1 | 0 | 0 | 1 |
| reflection | 50 | 48 | 0 | 2 | 0 | 0 | 0 |
| funInterface | 48 | 46 | 0 | 2 | 0 | 0 | 0 |
| unsignedTypes | 44 | 43 | 0 | 0 | 0 | 0 | 1 |
| boxingOptimization | 41 | 41 | 0 | 0 | 0 | 0 | 0 |
| ir | 41 | 38 | 0 | 3 | 0 | 0 | 0 |
| reified | 40 | 38 | 0 | 0 | 0 | 2 | 0 |
| localClasses | 39 | 37 | 0 | 1 | 0 | 1 | 0 |
| strings | 39 | 34 | 0 | 2 | 2 | 0 | 1 |
| super | 39 | 37 | 0 | 1 | 0 | 0 | 1 |
| ieee754 | 38 | 38 | 0 | 0 | 0 | 0 | 0 |
| evaluate | 37 | 37 | 0 | 0 | 0 | 0 | 0 |
| function | 37 | 37 | 0 | 0 | 0 | 0 | 0 |
| smartCasts | 36 | 32 | 0 | 4 | 0 | 0 | 0 |
| operatorConventions | 34 | 33 | 0 | 0 | 0 | 0 | 1 |
| traits | 34 | 34 | 0 | 0 | 0 | 0 | 0 |
| typeErasure | 32 | 32 | 0 | 0 | 0 | 0 | 0 |
| extensionProperties | 31 | 30 | 0 | 0 | 1 | 0 | 0 |
| typealias | 31 | 28 | 0 | 3 | 0 | 0 | 0 |
| basics | 30 | 30 | 0 | 0 | 0 | 0 | 0 |
| secondaryConstructors | 30 | 30 | 0 | 0 | 0 | 0 | 0 |
| syntheticAccessors | 30 | 30 | 0 | 0 | 0 | 0 | 0 |
| initializers | 26 | 26 | 0 | 0 | 0 | 0 | 0 |
| try | 26 | 26 | 0 | 0 | 0 | 0 | 0 |
| fir | 25 | 20 | 0 | 3 | 0 | 2 | 0 |
| fp | 25 | 25 | 0 | 0 | 0 | 0 | 0 |
| increment | 25 | 25 | 0 | 0 | 0 | 0 | 0 |
| specialBuiltins | 25 | 19 | 0 | 2 | 0 | 2 | 2 |
| finally | 24 | 23 | 0 | 0 | 0 | 0 | 1 |
| delegation | 23 | 21 | 0 | 2 | 0 | 0 | 0 |
| binaryOp | 22 | 19 | 0 | 0 | 0 | 0 | 3 |
| intrinsics | 22 | 21 | 0 | 1 | 0 | 0 | 0 |
| boxing | 21 | 19 | 0 | 0 | 0 | 0 | 2 |
| lambda | 21 | 21 | 0 | 0 | 0 | 0 | 0 |
| sam | 21 | 19 | 0 | 2 | 0 | 0 | 0 |
| contracts | 20 | 15 | 0 | 1 | 0 | 4 | 0 |
| controlflow | 20 | 20 | 0 | 0 | 0 | 0 | 0 |
| wasmStringSwitch | 20 | 20 | 0 | 0 | 0 | 0 | 0 |
| platformTypes | 19 | 19 | 0 | 0 | 0 | 0 | 0 |
| bce | 18 | 18 | 0 | 0 | 0 | 0 | 0 |
| vararg | 18 | 18 | 0 | 0 | 0 | 0 | 0 |
| inlineSizeReduction | 17 | 17 | 0 | 0 | 0 | 0 | 0 |
| lazyCodegen | 17 | 17 | 0 | 0 | 0 | 0 | 0 |
| directInvokeOptimization | 16 | 16 | 0 | 0 | 0 | 0 | 0 |
| escapeAnalysis | 16 | 16 | 0 | 0 | 0 | 0 | 0 |
| safeCall | 16 | 14 | 0 | 0 | 0 | 0 | 2 |
| argumentOrder | 15 | 15 | 0 | 0 | 0 | 0 | 0 |
| unit | 15 | 13 | 0 | 2 | 0 | 0 | 0 |
| fakeOverride | 14 | 13 | 0 | 1 | 0 | 0 | 0 |
| suspendConversion | 14 | 14 | 0 | 0 | 0 | 0 | 0 |
| constants | 13 | 11 | 0 | 0 | 0 | 0 | 2 |
| labels | 13 | 12 | 0 | 1 | 0 | 0 | 0 |
| branching | 12 | 12 | 0 | 0 | 0 | 0 | 0 |
| classDelegation | 12 | 12 | 0 | 0 | 0 | 0 | 0 |
| innerClass | 12 | 10 | 0 | 2 | 0 | 0 | 0 |
| privateConstructors | 12 | 12 | 0 | 0 | 0 | 0 | 0 |
| arithmetic | 11 | 11 | 0 | 0 | 0 | 0 | 0 |
| elvis | 11 | 11 | 0 | 0 | 0 | 0 | 0 |
| inlineArgsInPlace | 11 | 11 | 0 | 0 | 0 | 0 | 0 |
| package | 11 | 10 | 0 | 1 | 0 | 0 | 0 |
| lateinit | 10 | 10 | 0 | 0 | 0 | 0 | 0 |
| localClass | 10 | 9 | 0 | 0 | 0 | 0 | 1 |
| lower | 9 | 9 | 0 | 0 | 0 | 0 | 0 |
| object | 9 | 9 | 0 | 0 | 0 | 0 | 0 |
| classLiteral | 8 | 6 | 0 | 2 | 0 | 0 | 0 |
| exceptions | 8 | 8 | 0 | 0 | 0 | 0 | 0 |
| propertyCallableReference | 8 | 8 | 0 | 0 | 0 | 0 | 0 |
| sealed | 8 | 7 | 0 | 1 | 0 | 0 | 0 |
| statics | 8 | 8 | 0 | 0 | 0 | 0 | 0 |
| builtinStubMethods | 7 | 7 | 0 | 0 | 0 | 0 | 0 |
| collections | 7 | 7 | 0 | 0 | 0 | 0 | 0 |
| ktype | 7 | 6 | 0 | 1 | 0 | 0 | 0 |
| sameFileInSourceAndDependencies | 7 | 7 | 0 | 0 | 0 | 0 | 0 |
| serialization | 7 | 6 | 0 | 1 | 0 | 0 | 0 |
| typeMapping | 7 | 7 | 0 | 0 | 0 | 0 | 0 |
| destructuringDeclInLambdaParam | 6 | 6 | 0 | 0 | 0 | 0 | 0 |
| devirtualization | 6 | 6 | 0 | 0 | 0 | 0 | 0 |
| nullCheckOptimization | 6 | 6 | 0 | 0 | 0 | 0 | 0 |
| typeInfo | 6 | 6 | 0 | 0 | 0 | 0 | 0 |
| interfaceCallsNCasts | 5 | 5 | 0 | 0 | 0 | 0 | 0 |
| javaInterop | 5 | 3 | 0 | 0 | 0 | 2 | 0 |
| jdk | 5 | 5 | 0 | 0 | 0 | 0 | 0 |
| mangling | 5 | 5 | 0 | 0 | 0 | 0 | 0 |
| storeStackBeforeInline | 5 | 5 | 0 | 0 | 0 | 0 | 0 |
| deadCodeElimination | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| exclExcl | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| inlineClass | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| mixedNamedPosition | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| nonLocalReturns | 4 | 3 | 0 | 0 | 0 | 0 | 1 |
| objectExpression | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| unaryOp | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| underscoredTypeArguments | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| checkcastOptimization | 3 | 3 | 0 | 0 | 0 | 0 | 0 |
| companion | 3 | 3 | 0 | 0 | 0 | 0 | 0 |
| cycles | 3 | 3 | 0 | 0 | 0 | 0 | 0 |
| dataObjects | 3 | 2 | 0 | 0 | 0 | 1 | 0 |
| fieldRename | 3 | 3 | 0 | 0 | 0 | 0 | 0 |
| nothingValue | 3 | 3 | 0 | 0 | 0 | 0 | 0 |
| dataflow | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
| instructions | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
| invokedynamic | 2 | 1 | 0 | 0 | 0 | 0 | 1 |
| kclass | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
| literals | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
| multiDecl | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
| private | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
| publishedApi | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
| toArray | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
| topLevelInitializtion | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
| topLevelPrivate | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
| compatibility | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| correctFrontendCode | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| equivalentCalls | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| expression | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| inlineStdlib | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| jsAstOptimizations | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| localEscapeAnalysis | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| nameClashes | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| objectIntrinsics | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| optimizations | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| propertyAccess | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| rtti | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| stringTrim | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| suppressions | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| trailingComma | 1 | 1 | 0 | 0 | 0 | 0 | 0 |

## Check errors

Tests kotlinc compiles that kc's checker rejects, by its first error, with names replaced by _.

| First error | Tests | Examples |
|:---|---:|:---|
| `cannot infer type for type parameter '_'. Specify it explicitly.` | 13 | `callableReference/adaptedReferences/extensionPropertyToFun.kt`, `callableReference/adaptedReferences/innerConstructorToExtensionWithTypeParam.kt` |
| `unresolved reference: _` | 11 | `classLiteral/bound/primitives.kt`, `classLiteral/primitiveClassEquality.kt` |
| `only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type _` | 9 | `callableReference/function/nullableExtension.kt`, `casts/intersectionTypeMultipleBounds.kt` |
| `type argument is not within its bounds: inferred type Int of type _ '_' is not a subtype of Long` | 9 | `inlineClasses/checkBoxingForNonLocalAndLabeledReturnsGeneric.kt`, `inlineClasses/contextsAndAccessors/accessPrivateInlineClassCompanionMethod2Generic.kt` |
| `class '_' is not abstract and does not implement abstract member '_'` | 8 | `primitiveTypes/numberToChar/abstractMethodInSuperinterface.kt`, `primitiveTypes/numberToChar/fakeOverride.kt` |
| `operator '_' cannot be applied to String` | 5 | `extensionFunctions/kt1953.kt`, `extensionFunctions/kt1953_class.kt` |
| `type mismatch: inferred type is _ but _ was expected` | 4 | `delegation/delegationToIntersectionType.kt`, `delegation/delegationToIntersectionType3.kt` |
| `expression of type _ cannot be invoked as a function` | 3 | `functions/invoke/kt3631invokeOnString.kt`, `inline/operators.kt` |
| `many supertypes available, please specify the one you mean in angle brackets, e.g. '_'` | 3 | `compileKotlinAgainstKotlin/jvm8/jvm8against6/delegation/diamond.kt`, `compileKotlinAgainstKotlin/jvm8/jvm8against6/delegation/diamond3.kt` |
| `type argument is not within its bounds: inferred type I of type _ '_' is not a subtype of In` | 3 | `inference/tryCatchAtAssignmentWithSmartCast.kt`, `inference/whenAtAssignmentWithSmartCast.kt` |
| `'_' is not an expression, it can only be used on the left-hand side of a dot ('_')` | 2 | `closures/kt11634.kt`, `closures/kt11634_2.kt` |
| `cannot find a parameter with this name: _` | 2 | `compileKotlinAgainstKotlin/clashingFakeOverrideSignatures.kt`, `ir/serializationRegressions/signatureClash.kt` |
| `conflicting overloads: _` | 2 | `callableReference/function/extensionFunctionLocal.kt`, `extensionFunctions/extensionFunctionLocal.kt` |
| `enum entry '_' does not implement abstract member '_'` | 2 | `callableReference/function/local/enumExtendsTrait.kt`, `specialBuiltins/enumAsOrdinaled.kt` |
| `expression '_' of type _ cannot be invoked as a function` | 2 | `smartCasts/invokeRecieverSmartcastK2.kt`, `typealias/typeAliasObjectCallable.kt` |
| `initializer is not allowed here because this property has no backing field` | 2 | `properties/classFieldInsideNested.kt`, `properties/fieldInsideNested.kt` |
| `overload resolution ambiguity for Z::hashCode: specify the expected function type` | 2 | `inlineClasses/callableReferences/equalsHashCodeToString.kt`, `inlineClasses/callableReferences/equalsHashCodeToStringGeneric.kt` |
| `platform declaration clash: the following declarations have the same JVM signature (getData()I)` | 2 | `extensionFunctions/classMethodCallExtensionSuper.kt`, `extensionFunctions/defaultMethodInterfaceCallExtensionSuper.kt` |
| `type argument is not within its bounds: inferred type IcLong<Int> of type _ '_' is not a subtype of IcLong<Long>` | 2 | `inlineClasses/boundCallableReferencePassedToInlineFunctionGeneric.kt`, `inlineClasses/boundCallableReferencePassedToInlineFunctionGeneric2.kt` |
| `type mismatch: '_' branches have incompatible types Unit and Nothing?` | 2 | `inference/noCoercionToUnitWithEqualityConstraintForNullableReturnType.kt`, `unit/nullableUnitInWhen3.kt` |
| `val cannot be reassigned` | 2 | `extensionFunctions/kt475.kt`, `fakeOverride/intersectionOverrideBetweenValAndVar.kt` |
| `'_' hides member of supertype '_' and needs '_' modifier` | 1 | `annotations/platformDependent.kt` |
| `'_' in '_' is final and cannot be overridden` | 1 | `fir/functionsDifferInTypeParameterBounds.kt` |
| `'_' is not allowed here` | 1 | `arrays/nonLocalReturnArrayConstructor.kt` |
| `'_' is not defined in this context` | 1 | `objects/thisRefToObjectInNestedClassConstructorCall.kt` |
| `'_' modifier is inapplicable on this function: must have no value parameters` | 1 | `extensionFunctions/kt3969.kt` |
| `ambiguous call: n candidates for '_' take (_)` | 1 | `callableReference/function/overloadedFun.kt` |
| `cannot create an instance of Result` | 1 | `inlineClasses/resultGeneric.kt` |
| `class '_' cannot be used as a value` | 1 | `package/privateMembersInImportList.kt` |
| `class A.X is not public` | 1 | `ir/serializationRegressions/innerClassInEnumEntryClass.kt` |
| `class Foo.FOO is not public` | 1 | `enum/enumEntryMembers.kt` |
| `expected parameter of type _` | 1 | `diagnostics/functions/tailRecursion/realIteratorFoldl.kt` |
| `expression '_' of type _ String> cannot be invoked as a function` | 1 | `callableReference/bound/dontShareReceiver.kt` |
| `function invocation '_' expected; use ::box for a reference` | 1 | `functions/kt58825_arrayOverloads.kt` |
| `function invocation '_' expected; use ::right for a reference` | 1 | `delegatedProperty/functionRefDelefate.kt` |
| `local inline functions are not yet supported` | 1 | `inline/nestedLabelsInlinedClashingAtFunctionsWithClosure.kt` |
| `no type arguments expected for class Generic.Inner` | 1 | `reflection/classes/qualifiedName.kt` |
| `not enough information to infer type arguments of A::foo: specify the expected function type` | 1 | `callableReference/generticReferences.kt` |
| `not enough information to infer type arguments of Foo::InnerAlias: specify the expected function type` | 1 | `typealias/typeArgumentsMappingForTopLevelTAWithInnerRHS.kt` |
| `one type argument expected for class Outer.Inner` | 1 | `ktype/ktype1_inner.kt` |
| *18 more* | 18 | |

## Code generation errors

kc checked the test but failed to generate its code.

| Error | Tests | Examples |
|:---|---:|:---|
| `code generation: operand stack underflow at offset n` | 2 | `coroutines/simpleSuspendCallableReference.kt`, `extensionProperties/kt9897.kt` |
| `code generation: string constant too long` | 2 | `strings/kt13213.kt`, `strings/kt47917.kt` |
| `code generation: a suspension point in the arguments of a constructor call` | 1 | `coroutines/kt15016.kt` |
| `code generation: local n is not allocated` | 1 | `regressions/kt5953.kt` |
| `code generation: no metadata for type <error>` | 1 | `coroutines/deserializedSuspendFunctionProperty.kt` |

## VerifyError

The JVM rejected a class kc wrote.

| Error | Tests | Examples |
|:---|---:|:---|
| `java.lang.VerifyError: Bad local variable type` | 7 | `contracts/functionParameter.kt`, `contracts/nestedLambdaInNonInlineCallExactlyOnce.kt` |
| `java.lang.ClassFormatError: Method "_" in class _ has illegal signature "_"` | 3 | `inlineClasses/dnnInValue.kt`, `inlineClasses/genericInlineClassSynthMembers.kt` |
| `java.lang.ClassFormatError: Duplicate method name "_" with signature "_" in class _ NotEmptyMap` | 2 | `bridges/special.kt`, `specialBuiltins/bridgeNotEmptyMap.kt` |
| `java.lang.ClassFormatError: Duplicate method name "_" with signature "_" in class _ Y` | 2 | `fir/functionsDifferInTypeParameterBounds2.kt`, `fir/functionsDifferInTypeParameterBounds3.kt` |
| `java.lang.VerifyError: Bad invokespecial instruction: interface method reference is in an indirect superinterface.` | 2 | `inlineClasses/smartCastOnThisOfInlineClassType.kt`, `inlineClasses/smartCastOnThisOfInlineClassTypeGeneric.kt` |
| `java.lang.ClassFormatError: Duplicate method name "_" with signature "_" in class _ EmptyMap` | 1 | `specialBuiltins/emptyMap.kt` |
| `java.lang.VerifyError: Bad type on operand stack` | 1 | `inlineClasses/kt57973.kt` |
| `java.lang.VerifyError: Bad type on operand stack in putfield` | 1 | `properties/classFieldInsideLocalInSetter.kt` |
| `java.lang.VerifyError: Instruction type does not match stack map` | 1 | `inlineClasses/checkCastToInlineClassGeneric.kt` |
| `java.lang.VerifyError: get long/double overflows locals` | 1 | `contracts/exactlyOnceNotInline.kt` |

## Linkage errors

A class kc wrote refers to a member or class that doesn't exist, or doesn't fit.

| Error | Tests | Examples |
|:---|---:|:---|
| `java.lang.NoSuchMethodError` | 4 | `classes/kt5347.kt`, `javaInterop/objectMethods/cloneCallsSuper.kt` |
| `java.lang.IncompatibleClassChangeError` | 2 | `dataClasses/nonTrivialFinalMemberInSuperClass.kt`, `dataObjects/toString.kt` |
| `java.lang.AbstractMethodError` | 1 | `coroutines/inlineClasses/kt84305.kt` |
| `java.lang.NoSuchFieldError` | 1 | `objects/interfaceCompanionObjectReference.kt` |

## Exceptions

box() threw.

| Exception | Tests | Examples |
|:---|---:|:---|
| `java.lang.NullPointerException` | 13 | `boxing/kt84727.kt`, `boxing/kt84727_inline.kt` |
| `kotlin.reflect.jvm.internal.KotlinReflectionInternalError` | 1 | `compileKotlinAgainstKotlin/inlineClasses/privateCompanionObjectValInDifferentModule.kt` |

## Wrong results

box() returned something other than `"OK"`, with numbers replaced by n.

| Result | Tests | Examples |
|:---|---:|:---|
| `FAIL n` | 2 | `casts/kt59022.kt`, `constants/doNotTriggerInit.kt` |
| `FAILn` | 2 | `controlStructures/breakContinueInExpressions/breakInLoopConditions.kt`, `finally/returnNullFromInlined.kt` |
| `fail` | 2 | `callableReference/property/extensionPropertyReferenceWithTypeParameter.kt`, `unsignedTypes/unsignedLiteralsWithSignedOverflow.kt` |
| `fail: _` | 2 | `coroutines/unitTypeReturn/unitSafeCall.kt`, `inference/noCoercionToUniForNullableLambdaReturnType.kt` |
| `FAIL: expected NPE` | 1 | `casts/castToDefinitelyNotNullType.kt` |
| `Fail` | 1 | `invokedynamic/extensionFunInvoke.kt` |
| `Fail n` | 1 | `primitiveTypes/comparisonWithNullCallsFun.kt` |
| `Fail n: A.bNonConst !== B.bNonConst` | 1 | `constants/kt9532.kt` |
| `Fail: _` | 1 | `closures/captureInSuperConstructorCall/properValueCapturedByClosure2.kt` |
| `Failed: should throw NPE` | 1 | `casts/kt50577.kt` |
| `OK Fail` | 1 | `strings/stringPlusOverride.kt` |
| `OKOK` | 1 | `binaryOp/eqNullableToPrimitiveWithSideEffects.kt` |
| `fail n` | 1 | `operatorConventions/infixFunctionOverBuiltinMember.kt` |
| `fail n: _` | 1 | `callableReference/function/referenceToCompanionMember.kt` |
| `fail: Char.MIN_VALUE - n should overflow to positive.` | 1 | `binaryOp/overflowChar.kt` |
| `fail: in this case we should add to ints and than cast the r…` | 1 | `binaryOp/overflowLong.kt` |
| `failn` | 1 | `annotations/resolveWithLowPriorityAnnotation.kt` |

