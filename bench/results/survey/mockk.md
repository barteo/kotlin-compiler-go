# Survey: mockk v1.14.11

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 144 files, 17302 lines

Directories: `modules/mockk/src/commonMain`, `modules/mockk/src/jvmMain`, `modules/mockk-dsl/src/commonMain`, `modules/mockk-dsl/src/jvmMain`, `modules/mockk-core/src/commonMain`, `modules/mockk-core/src/jvmMain`, `modules/mockk-agent-api/src/commonMain`, `modules/mockk-agent-api/src/jvmMain`, `modules/mockk-agent/src/jvmMain`

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

`require`, `check`, `requireNotNull`, `checkNotNull`, `error` and `TODO` are built in; kc inlines the others, also those with reified type parameters.

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

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/Cancelable.kt | 7 |  |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKAgentException.kt | 7 |  |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKAgentFactory.kt | 13 |  |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKAgentLogFactory.kt | 41 |  |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKAgentLogger.kt | 19 |  |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKConstructorProxyMaker.kt | 8 |  |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKInstantiatior.kt | 5 |  |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKInvocationHandler.kt | 14 |  |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKProxyMaker.kt | 11 |  |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/MockKStaticProxyMaker.kt | 8 |  |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/common/transformation/InlineInstrumentation.kt | 5 |  |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/common/transformation/SubclassInstrumentation.kt | 15 |  |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/common/transformation/TransformationRequest.kt | 14 |  |
| modules/mockk-agent-api/src/commonMain/kotlin/io/mockk/proxy/common/transformation/TransformationType.kt | 7 |  |
| modules/mockk-agent-api/src/jvmMain/kotlin/io/mockk/proxy/common/CancelableResult.kt | 33 |  |
| modules/mockk-agent-api/src/jvmMain/kotlin/io/mockk/proxy/common/ProxyInvocationHandler.kt | 92 |  |
| modules/mockk-agent-api/src/jvmMain/kotlin/io/mockk/proxy/common/ProxyMaker.kt | 242 |  |
| modules/mockk-agent-api/src/jvmMain/kotlin/io/mockk/proxy/common/transformation/ClassTransformationSpec.kt | 52 |  |
| modules/mockk-agent-api/src/jvmMain/kotlin/io/mockk/proxy/common/transformation/ClassTransformationSpecMap.kt | 66 |  |
| modules/mockk-agent-api/src/jvmMain/kotlin/io/mockk/proxy/common/transformation/RetransformInlineInstrumentation.kt | 44 |  |
| modules/mockk-agent/src/jvmMain/java/io/mockk/proxy/jvm/advice/jvm/MockHandlerMap.kt | 36 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/ConstructorProxyMaker.kt | 45 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/JvmMockKAgentFactory.kt | 171 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/ObjenesisInstantiator.kt | 92 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/ProxyMaker.kt | 230 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/StaticProxyMaker.kt | 46 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/advice/BaseAdvice.kt | 62 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/advice/Interceptor.kt | 30 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/advice/MethodCall.kt | 19 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/advice/ProxyAdviceId.kt | 4 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/advice/SelfCallEliminator.kt | 49 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/advice/SelfCallEliminatorCallable.kt | 15 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/dispatcher/BootJarLoader.kt | 115 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/transformation/CacheKey.kt | 6 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/transformation/FixParameterNamesVisitor.kt | 95 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/transformation/InliningClassTransformer.kt | 181 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/transformation/JvmInlineInstrumentation.kt | 32 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/transformation/SubclassInstrumentation.kt | 111 |  |
| modules/mockk-agent/src/jvmMain/kotlin/io/mockk/proxy/jvm/util/DefaultInterfaceMethodResolver.kt | 40 |  |
| modules/mockk-core/src/commonMain/kotlin/io/mockk/core/ValueClassSupport.kt | 29 |  |
| modules/mockk-core/src/commonMain/kotlin/io/mockk/core/config/PropertiesLoader.kt | 13 |  |
| modules/mockk-core/src/commonMain/kotlin/io/mockk/core/config/UnifiedPropertiesLoader.kt | 31 |  |
| modules/mockk-core/src/jvmMain/kotlin/io/mockk/core/ValueClassSupport.kt | 210 |  |
| modules/mockk-dsl/src/commonMain/kotlin/io/mockk/API.kt | 5514 |  |
| modules/mockk-dsl/src/commonMain/kotlin/io/mockk/Answers.kt | 100 |  |
| modules/mockk-dsl/src/commonMain/kotlin/io/mockk/InternalPlatformDsl.kt | 78 |  |
| modules/mockk-dsl/src/commonMain/kotlin/io/mockk/Matchers.kt | 620 |  |
| modules/mockk-dsl/src/commonMain/kotlin/io/mockk/MockKGateway.kt | 328 |  |
| modules/mockk-dsl/src/commonMain/kotlin/io/mockk/MockKSettings.kt | 39 |  |
| modules/mockk-dsl/src/jvmMain/kotlin/io/mockk/InternalPlatformDsl.kt | 269 |  |
| modules/mockk-dsl/src/jvmMain/kotlin/io/mockk/MockKSettings.kt | 59 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/MockK.kt | 857 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/InternalPlatform.kt | 76 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/annotations/AdditionalInterface.kt | 3 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/annotations/InjectMockKs.kt | 27 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/annotations/MockK.kt | 28 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/annotations/RelaxedMockK.kt | 22 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/annotations/SpyK.kt | 26 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/eval/EveryBlockEvaluator.kt | 36 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/eval/ExcludeBlockEvaluator.kt | 37 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/eval/RecordedBlockEvaluator.kt | 87 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/eval/VerifyBlockEvaluator.kt | 37 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/instantiation/AbstractInstantiator.kt | 29 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/instantiation/AbstractMockFactory.kt | 148 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/instantiation/AnyValueGenerator.kt | 40 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/instantiation/CommonInstanceFactoryRegistry.kt | 23 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/instantiation/CommonMockTypeChecker.kt | 39 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/log/FilterLogger.kt | 81 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/log/LogLevel.kt | 10 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/log/Logger.kt | 46 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/log/NoOpLogger.kt | 38 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/log/SafeLogger.kt | 87 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/log/SafeToString.kt | 11 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/platform/CommonIdentityHashMapOf.kt | 72 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/platform/CommonRef.kt | 24 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/platform/Disposable.kt | 5 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/AutoHinter.kt | 15 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/CallRecorderFactories.kt | 33 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/CallRound.kt | 6 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/CallRoundBuilder.kt | 58 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/ChainedCallDetector.kt | 188 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/ChildHinter.kt | 28 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/CommonCallRecorder.kt | 101 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/CommonVerificationAcknowledger.kt | 67 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/PermanentMocker.kt | 154 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/SignatureMatcherDetector.kt | 121 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/SignatureValueGenerator.kt | 13 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/SignedCall.kt | 14 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/SignedMatcher.kt | 8 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/VerificationCallSorter.kt | 34 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/WasNotCalled.kt | 20 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/AnsweringState.kt | 30 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/CallRecordingState.kt | 53 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/ExclusionState.kt | 32 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/RecordingState.kt | 226 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/SafeLoggingState.kt | 11 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/StubbingAwaitingAnswerState.kt | 95 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/StubbingState.kt | 24 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/recording/states/VerifyingState.kt | 111 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/restrict/MockkValidator.kt | 37 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/restrict/RestrictMockkConfiguration.kt | 55 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/AnswerAnsweringOpportunity.kt | 45 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/CommonClearer.kt | 41 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/ConstructorStub.kt | 129 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/MockKStub.kt | 355 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/MockType.kt | 10 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/SpyKStub.kt | 14 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/Stub.kt | 62 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/StubGatewayAccess.kt | 14 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/stub/StubRepository.kt | 68 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/AllCallsCallVerifier.kt | 46 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/LCSMatchingAlgo.kt | 81 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/OrderedCallVerifier.kt | 47 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/SequenceCallVerifier.kt | 56 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/TimeoutVerifier.kt | 55 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/UnorderedCallVerifier.kt | 221 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/VerificationHelpers.kt | 107 |  |
| modules/mockk/src/commonMain/kotlin/io/mockk/impl/verify/VerificationReportFormatter.kt | 17 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/MockK.kt | 108 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/InternalPlatform.kt | 199 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/JvmMockKGateway.kt | 233 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/JvmMultiNotifier.kt | 89 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/annotations/AdditionalInterface.kt | 7 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/annotations/InjectionHelpers.kt | 84 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/annotations/JvmMockInitializer.kt | 375 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/annotations/MockInjector.kt | 160 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmAnyValueGenerator.kt | 44 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmConstructorMockFactory.kt | 315 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmInstantiator.kt | 22 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmMockFactory.kt | 69 | safe-call assignment |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmMockFactoryHelper.kt | 258 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmMockTypeChecker.kt | 20 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmObjectMockFactory.kt | 104 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/JvmStaticMockFactory.kt | 85 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/instantiation/RefCounterMap.kt | 21 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/log/JULLogger.kt | 47 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/log/JvmLogging.kt | 50 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/log/Slf4jLogger.kt | 45 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/platform/JvmWeakConcurrentMap.kt | 155 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/recording/JvmAutoHinter.kt | 64 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/recording/JvmSignatureValueGenerator.kt | 109 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/impl/verify/JvmFunctionScopedVerificationAcknowledger.kt | 142 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/junit4/MockKRule.kt | 47 |  |
| modules/mockk/src/jvmMain/kotlin/io/mockk/junit5/MockKExtension.kt | 287 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 117 files, 13698 lines

Directories: `modules/mockk/src/commonTest`, `modules/mockk/src/jvmTest`

### Features left, by files blocked

117 of 117 files use no feature that kc doesn't support.

| Feature | Files | Only blocker | Uses |
|:---|---:|---:|---:|

### Greedy order of the features left

At each step, the feature that unblocks the most files given the ones before it, or, if none unblocks a file, the one that blocks the most files.

| Step | Feature | Files unblocked |
|---:|:---|---:|

### Inline-only functions called

`require`, `check`, `requireNotNull`, `checkNotNull`, `error` and `TODO` are built in; kc inlines the others, also those with reified type parameters.

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

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| modules/mockk/src/commonTest/kotlin/io/mockk/ManyAnswersAnswerTest.kt | 48 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/ReflectiveMockTests.kt | 106 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/eval/RecordedBlockEvaluatorTest.kt | 102 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/instantiation/AbstractInstantiatorTest.kt | 47 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/instantiation/AbstractMockFactoryTest.kt | 119 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/instantiation/AnyValueGeneratorTest.kt | 122 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/instantiation/CommonInstanceFactoryRegistryTest.kt | 37 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/log/FilterLoggerTest.kt | 234 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/log/NoOpLoggerTest.kt | 110 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/platform/CommonRefTest.kt | 67 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/AutoHinterTest.kt | 20 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/CallRoundBuilderTest.kt | 41 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/ChainedCallDetectorTest.kt | 63 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/ChildHinterTest.kt | 36 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/CommonCallRecorderTest.kt | 139 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/states/AnsweringStateTest.kt | 69 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/states/CallRecordingStateTest.kt | 43 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/states/RecordingStateTest.kt | 46 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/states/StubbingAwaitingAnswerStateTest.kt | 76 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/states/StubbingStateTest.kt | 42 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/recording/states/VerifyingStateTest.kt | 131 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/stub/MockKStubTest.kt | 65 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/impl/stub/StubRepositoryTest.kt | 63 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/AdditionalAnswerTest.kt | 212 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/AnnotationsTest.kt | 152 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/AnswersTest.kt | 125 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ArraysTest.kt | 243 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/BackingFieldTest.kt | 66 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/CaptureSubclassVerificationTest.kt | 102 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/CapturingGenericArgumentsTest.kt | 34 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/CapturingTest.kt | 269 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ChainedCallsMatchingTest.kt | 112 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ClearMocksTest.kt | 193 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/CoJustAwaitTest.kt | 82 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/CoVerifyTest.kt | 179 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ConstructorMockTest.kt | 307 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/CoroutinesTest.kt | 97 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/EnumTests.kt | 48 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ExtensionFunctionsTest.kt | 74 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/HierarchicalMockingTest.kt | 43 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/InitializationBlockTest.kt | 37 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/InjectMocksTest.kt | 392 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/IntTest.kt | 55 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/LambdaTest.kt | 32 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/MapsTest.kt | 50 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/MatcherTest.kt | 409 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/MockTypesTest.kt | 79 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/MockkClassTest.kt | 47 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/NullsTest.kt | 55 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ObjectMockTest.kt | 53 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ParametersWhitDefaultTest.kt | 31 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/PartialArgumentMatchingTest.kt | 195 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/PrivateFunctionsTest.kt | 158 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/PrivatePropertiesTest.kt | 56 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/RelaxedMockingTest.kt | 76 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/SealedClassMockingTest.kt | 25 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/SealedClassTest.kt | 69 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/SealedInterfaceTest.kt | 69 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/SetsTest.kt | 37 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/SpyGenericClassTest.kt | 45 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/SpyTest.kt | 151 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/StaticMockTest.kt | 52 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/ValueClassTest.kt | 1042 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/VarargsTest.kt | 982 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/VerificationErrorsTest.kt | 265 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/VerifyAtLeastAtMostExactlyTest.kt | 306 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/VerifyTest.kt | 202 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/it/WasNotCalledTest.kt | 43 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/restrict/RestrictMockkTest.kt | 313 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/restrict/TestPropertiesLoader.kt | 16 |  |
| modules/mockk/src/commonTest/kotlin/io/mockk/util/ArrayAsserts.kt | 79 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/AnyMatcherTypeTest.kt | 71 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/CapturingSlotTest.kt | 41 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/impl/JvmMockKGatewayTest.kt | 10 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/impl/annotations/MockInjectorTest.kt | 599 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/impl/instantiation/JvmMockFactoryHelperTest.kt | 57 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/impl/recording/JvmAutoHinterTest.kt | 67 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/impl/recording/JvmSignatureValueGeneratorTest.kt | 86 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/BackingFieldsTest.kt | 49 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/CallOriginalOnDefaultInterfaceMethodTest.kt | 48 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/CallableTest.kt | 31 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/CollectionChainStubbingTest.kt | 67 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/CollectionsReturnOnRelaxedMockTest.kt | 77 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/ConcurrentStubInvocationTest.kt | 50 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/CoroutineTest.kt | 223 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/InlineMemberFunctionFailFastTest.kt | 57 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/JavaClassParamTest.kt | 25 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/JvmConstructorOverridenTest.kt | 56 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/JvmVarargsTest.kt | 36 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/LocaleTest.kt | 26 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/MockKExtensionOnNestedClassTest.kt | 28 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/NullableValueTest.kt | 86 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/PackagePrivateParentTest.kt | 17 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/ParallelTest.kt | 101 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/PrivateParentMethodTest.kt | 71 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/ProxyWeakReferenceTest.kt | 26 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/RaceConditionInMockCreationTest.kt | 49 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/RelaxedSuspendingMockingTest.kt | 57 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/RurfTest.kt | 76 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/SafeCallSuspendingTest.kt | 27 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/StackTraceLineNumberTest.kt | 74 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/StaticMockkTest.kt | 225 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/ThrowExceptionOnNonMockedClassTest.kt | 25 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/TimeoutTest.kt | 128 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/VerificationAcknowledgeTest.kt | 667 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/it/VoidReturnTest.kt | 25 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit4/MockKRuleTest.kt | 85 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit5/ExtensionAndInitMocksTest.kt | 64 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit5/MockExtensionMetaAnnotations.kt | 35 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit5/MockKExtensionAfterAllTestTest.kt | 64 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit5/MockKExtensionConstructorBindingTest.kt | 79 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit5/MockKExtensionRequireParallelTestingTest.kt | 62 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit5/MockKExtensionTest.kt | 110 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/junit5/MockKExtensionWithoutRequireParallelTestingTest.kt | 59 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/jvm/ClassLoadingStrategyTest.kt | 19 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/jvm/HashMapMockTest.kt | 55 |  |
| modules/mockk/src/jvmTest/kotlin/io/mockk/jvm/JvmAnyValueGeneratorTest.kt | 123 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

