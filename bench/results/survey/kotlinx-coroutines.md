# Survey: kotlinx-coroutines 1.11.0

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 169 files, 34248 lines

Directories: `kotlinx-coroutines-core/common/src`, `kotlinx-coroutines-core/concurrent/src`, `kotlinx-coroutines-core/jvm/src`

### Features left, by files blocked

169 of 169 files use no feature that kc doesn't support.

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
| `let` | 31 | 69 |
| `assert` | 25 | 89 |
| `error` | 23 | 60 |
| `require` | 23 | 42 |
| `suspend` | 23 | 102 |
| `also` | 17 | 32 |
| `check` | 14 | 30 |
| `synchronized` | 11 | 35 |
| `run` | 10 | 14 |
| `apply` | 9 | 10 |
| `List` | 6 | 14 |
| `runCatching` | 6 | 10 |
| `getOrElse` | 5 | 7 |
| `onSuccess` | 4 | 7 |
| `with` | 4 | 4 |
| `buildString` | 3 | 4 |
| `copyOf` | 3 | 3 |
| `onFailure` | 3 | 5 |
| `takeIf` | 3 | 4 |
| `AutoCloseable` | 2 | 2 |
| `buildList` | 2 | 2 |
| `getOrThrow` | 2 | 2 |
| `isNotEmpty` | 2 | 3 |
| `toBoolean` | 2 | 2 |
| `countLeadingZeroBits` | 1 | 1 |
| `deleteAt` | 1 | 1 |
| `find` | 1 | 1 |
| `format` | 1 | 1 |
| `printStackTrace` | 1 | 2 |
| `replaceFirstChar` | 1 | 1 |
| `takeHighestOneBit` | 1 | 1 |
| `use` | 1 | 4 |

### Files

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| kotlinx-coroutines-core/common/src/AbstractCoroutine.kt | 136 |  |
| kotlinx-coroutines-core/common/src/Annotations.kt | 118 |  |
| kotlinx-coroutines-core/common/src/Await.kt | 120 |  |
| kotlinx-coroutines-core/common/src/Builders.common.kt | 599 |  |
| kotlinx-coroutines-core/common/src/CancellableContinuation.kt | 505 |  |
| kotlinx-coroutines-core/common/src/CancellableContinuationImpl.kt | 700 |  |
| kotlinx-coroutines-core/common/src/CloseableCoroutineDispatcher.kt | 24 |  |
| kotlinx-coroutines-core/common/src/CompletableDeferred.kt | 121 |  |
| kotlinx-coroutines-core/common/src/CompletableJob.kt | 44 |  |
| kotlinx-coroutines-core/common/src/CompletionHandler.common.kt | 27 |  |
| kotlinx-coroutines-core/common/src/CompletionState.kt | 50 |  |
| kotlinx-coroutines-core/common/src/CoroutineContext.common.kt | 36 |  |
| kotlinx-coroutines-core/common/src/CoroutineDispatcher.kt | 308 |  |
| kotlinx-coroutines-core/common/src/CoroutineExceptionHandler.kt | 302 |  |
| kotlinx-coroutines-core/common/src/CoroutineName.kt | 25 |  |
| kotlinx-coroutines-core/common/src/CoroutineScope.kt | 1307 |  |
| kotlinx-coroutines-core/common/src/CoroutineStart.kt | 371 |  |
| kotlinx-coroutines-core/common/src/Debug.common.kt | 42 |  |
| kotlinx-coroutines-core/common/src/Deferred.kt | 102 |  |
| kotlinx-coroutines-core/common/src/Delay.kt | 158 |  |
| kotlinx-coroutines-core/common/src/Dispatchers.common.kt | 73 |  |
| kotlinx-coroutines-core/common/src/EventLoop.common.kt | 539 |  |
| kotlinx-coroutines-core/common/src/Exceptions.common.kt | 26 |  |
| kotlinx-coroutines-core/common/src/Guidance.kt | 505 |  |
| kotlinx-coroutines-core/common/src/Job.kt | 691 |  |
| kotlinx-coroutines-core/common/src/JobSupport.kt | 1582 |  |
| kotlinx-coroutines-core/common/src/MainCoroutineDispatcher.kt | 73 |  |
| kotlinx-coroutines-core/common/src/NonCancellable.kt | 282 |  |
| kotlinx-coroutines-core/common/src/Runnable.common.kt | 14 |  |
| kotlinx-coroutines-core/common/src/SchedulerTask.common.kt | 16 |  |
| kotlinx-coroutines-core/common/src/Supervisor.kt | 118 |  |
| kotlinx-coroutines-core/common/src/Timeout.kt | 282 |  |
| kotlinx-coroutines-core/common/src/Unconfined.kt | 42 |  |
| kotlinx-coroutines-core/common/src/Waiter.kt | 17 |  |
| kotlinx-coroutines-core/common/src/Yield.kt | 166 |  |
| kotlinx-coroutines-core/common/src/channels/Broadcast.kt | 123 |  |
| kotlinx-coroutines-core/common/src/channels/BroadcastChannel.kt | 362 |  |
| kotlinx-coroutines-core/common/src/channels/BufferOverflow.kt | 39 |  |
| kotlinx-coroutines-core/common/src/channels/BufferedChannel.kt | 3091 |  |
| kotlinx-coroutines-core/common/src/channels/Channel.kt | 1528 |  |
| kotlinx-coroutines-core/common/src/channels/ChannelCoroutine.kt | 38 |  |
| kotlinx-coroutines-core/common/src/channels/Channels.common.kt | 271 |  |
| kotlinx-coroutines-core/common/src/channels/ConflatedBufferedChannel.kt | 111 |  |
| kotlinx-coroutines-core/common/src/channels/Deprecated.kt | 661 |  |
| kotlinx-coroutines-core/common/src/channels/Produce.kt | 358 |  |
| kotlinx-coroutines-core/common/src/flow/Builders.kt | 349 |  |
| kotlinx-coroutines-core/common/src/flow/Channels.kt | 157 |  |
| kotlinx-coroutines-core/common/src/flow/Flow.kt | 246 |  |
| kotlinx-coroutines-core/common/src/flow/FlowCollector.kt | 32 |  |
| kotlinx-coroutines-core/common/src/flow/Migration.kt | 491 |  |
| kotlinx-coroutines-core/common/src/flow/SharedFlow.kt | 740 |  |
| kotlinx-coroutines-core/common/src/flow/SharingStarted.kt | 204 |  |
| kotlinx-coroutines-core/common/src/flow/StateFlow.kt | 432 |  |
| kotlinx-coroutines-core/common/src/flow/internal/AbstractSharedFlow.kt | 129 |  |
| kotlinx-coroutines-core/common/src/flow/internal/ChannelFlow.kt | 239 |  |
| kotlinx-coroutines-core/common/src/flow/internal/Combine.kt | 139 |  |
| kotlinx-coroutines-core/common/src/flow/internal/FlowCoroutine.kt | 58 |  |
| kotlinx-coroutines-core/common/src/flow/internal/FlowExceptions.common.kt | 33 |  |
| kotlinx-coroutines-core/common/src/flow/internal/Merge.kt | 95 |  |
| kotlinx-coroutines-core/common/src/flow/internal/NopCollector.kt | 9 |  |
| kotlinx-coroutines-core/common/src/flow/internal/NullSurrogate.kt | 26 |  |
| kotlinx-coroutines-core/common/src/flow/internal/SafeCollector.common.kt | 110 |  |
| kotlinx-coroutines-core/common/src/flow/internal/SendingCollector.kt | 16 |  |
| kotlinx-coroutines-core/common/src/flow/operators/Context.kt | 289 |  |
| kotlinx-coroutines-core/common/src/flow/operators/Delay.kt | 406 |  |
| kotlinx-coroutines-core/common/src/flow/operators/Distinct.kt | 77 |  |
| kotlinx-coroutines-core/common/src/flow/operators/Emitters.kt | 215 |  |
| kotlinx-coroutines-core/common/src/flow/operators/Errors.kt | 219 |  |
| kotlinx-coroutines-core/common/src/flow/operators/Limit.kt | 140 |  |
| kotlinx-coroutines-core/common/src/flow/operators/Lint.kt | 197 |  |
| kotlinx-coroutines-core/common/src/flow/operators/Merge.kt | 213 |  |
| kotlinx-coroutines-core/common/src/flow/operators/Share.kt | 481 |  |
| kotlinx-coroutines-core/common/src/flow/operators/Transform.kt | 166 |  |
| kotlinx-coroutines-core/common/src/flow/operators/Zip.kt | 327 |  |
| kotlinx-coroutines-core/common/src/flow/terminal/Collect.kt | 131 |  |
| kotlinx-coroutines-core/common/src/flow/terminal/Collection.kt | 363 |  |
| kotlinx-coroutines-core/common/src/flow/terminal/Count.kt | 32 |  |
| kotlinx-coroutines-core/common/src/flow/terminal/Logic.kt | 107 |  |
| kotlinx-coroutines-core/common/src/flow/terminal/Reduce.kt | 167 |  |
| kotlinx-coroutines-core/common/src/internal/Concurrent.common.kt | 37 |  |
| kotlinx-coroutines-core/common/src/internal/ConcurrentLinkedList.kt | 264 |  |
| kotlinx-coroutines-core/common/src/internal/CoroutineExceptionHandlerImpl.common.kt | 68 |  |
| kotlinx-coroutines-core/common/src/internal/DispatchedContinuation.kt | 321 |  |
| kotlinx-coroutines-core/common/src/internal/DispatchedTask.kt | 219 |  |
| kotlinx-coroutines-core/common/src/internal/InlineList.kt | 44 |  |
| kotlinx-coroutines-core/common/src/internal/InternalAnnotations.common.kt | 13 |  |
| kotlinx-coroutines-core/common/src/internal/LimitedDispatcher.kt | 153 |  |
| kotlinx-coroutines-core/common/src/internal/LocalAtomics.common.kt | 15 |  |
| kotlinx-coroutines-core/common/src/internal/LockFreeLinkedList.common.kt | 23 |  |
| kotlinx-coroutines-core/common/src/internal/LockFreeTaskQueue.kt | 303 |  |
| kotlinx-coroutines-core/common/src/internal/MainDispatcherFactory.kt | 25 |  |
| kotlinx-coroutines-core/common/src/internal/NamedDispatcher.kt | 25 |  |
| kotlinx-coroutines-core/common/src/internal/OnUndeliveredElement.kt | 36 |  |
| kotlinx-coroutines-core/common/src/internal/ProbesSupport.common.kt | 7 |  |
| kotlinx-coroutines-core/common/src/internal/Scopes.kt | 43 |  |
| kotlinx-coroutines-core/common/src/internal/StackTraceRecovery.common.kt | 47 |  |
| kotlinx-coroutines-core/common/src/internal/Symbol.kt | 15 |  |
| kotlinx-coroutines-core/common/src/internal/Synchronized.common.kt | 27 |  |
| kotlinx-coroutines-core/common/src/internal/SystemProps.common.kt | 72 |  |
| kotlinx-coroutines-core/common/src/internal/ThreadContext.common.kt | 5 |  |
| kotlinx-coroutines-core/common/src/internal/ThreadLocal.common.kt | 15 |  |
| kotlinx-coroutines-core/common/src/internal/ThreadSafeHeap.kt | 158 |  |
| kotlinx-coroutines-core/common/src/intrinsics/Cancellable.kt | 64 |  |
| kotlinx-coroutines-core/common/src/intrinsics/Undispatched.kt | 104 |  |
| kotlinx-coroutines-core/common/src/selects/OnTimeout.kt | 61 |  |
| kotlinx-coroutines-core/common/src/selects/Select.kt | 900 |  |
| kotlinx-coroutines-core/common/src/selects/SelectOld.kt | 141 |  |
| kotlinx-coroutines-core/common/src/selects/SelectUnbiased.kt | 64 |  |
| kotlinx-coroutines-core/common/src/selects/WhileSelect.kt | 28 |  |
| kotlinx-coroutines-core/common/src/sync/Mutex.kt | 313 |  |
| kotlinx-coroutines-core/common/src/sync/Semaphore.kt | 395 |  |
| kotlinx-coroutines-core/concurrent/src/Builders.concurrent.kt | 178 |  |
| kotlinx-coroutines-core/concurrent/src/Dispatchers.kt | 47 |  |
| kotlinx-coroutines-core/concurrent/src/MultithreadedDispatchers.common.kt | 65 |  |
| kotlinx-coroutines-core/concurrent/src/channels/Channels.kt | 60 |  |
| kotlinx-coroutines-core/concurrent/src/internal/LockFreeLinkedList.kt | 287 |  |
| kotlinx-coroutines-core/concurrent/src/internal/OnDemandAllocatingPool.kt | 102 |  |
| kotlinx-coroutines-core/jvm/src/AbstractTimeSource.kt | 70 |  |
| kotlinx-coroutines-core/jvm/src/Builders.kt | 71 |  |
| kotlinx-coroutines-core/jvm/src/CoroutineContext.kt | 330 |  |
| kotlinx-coroutines-core/jvm/src/Debug.kt | 92 |  |
| kotlinx-coroutines-core/jvm/src/DebugStrings.kt | 17 |  |
| kotlinx-coroutines-core/jvm/src/DefaultExecutor.kt | 194 |  |
| kotlinx-coroutines-core/jvm/src/Dispatchers.kt | 103 |  |
| kotlinx-coroutines-core/jvm/src/EventLoop.kt | 24 |  |
| kotlinx-coroutines-core/jvm/src/Exceptions.kt | 73 |  |
| kotlinx-coroutines-core/jvm/src/Executors.kt | 215 |  |
| kotlinx-coroutines-core/jvm/src/Future.kt | 31 |  |
| kotlinx-coroutines-core/jvm/src/GuidanceJvm.kt | 23 |  |
| kotlinx-coroutines-core/jvm/src/Interruptible.kt | 160 |  |
| kotlinx-coroutines-core/jvm/src/Runnable.kt | 10 |  |
| kotlinx-coroutines-core/jvm/src/SchedulerTask.kt | 5 |  |
| kotlinx-coroutines-core/jvm/src/ThreadContextElement.kt | 284 |  |
| kotlinx-coroutines-core/jvm/src/ThreadPoolDispatcher.kt | 17 |  |
| kotlinx-coroutines-core/jvm/src/channels/Actor.kt | 198 |  |
| kotlinx-coroutines-core/jvm/src/channels/TickerChannels.kt | 107 |  |
| kotlinx-coroutines-core/jvm/src/debug/CoroutineDebugging.kt | 61 |  |
| kotlinx-coroutines-core/jvm/src/debug/internal/AgentInstallationType.kt | 18 |  |
| kotlinx-coroutines-core/jvm/src/debug/internal/AgentPremain.kt | 72 |  |
| kotlinx-coroutines-core/jvm/src/debug/internal/ConcurrentWeakMap.kt | 280 |  |
| kotlinx-coroutines-core/jvm/src/debug/internal/DebugCoroutineInfo.kt | 26 |  |
| kotlinx-coroutines-core/jvm/src/debug/internal/DebugCoroutineInfoImpl.kt | 178 |  |
| kotlinx-coroutines-core/jvm/src/debug/internal/DebugProbes.kt | 18 |  |
| kotlinx-coroutines-core/jvm/src/debug/internal/DebugProbesImpl.kt | 622 |  |
| kotlinx-coroutines-core/jvm/src/debug/internal/DebuggerInfo.kt | 24 |  |
| kotlinx-coroutines-core/jvm/src/debug/internal/StackTraceFrame.kt | 13 |  |
| kotlinx-coroutines-core/jvm/src/flow/internal/FlowExceptions.kt | 28 |  |
| kotlinx-coroutines-core/jvm/src/flow/internal/SafeCollector.kt | 181 |  |
| kotlinx-coroutines-core/jvm/src/future/Future.kt | 247 |  |
| kotlinx-coroutines-core/jvm/src/internal/Concurrent.kt | 42 |  |
| kotlinx-coroutines-core/jvm/src/internal/CoroutineExceptionHandlerImpl.kt | 60 |  |
| kotlinx-coroutines-core/jvm/src/internal/ExceptionsConstructor.kt | 133 |  |
| kotlinx-coroutines-core/jvm/src/internal/FastServiceLoader.kt | 168 |  |
| kotlinx-coroutines-core/jvm/src/internal/InternalAnnotations.kt | 3 |  |
| kotlinx-coroutines-core/jvm/src/internal/LocalAtomics.kt | 3 |  |
| kotlinx-coroutines-core/jvm/src/internal/MainDispatchers.kt | 129 |  |
| kotlinx-coroutines-core/jvm/src/internal/ProbesSupport.kt | 12 |  |
| kotlinx-coroutines-core/jvm/src/internal/ResizableAtomicArray.kt | 35 |  |
| kotlinx-coroutines-core/jvm/src/internal/StackTraceRecovery.kt | 209 |  |
| kotlinx-coroutines-core/jvm/src/internal/Synchronized.kt | 16 |  |
| kotlinx-coroutines-core/jvm/src/internal/SystemProps.kt | 16 |  |
| kotlinx-coroutines-core/jvm/src/internal/ThreadContext.kt | 127 |  |
| kotlinx-coroutines-core/jvm/src/internal/ThreadLocal.kt | 7 |  |
| kotlinx-coroutines-core/jvm/src/scheduling/CoroutineScheduler.kt | 1010 |  |
| kotlinx-coroutines-core/jvm/src/scheduling/Dispatcher.kt | 152 |  |
| kotlinx-coroutines-core/jvm/src/scheduling/Tasks.kt | 106 |  |
| kotlinx-coroutines-core/jvm/src/scheduling/WorkQueue.kt | 245 |  |
| kotlinx-coroutines-core/jvm/src/stream/Stream.kt | 28 |  |
| kotlinx-coroutines-core/jvm/src/time/Time.kt | 73 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 456 files, 41342 lines

Directories: `kotlinx-coroutines-core/common/test`, `kotlinx-coroutines-core/concurrent/test`, `kotlinx-coroutines-core/jvm/test`

### Features left, by files blocked

455 of 456 files use no feature that kc doesn't support.

| Feature | Files | Only blocker | Uses |
|:---|---:|---:|---:|
| safe-call assignment | 1 | 1 | 1 |

### Greedy order of the features left

At each step, the feature that unblocks the most files given the ones before it, or, if none unblocks a file, the one that blocks the most files.

| Step | Feature | Files unblocked |
|---:|:---|---:|
| 1 | safe-call assignment | 456 |

### Inline-only functions called

`require`, `check`, `requireNotNull`, `checkNotNull`, `error` and `TODO` are built in; kc inlines the others, also those with reified type parameters.

| Function | Files | Calls |
|:---|---:|---:|
| `check` | 42 | 116 |
| `error` | 38 | 69 |
| `suspend` | 33 | 43 |
| `List` | 20 | 32 |
| `let` | 19 | 20 |
| `require` | 16 | 37 |
| `use` | 13 | 29 |
| `also` | 12 | 19 |
| `getOrThrow` | 11 | 21 |
| `runCatching` | 11 | 23 |
| `apply` | 10 | 15 |
| `run` | 10 | 13 |
| `assert` | 5 | 6 |
| `printStackTrace` | 5 | 5 |
| `TODO` | 4 | 4 |
| `onFailure` | 4 | 7 |
| `onSuccess` | 4 | 9 |
| `getOrElse` | 3 | 4 |
| `isNotEmpty` | 3 | 8 |
| `with` | 3 | 3 |
| `MutableList` | 2 | 3 |
| `mod` | 2 | 3 |
| `sumOf` | 2 | 2 |
| `inputStream` | 1 | 1 |
| `matches` | 1 | 1 |
| `reader` | 1 | 1 |
| `synchronized` | 1 | 2 |
| `toRegex` | 1 | 1 |

### Files

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| kotlinx-coroutines-core/common/test/AbstractCoroutineTest.kt | 91 |  |
| kotlinx-coroutines-core/common/test/AsyncLazyTest.kt | 182 |  |
| kotlinx-coroutines-core/common/test/AsyncTest.kt | 293 |  |
| kotlinx-coroutines-core/common/test/AtomicCancellationCommonTest.kt | 155 |  |
| kotlinx-coroutines-core/common/test/AwaitCancellationTest.kt | 24 |  |
| kotlinx-coroutines-core/common/test/AwaitTest.kt | 381 |  |
| kotlinx-coroutines-core/common/test/BuilderContractsTest.kt | 63 |  |
| kotlinx-coroutines-core/common/test/CancellableContinuationHandlersTest.kt | 185 |  |
| kotlinx-coroutines-core/common/test/CancellableContinuationTest.kt | 148 |  |
| kotlinx-coroutines-core/common/test/CancellableResumeOldTest.kt | 289 |  |
| kotlinx-coroutines-core/common/test/CancellableResumeTest.kt | 314 |  |
| kotlinx-coroutines-core/common/test/CancelledParentAttachTest.kt | 113 |  |
| kotlinx-coroutines-core/common/test/CompletableDeferredTest.kt | 213 |  |
| kotlinx-coroutines-core/common/test/CompletableJobTest.kt | 99 |  |
| kotlinx-coroutines-core/common/test/CoroutineDispatcherOperatorFunInvokeTest.kt | 74 |  |
| kotlinx-coroutines-core/common/test/CoroutineExceptionHandlerTest.kt | 44 |  |
| kotlinx-coroutines-core/common/test/CoroutineScopeTest.kt | 313 |  |
| kotlinx-coroutines-core/common/test/CoroutinesTest.kt | 330 |  |
| kotlinx-coroutines-core/common/test/DelayDurationTest.kt | 66 |  |
| kotlinx-coroutines-core/common/test/DelayTest.kt | 52 |  |
| kotlinx-coroutines-core/common/test/DispatchedContinuationTest.kt | 75 |  |
| kotlinx-coroutines-core/common/test/DurationToMillisTest.kt | 66 |  |
| kotlinx-coroutines-core/common/test/EmptyContext.kt | 30 |  |
| kotlinx-coroutines-core/common/test/FailedJobTest.kt | 57 |  |
| kotlinx-coroutines-core/common/test/ImmediateYieldTest.kt | 54 |  |
| kotlinx-coroutines-core/common/test/JobExtensionsTest.kt | 92 |  |
| kotlinx-coroutines-core/common/test/JobStatesTest.kt | 162 |  |
| kotlinx-coroutines-core/common/test/JobTest.kt | 250 |  |
| kotlinx-coroutines-core/common/test/LaunchLazyTest.kt | 68 |  |
| kotlinx-coroutines-core/common/test/LimitedParallelismSharedTest.kt | 59 |  |
| kotlinx-coroutines-core/common/test/NonCancellableTest.kt | 134 |  |
| kotlinx-coroutines-core/common/test/ParentCancellationTest.kt | 167 |  |
| kotlinx-coroutines-core/common/test/ScopedBuildersCancelledStartTest.kt | 57 |  |
| kotlinx-coroutines-core/common/test/SupervisorTest.kt | 266 |  |
| kotlinx-coroutines-core/common/test/UnconfinedCancellationTest.kt | 99 |  |
| kotlinx-coroutines-core/common/test/UnconfinedTest.kt | 110 |  |
| kotlinx-coroutines-core/common/test/UndispatchedResultTest.kt | 64 |  |
| kotlinx-coroutines-core/common/test/WithContextTest.kt | 375 |  |
| kotlinx-coroutines-core/common/test/WithTimeoutDurationTest.kt | 211 |  |
| kotlinx-coroutines-core/common/test/WithTimeoutOrNullDurationTest.kt | 240 |  |
| kotlinx-coroutines-core/common/test/WithTimeoutOrNullTest.kt | 231 |  |
| kotlinx-coroutines-core/common/test/WithTimeoutTest.kt | 215 |  |
| kotlinx-coroutines-core/common/test/channels/BasicOperationsTest.kt | 224 |  |
| kotlinx-coroutines-core/common/test/channels/BroadcastChannelFactoryTest.kt | 34 |  |
| kotlinx-coroutines-core/common/test/channels/BroadcastTest.kt | 139 |  |
| kotlinx-coroutines-core/common/test/channels/BufferedBroadcastChannelTest.kt | 211 |  |
| kotlinx-coroutines-core/common/test/channels/BufferedChannelTest.kt | 250 |  |
| kotlinx-coroutines-core/common/test/channels/ChannelBufferOverflowTest.kt | 37 |  |
| kotlinx-coroutines-core/common/test/channels/ChannelFactoryTest.kt | 45 |  |
| kotlinx-coroutines-core/common/test/channels/ChannelIteratorTest.kt | 26 |  |
| kotlinx-coroutines-core/common/test/channels/ChannelPropertiesTest.kt | 54 |  |
| kotlinx-coroutines-core/common/test/channels/ChannelReceiveCatchingTest.kt | 143 |  |
| kotlinx-coroutines-core/common/test/channels/ChannelUndeliveredElementFailureTest.kt | 173 |  |
| kotlinx-coroutines-core/common/test/channels/ChannelUndeliveredElementTest.kt | 197 |  |
| kotlinx-coroutines-core/common/test/channels/ChannelsTest.kt | 144 |  |
| kotlinx-coroutines-core/common/test/channels/ConflatedBroadcastChannelTest.kt | 126 |  |
| kotlinx-coroutines-core/common/test/channels/ConflatedChannelTest.kt | 92 |  |
| kotlinx-coroutines-core/common/test/channels/ConsumeTest.kt | 146 |  |
| kotlinx-coroutines-core/common/test/channels/ProduceConsumeTest.kt | 56 |  |
| kotlinx-coroutines-core/common/test/channels/ProduceTest.kt | 293 |  |
| kotlinx-coroutines-core/common/test/channels/RendezvousChannelTest.kt | 296 |  |
| kotlinx-coroutines-core/common/test/channels/SendReceiveStressTest.kt | 46 |  |
| kotlinx-coroutines-core/common/test/channels/TestBroadcastChannelKind.kt | 22 |  |
| kotlinx-coroutines-core/common/test/channels/TestChannelKind.kt | 56 |  |
| kotlinx-coroutines-core/common/test/channels/UnlimitedChannelTest.kt | 40 |  |
| kotlinx-coroutines-core/common/test/flow/BuildersTest.kt | 39 |  |
| kotlinx-coroutines-core/common/test/flow/FlowInvariantsTest.kt | 277 |  |
| kotlinx-coroutines-core/common/test/flow/IdFlowTest.kt | 59 |  |
| kotlinx-coroutines-core/common/test/flow/NamedDispatchers.kt | 62 |  |
| kotlinx-coroutines-core/common/test/flow/SafeFlowTest.kt | 28 |  |
| kotlinx-coroutines-core/common/test/flow/VirtualTime.kt | 99 |  |
| kotlinx-coroutines-core/common/test/flow/channels/ChannelBuildersFlowTest.kt | 137 |  |
| kotlinx-coroutines-core/common/test/flow/channels/ChannelFlowTest.kt | 207 |  |
| kotlinx-coroutines-core/common/test/flow/channels/FlowCallbackTest.kt | 52 |  |
| kotlinx-coroutines-core/common/test/flow/internal/FlowScopeTest.kt | 74 |  |
| kotlinx-coroutines-core/common/test/flow/operators/AsFlowTest.kt | 34 |  |
| kotlinx-coroutines-core/common/test/flow/operators/BooleanTerminationTest.kt | 106 |  |
| kotlinx-coroutines-core/common/test/flow/operators/BufferConflationTest.kt | 143 |  |
| kotlinx-coroutines-core/common/test/flow/operators/BufferTest.kt | 194 |  |
| kotlinx-coroutines-core/common/test/flow/operators/CancellableTest.kt | 34 |  |
| kotlinx-coroutines-core/common/test/flow/operators/CatchTest.kt | 201 |  |
| kotlinx-coroutines-core/common/test/flow/operators/ChunkedTest.kt | 88 |  |
| kotlinx-coroutines-core/common/test/flow/operators/CombineParametersTest.kt | 187 |  |
| kotlinx-coroutines-core/common/test/flow/operators/CombineTest.kt | 312 |  |
| kotlinx-coroutines-core/common/test/flow/operators/ConflateTest.kt | 23 |  |
| kotlinx-coroutines-core/common/test/flow/operators/DebounceTest.kt | 321 |  |
| kotlinx-coroutines-core/common/test/flow/operators/DistinctUntilChangedTest.kt | 123 |  |
| kotlinx-coroutines-core/common/test/flow/operators/DropTest.kt | 57 |  |
| kotlinx-coroutines-core/common/test/flow/operators/DropWhileTest.kt | 48 |  |
| kotlinx-coroutines-core/common/test/flow/operators/FilterTest.kt | 78 |  |
| kotlinx-coroutines-core/common/test/flow/operators/FilterTrivialTest.kt | 74 |  |
| kotlinx-coroutines-core/common/test/flow/operators/FlatMapBaseTest.kt | 88 |  |
| kotlinx-coroutines-core/common/test/flow/operators/FlatMapConcatTest.kt | 35 |  |
| kotlinx-coroutines-core/common/test/flow/operators/FlatMapLatestTest.kt | 134 |  |
| kotlinx-coroutines-core/common/test/flow/operators/FlatMapMergeBaseTest.kt | 90 |  |
| kotlinx-coroutines-core/common/test/flow/operators/FlatMapMergeFastPathTest.kt | 85 |  |
| kotlinx-coroutines-core/common/test/flow/operators/FlatMapMergeTest.kt | 113 |  |
| kotlinx-coroutines-core/common/test/flow/operators/FlattenConcatTest.kt | 49 |  |
| kotlinx-coroutines-core/common/test/flow/operators/FlattenMergeTest.kt | 50 |  |
| kotlinx-coroutines-core/common/test/flow/operators/FlowContextOptimizationsTest.kt | 113 |  |
| kotlinx-coroutines-core/common/test/flow/operators/FlowOnTest.kt | 358 |  |
| kotlinx-coroutines-core/common/test/flow/operators/IndexedTest.kt | 42 |  |
| kotlinx-coroutines-core/common/test/flow/operators/LintTest.kt | 27 |  |
| kotlinx-coroutines-core/common/test/flow/operators/MapNotNullTest.kt | 47 |  |
| kotlinx-coroutines-core/common/test/flow/operators/MapTest.kt | 47 |  |
| kotlinx-coroutines-core/common/test/flow/operators/MergeTest.kt | 123 |  |
| kotlinx-coroutines-core/common/test/flow/operators/OnCompletionTest.kt | 397 |  |
| kotlinx-coroutines-core/common/test/flow/operators/OnEachTest.kt | 47 |  |
| kotlinx-coroutines-core/common/test/flow/operators/OnEmptyTest.kt | 77 |  |
| kotlinx-coroutines-core/common/test/flow/operators/OnStartTest.kt | 33 |  |
| kotlinx-coroutines-core/common/test/flow/operators/RetryTest.kt | 161 |  |
| kotlinx-coroutines-core/common/test/flow/operators/SampleTest.kt | 301 |  |
| kotlinx-coroutines-core/common/test/flow/operators/ScanTest.kt | 72 |  |
| kotlinx-coroutines-core/common/test/flow/operators/TakeTest.kt | 144 |  |
| kotlinx-coroutines-core/common/test/flow/operators/TakeWhileTest.kt | 64 |  |
| kotlinx-coroutines-core/common/test/flow/operators/TimeoutTest.kt | 258 |  |
| kotlinx-coroutines-core/common/test/flow/operators/TransformLatestTest.kt | 157 |  |
| kotlinx-coroutines-core/common/test/flow/operators/TransformTest.kt | 17 |  |
| kotlinx-coroutines-core/common/test/flow/operators/TransformWhileTest.kt | 67 |  |
| kotlinx-coroutines-core/common/test/flow/operators/ZipTest.kt | 251 |  |
| kotlinx-coroutines-core/common/test/flow/sharing/ShareInBufferTest.kt | 95 |  |
| kotlinx-coroutines-core/common/test/flow/sharing/ShareInConflationTest.kt | 159 |  |
| kotlinx-coroutines-core/common/test/flow/sharing/ShareInFusionTest.kt | 53 |  |
| kotlinx-coroutines-core/common/test/flow/sharing/ShareInTest.kt | 241 |  |
| kotlinx-coroutines-core/common/test/flow/sharing/SharedFlowScenarioTest.kt | 403 |  |
| kotlinx-coroutines-core/common/test/flow/sharing/SharedFlowTest.kt | 870 |  |
| kotlinx-coroutines-core/common/test/flow/sharing/SharingStartedTest.kt | 180 |  |
| kotlinx-coroutines-core/common/test/flow/sharing/SharingStartedWhileSubscribedTest.kt | 76 |  |
| kotlinx-coroutines-core/common/test/flow/sharing/StateFlowTest.kt | 247 |  |
| kotlinx-coroutines-core/common/test/flow/sharing/StateInTest.kt | 111 |  |
| kotlinx-coroutines-core/common/test/flow/terminal/CollectLatestTest.kt | 53 |  |
| kotlinx-coroutines-core/common/test/flow/terminal/CountTest.kt | 44 |  |
| kotlinx-coroutines-core/common/test/flow/terminal/FirstTest.kt | 195 |  |
| kotlinx-coroutines-core/common/test/flow/terminal/FoldTest.kt | 51 |  |
| kotlinx-coroutines-core/common/test/flow/terminal/LastTest.kt | 42 |  |
| kotlinx-coroutines-core/common/test/flow/terminal/LaunchInTest.kt | 53 |  |
| kotlinx-coroutines-core/common/test/flow/terminal/ReduceTest.kt | 71 |  |
| kotlinx-coroutines-core/common/test/flow/terminal/SingleTest.kt | 91 |  |
| kotlinx-coroutines-core/common/test/flow/terminal/ToCollectionTest.kt | 28 |  |
| kotlinx-coroutines-core/common/test/flow/terminal/ToMapCollectionSamplesTest.kt | 115 |  |
| kotlinx-coroutines-core/common/test/selects/SelectBiasTest.kt | 41 |  |
| kotlinx-coroutines-core/common/test/selects/SelectBufferedChannelTest.kt | 414 |  |
| kotlinx-coroutines-core/common/test/selects/SelectDeferredTest.kt | 185 |  |
| kotlinx-coroutines-core/common/test/selects/SelectJobTest.kt | 64 |  |
| kotlinx-coroutines-core/common/test/selects/SelectLoopTest.kt | 38 |  |
| kotlinx-coroutines-core/common/test/selects/SelectMutexTest.kt | 54 |  |
| kotlinx-coroutines-core/common/test/selects/SelectOldTest.kt | 149 |  |
| kotlinx-coroutines-core/common/test/selects/SelectRendezvousChannelTest.kt | 474 |  |
| kotlinx-coroutines-core/common/test/selects/SelectTimeoutDurationTest.kt | 89 |  |
| kotlinx-coroutines-core/common/test/selects/SelectTimeoutTest.kt | 86 |  |
| kotlinx-coroutines-core/common/test/selects/SelectUnlimitedChannelTest.kt | 26 |  |
| kotlinx-coroutines-core/common/test/sync/MutexTest.kt | 201 |  |
| kotlinx-coroutines-core/common/test/sync/SemaphoreTest.kt | 214 |  |
| kotlinx-coroutines-core/concurrent/test/AbstractDispatcherConcurrencyTest.kt | 52 |  |
| kotlinx-coroutines-core/concurrent/test/AtomicCancellationTest.kt | 143 |  |
| kotlinx-coroutines-core/concurrent/test/CommonThreadLocalTest.kt | 72 |  |
| kotlinx-coroutines-core/concurrent/test/ConcurrentExceptionsStressTest.kt | 64 |  |
| kotlinx-coroutines-core/concurrent/test/ConcurrentTestUtilities.common.kt | 33 |  |
| kotlinx-coroutines-core/concurrent/test/DefaultDispatchersConcurrencyTest.kt | 9 |  |
| kotlinx-coroutines-core/concurrent/test/JobStructuredJoinStressTest.kt | 61 |  |
| kotlinx-coroutines-core/concurrent/test/LimitedParallelismConcurrentTest.kt | 92 |  |
| kotlinx-coroutines-core/concurrent/test/MultithreadedDispatcherStressTest.kt | 31 |  |
| kotlinx-coroutines-core/concurrent/test/RunBlockingTest.kt | 207 |  |
| kotlinx-coroutines-core/concurrent/test/channels/BroadcastChannelSubStressTest.kt | 57 |  |
| kotlinx-coroutines-core/concurrent/test/channels/ChannelCancelUndeliveredElementStressTest.kt | 100 |  |
| kotlinx-coroutines-core/concurrent/test/channels/ConflatedBroadcastChannelNotifyStressTest.kt | 86 |  |
| kotlinx-coroutines-core/concurrent/test/channels/TrySendBlockingTest.kt | 50 |  |
| kotlinx-coroutines-core/concurrent/test/flow/CombineStressTest.kt | 50 |  |
| kotlinx-coroutines-core/concurrent/test/flow/FlowCancellationTest.kt | 60 |  |
| kotlinx-coroutines-core/concurrent/test/flow/StateFlowCommonStressTest.kt | 44 |  |
| kotlinx-coroutines-core/concurrent/test/flow/StateFlowUpdateCommonTest.kt | 36 |  |
| kotlinx-coroutines-core/concurrent/test/selects/SelectChannelStressTest.kt | 67 |  |
| kotlinx-coroutines-core/concurrent/test/selects/SelectMutexStressTest.kt | 30 |  |
| kotlinx-coroutines-core/concurrent/test/sync/MutexStressTest.kt | 135 |  |
| kotlinx-coroutines-core/concurrent/test/sync/SemaphoreStressTest.kt | 135 |  |
| kotlinx-coroutines-core/jvm/test/AbstractLincheckTest.kt | 41 |  |
| kotlinx-coroutines-core/jvm/test/AsyncJvmTest.kt | 46 |  |
| kotlinx-coroutines-core/jvm/test/AwaitJvmTest.kt | 24 |  |
| kotlinx-coroutines-core/jvm/test/AwaitStressTest.kt | 122 |  |
| kotlinx-coroutines-core/jvm/test/CancellableContinuationJvmTest.kt | 79 |  |
| kotlinx-coroutines-core/jvm/test/CancellableContinuationResumeCloseStressTest.kt | 61 |  |
| kotlinx-coroutines-core/jvm/test/CancelledAwaitStressTest.kt | 52 |  |
| kotlinx-coroutines-core/jvm/test/ConcurrentTestUtilities.kt | 6 |  |
| kotlinx-coroutines-core/jvm/test/CoroutineScopeTestJvm.kt | 74 |  |
| kotlinx-coroutines-core/jvm/test/CoroutinesJvmTest.kt | 36 |  |
| kotlinx-coroutines-core/jvm/test/DebugThreadNameTest.kt | 70 |  |
| kotlinx-coroutines-core/jvm/test/DefaultExecutorStressTest.kt | 62 |  |
| kotlinx-coroutines-core/jvm/test/DelayJvmTest.kt | 68 |  |
| kotlinx-coroutines-core/jvm/test/DispatcherKeyTest.kt | 49 |  |
| kotlinx-coroutines-core/jvm/test/DispatchersToStringTest.kt | 55 |  |
| kotlinx-coroutines-core/jvm/test/EventLoopsTest.kt | 125 |  |
| kotlinx-coroutines-core/jvm/test/ExecutorAsCoroutineDispatcherDelayTest.kt | 58 |  |
| kotlinx-coroutines-core/jvm/test/ExecutorsTest.kt | 148 |  |
| kotlinx-coroutines-core/jvm/test/FailFastOnStartTest.kt | 110 |  |
| kotlinx-coroutines-core/jvm/test/FailingCoroutinesMachineryTest.kt | 151 |  |
| kotlinx-coroutines-core/jvm/test/IODispatcherTest.kt | 22 |  |
| kotlinx-coroutines-core/jvm/test/IntellijIdeaDebuggerEvaluatorCompatibilityTest.kt | 52 |  |
| kotlinx-coroutines-core/jvm/test/JobActivationStressTest.kt | 68 |  |
| kotlinx-coroutines-core/jvm/test/JobCancellationExceptionSerializerTest.kt | 65 |  |
| kotlinx-coroutines-core/jvm/test/JobChildStressTest.kt | 115 |  |
| kotlinx-coroutines-core/jvm/test/JobDisposeStressTest.kt | 77 |  |
| kotlinx-coroutines-core/jvm/test/JobHandlersUpgradeStressTest.kt | 97 |  |
| kotlinx-coroutines-core/jvm/test/JobOnCompletionStressTest.kt | 192 |  |
| kotlinx-coroutines-core/jvm/test/JobStressTest.kt | 14 |  |
| kotlinx-coroutines-core/jvm/test/JoinStressTest.kt | 90 |  |
| kotlinx-coroutines-core/jvm/test/LimitedParallelismStressTest.kt | 148 |  |
| kotlinx-coroutines-core/jvm/test/LimitedParallelismUnhandledExceptionTest.kt | 29 |  |
| kotlinx-coroutines-core/jvm/test/MemoryFootprintTest.kt | 43 |  |
| kotlinx-coroutines-core/jvm/test/MultithreadedDispatchersJvmTest.kt | 30 | safe-call assignment |
| kotlinx-coroutines-core/jvm/test/MutexCancellationStressTest.kt | 83 |  |
| kotlinx-coroutines-core/jvm/test/NoParamAssertionsTest.kt | 27 |  |
| kotlinx-coroutines-core/jvm/test/RejectedExecutionTest.kt | 166 |  |
| kotlinx-coroutines-core/jvm/test/ReusableCancellableContinuationInvariantStressTest.kt | 50 |  |
| kotlinx-coroutines-core/jvm/test/ReusableCancellableContinuationLeakStressTest.kt | 38 |  |
| kotlinx-coroutines-core/jvm/test/ReusableCancellableContinuationTest.kt | 218 |  |
| kotlinx-coroutines-core/jvm/test/ReusableContinuationStressTest.kt | 38 |  |
| kotlinx-coroutines-core/jvm/test/RunBlockingJvmTest.kt | 184 |  |
| kotlinx-coroutines-core/jvm/test/RunInterruptibleStressTest.kt | 53 |  |
| kotlinx-coroutines-core/jvm/test/RunInterruptibleTest.kt | 60 |  |
| kotlinx-coroutines-core/jvm/test/TestBaseTest.kt | 24 |  |
| kotlinx-coroutines-core/jvm/test/ThreadContextElementRestoreTest.kt | 195 |  |
| kotlinx-coroutines-core/jvm/test/ThreadContextElementTest.kt | 377 |  |
| kotlinx-coroutines-core/jvm/test/ThreadContextMutableCopiesTest.kt | 158 |  |
| kotlinx-coroutines-core/jvm/test/ThreadContextOrderTest.kt | 62 |  |
| kotlinx-coroutines-core/jvm/test/ThreadLocalStressTest.kt | 162 |  |
| kotlinx-coroutines-core/jvm/test/ThreadLocalTest.kt | 230 |  |
| kotlinx-coroutines-core/jvm/test/ThreadLocalsLeaksTest.kt | 106 |  |
| kotlinx-coroutines-core/jvm/test/UnconfinedConcurrentStressTest.kt | 47 |  |
| kotlinx-coroutines-core/jvm/test/VirtualTimeSource.kt | 154 |  |
| kotlinx-coroutines-core/jvm/test/WithDefaultContextTest.kt | 30 |  |
| kotlinx-coroutines-core/jvm/test/WithTimeoutChildDispatchStressTest.kt | 29 |  |
| kotlinx-coroutines-core/jvm/test/WithTimeoutOrNullJvmTest.kt | 64 |  |
| kotlinx-coroutines-core/jvm/test/WithTimeoutOrNullThreadDispatchTest.kt | 79 |  |
| kotlinx-coroutines-core/jvm/test/WithTimeoutThreadDispatchTest.kt | 82 |  |
| kotlinx-coroutines-core/jvm/test/channels/ActorLazyTest.kt | 88 |  |
| kotlinx-coroutines-core/jvm/test/channels/ActorTest.kt | 193 |  |
| kotlinx-coroutines-core/jvm/test/channels/BroadcastChannelLeakTest.kt | 35 |  |
| kotlinx-coroutines-core/jvm/test/channels/BroadcastChannelMultiReceiveStressTest.kt | 166 |  |
| kotlinx-coroutines-core/jvm/test/channels/BufferedChannelStressTest.kt | 61 |  |
| kotlinx-coroutines-core/jvm/test/channels/CancelledChannelLeakTest.kt | 27 |  |
| kotlinx-coroutines-core/jvm/test/channels/ChannelMemoryLeakStressTest.kt | 21 |  |
| kotlinx-coroutines-core/jvm/test/channels/ChannelSelectStressTest.kt | 78 |  |
| kotlinx-coroutines-core/jvm/test/channels/ChannelSendReceiveStressTest.kt | 182 |  |
| kotlinx-coroutines-core/jvm/test/channels/ChannelUndeliveredElementSelectOldStressTest.kt | 251 |  |
| kotlinx-coroutines-core/jvm/test/channels/ChannelUndeliveredElementStressTest.kt | 267 |  |
| kotlinx-coroutines-core/jvm/test/channels/ConflatedChannelCloseStressTest.kt | 106 |  |
| kotlinx-coroutines-core/jvm/test/channels/DoubleChannelCloseStressTest.kt | 27 |  |
| kotlinx-coroutines-core/jvm/test/channels/InvokeOnCloseStressTest.kt | 61 |  |
| kotlinx-coroutines-core/jvm/test/channels/ProduceConsumeJvmTest.kt | 59 |  |
| kotlinx-coroutines-core/jvm/test/channels/SendReceiveJvmStressTest.kt | 45 |  |
| kotlinx-coroutines-core/jvm/test/channels/SimpleSendReceiveJvmTest.kt | 50 |  |
| kotlinx-coroutines-core/jvm/test/channels/TickerChannelCommonTest.kt | 164 |  |
| kotlinx-coroutines-core/jvm/test/channels/TickerChannelTest.kt | 62 |  |
| kotlinx-coroutines-core/jvm/test/examples/example-delay-01.kt | 21 |  |
| kotlinx-coroutines-core/jvm/test/examples/example-delay-02.kt | 27 |  |
| kotlinx-coroutines-core/jvm/test/examples/example-delay-03.kt | 16 |  |
| kotlinx-coroutines-core/jvm/test/examples/example-delay-duration-01.kt | 21 |  |
| kotlinx-coroutines-core/jvm/test/examples/example-delay-duration-02.kt | 27 |  |
| kotlinx-coroutines-core/jvm/test/examples/example-delay-duration-03.kt | 16 |  |
| kotlinx-coroutines-core/jvm/test/examples/example-timeout-duration-01.kt | 30 |  |
| kotlinx-coroutines-core/jvm/test/examples/test/FlowDelayTest.kt | 56 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/CoroutineExceptionHandlerJvmTest.kt | 75 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/FlowSuppressionTest.kt | 71 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/JobBasicCancellationTest.kt | 156 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/JobExceptionHandlingTest.kt | 377 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/JobExceptionsStressTest.kt | 64 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/JobNestedExceptionsTest.kt | 116 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/ProduceExceptionsTest.kt | 166 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryChannelsTest.kt | 150 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryCopyableThrowableTest.kt | 76 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryExceptionConstructorsTest.kt | 91 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryNestedScopesTest.kt | 99 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryNestedTest.kt | 84 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryResumeModeTest.kt | 146 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoverySelectTest.kt | 68 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryStdlibInterfaceTest.kt | 104 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryStdlibInterfaceWithTypeBoundsTest.kt | 108 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryTest.kt | 262 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryWithTimeoutTest.kt | 85 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/Stacktraces.kt | 42 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/SuppressionTests.kt | 83 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/WithContextCancellationStressTest.kt | 104 |  |
| kotlinx-coroutines-core/jvm/test/exceptions/WithContextExceptionHandlingTest.kt | 278 |  |
| kotlinx-coroutines-core/jvm/test/flow/CallbackFlowTest.kt | 125 |  |
| kotlinx-coroutines-core/jvm/test/flow/ExceptionTransparencyTest.kt | 74 |  |
| kotlinx-coroutines-core/jvm/test/flow/FirstJvmTest.kt | 25 |  |
| kotlinx-coroutines-core/jvm/test/flow/FlatMapStressTest.kt | 99 |  |
| kotlinx-coroutines-core/jvm/test/flow/OnCompletionInterceptedReleaseTest.kt | 42 |  |
| kotlinx-coroutines-core/jvm/test/flow/SafeCollectorMemoryLeakTest.kt | 45 |  |
| kotlinx-coroutines-core/jvm/test/flow/SharedFlowStressTest.kt | 83 |  |
| kotlinx-coroutines-core/jvm/test/flow/SharingReferenceTest.kt | 57 |  |
| kotlinx-coroutines-core/jvm/test/flow/SharingStressTest.kt | 193 |  |
| kotlinx-coroutines-core/jvm/test/flow/StateFlowCancellabilityTest.kt | 53 |  |
| kotlinx-coroutines-core/jvm/test/flow/StateFlowStressTest.kt | 79 |  |
| kotlinx-coroutines-core/jvm/test/flow/StateFlowUpdateStressTest.kt | 41 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-basic-01.kt | 12 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-basic-02.kt | 15 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-basic-03.kt | 16 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-basic-04.kt | 23 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-basic-05.kt | 14 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-basic-06.kt | 13 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-01.kt | 18 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-02.kt | 23 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-03.kt | 23 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-04.kt | 23 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-05.kt | 21 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-06.kt | 25 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-07.kt | 13 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-08.kt | 15 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-09.kt | 27 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-10.kt | 32 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-01.kt | 17 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-02.kt | 16 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-03.kt | 15 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-04.kt | 24 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-05.kt | 24 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-06.kt | 26 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-07.kt | 22 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-08.kt | 18 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-09.kt | 25 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-10.kt | 29 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-compose-01.kt | 24 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-compose-02.kt | 24 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-compose-03.kt | 27 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-compose-04.kt | 40 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-compose-05.kt | 28 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-compose-06.kt | 28 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-context-01.kt | 19 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-context-02.kt | 17 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-context-03.kt | 18 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-context-04.kt | 20 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-context-05.kt | 8 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-context-06.kt | 27 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-context-07.kt | 19 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-context-08.kt | 22 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-context-09.kt | 10 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-context-10.kt | 32 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-context-11.kt | 18 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-exceptions-01.kt | 24 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-exceptions-02.kt | 18 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-exceptions-03.kt | 23 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-exceptions-04.kt | 30 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-exceptions-05.kt | 29 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-exceptions-06.kt | 28 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-01.kt | 8 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-02.kt | 13 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-03.kt | 13 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-04.kt | 24 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-05.kt | 22 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-06.kt | 20 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-07.kt | 10 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-08.kt | 16 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-09.kt | 19 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-10.kt | 22 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-11.kt | 12 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-12.kt | 19 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-13.kt | 18 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-14.kt | 19 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-15.kt | 21 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-16.kt | 23 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-17.kt | 25 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-18.kt | 25 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-19.kt | 25 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-20.kt | 12 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-21.kt | 15 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-22.kt | 15 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-23.kt | 20 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-24.kt | 20 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-25.kt | 20 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-26.kt | 23 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-27.kt | 25 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-28.kt | 23 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-29.kt | 21 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-30.kt | 22 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-31.kt | 15 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-32.kt | 13 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-33.kt | 17 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-34.kt | 16 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-35.kt | 15 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-36.kt | 15 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-37.kt | 19 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-38.kt | 12 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-39.kt | 12 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-select-01.kt | 40 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-select-02.kt | 39 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-select-03.kt | 29 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-select-04.kt | 30 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-select-05.kt | 50 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-supervision-01.kt | 32 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-supervision-02.kt | 26 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-supervision-03.kt | 19 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-sync-01.kt | 31 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-sync-02.kt | 32 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-sync-03.kt | 32 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-sync-04.kt | 35 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-sync-05.kt | 33 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-sync-06.kt | 36 |  |
| kotlinx-coroutines-core/jvm/test/guide/example-sync-07.kt | 51 |  |
| kotlinx-coroutines-core/jvm/test/guide/test/BasicsGuideTest.kt | 57 |  |
| kotlinx-coroutines-core/jvm/test/guide/test/CancellationGuideTest.kt | 94 |  |
| kotlinx-coroutines-core/jvm/test/guide/test/ChannelsGuideTest.kt | 123 |  |
| kotlinx-coroutines-core/jvm/test/guide/test/ComposingGuideTest.kt | 56 |  |
| kotlinx-coroutines-core/jvm/test/guide/test/DispatcherGuideTest.kt | 110 |  |
| kotlinx-coroutines-core/jvm/test/guide/test/ExceptionsGuideTest.kt | 89 |  |
| kotlinx-coroutines-core/jvm/test/guide/test/FlowGuideTest.kt | 417 |  |
| kotlinx-coroutines-core/jvm/test/guide/test/SelectGuideTest.kt | 69 |  |
| kotlinx-coroutines-core/jvm/test/guide/test/SharedStateGuideTest.kt | 55 |  |
| kotlinx-coroutines-core/jvm/test/internal/ConcurrentWeakMapCollectionStressTest.kt | 28 |  |
| kotlinx-coroutines-core/jvm/test/internal/ConcurrentWeakMapOperationStressTest.kt | 70 |  |
| kotlinx-coroutines-core/jvm/test/internal/ConcurrentWeakMapTest.kt | 42 |  |
| kotlinx-coroutines-core/jvm/test/internal/FastServiceLoaderTest.kt | 17 |  |
| kotlinx-coroutines-core/jvm/test/internal/LockFreeLinkedListLongStressTest.kt | 81 |  |
| kotlinx-coroutines-core/jvm/test/internal/LockFreeTaskQueueStressTest.kt | 116 |  |
| kotlinx-coroutines-core/jvm/test/internal/LockFreeTaskQueueTest.kt | 73 |  |
| kotlinx-coroutines-core/jvm/test/internal/OnDemandAllocatingPoolLincheckTest.kt | 54 |  |
| kotlinx-coroutines-core/jvm/test/internal/ThreadSafeHeapStressTest.kt | 63 |  |
| kotlinx-coroutines-core/jvm/test/internal/ThreadSafeHeapTest.kt | 93 |  |
| kotlinx-coroutines-core/jvm/test/jdk8/future/AsFutureTest.kt | 121 |  |
| kotlinx-coroutines-core/jvm/test/jdk8/future/FutureAsDeferredUnhandledCompletionExceptionTest.kt | 35 |  |
| kotlinx-coroutines-core/jvm/test/jdk8/future/FutureExceptionsTest.kt | 84 |  |
| kotlinx-coroutines-core/jvm/test/jdk8/future/FutureTest.kt | 614 |  |
| kotlinx-coroutines-core/jvm/test/jdk8/stream/ConsumeAsFlowTest.kt | 35 |  |
| kotlinx-coroutines-core/jvm/test/jdk8/time/DurationOverflowTest.kt | 76 |  |
| kotlinx-coroutines-core/jvm/test/jdk8/time/FlowDebounceTest.kt | 35 |  |
| kotlinx-coroutines-core/jvm/test/jdk8/time/FlowSampleTest.kt | 35 |  |
| kotlinx-coroutines-core/jvm/test/jdk8/time/WithTimeoutTest.kt | 65 |  |
| kotlinx-coroutines-core/jvm/test/knit/ClosedAfterGuideTestExecutor.kt | 36 |  |
| kotlinx-coroutines-core/jvm/test/knit/TestUtil.kt | 171 |  |
| kotlinx-coroutines-core/jvm/test/lincheck/ChannelsLincheckTest.kt | 273 |  |
| kotlinx-coroutines-core/jvm/test/lincheck/LockFreeTaskQueueLincheckTest.kt | 44 |  |
| kotlinx-coroutines-core/jvm/test/lincheck/MutexLincheckTest.kt | 42 |  |
| kotlinx-coroutines-core/jvm/test/lincheck/ResizableAtomicArrayLincheckTest.kt | 20 |  |
| kotlinx-coroutines-core/jvm/test/lincheck/SemaphoreLincheckTest.kt | 31 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/BlockingCoroutineDispatcherLivenessStressTest.kt | 61 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/BlockingCoroutineDispatcherMixedStealingStressTest.kt | 77 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/BlockingCoroutineDispatcherTerminationStressTest.kt | 39 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/BlockingCoroutineDispatcherTest.kt | 174 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/BlockingCoroutineDispatcherThreadLimitStressTest.kt | 58 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/BlockingCoroutineDispatcherWorkSignallingStressTest.kt | 95 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/CoroutineDispatcherTest.kt | 144 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/CoroutineSchedulerCloseStressTest.kt | 76 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/CoroutineSchedulerLivenessStressTest.kt | 49 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/CoroutineSchedulerOversubscriptionTest.kt | 86 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/CoroutineSchedulerStressTest.kt | 105 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/CoroutineSchedulerTest.kt | 166 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/DefaultDispatchersTest.kt | 72 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/LimitingCoroutineDispatcherStressTest.kt | 52 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/LimitingDispatcherTest.kt | 41 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/SchedulerTestBase.kt | 116 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/SharingWorkerClassTest.kt | 43 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/TestTimeSource.kt | 11 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/WorkQueueStressTest.kt | 121 |  |
| kotlinx-coroutines-core/jvm/test/scheduling/WorkQueueTest.kt | 108 |  |
| kotlinx-coroutines-core/jvm/test/selects/SelectDeadlockStressTest.kt | 59 |  |
| kotlinx-coroutines-core/jvm/test/selects/SelectMemoryLeakStressTest.kt | 56 |  |
| kotlinx-coroutines-core/jvm/test/selects/SelectPhilosophersStressTest.kt | 63 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

