# Kotlin's box tests on kc

The box tests of JetBrains/kotlin v2.4.21 (`compiler/testData/codegen/box`), each compiled by kc and run: its `box()` must return `"OK"`. `bench/kotlin-tests.sh box` writes this file, and [docs/conformance.md](../../docs/conformance.md) explains it.

Of 7480 test files, 937 are skipped and 6543 run. 6542 pass (100.0%).

| Result | Tests | Share |
|:---|---:|---:|
| pass | 6542 | 100.0% |
| parse/unsupported | 0 | 0.0% |
| check error | 0 | 0.0% |
| crash | 0 | 0.0% |
| codegen error | 1 | 0.0% |
| VerifyError | 0 | 0.0% |
| linkage error | 0 | 0.0% |
| exception | 0 | 0.0% |
| wrong result | 0 | 0.0% |
| timeout | 0 | 0.0% |

## Skipped

Tests for other backends, that kotlinc's JVM backend ignores, or that need what the harness doesn't do: language features off by default, compiler options, Java sources, or modules other than plain dependencies.

| Reason | Tests | Examples |
|:---|---:|:---|
| `LANGUAGE` | 564 | `annotations/checkReturnValue.kt`, `annotations/instances/multiplatformInstantiation.kt` |
| `TARGET_BACKEND: WASM` | 122 | `casts/kt73931.kt`, `casts/wasmCastMessages.kt` |
| `IGNORE_BACKEND: JVM_IR` | 52 | `binaryOp/call.kt`, `binaryOp/callNullable.kt` |
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
| inlineClasses | 787 | 787 | 0 | 0 | 0 | 0 | 0 |
| coroutines | 499 | 498 | 0 | 0 | 1 | 0 | 0 |
| callableReference | 299 | 299 | 0 | 0 | 0 | 0 | 0 |
| inference | 207 | 207 | 0 | 0 | 0 | 0 | 0 |
| controlStructures | 158 | 158 | 0 | 0 | 0 | 0 | 0 |
| delegatedProperty | 149 | 149 | 0 | 0 | 0 | 0 | 0 |
| bridges | 137 | 137 | 0 | 0 | 0 | 0 | 0 |
| inline | 132 | 132 | 0 | 0 | 0 | 0 | 0 |
| properties | 124 | 124 | 0 | 0 | 0 | 0 | 0 |
| classes | 120 | 120 | 0 | 0 | 0 | 0 | 0 |
| objects | 116 | 116 | 0 | 0 | 0 | 0 | 0 |
| closures | 107 | 107 | 0 | 0 | 0 | 0 | 0 |
| functions | 106 | 106 | 0 | 0 | 0 | 0 | 0 |
| when | 103 | 103 | 0 | 0 | 0 | 0 | 0 |
| enum | 102 | 102 | 0 | 0 | 0 | 0 | 0 |
| primitiveTypes | 98 | 98 | 0 | 0 | 0 | 0 | 0 |
| defaultArguments | 82 | 82 | 0 | 0 | 0 | 0 | 0 |
| arrays | 78 | 78 | 0 | 0 | 0 | 0 | 0 |
| regressions | 75 | 75 | 0 | 0 | 0 | 0 | 0 |
| contextParameters | 70 | 70 | 0 | 0 | 0 | 0 | 0 |
| casts | 68 | 68 | 0 | 0 | 0 | 0 | 0 |
| compileKotlinAgainstKotlin | 65 | 65 | 0 | 0 | 0 | 0 | 0 |
| diagnostics | 60 | 60 | 0 | 0 | 0 | 0 | 0 |
| dataClasses | 53 | 53 | 0 | 0 | 0 | 0 | 0 |
| extensionFunctions | 53 | 53 | 0 | 0 | 0 | 0 | 0 |
| inlineEvaluationOrder | 52 | 52 | 0 | 0 | 0 | 0 | 0 |
| innerNested | 52 | 52 | 0 | 0 | 0 | 0 | 0 |
| annotations | 51 | 51 | 0 | 0 | 0 | 0 | 0 |
| reflection | 50 | 50 | 0 | 0 | 0 | 0 | 0 |
| funInterface | 48 | 48 | 0 | 0 | 0 | 0 | 0 |
| reified | 46 | 46 | 0 | 0 | 0 | 0 | 0 |
| unsignedTypes | 44 | 44 | 0 | 0 | 0 | 0 | 0 |
| boxingOptimization | 41 | 41 | 0 | 0 | 0 | 0 | 0 |
| ir | 41 | 41 | 0 | 0 | 0 | 0 | 0 |
| strings | 40 | 40 | 0 | 0 | 0 | 0 | 0 |
| ieee754 | 39 | 39 | 0 | 0 | 0 | 0 | 0 |
| localClasses | 39 | 39 | 0 | 0 | 0 | 0 | 0 |
| super | 39 | 39 | 0 | 0 | 0 | 0 | 0 |
| evaluate | 37 | 37 | 0 | 0 | 0 | 0 | 0 |
| function | 37 | 37 | 0 | 0 | 0 | 0 | 0 |
| smartCasts | 36 | 36 | 0 | 0 | 0 | 0 | 0 |
| operatorConventions | 35 | 35 | 0 | 0 | 0 | 0 | 0 |
| traits | 34 | 34 | 0 | 0 | 0 | 0 | 0 |
| typealias | 34 | 34 | 0 | 0 | 0 | 0 | 0 |
| typeErasure | 32 | 32 | 0 | 0 | 0 | 0 | 0 |
| extensionProperties | 31 | 31 | 0 | 0 | 0 | 0 | 0 |
| basics | 30 | 30 | 0 | 0 | 0 | 0 | 0 |
| secondaryConstructors | 30 | 30 | 0 | 0 | 0 | 0 | 0 |
| syntheticAccessors | 30 | 30 | 0 | 0 | 0 | 0 | 0 |
| increment | 26 | 26 | 0 | 0 | 0 | 0 | 0 |
| initializers | 26 | 26 | 0 | 0 | 0 | 0 | 0 |
| try | 26 | 26 | 0 | 0 | 0 | 0 | 0 |
| fir | 25 | 25 | 0 | 0 | 0 | 0 | 0 |
| fp | 25 | 25 | 0 | 0 | 0 | 0 | 0 |
| specialBuiltins | 25 | 25 | 0 | 0 | 0 | 0 | 0 |
| finally | 24 | 24 | 0 | 0 | 0 | 0 | 0 |
| delegation | 23 | 23 | 0 | 0 | 0 | 0 | 0 |
| intrinsics | 23 | 23 | 0 | 0 | 0 | 0 | 0 |
| binaryOp | 22 | 22 | 0 | 0 | 0 | 0 | 0 |
| sam | 22 | 22 | 0 | 0 | 0 | 0 | 0 |
| boxing | 21 | 21 | 0 | 0 | 0 | 0 | 0 |
| lambda | 21 | 21 | 0 | 0 | 0 | 0 | 0 |
| contracts | 20 | 20 | 0 | 0 | 0 | 0 | 0 |
| controlflow | 20 | 20 | 0 | 0 | 0 | 0 | 0 |
| wasmStringSwitch | 20 | 20 | 0 | 0 | 0 | 0 | 0 |
| platformTypes | 19 | 19 | 0 | 0 | 0 | 0 | 0 |
| bce | 18 | 18 | 0 | 0 | 0 | 0 | 0 |
| lazyCodegen | 18 | 18 | 0 | 0 | 0 | 0 | 0 |
| vararg | 18 | 18 | 0 | 0 | 0 | 0 | 0 |
| inlineSizeReduction | 17 | 17 | 0 | 0 | 0 | 0 | 0 |
| safeCall | 17 | 17 | 0 | 0 | 0 | 0 | 0 |
| directInvokeOptimization | 16 | 16 | 0 | 0 | 0 | 0 | 0 |
| escapeAnalysis | 16 | 16 | 0 | 0 | 0 | 0 | 0 |
| argumentOrder | 15 | 15 | 0 | 0 | 0 | 0 | 0 |
| unit | 15 | 15 | 0 | 0 | 0 | 0 | 0 |
| fakeOverride | 14 | 14 | 0 | 0 | 0 | 0 | 0 |
| suspendConversion | 14 | 14 | 0 | 0 | 0 | 0 | 0 |
| constants | 13 | 13 | 0 | 0 | 0 | 0 | 0 |
| labels | 13 | 13 | 0 | 0 | 0 | 0 | 0 |
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
| classLiteral | 8 | 8 | 0 | 0 | 0 | 0 | 0 |
| exceptions | 8 | 8 | 0 | 0 | 0 | 0 | 0 |
| propertyCallableReference | 8 | 8 | 0 | 0 | 0 | 0 | 0 |
| sealed | 8 | 8 | 0 | 0 | 0 | 0 | 0 |
| statics | 8 | 8 | 0 | 0 | 0 | 0 | 0 |
| builtinStubMethods | 7 | 7 | 0 | 0 | 0 | 0 | 0 |
| collections | 7 | 7 | 0 | 0 | 0 | 0 | 0 |
| ktype | 7 | 7 | 0 | 0 | 0 | 0 | 0 |
| sameFileInSourceAndDependencies | 7 | 7 | 0 | 0 | 0 | 0 | 0 |
| serialization | 7 | 7 | 0 | 0 | 0 | 0 | 0 |
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
| companion | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| deadCodeElimination | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| exclExcl | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| inlineClass | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| mixedNamedPosition | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| nonLocalReturns | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| objectExpression | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| unaryOp | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| underscoredTypeArguments | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| checkcastOptimization | 3 | 3 | 0 | 0 | 0 | 0 | 0 |
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
| returnInExpressionBody | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| rtti | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| stringTrim | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| suppressions | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| trailingComma | 1 | 1 | 0 | 0 | 0 | 0 | 0 |

## Code generation errors

kc checked the test but failed to generate its code.

| Error | Tests | Examples |
|:---|---:|:---|
| `code generation: method too large: branch offset n` | 1 | `coroutines/kt51530.kt` |

