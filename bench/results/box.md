# Kotlin's box tests on kc

The box tests of JetBrains/kotlin v2.4.21 (`compiler/testData/codegen/box`), each compiled by kc and run: its `box()` must return `"OK"`. `bench/kotlin-tests.sh box` writes this file, and [docs/conformance.md](../../docs/conformance.md) explains it.

Of 7480 test files, 997 are skipped and 6483 run. 6434 pass (99.2%).

| Result | Tests | Share |
|:---|---:|---:|
| pass | 6434 | 99.2% |
| parse/unsupported | 0 | 0.0% |
| check error | 44 | 0.7% |
| crash | 0 | 0.0% |
| codegen error | 0 | 0.0% |
| VerifyError | 0 | 0.0% |
| linkage error | 1 | 0.0% |
| exception | 1 | 0.0% |
| wrong result | 3 | 0.0% |
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
| ranges | 805 | 805 | 0 | 0 | 0 | 0 | 0 |
| inlineClasses | 784 | 780 | 0 | 4 | 0 | 0 | 0 |
| coroutines | 497 | 496 | 0 | 0 | 0 | 0 | 1 |
| callableReference | 297 | 297 | 0 | 0 | 0 | 0 | 0 |
| inference | 205 | 203 | 0 | 2 | 0 | 0 | 0 |
| delegatedProperty | 149 | 146 | 0 | 3 | 0 | 0 | 0 |
| controlStructures | 146 | 145 | 0 | 0 | 0 | 0 | 1 |
| bridges | 137 | 137 | 0 | 0 | 0 | 0 | 0 |
| inline | 132 | 130 | 0 | 1 | 0 | 1 | 0 |
| classes | 120 | 120 | 0 | 0 | 0 | 0 | 0 |
| objects | 116 | 115 | 0 | 1 | 0 | 0 | 0 |
| properties | 109 | 107 | 0 | 2 | 0 | 0 | 0 |
| closures | 107 | 104 | 0 | 2 | 0 | 0 | 1 |
| functions | 106 | 105 | 0 | 1 | 0 | 0 | 0 |
| enum | 101 | 101 | 0 | 0 | 0 | 0 | 0 |
| when | 99 | 99 | 0 | 0 | 0 | 0 | 0 |
| primitiveTypes | 98 | 98 | 0 | 0 | 0 | 0 | 0 |
| defaultArguments | 82 | 82 | 0 | 0 | 0 | 0 | 0 |
| arrays | 78 | 77 | 0 | 1 | 0 | 0 | 0 |
| regressions | 75 | 74 | 0 | 1 | 0 | 0 | 0 |
| contextParameters | 70 | 70 | 0 | 0 | 0 | 0 | 0 |
| casts | 68 | 68 | 0 | 0 | 0 | 0 | 0 |
| compileKotlinAgainstKotlin | 65 | 61 | 0 | 3 | 0 | 0 | 1 |
| diagnostics | 60 | 58 | 0 | 2 | 0 | 0 | 0 |
| dataClasses | 53 | 53 | 0 | 0 | 0 | 0 | 0 |
| extensionFunctions | 53 | 51 | 0 | 2 | 0 | 0 | 0 |
| inlineEvaluationOrder | 52 | 52 | 0 | 0 | 0 | 0 | 0 |
| innerNested | 52 | 52 | 0 | 0 | 0 | 0 | 0 |
| annotations | 51 | 50 | 0 | 1 | 0 | 0 | 0 |
| reflection | 50 | 50 | 0 | 0 | 0 | 0 | 0 |
| funInterface | 48 | 47 | 0 | 1 | 0 | 0 | 0 |
| unsignedTypes | 44 | 44 | 0 | 0 | 0 | 0 | 0 |
| boxingOptimization | 41 | 41 | 0 | 0 | 0 | 0 | 0 |
| ir | 41 | 39 | 0 | 2 | 0 | 0 | 0 |
| reified | 40 | 40 | 0 | 0 | 0 | 0 | 0 |
| localClasses | 39 | 38 | 0 | 1 | 0 | 0 | 0 |
| strings | 39 | 39 | 0 | 0 | 0 | 0 | 0 |
| super | 39 | 38 | 0 | 1 | 0 | 0 | 0 |
| ieee754 | 38 | 38 | 0 | 0 | 0 | 0 | 0 |
| evaluate | 37 | 37 | 0 | 0 | 0 | 0 | 0 |
| function | 37 | 37 | 0 | 0 | 0 | 0 | 0 |
| smartCasts | 36 | 35 | 0 | 1 | 0 | 0 | 0 |
| operatorConventions | 34 | 34 | 0 | 0 | 0 | 0 | 0 |
| traits | 34 | 34 | 0 | 0 | 0 | 0 | 0 |
| typeErasure | 32 | 32 | 0 | 0 | 0 | 0 | 0 |
| extensionProperties | 31 | 31 | 0 | 0 | 0 | 0 | 0 |
| typealias | 31 | 31 | 0 | 0 | 0 | 0 | 0 |
| basics | 30 | 30 | 0 | 0 | 0 | 0 | 0 |
| secondaryConstructors | 30 | 30 | 0 | 0 | 0 | 0 | 0 |
| syntheticAccessors | 30 | 30 | 0 | 0 | 0 | 0 | 0 |
| initializers | 26 | 26 | 0 | 0 | 0 | 0 | 0 |
| try | 26 | 26 | 0 | 0 | 0 | 0 | 0 |
| fir | 25 | 23 | 0 | 2 | 0 | 0 | 0 |
| fp | 25 | 25 | 0 | 0 | 0 | 0 | 0 |
| increment | 25 | 25 | 0 | 0 | 0 | 0 | 0 |
| specialBuiltins | 25 | 25 | 0 | 0 | 0 | 0 | 0 |
| finally | 24 | 24 | 0 | 0 | 0 | 0 | 0 |
| delegation | 23 | 21 | 0 | 2 | 0 | 0 | 0 |
| binaryOp | 22 | 22 | 0 | 0 | 0 | 0 | 0 |
| intrinsics | 22 | 22 | 0 | 0 | 0 | 0 | 0 |
| boxing | 21 | 21 | 0 | 0 | 0 | 0 | 0 |
| lambda | 21 | 21 | 0 | 0 | 0 | 0 | 0 |
| sam | 21 | 19 | 0 | 2 | 0 | 0 | 0 |
| contracts | 20 | 19 | 0 | 1 | 0 | 0 | 0 |
| controlflow | 20 | 20 | 0 | 0 | 0 | 0 | 0 |
| wasmStringSwitch | 20 | 20 | 0 | 0 | 0 | 0 | 0 |
| platformTypes | 19 | 19 | 0 | 0 | 0 | 0 | 0 |
| bce | 18 | 18 | 0 | 0 | 0 | 0 | 0 |
| vararg | 18 | 18 | 0 | 0 | 0 | 0 | 0 |
| inlineSizeReduction | 17 | 17 | 0 | 0 | 0 | 0 | 0 |
| lazyCodegen | 17 | 17 | 0 | 0 | 0 | 0 | 0 |
| directInvokeOptimization | 16 | 16 | 0 | 0 | 0 | 0 | 0 |
| escapeAnalysis | 16 | 16 | 0 | 0 | 0 | 0 | 0 |
| safeCall | 16 | 16 | 0 | 0 | 0 | 0 | 0 |
| argumentOrder | 15 | 15 | 0 | 0 | 0 | 0 | 0 |
| unit | 15 | 15 | 0 | 0 | 0 | 0 | 0 |
| fakeOverride | 14 | 13 | 0 | 1 | 0 | 0 | 0 |
| suspendConversion | 14 | 14 | 0 | 0 | 0 | 0 | 0 |
| constants | 13 | 13 | 0 | 0 | 0 | 0 | 0 |
| labels | 13 | 12 | 0 | 1 | 0 | 0 | 0 |
| branching | 12 | 12 | 0 | 0 | 0 | 0 | 0 |
| classDelegation | 12 | 12 | 0 | 0 | 0 | 0 | 0 |
| innerClass | 12 | 12 | 0 | 0 | 0 | 0 | 0 |
| privateConstructors | 12 | 12 | 0 | 0 | 0 | 0 | 0 |
| arithmetic | 11 | 11 | 0 | 0 | 0 | 0 | 0 |
| elvis | 11 | 11 | 0 | 0 | 0 | 0 | 0 |
| inlineArgsInPlace | 11 | 11 | 0 | 0 | 0 | 0 | 0 |
| package | 11 | 11 | 0 | 0 | 0 | 0 | 0 |
| lateinit | 10 | 10 | 0 | 0 | 0 | 0 | 0 |
| localClass | 10 | 10 | 0 | 0 | 0 | 0 | 0 |
| lower | 9 | 9 | 0 | 0 | 0 | 0 | 0 |
| object | 9 | 9 | 0 | 0 | 0 | 0 | 0 |
| classLiteral | 8 | 6 | 0 | 2 | 0 | 0 | 0 |
| exceptions | 8 | 8 | 0 | 0 | 0 | 0 | 0 |
| propertyCallableReference | 8 | 8 | 0 | 0 | 0 | 0 | 0 |
| sealed | 8 | 8 | 0 | 0 | 0 | 0 | 0 |
| statics | 8 | 8 | 0 | 0 | 0 | 0 | 0 |
| builtinStubMethods | 7 | 7 | 0 | 0 | 0 | 0 | 0 |
| collections | 7 | 7 | 0 | 0 | 0 | 0 | 0 |
| ktype | 7 | 7 | 0 | 0 | 0 | 0 | 0 |
| sameFileInSourceAndDependencies | 7 | 7 | 0 | 0 | 0 | 0 | 0 |
| serialization | 7 | 6 | 0 | 1 | 0 | 0 | 0 |
| typeMapping | 7 | 7 | 0 | 0 | 0 | 0 | 0 |
| destructuringDeclInLambdaParam | 6 | 6 | 0 | 0 | 0 | 0 | 0 |
| devirtualization | 6 | 6 | 0 | 0 | 0 | 0 | 0 |
| nullCheckOptimization | 6 | 6 | 0 | 0 | 0 | 0 | 0 |
| typeInfo | 6 | 6 | 0 | 0 | 0 | 0 | 0 |
| interfaceCallsNCasts | 5 | 5 | 0 | 0 | 0 | 0 | 0 |
| javaInterop | 5 | 5 | 0 | 0 | 0 | 0 | 0 |
| jdk | 5 | 5 | 0 | 0 | 0 | 0 | 0 |
| mangling | 5 | 5 | 0 | 0 | 0 | 0 | 0 |
| storeStackBeforeInline | 5 | 5 | 0 | 0 | 0 | 0 | 0 |
| deadCodeElimination | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| exclExcl | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| inlineClass | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| mixedNamedPosition | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| nonLocalReturns | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| objectExpression | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| unaryOp | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| underscoredTypeArguments | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| checkcastOptimization | 3 | 3 | 0 | 0 | 0 | 0 | 0 |
| companion | 3 | 3 | 0 | 0 | 0 | 0 | 0 |
| cycles | 3 | 3 | 0 | 0 | 0 | 0 | 0 |
| dataObjects | 3 | 3 | 0 | 0 | 0 | 0 | 0 |
| fieldRename | 3 | 3 | 0 | 0 | 0 | 0 | 0 |
| nothingValue | 3 | 3 | 0 | 0 | 0 | 0 | 0 |
| dataflow | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
| instructions | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
| invokedynamic | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
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
| `type mismatch: inferred type is _ but _ was expected` | 6 | `delegation/delegationToIntersectionType.kt`, `delegation/delegationToIntersectionType3.kt` |
| `unresolved reference: _` | 6 | `classLiteral/bound/primitives.kt`, `classLiteral/primitiveClassEquality.kt` |
| `cannot infer type for type parameter '_'. Specify it explicitly.` | 4 | `delegatedProperty/local/kt78073.kt`, `fir/localInvokeExtension.kt` |
| `'_' is not an expression, it can only be used on the left-hand side of a dot ('_')` | 2 | `closures/kt11634.kt`, `closures/kt11634_2.kt` |
| `cannot find a parameter with this name: _` | 2 | `compileKotlinAgainstKotlin/clashingFakeOverrideSignatures.kt`, `ir/serializationRegressions/signatureClash.kt` |
| `initializer is not allowed here because this property has no backing field` | 2 | `properties/classFieldInsideNested.kt`, `properties/fieldInsideNested.kt` |
| `many supertypes available, please specify the one you mean in angle brackets, e.g. '_'` | 2 | `compileKotlinAgainstKotlin/jvm8/jvm8against6/delegation/diamond.kt`, `super/unqualifiedSuperWithMethodsOfAny.kt` |
| `only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type _` | 2 | `inlineClasses/kt70461.kt`, `ir/serializationRegressions/anonFakeOverride.kt` |
| `val cannot be reassigned` | 2 | `extensionFunctions/kt475.kt`, `fakeOverride/intersectionOverrideBetweenValAndVar.kt` |
| `'_' hides member of supertype '_' and needs '_' modifier` | 1 | `annotations/platformDependent.kt` |
| `'_' is not allowed here` | 1 | `arrays/nonLocalReturnArrayConstructor.kt` |
| `'_' is not defined in this context` | 1 | `objects/thisRefToObjectInNestedClassConstructorCall.kt` |
| `cannot create an instance of Result` | 1 | `inlineClasses/resultGeneric.kt` |
| `expected parameter of type _` | 1 | `diagnostics/functions/tailRecursion/realIteratorFoldl.kt` |
| `function invocation '_' expected; use ::right for a reference` | 1 | `delegatedProperty/functionRefDelefate.kt` |
| `local inline functions are not yet supported` | 1 | `inline/nestedLabelsInlinedClashingAtFunctionsWithClosure.kt` |
| `no function '_' takes n arguments` | 1 | `serialization/deserializedMembers.kt` |
| `operator '_' cannot be applied to T and Int` | 1 | `extensionFunctions/thisMethodInObjectLiteral.kt` |
| `platform declaration clash: the following declarations have the same JVM signature (test()Ljava/lang/String;)` | 1 | `compileKotlinAgainstKotlin/jvm8/jvm8against6/delegation/diamond3.kt` |
| `property must be initialized or be abstract` | 1 | `fir/initializationInLocalInnerClassWithSecondaryConstructor.kt` |
| `tailrec is not allowed on open members` | 1 | `diagnostics/functions/tailRecursion/defaultArgsOverridden.kt` |
| `type argument is not within its bounds: inferred type String of type _ '_' is not a subtype of MyClass` | 1 | `delegatedProperty/provideDelegate/generic.kt` |
| `type checking has run into a recursive problem: '_' needs an explicit return type` | 1 | `functions/localFunctions/overloadedLocalFunWithoutClosure.kt` |
| `type parameter L is declared as '_' but occurs in '_' position in type (L) -> Unit` | 1 | `inference/capturedTypesSubstitutionIntoAbbreviation.kt` |
| `unresolved label: _` | 1 | `labels/infixCallLabelling.kt` |

## Linkage errors

A class kc wrote refers to a member or class that doesn't exist, or doesn't fit.

| Error | Tests | Examples |
|:---|---:|:---|
| `java.lang.IllegalAccessError` | 1 | `inline/innerInlineFunCapturesOuter.kt` |

## Exceptions

box() threw.

| Exception | Tests | Examples |
|:---|---:|:---|
| `kotlin.reflect.jvm.internal.KotlinReflectionInternalError` | 1 | `compileKotlinAgainstKotlin/inlineClasses/privateCompanionObjectValInDifferentModule.kt` |

## Wrong results

box() returned something other than `"OK"`, with numbers replaced by n.

| Result | Tests | Examples |
|:---|---:|:---|
| `FAILn` | 1 | `controlStructures/breakContinueInExpressions/breakInLoopConditions.kt` |
| `Fail: _` | 1 | `closures/captureInSuperConstructorCall/properValueCapturedByClosure2.kt` |
| `fail: _` | 1 | `coroutines/unitTypeReturn/unitSafeCall.kt` |

