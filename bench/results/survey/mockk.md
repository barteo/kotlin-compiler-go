# Survey: mockk v1.14.11

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 144 files, 17302 lines

Directories: `modules/mockk/src/commonMain`, `modules/mockk/src/jvmMain`, `modules/mockk-dsl/src/commonMain`, `modules/mockk-dsl/src/jvmMain`, `modules/mockk-core/src/commonMain`, `modules/mockk-core/src/jvmMain`, `modules/mockk-agent-api/src/commonMain`, `modules/mockk-agent-api/src/jvmMain`, `modules/mockk-agent/src/jvmMain`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 144 | 6 |
| 1: packages, annotations | 0 | 144 | 5 |
| 2: interfaces, inheritance | 16 | 128 | 3 |
| 3: type checks | 19 | 125 | 3 |
| 4: exceptions | 27 | 117 | 3 |
| 5: objects | 31 | 113 | 2 |
| 6: data, enum, sealed | 42 | 102 | 2 |
| 7: arrays, vararg, spread | 48 | 96 | 2 |
| 8: Char, Float, Byte, Short | 48 | 96 | 2 |
| 9: inliner | 60 | 84 | 2 |
| 10: lambdas with receivers | 60 | 84 | 2 |
| 11: extensions | 62 | 82 | 1 |
| 12: operator conventions | 65 | 79 | 2 |
| 13: identity, class literals, references | 89 | 55 | 1 |
| 14: class delegation | 89 | 55 | 1 |
| 15: secondary constructors | 90 | 54 | 1 |
| 16: local functions | 90 | 54 | 1 |
| 17: expect and actual | 94 | 50 | 1 |
| 18: annotation classes | 102 | 42 | 1 |
| 19: value classes | 102 | 42 | 1 |
| 20: unsigned integers | 102 | 42 | 1 |
| definite assignment | 102 | 42 | 1 |
| 21: inline functions | 103 | 41 | 1 |
| 22: reified type parameters | 103 | 41 | 1 |
| 23: labels, do-while | 103 | 41 | 1 |
| 24: delegated properties | 105 | 39 | 1 |
| 25: object expressions | 109 | 35 | 1 |
| 26: destructuring lambda parameters | 114 | 30 | 1 |
| 42-49: coroutines | 122 | 22 | 1 |
| 52: bitwise operators | 122 | 22 | 1 |
| 53: typealias | 123 | 21 | 1 |
| 54: setters, backing fields | 124 | 20 | 1 |
| 55: lateinit, tailrec | 128 | 16 | 1 |
| 56-57: collection supertypes | 130 | 14 | 1 |
| 58: member extension functions | 130 | 14 | 1 |
| 59: reified member type parameters | 134 | 10 | 1 |
| 60: inner and local classes | 140 | 4 | 1 |
| 61: when guards, anonymous functions, local extension functions | 143 | 1 | 1 |
| 62a: @JvmField and @JvmStatic in objects | 143 | 1 | 1 |
| 62b: generic value classes | 143 | 1 | 1 |
| 63: definitely non-nullable types | 143 | 1 | 1 |
| 70a: inline property accessors | 143 | 1 | 1 |
| 70b: where clauses | 143 | 1 | 1 |
| 70c: fun interfaces | 143 | 1 | 1 |
| delegated extension properties | 143 | 1 | 1 |
| qualified super | 143 | 1 | 1 |
| context parameters of top-level functions | 143 | 1 | 1 |
| also supported | 143 | 1 | 1 |

### Features left, by files blocked

143 of 144 files use no feature that kc doesn't support.

| Feature | Files | Only blocker | Uses |
|:---|---:|---:|---:|
| safe-call assignment | 1 | 1 | 1 |

### Greedy order of the features left

At each step, the feature that unblocks the most files given the ones before it, or, if none unblocks a file, the one that blocks the most files.

| Step | Feature | Files unblocked |
|---:|:---|---:|
| 1 | safe-call assignment | 144 |

### Inline-only functions called

`require`, `check`, `requireNotNull`, `checkNotNull`, `error` and `TODO` are built in; kc inlines the others, since milestone 22 also those with reified type parameters.

| Function | Files | Calls |
|:---|---:|---:|
| `let` | 17 | 21 |
| `isNotEmpty` | 12 | 13 |
| `apply` | 11 | 11 |
| `error` | 7 | 10 |
| `synchronized` | 7 | 26 |
| `List` | 6 | 11 |
| `also` | 5 | 6 |
| `with` | 5 | 6 |
| `suspend` | 4 | 54 |
| `printStackTrace` | 3 | 8 |
| `toBoolean` | 3 | 8 |
| `use` | 3 | 6 |
| `checkNotNull` | 2 | 2 |
| `find` | 2 | 2 |
| `isNullOrEmpty` | 2 | 2 |
| `maxOfOrNull` | 2 | 3 |
| `run` | 2 | 4 |
| `runCatching` | 2 | 2 |
| `takeIf` | 2 | 4 |
| `contentDeepToString` | 1 | 1 |
| `findLast` | 1 | 1 |
| `firstNotNullOfOrNull` | 1 | 1 |
| `getOrElse` | 1 | 1 |
| `ifBlank` | 1 | 1 |
| `intern` | 1 | 2 |
| `matches` | 1 | 2 |
| `require` | 1 | 1 |

### Files

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/Cancelable.kt | 7 | (function without body, interface, package) |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKAgentException.kt | 7 | (inheritance, package, secondary constructor, super) |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKAgentFactory.kt | 13 | (function without body, interface, package, property without initializer) |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKAgentLogFactory.kt | 41 | (function without body, inheritance, inline-only call, interface, modifier companion, modifier override, object expression, package) |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKAgentLogger.kt | 19 | (function without body, interface, package) |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKConstructorProxyMaker.kt | 8 | (function without body, interface, package) |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKInstantiatior.kt | 5 | (function without body, interface, package) |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKInvocationHandler.kt | 14 | (annotation, arrays, class literal, function without body, interface, package) |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKProxyMaker.kt | 11 | (arrays, function without body, interface, package) |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKStaticProxyMaker.kt | 8 | (function without body, interface, package) |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/common/transformation/InlineInstrumentation.kt | 5 | (function without body, interface, package) |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/common/transformation/SubclassInstrumentation.kt | 15 | (arrays, function without body, interface, package) |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/common/transformation/TransformationRequest.kt | 14 | (default value, modifier data, package) |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/common/transformation/TransformationType.kt | 7 | (modifier enum, package) |
| modules/mockk-agent-api/src/jvmMain/kotlin/io/mockk/proxy/common/CancelableResult.kt | 33 | (default value, inheritance, inline-only call, modifier open, modifier override, package, throw) |
| modules/mockk-agent-api/src/jvmMain/kotlin/io/mockk/proxy/common/ProxyInvocationHandler.kt | 92 | (Char, arrays, class literal, identity equality, inheritance, inline-only call, modifier companion, modifier override, nested class, package, spread operator, throw, try) |
| modules/mockk-agent-api/src/jvmMain/kotlin/io/mockk/proxy/common/ProxyMaker.kt | 242 | (Byte and Short, Char, Float, arrays, as, class literal, inheritance, inline-only call, modifier companion, modifier override, named argument, package, throw, try) |
| modules/mockk-agent-api/src/jvmMain/kotlin/io/mockk/proxy/common/transformation/ClassTransformationSpec.kt | 52 | (default value, modifier data, modifier infix, modifier override, nested class, package, property getter) |
| modules/mockk-agent-api/src/jvmMain/kotlin/io/mockk/proxy/common/transformation/ClassTransformationSpecMap.kt | 66 | (inline-only call, modifier operator, named argument, package) |
| modules/mockk-agent-api/src/jvmMain/kotlin/io/mockk/proxy/common/transformation/RetransformInlineInstrumentation.kt | 44 | (function without body, inheritance, inline-only call, modifier abstract, modifier override, modifier protected, package, try) |
| modules/mockk-agent/src/jvmMain/java/io/mockk/proxy/jvm/advice/jvm/MockHandlerMap.kt | 36 | (class delegation, collection supertype, function without body, identity equality, inheritance, interface, modifier companion, modifier override, package, secondary constructor) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/ConstructorProxyMaker.kt | 45 | (inheritance, modifier override, package, throw) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/JvmMockKAgentFactory.kt | 171 | (class literal, inheritance, inline-only call, local class, modifier lateinit, modifier override, package, property without initializer, try) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/ObjenesisInstantiator.kt | 92 | (annotation, as, class literal, inheritance, inline-only call, modifier companion, modifier override, package, spread operator, try) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/ProxyMaker.kt | 230 | (Byte and Short, Char, Float, annotation, arrays, as, class literal, inheritance, inline-only call, modifier companion, modifier override, named argument, package, require, check, error, throw, try) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/StaticProxyMaker.kt | 46 | (inheritance, modifier override, package, throw) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/advice/BaseAdvice.kt | 62 | (arrays, class literal, inheritance, modifier companion, modifier open, modifier override, package) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/advice/Interceptor.kt | 30 | (arrays, inheritance, modifier override, package) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/advice/MethodCall.kt | 19 | (arrays, inheritance, modifier override, package, spread operator, try) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/advice/ProxyAdviceId.kt | 4 | (annotation, modifier annotation, package) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/advice/SelfCallEliminator.kt | 49 | (destructuring lambda parameter, identity equality, inline-only call, modifier inline, object, package, try) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/advice/SelfCallEliminatorCallable.kt | 15 | (inheritance, inline-only call, modifier override, package) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/dispatcher/BootJarLoader.kt | 115 | (Char, annotation, arrays, class literal, extension function, inline-only call, modifier companion, modifier tailrec, package, try) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/transformation/CacheKey.kt | 6 | (modifier data, package) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/transformation/FixParameterNamesVisitor.kt | 95 | (arrays, inheritance, modifier override, nested class, object expression, package, super) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/transformation/InliningClassTransformer.kt | 181 | (annotation, arrays, callable reference, class literal, inheritance, inline-only call, local class, modifier companion, modifier lateinit, modifier override, package, property without initializer, try) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/transformation/JvmInlineInstrumentation.kt | 32 | (arrays, inheritance, inline-only call, modifier override, package, spread operator) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/transformation/SubclassInstrumentation.kt | 111 | (annotation, arrays, as, class literal, local class, modifier companion, modifier lateinit, package, property without initializer, spread operator, try) |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/util/DefaultInterfaceMethodResolver.kt | 40 | (arrays, class literal, inline-only call, modifier companion, package, spread operator) |
| modules/mockk-core/src/commonMain/kotlin/io/mockk/core/ValueClassSupport.kt | 29 | (extension function, extension property, function without body, modifier expect, object, package, property without initializer) |
| modules/mockk-core/src/commonMain/kotlin/io/mockk/core/config/PropertiesLoader.kt | 13 | (function without body, interface, package) |
| modules/mockk-core/src/commonMain/kotlin/io/mockk/core/config/UnifiedPropertiesLoader.kt | 31 | (callable reference, class literal, inheritance, inline-only call, modifier const, modifier override, object, package) |
| modules/mockk-core/src/jvmMain/kotlin/io/mockk/core/ValueClassSupport.kt | 210 | (annotation, as, class literal, default value, extension function, extension property, inline-only call, is, modifier actual, object, package, property getter, property without initializer, throw, try) |
| modules/mockk-dsl/src/commonMain/kotlin/io/mockk/API.kt | 5514 | (Byte and Short, Char, Float, annotation, arrays, as, callable reference, class literal, default value, delegated property, destructuring declaration, extension function, function type with receiver, function without body, identity equality, inheritance, inline-only call, interface, is, modifier abstract, modifier companion, modifier data, modifier enum, modifier infix, modifier inline, modifier noinline, modifier open, modifier operator, modifier override, modifier protected, modifier reified, modifier sealed, modifier suspend, modifier vararg, named argument, nested class, object, object expression, package, property getter, property setter, property without initializer, reified member type parameter, spread operator, suspend function type, throw, try, typealias, when subject variable) |
| modules/mockk-dsl/src/commonMain/kotlin/io/mockk/Answers.kt | 100 | (inheritance, inline-only call, interface, is, modifier companion, modifier data, modifier override, modifier suspend, package, property getter, property without initializer, suspend function type, throw) |
| modules/mockk-dsl/src/commonMain/kotlin/io/mockk/InternalPlatformDsl.kt | 78 | (arrays, extension function, function without body, inline-only call, interface, modifier expect, object, package, property without initializer, suspend function type) |
| modules/mockk-dsl/src/commonMain/kotlin/io/mockk/Matchers.kt | 620 | (annotation, as, class literal, default value, delegated property, extension function, function type with receiver, identity equality, inheritance, inline-only call, is, modifier data, modifier override, named argument, package, property getter, super, throw, try) |
| modules/mockk-dsl/src/commonMain/kotlin/io/mockk/MockKGateway.kt | 328 | (arrays, default value, function type with receiver, function without body, inheritance, interface, is, modifier companion, modifier data, modifier lateinit, modifier sealed, modifier suspend, nested class, package, property getter, property without initializer) |
| modules/mockk-dsl/src/commonMain/kotlin/io/mockk/MockKSettings.kt | 39 | (function without body, modifier enum, modifier expect, object, package, property without initializer, try) |
| modules/mockk-dsl/src/jvmMain/kotlin/io/mockk/InternalPlatformDsl.kt | 269 | (annotation, arrays, as, class literal, destructuring declaration, extension function, identity equality, inheritance, inline-only call, is, local class, modifier actual, modifier companion, modifier override, modifier suspend, object, object expression, package, property getter, spread operator, suspend function type, throw, try, when subject variable) |
| modules/mockk-dsl/src/jvmMain/kotlin/io/mockk/MockKSettings.kt | 59 | (inline-only call, modifier actual, object, package, property getter) |
| modules/mockk/src/commonMain/kotlin/io/mockk/MockK.kt | 857 | (annotation, arrays, as, class literal, default value, function type with receiver, function without body, modifier expect, modifier inline, modifier noinline, modifier reified, modifier suspend, modifier vararg, named argument, object, package, spread operator, try) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/InternalPlatform.kt | 76 | (collection supertype, extension function, function without body, inheritance, inline-only call, interface, modifier expect, modifier inline, nested class, object, package, property without initializer) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/annotations/AdditionalInterface.kt | 3 | (modifier annotation, modifier expect, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/annotations/InjectMockKs.kt | 27 | (annotation, default value, modifier annotation, modifier enum, package, property getter) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/annotations/MockK.kt | 28 | (default value, modifier annotation, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/annotations/RelaxedMockK.kt | 22 | (default value, modifier annotation, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/annotations/SpyK.kt | 26 | (default value, modifier annotation, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/eval/EveryBlockEvaluator.kt | 36 | (annotation, as, function type with receiver, inheritance, modifier override, modifier suspend, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/eval/ExcludeBlockEvaluator.kt | 37 | (function type with receiver, inheritance, modifier override, modifier suspend, package, try) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/eval/RecordedBlockEvaluator.kt | 87 | (callable reference, function type with receiver, inheritance, modifier abstract, modifier protected, modifier suspend, nested class, package, throw, try) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/eval/VerifyBlockEvaluator.kt | 37 | (function type with receiver, inheritance, modifier override, modifier suspend, package, try) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/instantiation/AbstractInstantiator.kt | 29 | (annotation, as, function without body, modifier abstract, modifier companion, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/instantiation/AbstractMockFactory.kt | 148 | (arrays, class literal, default value, function without body, inheritance, modifier abstract, modifier companion, modifier override, named argument, package, throw) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/instantiation/AnyValueGenerator.kt | 40 | (Byte and Short, Char, Float, arrays, class literal, inline-only call, modifier open, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/instantiation/CommonInstanceFactoryRegistry.kt | 23 | (inheritance, inline-only call, modifier override, package, property getter) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/instantiation/CommonMockTypeChecker.kt | 39 | (as, inheritance, is, modifier open, modifier override, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/log/FilterLogger.kt | 81 | (inheritance, modifier override, package, require, check, error) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/log/LogLevel.kt | 10 | (modifier enum, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/log/Logger.kt | 46 | (class literal, function without body, interface, modifier companion, modifier inline, modifier operator, modifier reified, package, reified member type parameter) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/log/NoOpLogger.kt | 38 | (inheritance, modifier override, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/log/SafeLogger.kt | 87 | (inheritance, modifier override, package, require, check, error) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/log/SafeToString.kt | 11 | (modifier operator, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/platform/CommonIdentityHashMapOf.kt | 72 | (annotation, as, inheritance, inline-only call, is, modifier override, package, property getter, throw, try) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/platform/CommonRef.kt | 24 | (class literal, identity equality, inheritance, is, modifier override, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/platform/Disposable.kt | 5 | (function without body, interface, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/AutoHinter.kt | 15 | (modifier open, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/CallRecorderFactories.kt | 33 | (modifier data, package, typealias) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/CallRound.kt | 6 | (modifier data, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/CallRoundBuilder.kt | 58 | (class literal, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/ChainedCallDetector.kt | 188 | (annotation, as, class literal, extension function, inline-only call, is, local extension function, local function, modifier companion, modifier lateinit, named argument, package, property without initializer, throw) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/ChildHinter.kt | 28 | (destructuring lambda parameter, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/CommonCallRecorder.kt | 101 | (inheritance, modifier companion, modifier override, package, try) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/CommonVerificationAcknowledger.kt | 67 | (as, inheritance, modifier override, package, throw) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/PermanentMocker.kt | 154 | (inline-only call, is, named argument, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/SignatureMatcherDetector.kt | 121 | (annotation, as, destructuring lambda parameter, inline-only call, is, local function, package, throw) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/SignatureValueGenerator.kt | 13 | (function without body, interface, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/SignedCall.kt | 14 | (modifier data, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/SignedMatcher.kt | 8 | (modifier data, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/VerificationCallSorter.kt | 34 | (as, callable reference, local function, modifier lateinit, modifier tailrec, package, property without initializer) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/WasNotCalled.kt | 20 | (class literal, named argument, object, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/AnsweringState.kt | 30 | (inheritance, modifier open, modifier override, named argument, package, throw, try) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/CallRecordingState.kt | 53 | (class literal, is, modifier abstract, modifier open, package, throw) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/ExclusionState.kt | 32 | (inheritance, modifier override, package, throw) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/RecordingState.kt | 226 | (Byte and Short, Char, Float, annotation, arrays, callable reference, class literal, inheritance, inline-only call, is, modifier abstract, modifier companion, modifier override, modifier protected, named argument, package, require, check, error, throw) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/SafeLoggingState.kt | 11 | (as, inheritance, modifier override, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/StubbingAwaitingAnswerState.kt | 95 | (class literal, destructuring declaration, extension function, inheritance, is, modifier override, package, require, check, error, throw, try) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/StubbingState.kt | 24 | (inheritance, modifier override, package, throw) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/VerifyingState.kt | 111 | (inheritance, inline-only call, is, modifier companion, modifier override, package, throw) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/restrict/MockkValidator.kt | 37 | (delegated property, package, throw, try) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/restrict/RestrictMockkConfiguration.kt | 55 | (default value, inline-only call, modifier companion, modifier const, package, property without initializer) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/AnswerAnsweringOpportunity.kt | 45 | (inheritance, inline-only call, modifier override, modifier suspend, package, throw) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/CommonClearer.kt | 41 | (arrays, inheritance, modifier override, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/ConstructorStub.kt | 129 | (arrays, inheritance, inline-only call, modifier override, modifier vararg, package, property getter) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/MockKStub.kt | 355 | (arrays, callable reference, default value, extension function, identity equality, inheritance, inline-only call, local extension function, local function, modifier companion, modifier data, modifier inline, modifier lateinit, modifier open, modifier override, modifier protected, named argument, nested class, package, property without initializer, throw) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/MockType.kt | 10 | (modifier enum, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/SpyKStub.kt | 14 | (inheritance, modifier override, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/Stub.kt | 62 | (arrays, function without body, inheritance, interface, package, property without initializer) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/StubGatewayAccess.kt | 14 | (default value, modifier data, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/StubRepository.kt | 68 | (as, inline-only call, modifier operator, package, property getter, throw) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/AllCallsCallVerifier.kt | 46 | (inheritance, inline-only call, modifier override, package, super) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/LCSMatchingAlgo.kt | 81 | (Char, arrays, default value, destructuring declaration, local function, modifier tailrec, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/OrderedCallVerifier.kt | 47 | (inheritance, modifier override, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/SequenceCallVerifier.kt | 56 | (destructuring declaration, inheritance, modifier override, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/TimeoutVerifier.kt | 55 | (extension function, inheritance, is, modifier override, package, try) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/UnorderedCallVerifier.kt | 221 | (as, callable reference, destructuring declaration, inheritance, is, modifier open, modifier override, package, throw, when subject variable) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/VerificationHelpers.kt | 107 | (annotation, default value, extension function, function type with receiver, inline-only call, local extension function, local function, object, package) |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/VerificationReportFormatter.kt | 17 | (object, package) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/MockK.kt | 108 | (annotation, arrays, callable reference, default value, extension property, modifier actual, modifier inline, modifier vararg, named argument, object, package, property getter, require, check, error, spread operator) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/InternalPlatform.kt | 199 | (arrays, class literal, default value, extension function, inheritance, inline-only call, is, local function, modifier actual, modifier inline, modifier override, modifier reified, object, object expression, package, property getter, reified member type parameter, throw, try) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/JvmMockKGateway.kt | 233 | (callable reference, class literal, inheritance, modifier companion, modifier override, named argument, object expression, package, property backing field, property getter, property setter, property without initializer) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/JvmMultiNotifier.kt | 89 | (inheritance, inline-only call, modifier inner, modifier override, nested class, package) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/annotations/AdditionalInterface.kt | 7 | (modifier actual, modifier annotation, package) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/annotations/InjectionHelpers.kt | 84 | (annotation, as, class literal, extension function, is, object, package, throw, try) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/annotations/JvmMockInitializer.kt | 375 | (annotation, arrays, as, class literal, default value, destructuring lambda parameter, extension function, inheritance, inline-only call, is, modifier companion, modifier inline, modifier override, modifier reified, named argument, package, reified member type parameter, throw, try) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/annotations/MockInjector.kt | 160 | (as, class literal, extension function, inline-only call, is, modifier companion, package, throw) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmAnyValueGenerator.kt | 44 | (Float, class literal, inheritance, inline-only call, modifier open, modifier override, package, super) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmConstructorMockFactory.kt | 315 | (annotation, arrays, as, callable reference, class literal, default value, inheritance, inline-only call, modifier inner, modifier override, nested class, package, property getter, qualified this, throw) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmInstantiator.kt | 22 | (inheritance, modifier companion, modifier override, package) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmMockFactory.kt | 69 | safe-call assignment (annotation, arrays, as, callable reference, inheritance, modifier override, named argument, package, throw, try) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmMockFactoryHelper.kt | 258 | (arrays, as, class literal, extension function, identity equality, inheritance, inline-only call, is, modifier inline, modifier override, object, object expression, package, qualified this, throw, try) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmMockTypeChecker.kt | 20 | (inheritance, is, modifier override, package, super) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmObjectMockFactory.kt | 104 | (arrays, callable reference, class literal, inheritance, inline-only call, modifier companion, modifier override, package, throw, try) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmStaticMockFactory.kt | 85 | (callable reference, inheritance, inline-only call, modifier companion, modifier override, package, throw, try) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/RefCounterMap.kt | 21 | (inline-only call, package) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/log/JULLogger.kt | 47 | (inheritance, modifier override, package) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/log/JvmLogging.kt | 50 | (class literal, extension function, inheritance, is, modifier override, object, object expression, package, qualified this, throw, try) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/log/Slf4jLogger.kt | 45 | (inheritance, modifier override, package, require, check, error) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/platform/JvmWeakConcurrentMap.kt | 155 | (annotation, as, identity equality, inheritance, inline-only call, is, modifier override, nested class, package, property getter, property without initializer, throw, try) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/recording/JvmAutoHinter.kt | 64 | (destructuring lambda parameter, inheritance, inline-only call, modifier companion, modifier override, package, throw, try) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/recording/JvmSignatureValueGenerator.kt | 109 | (Byte and Short, Float, annotation, as, class literal, inheritance, inline-only call, modifier companion, modifier const, modifier override, named argument, package, require, check, error) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/verify/JvmFunctionScopedVerificationAcknowledger.kt | 142 | (arrays, as, callable reference, default value, destructuring lambda parameter, function without body, inheritance, inline-only call, interface, modifier override, named argument, object, package, require, check, error, throw) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/junit4/MockKRule.kt | 47 | (inheritance, modifier override, package, super) |
| modules/mockk/src/jvmMain/kotlin/io/mockk/junit5/MockKExtension.kt | 287 | (annotation, arrays, as, class literal, default value, extension function, extension property, inheritance, inline-only call, is, modifier annotation, modifier companion, modifier const, modifier override, named argument, nested class, package, property getter, spread operator, throw, try) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 117 files, 13698 lines

Directories: `modules/mockk/src/commonTest`, `modules/mockk/src/jvmTest`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 117 | 6 |
| 1: packages, annotations | 5 | 112 | 4 |
| 2: interfaces, inheritance | 5 | 112 | 3 |
| 3: type checks | 5 | 112 | 3 |
| 4: exceptions | 5 | 112 | 3 |
| 5: objects | 17 | 100 | 2 |
| 6: data, enum, sealed | 33 | 84 | 2 |
| 7: arrays, vararg, spread | 35 | 82 | 2 |
| 8: Char, Float, Byte, Short | 37 | 80 | 1 |
| 9: inliner | 44 | 73 | 1 |
| 10: lambdas with receivers | 45 | 72 | 1 |
| 11: extensions | 50 | 67 | 1 |
| 12: operator conventions | 51 | 66 | 2 |
| 13: identity, class literals, references | 68 | 49 | 1 |
| 14: class delegation | 68 | 49 | 1 |
| 15: secondary constructors | 69 | 48 | 1 |
| 16: local functions | 69 | 48 | 1 |
| 17: expect and actual | 69 | 48 | 1 |
| 18: annotation classes | 69 | 48 | 1 |
| 19: value classes | 70 | 47 | 1 |
| 20: unsigned integers | 70 | 47 | 1 |
| definite assignment | 70 | 47 | 1 |
| 21: inline functions | 71 | 46 | 1 |
| 22: reified type parameters | 71 | 46 | 1 |
| 23: labels, do-while | 71 | 46 | 1 |
| 24: delegated properties | 71 | 46 | 1 |
| 25: object expressions | 72 | 45 | 1 |
| 26: destructuring lambda parameters | 74 | 43 | 1 |
| 42-49: coroutines | 82 | 35 | 1 |
| 52: bitwise operators | 82 | 35 | 1 |
| 53: typealias | 82 | 35 | 1 |
| 54: setters, backing fields | 83 | 34 | 1 |
| 55: lateinit, tailrec | 109 | 8 | 1 |
| 56-57: collection supertypes | 109 | 8 | 1 |
| 58: member extension functions | 109 | 8 | 1 |
| 59: reified member type parameters | 111 | 6 | 1 |
| 60: inner and local classes | 117 | 0 | – |
| 61: when guards, anonymous functions, local extension functions | 117 | 0 | – |
| 62a: @JvmField and @JvmStatic in objects | 117 | 0 | – |
| 62b: generic value classes | 117 | 0 | – |
| 63: definitely non-nullable types | 117 | 0 | – |
| 70a: inline property accessors | 117 | 0 | – |
| 70b: where clauses | 117 | 0 | – |
| 70c: fun interfaces | 117 | 0 | – |
| delegated extension properties | 117 | 0 | – |
| qualified super | 117 | 0 | – |
| context parameters of top-level functions | 117 | 0 | – |
| also supported | 117 | 0 | – |

### Features left, by files blocked

117 of 117 files use no feature that kc doesn't support.

| Feature | Files | Only blocker | Uses |
|:---|---:|---:|---:|

### Greedy order of the features left

At each step, the feature that unblocks the most files given the ones before it, or, if none unblocks a file, the one that blocks the most files.

| Step | Feature | Files unblocked |
|---:|:---|---:|

### Inline-only functions called

`require`, `check`, `requireNotNull`, `checkNotNull`, `error` and `TODO` are built in; kc inlines the others, since milestone 22 also those with reified type parameters.

| Function | Files | Calls |
|:---|---:|---:|
| `error` | 4 | 10 |
| `apply` | 3 | 7 |
| `assert` | 3 | 10 |
| `List` | 2 | 3 |
| `also` | 2 | 4 |
| `format` | 2 | 4 |
| `suspend` | 2 | 4 |
| `TODO` | 1 | 1 |
| `find` | 1 | 4 |
| `isNotEmpty` | 1 | 1 |
| `require` | 1 | 1 |
| `run` | 1 | 18 |
| `runCatching` | 1 | 1 |
| `sumOf` | 1 | 3 |
| `with` | 1 | 2 |

### Files

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| modules/mockk/src/commonTest/kotlin/io/mockk/ManyAnswersAnswerTest.kt | 48 | (annotation, arrays, extension function, inline-only call, modifier vararg, package, spread operator) |
| modules/mockk/src/commonTest/kotlin/io/mockk/ReflectiveMockTests.kt | 106 | (annotation, arrays, as, class literal, function type with receiver, function without body, interface, modifier inline, modifier noinline, modifier reified, nested class, package, reified member type parameter, spread operator) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/eval/RecordedBlockEvaluatorTest.kt | 102 | (annotation, function type with receiver, inheritance, modifier lateinit, modifier suspend, named argument, object expression, package, property without initializer) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/instantiation/AbstractInstantiatorTest.kt | 47 | (annotation, class literal, inheritance, modifier lateinit, modifier override, named argument, object expression, package, property without initializer, throw) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/instantiation/AbstractMockFactoryTest.kt | 119 | (annotation, arrays, class literal, inheritance, modifier inner, modifier lateinit, modifier override, named argument, nested class, package, property without initializer, throw) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/instantiation/AnyValueGeneratorTest.kt | 122 | (Byte and Short, Char, Float, annotation, arrays, as, class literal, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/instantiation/CommonInstanceFactoryRegistryTest.kt | 37 | (annotation, inheritance, modifier lateinit, modifier override, object expression, package, property without initializer) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/log/FilterLoggerTest.kt | 234 | (annotation, modifier lateinit, named argument, package, property without initializer, require, check, error) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/log/NoOpLoggerTest.kt | 110 | (annotation, modifier lateinit, named argument, package, property without initializer, require, check, error) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/platform/CommonRefTest.kt | 67 | (annotation, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/AutoHinterTest.kt | 20 | (annotation, named argument, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/CallRoundBuilderTest.kt | 41 | (annotation, class literal, named argument, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/ChainedCallDetectorTest.kt | 63 | (annotation, named argument, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/ChildHinterTest.kt | 36 | (annotation, class literal, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/CommonCallRecorderTest.kt | 139 | (annotation, modifier lateinit, named argument, package, property without initializer) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/states/AnsweringStateTest.kt | 69 | (annotation, modifier lateinit, named argument, package, property without initializer) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/states/CallRecordingStateTest.kt | 43 | (annotation, inheritance, modifier lateinit, named argument, object expression, package, property without initializer) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/states/RecordingStateTest.kt | 46 | (annotation, inheritance, modifier lateinit, named argument, object expression, package, property without initializer) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/states/StubbingAwaitingAnswerStateTest.kt | 76 | (annotation, class literal, modifier lateinit, named argument, package, property without initializer, try) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/states/StubbingStateTest.kt | 42 | (annotation, modifier lateinit, named argument, package, property without initializer) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/states/VerifyingStateTest.kt | 131 | (annotation, modifier lateinit, named argument, package, property without initializer) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/stub/MockKStubTest.kt | 65 | (annotation, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/stub/StubRepositoryTest.kt | 63 | (annotation, as, function without body, inline-only call, interface, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/AdditionalAnswerTest.kt | 212 | (annotation, class literal, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/AnnotationsTest.kt | 152 | (annotation, class literal, modifier lateinit, nested class, package, property without initializer) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/AnswersTest.kt | 125 | (annotation, default value, named argument, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ArraysTest.kt | 243 | (Byte and Short, Char, Float, annotation, arrays, as, modifier data, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/BackingFieldTest.kt | 66 | (annotation, class literal, modifier value, named argument, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/CaptureSubclassVerificationTest.kt | 102 | (annotation, function without body, inheritance, interface, named argument, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/CapturingGenericArgumentsTest.kt | 34 | (annotation, extension function, function without body, interface, modifier inline, modifier reified, nested class, package, reified member type parameter) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/CapturingTest.kt | 269 | (annotation, inline-only call, modifier open, modifier suspend, nested class, package, throw) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ChainedCallsMatchingTest.kt | 112 | (annotation, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ClearMocksTest.kt | 193 | (annotation, class literal, named argument, nested class, package, require, check, error) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/CoJustAwaitTest.kt | 82 | (annotation, class literal, default value, modifier suspend, named argument, nested class, package, require, check, error) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/CoVerifyTest.kt | 179 | (annotation, modifier suspend, named argument, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ConstructorMockTest.kt | 307 | (annotation, class literal, default value, function without body, interface, modifier data, named argument, nested class, package, secondary constructor) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/CoroutinesTest.kt | 97 | (annotation, default value, inline-only call, modifier suspend, named argument, nested class, package, suspend function type) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/EnumTests.kt | 48 | (annotation, modifier enum, modifier lateinit, nested class, package, property without initializer) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ExtensionFunctionsTest.kt | 74 | (annotation, extension function, inline-only call, modifier data, object, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/HierarchicalMockingTest.kt | 43 | (annotation, function without body, interface, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/InitializationBlockTest.kt | 37 | (annotation, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/InjectMocksTest.kt | 392 | (annotation, function without body, inheritance, interface, modifier abstract, modifier lateinit, modifier override, named argument, nested class, package, property without initializer) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/IntTest.kt | 55 | (annotation, function without body, modifier abstract, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/LambdaTest.kt | 32 | (annotation, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/MapsTest.kt | 50 | (annotation, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/MatcherTest.kt | 409 | (annotation, class literal, inheritance, interface, is, modifier data, modifier lateinit, modifier open, named argument, nested class, object expression, package, property without initializer) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/MockTypesTest.kt | 79 | (annotation, class literal, modifier enum, named argument, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/MockkClassTest.kt | 47 | (annotation, class literal, nested class, package, throw) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/NullsTest.kt | 55 | (annotation, inheritance, interface, is, modifier data, modifier lateinit, nested class, package, property without initializer) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ObjectMockTest.kt | 53 | (annotation, named argument, object, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ParametersWhitDefaultTest.kt | 31 | (annotation, default value, function without body, interface, modifier companion, modifier const, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/PartialArgumentMatchingTest.kt | 195 | (Byte and Short, Char, Float, annotation, default value, extension function, modifier data, named argument, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/PrivateFunctionsTest.kt | 158 | (annotation, as, named argument, nested class, object, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/PrivatePropertiesTest.kt | 56 | (annotation, modifier data, modifier protected, modifier value, named argument, nested class, package, property backing field, property getter, property setter) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/RelaxedMockingTest.kt | 76 | (annotation, named argument, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/SealedClassMockingTest.kt | 25 | (annotation, inheritance, modifier abstract, modifier data, modifier override, modifier sealed, nested class, package, property without initializer) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/SealedClassTest.kt | 69 | (annotation, function without body, inheritance, inline-only call, interface, modifier companion, modifier data, modifier override, modifier sealed, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/SealedInterfaceTest.kt | 69 | (annotation, function without body, inheritance, inline-only call, interface, modifier companion, modifier data, modifier override, modifier sealed, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/SetsTest.kt | 37 | (annotation, function without body, interface, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/SpyGenericClassTest.kt | 45 | (annotation, function without body, inheritance, interface, modifier abstract, modifier override, named argument, nested class, package, super, throw) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/SpyTest.kt | 151 | (annotation, arrays, default value, inheritance, modifier open, modifier override, nested class, package, super) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/StaticMockTest.kt | 52 | (annotation, extension function, modifier infix, named argument, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ValueClassTest.kt | 1042 | (annotation, as, function type with receiver, function without body, inline-only call, interface, modifier companion, modifier suspend, modifier value, nested class, package, require, check, error, unsigned integers) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/VarargsTest.kt | 982 | (Byte and Short, Char, Float, annotation, default value, inline-only call, modifier data, modifier vararg, named argument, nested class, package, spread operator) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/VerificationErrorsTest.kt | 265 | (Byte and Short, Char, Float, annotation, default value, function without body, modifier abstract, modifier data, modifier vararg, named argument, nested class, package, try) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/VerifyAtLeastAtMostExactlyTest.kt | 306 | (annotation, class literal, function without body, interface, named argument, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/VerifyTest.kt | 202 | (annotation, modifier override, named argument, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/WasNotCalledTest.kt | 43 | (annotation, default value, nested class, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/restrict/RestrictMockkTest.kt | 313 | (annotation, inheritance, modifier open, named argument, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/restrict/TestPropertiesLoader.kt | 16 | (default value, destructuring lambda parameter, inheritance, modifier override, package) |
| modules/mockk/src/commonTest/kotlin/io/mockk/util/ArrayAsserts.kt | 79 | (Float, arrays, destructuring lambda parameter, modifier actual, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/AnyMatcherTypeTest.kt | 71 | (annotation, class literal, function without body, interface, nested class, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/CapturingSlotTest.kt | 41 | (annotation, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/impl/JvmMockKGatewayTest.kt | 10 | (annotation, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/impl/annotations/MockInjectorTest.kt | 599 | (annotation, class literal, default value, inheritance, interface, local class, modifier lateinit, named argument, nested class, package, property without initializer, secondary constructor) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/impl/instantiation/JvmMockFactoryHelperTest.kt | 57 | (annotation, arrays, class literal, modifier vararg, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/impl/recording/JvmAutoHinterTest.kt | 67 | (annotation, inline-only call, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/impl/recording/JvmSignatureValueGeneratorTest.kt | 86 | (annotation, class literal, inheritance, modifier companion, modifier const, modifier override, object expression, package, try) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/BackingFieldsTest.kt | 49 | (annotation, inline-only call, nested class, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/CallOriginalOnDefaultInterfaceMethodTest.kt | 48 | (annotation, function without body, interface, named argument, nested class, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/CallableTest.kt | 31 | (annotation, inline-only call, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/CollectionChainStubbingTest.kt | 67 | (annotation, interface, nested class, package, property without initializer) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/CollectionsReturnOnRelaxedMockTest.kt | 77 | (annotation, modifier lateinit, named argument, nested class, package, property without initializer) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/ConcurrentStubInvocationTest.kt | 50 | (annotation, local class, named argument, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/CoroutineTest.kt | 223 | (annotation, class literal, function without body, inline-only call, interface, local function, modifier data, modifier suspend, nested class, package, suspend function type, try) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/InlineMemberFunctionFailFastTest.kt | 57 | (annotation, as, class literal, modifier inline, package, require, check, error) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/JavaClassParamTest.kt | 25 | (annotation, class literal, nested class, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/JvmConstructorOverridenTest.kt | 56 | (annotation, class literal, identity equality, inline-only call, modifier data, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/JvmVarargsTest.kt | 36 | (annotation, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/LocaleTest.kt | 26 | (annotation, function without body, interface, nested class, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/MockKExtensionOnNestedClassTest.kt | 28 | (annotation, class literal, modifier inner, modifier open, named argument, nested class, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/NullableValueTest.kt | 86 | (annotation, inheritance, local class, modifier override, modifier suspend, named argument, nested class, package, super) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/PackagePrivateParentTest.kt | 17 | (annotation, inheritance, named argument, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/ParallelTest.kt | 101 | (annotation, bitwise operators, class literal, inline-only call, modifier companion, modifier data, modifier lateinit, nested class, package, property without initializer, try) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/PrivateParentMethodTest.kt | 71 | (annotation, inheritance, modifier open, modifier override, named argument, nested class, package, qualified this) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/ProxyWeakReferenceTest.kt | 26 | (annotation, arrays, nested class, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/RaceConditionInMockCreationTest.kt | 49 | (annotation, nested class, package, try) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/RelaxedSuspendingMockingTest.kt | 57 | (annotation, modifier suspend, named argument, nested class, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/RurfTest.kt | 76 | (annotation, modifier lateinit, named argument, package, property without initializer) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/SafeCallSuspendingTest.kt | 27 | (annotation, modifier suspend, named argument, nested class, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/StackTraceLineNumberTest.kt | 74 | (annotation, arrays, extension function, package, throw, try) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/StaticMockkTest.kt | 225 | (annotation, callable reference, class literal, extension function, extension property, modifier infix, named argument, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/ThrowExceptionOnNonMockedClassTest.kt | 25 | (annotation, nested class, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/TimeoutTest.kt | 128 | (annotation, class literal, inline-only call, modifier lateinit, named argument, nested class, package, property without initializer) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/VerificationAcknowledgeTest.kt | 667 | (annotation, class literal, inline-only call, named argument, nested class, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/VoidReturnTest.kt | 25 | (annotation, function without body, modifier abstract, modifier operator, nested class, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit4/MockKRuleTest.kt | 85 | (annotation, is, modifier enum, modifier lateinit, named argument, nested class, package, property without initializer) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit5/ExtensionAndInitMocksTest.kt | 64 | (annotation, as, class literal, is, modifier lateinit, modifier open, nested class, package, property without initializer) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit5/MockExtensionMetaAnnotations.kt | 35 | (annotation, class literal, local class, modifier annotation, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit5/MockKExtensionAfterAllTestTest.kt | 64 | (annotation, class literal, inheritance, modifier abstract, modifier open, modifier override, named argument, nested class, object, package, super) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit5/MockKExtensionConstructorBindingTest.kt | 79 | (annotation, class literal, modifier data, modifier enum, nested class, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit5/MockKExtensionRequireParallelTestingTest.kt | 62 | (annotation, class literal, function without body, inline-only call, interface, modifier companion, modifier lateinit, package, property without initializer) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit5/MockKExtensionTest.kt | 110 | (annotation, class literal, is, modifier enum, modifier lateinit, named argument, nested class, package, property without initializer) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit5/MockKExtensionWithoutRequireParallelTestingTest.kt | 59 | (annotation, class literal, inline-only call, modifier companion, modifier lateinit, package, property without initializer) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/jvm/ClassLoadingStrategyTest.kt | 19 | (annotation, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/jvm/HashMapMockTest.kt | 55 | (annotation, named argument, package) |
| modules/mockk/src/jvmTest/kotlin/io/mockk/jvm/JvmAnyValueGeneratorTest.kt | 123 | (Byte and Short, Char, Float, annotation, arrays, as, class literal, package) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

