# Survey: kotlinx-coroutines 1.11.0

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 169 files, 34248 lines

Directories: `kotlinx-coroutines-core/common/src`, `kotlinx-coroutines-core/concurrent/src`, `kotlinx-coroutines-core/jvm/src`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 169 | 10 |
| 1: packages, annotations | 3 | 166 | 8 |
| 2: interfaces, inheritance | 7 | 162 | 6 |
| 3: type checks | 8 | 161 | 6 |
| 4: exceptions | 8 | 161 | 5 |
| 5: objects | 10 | 159 | 5 |
| 6: data, enum, sealed | 13 | 156 | 4 |
| 7: arrays, vararg, spread | 13 | 156 | 4 |
| 8: Char, Float, Byte, Short | 13 | 156 | 4 |
| 9: inliner | 13 | 156 | 4 |
| 10: lambdas with receivers | 13 | 156 | 3 |
| 11: extensions | 16 | 153 | 3 |
| 12: operator conventions | 18 | 151 | 3 |
| 13: identity, class literals, references | 22 | 147 | 2 |
| 14: class delegation | 24 | 145 | 2 |
| 15: secondary constructors | 24 | 145 | 2 |
| 16: local functions | 24 | 145 | 2 |
| 17: expect and actual | 41 | 128 | 2 |
| 18: annotation classes | 43 | 126 | 2 |
| 19: value classes | 43 | 126 | 2 |
| 20: unsigned integers | 43 | 126 | 2 |
| definite assignment | 43 | 126 | 2 |
| 21: inline functions | 57 | 112 | 1 |
| 22: reified type parameters | 57 | 112 | 1 |
| 23: labels, do-while | 57 | 112 | 1 |
| 24: delegated properties | 57 | 112 | 1 |
| 25: object expressions | 58 | 111 | 1 |
| 26: destructuring lambda parameters | 58 | 111 | 1 |
| 42-49: coroutines | 112 | 57 | 1 |
| 52: bitwise operators | 114 | 55 | 1 |
| 53: typealias | 127 | 42 | 1 |
| 54: setters, backing fields | 129 | 40 | 1 |
| 55: lateinit, tailrec | 133 | 36 | 1 |
| 56-57: collection supertypes | 133 | 36 | 1 |
| 58: member extension functions | 136 | 33 | 1 |
| 59: reified member type parameters | 136 | 33 | 1 |
| 60: inner and local classes | 143 | 26 | 1 |
| 61: when guards, anonymous functions, local extension functions | 147 | 22 | 1 |
| 62a: @JvmField and @JvmStatic in objects | 151 | 18 | 1 |
| 62b: generic value classes | 154 | 15 | 1 |
| 63: definitely non-nullable types | 154 | 15 | 1 |
| 70a: inline property accessors | 154 | 15 | 1 |
| 70b: where clauses | 156 | 13 | 1 |
| 70c: fun interfaces | 160 | 9 | 1 |
| delegated extension properties | 160 | 9 | 1 |
| qualified super | 160 | 9 | 1 |
| context parameters of top-level functions | 160 | 9 | 1 |
| also supported | 169 | 0 | – |

### Features left, by files blocked

169 of 169 files use no feature that kc doesn't support.

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

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| kotlinx-coroutines-core/common/src/AbstractCoroutine.kt | 136 | (annotation, as, class literal, function type with receiver, identity equality, inheritance, is, modifier abstract, modifier final, modifier open, modifier override, modifier protected, modifier suspend, package, super) |
| kotlinx-coroutines-core/common/src/Annotations.kt | 118 | (annotation, modifier annotation, package) |
| kotlinx-coroutines-core/common/src/Await.kt | 120 | (arrays, extension function, inheritance, inline-only call, modifier inner, modifier lateinit, modifier override, modifier suspend, modifier vararg, named argument, nested class, package, property getter, property setter, property without initializer) |
| kotlinx-coroutines-core/common/src/Builders.common.kt | 599 | (annotation, as, class literal, default value, extension function, function type with receiver, identity equality, inheritance, is, labels, modifier const, modifier expect, modifier inline, modifier noinline, modifier open, modifier operator, modifier override, modifier suspend, named argument, package, require, check, error, throw) |
| kotlinx-coroutines-core/common/src/CancellableContinuation.kt | 505 | (annotation, class literal, default value, extension function, function without body, inheritance, inline-only call, interface, is, member extension function, modifier crossinline, modifier inline, modifier override, modifier suspend, named argument, package, property without initializer, throw, try) |
| kotlinx-coroutines-core/common/src/CancellableContinuationImpl.kt | 700 | (annotation, as, bitwise operators, class literal, default value, extension function, extension property, function without body, identity equality, inheritance, inline-only call, interface, is, member extension function, modifier const, modifier data, modifier final, modifier inline, modifier open, modifier override, modifier protected, named argument, nested class, object, package, property getter, require, check, error, super, throw, try) |
| kotlinx-coroutines-core/common/src/CloseableCoroutineDispatcher.kt | 24 | (annotation, function without body, inheritance, inline-only call, modifier abstract, modifier expect, modifier override, package) |
| kotlinx-coroutines-core/common/src/CompletableDeferred.kt | 121 | (annotation, as, class delegation, class literal, default value, extension function, function without body, inheritance, inline-only call, interface, modifier override, modifier suspend, package) |
| kotlinx-coroutines-core/common/src/CompletableJob.kt | 44 | (annotation, class literal, function without body, inheritance, interface, package) |
| kotlinx-coroutines-core/common/src/CompletionHandler.common.kt | 27 | (package, typealias) |
| kotlinx-coroutines-core/common/src/CompletionState.kt | 50 | (annotation, as, default value, extension function, inheritance, inline-only call, is, modifier open, modifier override, package) |
| kotlinx-coroutines-core/common/src/CoroutineContext.common.kt | 36 | (annotation, extension function, extension property, function without body, modifier expect, modifier inline, package, property without initializer) |
| kotlinx-coroutines-core/common/src/CoroutineDispatcher.kt | 308 | (annotation, as, default value, function without body, inheritance, modifier abstract, modifier companion, modifier final, modifier open, modifier operator, modifier override, package) |
| kotlinx-coroutines-core/common/src/CoroutineExceptionHandler.kt | 302 | (annotation, function without body, identity equality, inheritance, inline-only call, interface, is, modifier companion, modifier crossinline, modifier inline, modifier override, object expression, package, try) |
| kotlinx-coroutines-core/common/src/CoroutineName.kt | 25 | (inheritance, modifier companion, modifier data, modifier override, package) |
| kotlinx-coroutines-core/common/src/CoroutineScope.kt | 1307 | (annotation, class literal, default value, extension function, extension property, function type with receiver, inheritance, interface, modifier inline, modifier operator, modifier override, modifier suspend, object, package, property getter, property without initializer, require, check, error) |
| kotlinx-coroutines-core/common/src/CoroutineStart.kt | 371 | (annotation, function type with receiver, identity equality, modifier enum, modifier operator, modifier suspend, package) |
| kotlinx-coroutines-core/common/src/Debug.common.kt | 42 | (annotation, extension property, function without body, interface, modifier expect, package, property without initializer, where clause) |
| kotlinx-coroutines-core/common/src/Deferred.kt | 102 | (annotation, class literal, function without body, inheritance, interface, modifier suspend, package, property without initializer) |
| kotlinx-coroutines-core/common/src/Delay.kt | 158 | (annotation, as, extension function, extension property, function without body, inheritance, interface, labels, modifier suspend, package) |
| kotlinx-coroutines-core/common/src/Dispatchers.common.kt | 73 | (modifier expect, object, package, property without initializer) |
| kotlinx-coroutines-core/common/src/EventLoop.common.kt | 539 | (annotation, as, bitwise operators, default value, function without body, identity equality, inheritance, inline-only call, is, modifier abstract, modifier const, modifier crossinline, modifier expect, modifier final, modifier inline, modifier inner, modifier open, modifier override, modifier protected, named argument, nested class, object, package, property getter, property setter, property without initializer, require, check, error, super, typealias, when subject variable) |
| kotlinx-coroutines-core/common/src/Exceptions.common.kt | 26 | (annotation, function without body, inheritance, modifier expect, modifier open, package, property without initializer) |
| kotlinx-coroutines-core/common/src/Guidance.kt | 505 | (annotation, as, default value, extension function, function type with receiver, modifier suspend, package, throw) |
| kotlinx-coroutines-core/common/src/Job.kt | 691 | (annotation, as, callable reference, class literal, default value, extension function, extension property, fun interface, function without body, inheritance, interface, is, modifier companion, modifier operator, modifier override, modifier suspend, named argument, object, package, property getter, property without initializer, require, check, error, throw) |
| kotlinx-coroutines-core/common/src/JobSupport.kt | 1582 | (annotation, as, bitwise operators, callable reference, class literal, default value, extension function, extension property, function without body, identity equality, inheritance, inline-only call, interface, is, modifier abstract, modifier const, modifier final, modifier inline, modifier inner, modifier lateinit, modifier open, modifier override, modifier protected, modifier suspend, modifier tailrec, named argument, nested class, package, property getter, property setter, property without initializer, qualified this, require, check, error, super, throw, try, uninitialized local variable, when subject variable) |
| kotlinx-coroutines-core/common/src/MainCoroutineDispatcher.kt | 73 | (annotation, identity equality, inheritance, modifier abstract, modifier override, modifier protected, package, property without initializer, try) |
| kotlinx-coroutines-core/common/src/NonCancellable.kt | 282 | (annotation, class literal, inheritance, modifier const, modifier override, modifier suspend, object, package, property getter, throw) |
| kotlinx-coroutines-core/common/src/Runnable.common.kt | 14 | (fun interface, function without body, modifier expect, package) |
| kotlinx-coroutines-core/common/src/SchedulerTask.common.kt | 16 | (inheritance, modifier abstract, modifier expect, package) |
| kotlinx-coroutines-core/common/src/Supervisor.kt | 118 | (annotation, class literal, default value, function type with receiver, inheritance, modifier override, modifier suspend, package) |
| kotlinx-coroutines-core/common/src/Timeout.kt | 282 | (annotation, as, class literal, function type with receiver, identity equality, inheritance, inline-only call, modifier override, modifier suspend, package, secondary constructor, super, throw, try) |
| kotlinx-coroutines-core/common/src/Unconfined.kt | 42 | (annotation, inheritance, modifier companion, modifier override, object, package, throw) |
| kotlinx-coroutines-core/common/src/Waiter.kt | 17 | (function without body, interface, package) |
| kotlinx-coroutines-core/common/src/Yield.kt | 166 | (as, labels, modifier suspend, package) |
| kotlinx-coroutines-core/common/src/channels/Broadcast.kt | 123 | (annotation, class delegation, default value, extension function, function type with receiver, inheritance, modifier final, modifier open, modifier override, modifier protected, modifier suspend, named argument, package, property getter, super) |
| kotlinx-coroutines-core/common/src/channels/BroadcastChannel.kt | 362 | (annotation, as, class delegation, default value, function without body, identity equality, inheritance, inline-only call, interface, is, modifier inner, modifier override, modifier suspend, named argument, nested class, package, property getter, qualified this, require, check, error, secondary constructor, super, throw, try) |
| kotlinx-coroutines-core/common/src/channels/BufferOverflow.kt | 39 | (modifier enum, package) |
| kotlinx-coroutines-core/common/src/channels/BufferedChannel.kt | 3091 | (Char, annotation, as, bitwise operators, callable reference, class delegation, default value, extension function, extension property, function without body, identity equality, inheritance, inline-only call, is, labels, modifier const, modifier final, modifier inline, modifier inner, modifier open, modifier override, modifier protected, modifier suspend, named argument, nested class, package, property getter, property without initializer, qualified this, require, check, error, throw, when subject variable) |
| kotlinx-coroutines-core/common/src/channels/Channel.kt | 1528 | (annotation, as, class literal, default value, extension function, function without body, generic value class, identity equality, inheritance, inline-only call, interface, is, modifier companion, modifier const, modifier inline, modifier open, modifier operator, modifier override, modifier suspend, modifier value, nested class, package, property without initializer, require, check, error, throw) |
| kotlinx-coroutines-core/common/src/channels/ChannelCoroutine.kt | 38 | (annotation, class delegation, inheritance, modifier final, modifier open, modifier override, modifier protected, package) |
| kotlinx-coroutines-core/common/src/channels/Channels.common.kt | 271 | (annotation, as, callable reference, class literal, extension function, function type with receiver, inline-only call, modifier const, modifier inline, modifier suspend, package, throw, try) |
| kotlinx-coroutines-core/common/src/channels/ConflatedBufferedChannel.kt | 111 | (annotation, as, class literal, default value, identity equality, inheritance, inline-only call, modifier open, modifier override, modifier suspend, named argument, package, property getter, require, check, error, super, throw) |
| kotlinx-coroutines-core/common/src/channels/Deprecated.kt | 661 | (annotation, as, default value, extension function, function type with receiver, inline-only call, modifier infix, modifier inline, modifier suspend, modifier vararg, named argument, package, qualified this, require, check, error, suspend function type, throw, try) |
| kotlinx-coroutines-core/common/src/channels/Produce.kt | 358 | (annotation, default value, extension function, function type with receiver, identity equality, inheritance, interface, modifier override, modifier suspend, named argument, package, property getter, property without initializer, require, check, error, super, try) |
| kotlinx-coroutines-core/common/src/flow/Builders.kt | 349 | (annotation, anonymous function, arrays, default value, extension function, function type with receiver, inheritance, inline-only call, modifier open, modifier override, modifier suspend, modifier vararg, object, package, super, suspend function type, throw) |
| kotlinx-coroutines-core/common/src/flow/Channels.kt | 157 | (annotation, default value, extension function, inheritance, modifier override, modifier suspend, named argument, package, require, check, error, super, throw, try) |
| kotlinx-coroutines-core/common/src/flow/Flow.kt | 246 | (annotation, function without body, inheritance, interface, modifier abstract, modifier final, modifier override, modifier suspend, package, try) |
| kotlinx-coroutines-core/common/src/flow/FlowCollector.kt | 32 | (fun interface, function without body, modifier suspend, package) |
| kotlinx-coroutines-core/common/src/flow/Migration.kt | 491 | (annotation, default value, extension function, function type with receiver, inline-only call, package, suspend function type, throw) |
| kotlinx-coroutines-core/common/src/flow/SharedFlow.kt | 740 | (annotation, arrays, as, bitwise operators, class literal, default value, extension function, function without body, identity equality, inheritance, inline-only call, interface, is, labels, modifier open, modifier override, modifier protected, modifier suspend, named argument, nested class, package, property getter, property without initializer, require, check, error, try, uninitialized local variable, when subject variable) |
| kotlinx-coroutines-core/common/src/flow/SharingStarted.kt | 204 | (annotation, default value, extension function, fun interface, function without body, inheritance, inline-only call, is, modifier companion, modifier enum, modifier override, package, require, check, error) |
| kotlinx-coroutines-core/common/src/flow/StateFlow.kt | 432 | (annotation, arrays, as, bitwise operators, class literal, extension function, function without body, identity equality, inheritance, inline-only call, interface, is, labels, modifier inline, modifier override, modifier suspend, package, property getter, property setter, property without initializer, throw, try, uninitialized local variable) |
| kotlinx-coroutines-core/common/src/flow/internal/AbstractSharedFlow.kt | 129 | (annotation, arrays, as, class literal, function without body, inheritance, inline-only call, modifier abstract, modifier inline, modifier override, modifier protected, package, property getter, uninitialized local variable, when subject variable) |
| kotlinx-coroutines-core/common/src/flow/internal/ChannelFlow.kt | 239 | (annotation, as, default value, extension function, function without body, identity equality, inheritance, inline-only call, interface, is, modifier abstract, modifier open, modifier override, modifier protected, modifier suspend, named argument, package, property getter, super, suspend function type, uninitialized local variable) |
| kotlinx-coroutines-core/common/src/flow/internal/Combine.kt | 139 | (Byte and Short, annotation, arrays, as, extension function, function type with receiver, identity equality, inline-only call, modifier suspend, named argument, package, prefix increment, suspend function type, throw, try, typealias) |
| kotlinx-coroutines-core/common/src/flow/internal/FlowCoroutine.kt | 58 | (function type with receiver, inheritance, is, modifier override, modifier suspend, package, qualified this) |
| kotlinx-coroutines-core/common/src/flow/internal/FlowExceptions.common.kt | 33 | (annotation, extension function, identity equality, inheritance, modifier expect, modifier inline, package, property without initializer, throw) |
| kotlinx-coroutines-core/common/src/flow/internal/Merge.kt | 95 | (default value, function type with receiver, inheritance, inline-only call, is, modifier override, modifier suspend, named argument, package, try) |
| kotlinx-coroutines-core/common/src/flow/internal/NopCollector.kt | 9 | (inheritance, modifier override, modifier suspend, object, package) |
| kotlinx-coroutines-core/common/src/flow/internal/NullSurrogate.kt | 26 | (annotation, package) |
| kotlinx-coroutines-core/common/src/flow/internal/SafeCollector.common.kt | 110 | (annotation, as, extension function, function type with receiver, function without body, identity equality, inheritance, is, labels, modifier crossinline, modifier expect, modifier inline, modifier override, modifier suspend, modifier tailrec, object expression, package, property without initializer, require, check, error) |
| kotlinx-coroutines-core/common/src/flow/internal/SendingCollector.kt | 16 | (annotation, inheritance, modifier override, modifier suspend, package) |
| kotlinx-coroutines-core/common/src/flow/operators/Context.kt | 289 | (annotation, default value, extension function, inheritance, interface, is, modifier override, modifier suspend, named argument, package, require, check, error) |
| kotlinx-coroutines-core/common/src/flow/operators/Delay.kt | 406 | (annotation, extension function, identity equality, inline-only call, named argument, package, require, check, error, throw) |
| kotlinx-coroutines-core/common/src/flow/operators/Distinct.kt | 77 | (annotation, as, extension function, identity equality, inheritance, is, modifier override, modifier suspend, named argument, package) |
| kotlinx-coroutines-core/common/src/flow/operators/Emitters.kt | 215 | (annotation, extension function, function type with receiver, identity equality, inheritance, is, modifier crossinline, modifier inline, modifier override, modifier suspend, package, throw, try) |
| kotlinx-coroutines-core/common/src/flow/operators/Errors.kt | 219 | (annotation, default value, do-while, extension function, function type with receiver, inline-only call, is, modifier suspend, package, require, check, error, suspend function type, throw, try, uninitialized local variable) |
| kotlinx-coroutines-core/common/src/flow/operators/Limit.kt | 140 | (annotation, extension function, function type with receiver, inheritance, inline-only call, modifier crossinline, modifier inline, modifier override, modifier suspend, named argument, object expression, package, prefix increment, require, check, error, suspend function type, throw, try) |
| kotlinx-coroutines-core/common/src/flow/operators/Lint.kt | 197 | (annotation, as, default value, extension function, extension property, function type with receiver, inline-only call, modifier inline, modifier noinline, modifier suspend, package, property getter, suspend function type, throw) |
| kotlinx-coroutines-core/common/src/flow/operators/Merge.kt | 213 | (annotation, default value, extension function, function type with receiver, inline-only call, modifier const, modifier crossinline, modifier inline, modifier suspend, modifier vararg, package, require, check, error, suspend function type) |
| kotlinx-coroutines-core/common/src/flow/operators/Share.kt | 481 | (annotation, as, class delegation, class literal, default value, extension function, function type with receiver, identity equality, inheritance, inline-only call, is, modifier override, modifier suspend, named argument, package, throw, try) |
| kotlinx-coroutines-core/common/src/flow/operators/Transform.kt | 166 | (annotation, as, extension function, identity equality, inline-only call, is, modifier crossinline, modifier inline, modifier reified, package, require, check, error, suspend function type) |
| kotlinx-coroutines-core/common/src/flow/operators/Zip.kt | 327 | (annotation, arrays, as, extension function, function type with receiver, inline-only call, modifier crossinline, modifier inline, modifier reified, modifier suspend, modifier vararg, named argument, package, qualified this, suspend function type) |
| kotlinx-coroutines-core/common/src/flow/terminal/Collect.kt | 131 | (annotation, as, extension function, inheritance, inline-only call, modifier crossinline, modifier inline, modifier noinline, modifier override, modifier suspend, object expression, package, suspend function type, throw) |
| kotlinx-coroutines-core/common/src/flow/terminal/Collection.kt | 363 | (annotation, default value, extension function, inline-only call, modifier crossinline, modifier inline, modifier suspend, package, suspend function type) |
| kotlinx-coroutines-core/common/src/flow/terminal/Count.kt | 32 | (annotation, extension function, inline-only call, modifier suspend, package, prefix increment, suspend function type) |
| kotlinx-coroutines-core/common/src/flow/terminal/Logic.kt | 107 | (annotation, extension function, inline-only call, modifier suspend, package, suspend function type) |
| kotlinx-coroutines-core/common/src/flow/terminal/Reduce.kt | 167 | (annotation, as, extension function, identity equality, inline-only call, modifier crossinline, modifier inline, modifier suspend, package, require, check, error, suspend function type, throw) |
| kotlinx-coroutines-core/common/src/internal/Concurrent.common.kt | 37 | (annotation, extension function, extension property, function type with receiver, function without body, modifier annotation, modifier expect, modifier inline, package, property getter, property setter) |
| kotlinx-coroutines-core/common/src/internal/ConcurrentLinkedList.kt | 264 | (annotation, as, bitwise operators, extension function, function without body, generic value class, identity equality, inheritance, inline-only call, modifier abstract, modifier const, modifier inline, modifier noinline, modifier override, modifier value, package, property without initializer, require, check, error) |
| kotlinx-coroutines-core/common/src/internal/CoroutineExceptionHandlerImpl.common.kt | 68 | (function without body, inheritance, modifier expect, object, package, property without initializer, try) |
| kotlinx-coroutines-core/common/src/internal/DispatchedContinuation.kt | 321 | (annotation, as, class delegation, default value, extension function, identity equality, inheritance, inline-only call, is, modifier inline, modifier override, named argument, package, property getter, require, check, error, throw, try) |
| kotlinx-coroutines-core/common/src/internal/DispatchedTask.kt | 219 | (annotation, as, extension function, extension property, function without body, inheritance, inline-only call, is, modifier abstract, modifier const, modifier final, modifier inline, modifier open, modifier override, named argument, package, property without initializer, try) |
| kotlinx-coroutines-core/common/src/internal/InlineList.kt | 44 | (annotation, as, default value, generic value class, inline-only call, is, modifier inline, modifier operator, modifier value, package) |
| kotlinx-coroutines-core/common/src/internal/InternalAnnotations.common.kt | 13 | (annotation, modifier annotation, modifier expect, package) |
| kotlinx-coroutines-core/common/src/internal/LimitedDispatcher.kt | 153 | (annotation, as, class delegation, extension function, inheritance, inline-only call, modifier inline, modifier inner, modifier override, named argument, nested class, package, prefix increment, qualified this, require, check, error, super, throw, try, when subject variable) |
| kotlinx-coroutines-core/common/src/internal/LocalAtomics.common.kt | 15 | (function without body, modifier expect, package) |
| kotlinx-coroutines-core/common/src/internal/LockFreeLinkedList.common.kt | 23 | (function without body, inheritance, modifier expect, modifier final, modifier inline, modifier open, modifier override, package, property without initializer) |
| kotlinx-coroutines-core/common/src/internal/LockFreeTaskQueue.kt | 303 | (@JvmField in object, annotation, as, bitwise operators, extension function, identity equality, inline-only call, is, modifier companion, modifier const, modifier infix, modifier inline, modifier open, nested class, package, require, check, error, typealias) |
| kotlinx-coroutines-core/common/src/internal/MainDispatcherFactory.kt | 25 | (annotation, function without body, interface, package, property without initializer) |
| kotlinx-coroutines-core/common/src/internal/NamedDispatcher.kt | 25 | (annotation, as, class delegation, inheritance, modifier override, package) |
| kotlinx-coroutines-core/common/src/internal/OnUndeliveredElement.kt | 36 | (default value, extension function, identity equality, inheritance, inline-only call, package, try, typealias) |
| kotlinx-coroutines-core/common/src/internal/ProbesSupport.common.kt | 7 | (function without body, modifier expect, modifier inline, package) |
| kotlinx-coroutines-core/common/src/internal/Scopes.kt | 43 | (annotation, as, inheritance, modifier final, modifier open, modifier override, package) |
| kotlinx-coroutines-core/common/src/internal/StackTraceRecovery.common.kt | 47 | (annotation, extension function, function without body, interface, modifier expect, modifier inline, modifier suspend, package, property without initializer) |
| kotlinx-coroutines-core/common/src/internal/Symbol.kt | 15 | (annotation, as, identity equality, modifier inline, modifier override, package) |
| kotlinx-coroutines-core/common/src/internal/Synchronized.common.kt | 27 | (annotation, class literal, function without body, inline-only call, modifier expect, modifier inline, modifier open, package) |
| kotlinx-coroutines-core/common/src/internal/SystemProps.common.kt | 72 | (annotation, default value, function without body, inline-only call, modifier expect, package, require, check, error) |
| kotlinx-coroutines-core/common/src/internal/ThreadContext.common.kt | 5 | (function without body, modifier expect, package) |
| kotlinx-coroutines-core/common/src/internal/ThreadLocal.common.kt | 15 | (function without body, modifier expect, package) |
| kotlinx-coroutines-core/common/src/internal/ThreadSafeHeap.kt | 158 | (annotation, arrays, as, identity equality, inheritance, inline-only call, interface, labels, modifier inline, modifier open, modifier tailrec, package, property getter, property setter, property without initializer, where clause) |
| kotlinx-coroutines-core/common/src/intrinsics/Cancellable.kt | 64 | (annotation, anonymous function, extension function, inline-only call, is, modifier inline, package, suspend function type, throw, try) |
| kotlinx-coroutines-core/common/src/intrinsics/Undispatched.kt | 104 | (annotation, anonymous function, as, extension function, function type with receiver, identity equality, inline-only call, is, modifier suspend, named argument, package, suspend function type, throw, try) |
| kotlinx-coroutines-core/common/src/selects/OnTimeout.kt | 61 | (annotation, as, callable reference, extension function, inline-only call, named argument, package, property getter, qualified this, suspend function type) |
| kotlinx-coroutines-core/common/src/selects/Select.kt | 900 | (annotation, as, class literal, default value, extension function, function type with receiver, function without body, identity equality, inheritance, inline-only call, interface, is, labels, member extension function, modifier const, modifier crossinline, modifier enum, modifier inline, modifier inner, modifier open, modifier operator, modifier override, modifier sealed, modifier suspend, named argument, nested class, package, property getter, property without initializer, qualified this, require, check, error, suspend function type, try, typealias, when subject variable) |
| kotlinx-coroutines-core/common/src/selects/SelectOld.kt | 141 | (annotation, as, extension function, function type with receiver, inheritance, modifier crossinline, modifier inline, modifier suspend, named argument, package, try) |
| kotlinx-coroutines-core/common/src/selects/SelectUnbiased.kt | 64 | (annotation, class literal, extension function, function type with receiver, inheritance, inline-only call, member extension function, modifier crossinline, modifier inline, modifier open, modifier override, modifier suspend, package, super, suspend function type, try) |
| kotlinx-coroutines-core/common/src/selects/WhileSelect.kt | 28 | (annotation, function type with receiver, modifier crossinline, modifier inline, modifier suspend, package) |
| kotlinx-coroutines-core/common/src/sync/Mutex.kt | 313 | (annotation, as, callable reference, class delegation, class literal, default value, extension function, function without body, identity equality, inheritance, inline-only call, interface, modifier const, modifier inline, modifier inner, modifier open, modifier override, modifier protected, modifier suspend, named argument, nested class, package, property without initializer, qualified this, require, check, error, try) |
| kotlinx-coroutines-core/common/src/sync/Semaphore.kt | 395 | (annotation, as, callable reference, class literal, default value, extension function, function without body, identity equality, inheritance, inline-only call, interface, is, labels, modifier inline, modifier open, modifier override, modifier protected, modifier suspend, named argument, package, property without initializer, qualified this, require, check, error, suspend function type, try) |
| kotlinx-coroutines-core/concurrent/src/Builders.concurrent.kt | 178 | (annotation, class literal, default value, function type with receiver, function without body, modifier expect, modifier suspend, package, uninitialized local variable) |
| kotlinx-coroutines-core/concurrent/src/Dispatchers.kt | 47 | (annotation, extension property, modifier expect, package, property without initializer) |
| kotlinx-coroutines-core/concurrent/src/MultithreadedDispatchers.common.kt | 65 | (annotation, function without body, modifier expect, package) |
| kotlinx-coroutines-core/concurrent/src/channels/Channels.kt | 60 | (annotation, extension function, inline-only call, package) |
| kotlinx-coroutines-core/concurrent/src/internal/LockFreeLinkedList.kt | 287 | (annotation, as, bitwise operators, callable reference, identity equality, inheritance, inline-only call, is, modifier actual, modifier final, modifier inline, modifier open, modifier override, modifier tailrec, package, property getter, require, check, error, typealias) |
| kotlinx-coroutines-core/concurrent/src/internal/OnDemandAllocatingPool.kt | 102 | (annotation, bitwise operators, extension function, inline-only call, modifier const, modifier inline, modifier override, package) |
| kotlinx-coroutines-core/jvm/src/AbstractTimeSource.kt | 70 | (annotation, function without body, modifier abstract, modifier actual, modifier inline, package) |
| kotlinx-coroutines-core/jvm/src/Builders.kt | 71 | (annotation, as, class literal, default value, function type with receiver, inheritance, inline-only call, modifier actual, modifier override, modifier suspend, package, throw, try) |
| kotlinx-coroutines-core/jvm/src/CoroutineContext.kt | 330 | (Char, annotation, as, destructuring lambda parameter, extension function, extension property, identity equality, inheritance, inline-only call, is, local function, modifier actual, modifier companion, modifier const, modifier data, modifier inline, modifier override, modifier tailrec, object, package, property getter, property without initializer, try) |
| kotlinx-coroutines-core/jvm/src/Debug.kt | 92 | (annotation, class literal, inline-only call, modifier actual, modifier const, modifier inline, package, require, check, error, throw) |
| kotlinx-coroutines-core/jvm/src/DebugStrings.kt | 17 | (class literal, extension function, extension property, inline-only call, is, modifier actual, package, property getter) |
| kotlinx-coroutines-core/jvm/src/DefaultExecutor.kt | 194 | (annotation, as, inheritance, inline-only call, is, modifier actual, modifier const, modifier override, object, package, property getter, property without initializer, qualified this, super, throw, try) |
| kotlinx-coroutines-core/jvm/src/Dispatchers.kt | 103 | (@JvmStatic property, annotation, extension property, modifier actual, modifier const, object, package) |
| kotlinx-coroutines-core/jvm/src/EventLoop.kt | 24 | (identity equality, inheritance, modifier abstract, modifier actual, modifier crossinline, modifier inline, modifier open, modifier override, modifier protected, package, property without initializer) |
| kotlinx-coroutines-core/jvm/src/Exceptions.kt | 73 | (annotation, arrays, identity equality, inline-only call, is, local function, modifier actual, modifier override, package, super, typealias) |
| kotlinx-coroutines-core/jvm/src/Executors.kt | 215 | (annotation, as, extension function, function without body, identity equality, inheritance, inline-only call, is, modifier abstract, modifier actual, modifier companion, modifier override, package, property without initializer, try, typealias) |
| kotlinx-coroutines-core/jvm/src/Future.kt | 31 | (annotation, extension function, inheritance, modifier override, named argument, package) |
| kotlinx-coroutines-core/jvm/src/GuidanceJvm.kt | 23 | (annotation, as, modifier suspend, package) |
| kotlinx-coroutines-core/jvm/src/Interruptible.kt | 160 | (default value, inheritance, modifier const, modifier override, modifier suspend, named argument, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/src/Runnable.kt | 10 | (modifier actual, package, typealias) |
| kotlinx-coroutines-core/jvm/src/SchedulerTask.kt | 5 | (modifier actual, package, typealias) |
| kotlinx-coroutines-core/jvm/src/ThreadContextElement.kt | 284 | (annotation, default value, extension function, function without body, identity equality, inheritance, interface, modifier inline, modifier suspend, package, require, check, error) |
| kotlinx-coroutines-core/jvm/src/ThreadPoolDispatcher.kt | 17 | (annotation, inline-only call, modifier actual, package, require, check, error) |
| kotlinx-coroutines-core/jvm/src/channels/Actor.kt | 198 | (annotation, as, callable reference, default value, extension function, function type with receiver, inheritance, inline-only call, interface, modifier open, modifier override, modifier suspend, named argument, package, property without initializer, super) |
| kotlinx-coroutines-core/jvm/src/channels/TickerChannels.kt | 107 | (annotation, default value, modifier enum, modifier suspend, named argument, package, require, check, error) |
| kotlinx-coroutines-core/jvm/src/debug/CoroutineDebugging.kt | 61 | (class literal, extension function, inline-only call, package) |
| kotlinx-coroutines-core/jvm/src/debug/internal/AgentInstallationType.kt | 18 | (annotation, object, package) |
| kotlinx-coroutines-core/jvm/src/debug/internal/AgentPremain.kt | 72 | (@JvmStatic in object, annotation, arrays, inheritance, inline-only call, modifier override, object, package, try) |
| kotlinx-coroutines-core/jvm/src/debug/internal/ConcurrentWeakMap.kt | 280 | (annotation, as, bitwise operators, collection supertype, default value, extension function, identity equality, inheritance, inline-only call, is, labels, modifier const, modifier inner, modifier lateinit, modifier override, nested class, package, prefix increment, property getter, property without initializer, qualified this, require, check, error, throw, try, uninitialized local variable) |
| kotlinx-coroutines-core/jvm/src/debug/internal/DebugCoroutineInfo.kt | 26 | (annotation, package) |
| kotlinx-coroutines-core/jvm/src/debug/internal/DebugCoroutineInfoImpl.kt | 178 | (annotation, as, extension function, inline-only call, modifier const, modifier override, modifier suspend, modifier tailrec, package, prefix increment, property getter, property setter) |
| kotlinx-coroutines-core/jvm/src/debug/internal/DebugProbes.kt | 18 | (annotation, package) |
| kotlinx-coroutines-core/jvm/src/debug/internal/DebugProbesImpl.kt | 622 | (Char, annotation, arrays, as, class delegation, destructuring declaration, extension function, extension property, identity equality, inheritance, inline-only call, is, modifier crossinline, modifier inline, modifier override, modifier tailrec, named argument, nested class, object, package, prefix increment, property getter, property without initializer, qualified this, require, check, error, try, uninitialized local variable) |
| kotlinx-coroutines-core/jvm/src/debug/internal/DebuggerInfo.kt | 24 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/jvm/src/debug/internal/StackTraceFrame.kt | 13 | (inheritance, modifier override, package) |
| kotlinx-coroutines-core/jvm/src/flow/internal/FlowExceptions.kt | 28 | (annotation, arrays, inheritance, local function, modifier actual, modifier override, package, super) |
| kotlinx-coroutines-core/jvm/src/flow/internal/SafeCollector.kt | 181 | (annotation, as, callable reference, class delegation, identity equality, inheritance, inline-only call, is, labels, local function, modifier actual, modifier override, modifier suspend, object, package, require, check, error, super, throw, try, uninitialized local variable) |
| kotlinx-coroutines-core/jvm/src/future/Future.kt | 247 | (annotation, as, default value, extension function, function type with receiver, identity equality, inheritance, inline-only call, modifier override, modifier suspend, named argument, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/src/internal/Concurrent.kt | 42 | (annotation, as, class literal, extension function, modifier actual, modifier annotation, modifier inline, package, try, typealias) |
| kotlinx-coroutines-core/jvm/src/internal/CoroutineExceptionHandlerImpl.kt | 60 | (annotation, arrays, class literal, local function, modifier actual, modifier override, package, require, check, error, try) |
| kotlinx-coroutines-core/jvm/src/internal/ExceptionsConstructor.kt | 133 | (annotation, as, callable reference, class literal, default value, extension function, function without body, inheritance, inline-only call, is, modifier abstract, modifier override, modifier tailrec, object, object expression, package, try, typealias) |
| kotlinx-coroutines-core/jvm/src/internal/FastServiceLoader.kt | 168 | (Char, annotation, class literal, extension function, identity equality, inline-only call, modifier const, modifier inline, object, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/src/internal/InternalAnnotations.kt | 3 | (modifier actual, package, typealias) |
| kotlinx-coroutines-core/jvm/src/internal/LocalAtomics.kt | 3 | (modifier actual, package, typealias) |
| kotlinx-coroutines-core/jvm/src/internal/MainDispatchers.kt | 129 | (@JvmField in object, annotation, class literal, default value, extension function, inheritance, inline-only call, is, modifier const, modifier override, object, package, property getter, throw, try) |
| kotlinx-coroutines-core/jvm/src/internal/ProbesSupport.kt | 12 | (annotation, modifier actual, modifier inline, package) |
| kotlinx-coroutines-core/jvm/src/internal/ResizableAtomicArray.kt | 35 | (annotation, modifier operator, package) |
| kotlinx-coroutines-core/jvm/src/internal/StackTraceRecovery.kt | 209 | (annotation, arrays, as, destructuring declaration, extension function, identity equality, inline-only call, is, modifier actual, modifier const, modifier inline, modifier suspend, package, throw, typealias) |
| kotlinx-coroutines-core/jvm/src/internal/Synchronized.kt | 16 | (annotation, inline-only call, modifier actual, modifier inline, package, typealias) |
| kotlinx-coroutines-core/jvm/src/internal/SystemProps.kt | 16 | (annotation, modifier actual, package, try) |
| kotlinx-coroutines-core/jvm/src/internal/ThreadContext.kt | 127 | (annotation, anonymous function, arrays, as, identity equality, inheritance, is, modifier actual, modifier data, modifier operator, modifier override, package) |
| kotlinx-coroutines-core/jvm/src/internal/ThreadLocal.kt | 7 | (modifier actual, package, typealias) |
| kotlinx-coroutines-core/jvm/src/scheduling/CoroutineScheduler.kt | 1010 | (@JvmField in object, annotation, as, bitwise operators, default value, extension function, identity equality, inheritance, inline property accessor, inline-only call, is, labels, modifier companion, modifier const, modifier enum, modifier inline, modifier inner, modifier override, named argument, nested class, package, prefix increment, property backing field, property setter, qualified this, require, check, error, secondary constructor, throw, try, uninitialized local variable) |
| kotlinx-coroutines-core/jvm/src/scheduling/Dispatcher.kt | 152 | (annotation, default value, inheritance, modifier open, modifier override, named argument, object, package, property getter, require, check, error, super, throw) |
| kotlinx-coroutines-core/jvm/src/scheduling/Tasks.kt | 106 | (annotation, extension function, extension property, function without body, inheritance, inline-only call, modifier abstract, modifier const, modifier inline, modifier override, named argument, object, package, secondary constructor, typealias) |
| kotlinx-coroutines-core/jvm/src/scheduling/WorkQueue.kt | 245 | (bitwise operators, default value, extension function, extension property, inline-only call, modifier const, modifier inline, named argument, package, prefix increment, property getter, typealias) |
| kotlinx-coroutines-core/jvm/src/stream/Stream.kt | 28 | (extension function, inheritance, modifier override, modifier suspend, package, require, check, error, try) |
| kotlinx-coroutines-core/jvm/src/time/Time.kt | 73 | (annotation, class literal, extension function, function type with receiver, inline-only call, modifier suspend, package, suspend function type) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 456 files, 41342 lines

Directories: `kotlinx-coroutines-core/common/test`, `kotlinx-coroutines-core/concurrent/test`, `kotlinx-coroutines-core/jvm/test`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 456 | 5 |
| 1: packages, annotations | 57 | 399 | 4 |
| 2: interfaces, inheritance | 85 | 371 | 3 |
| 3: type checks | 90 | 366 | 3 |
| 4: exceptions | 137 | 319 | 2 |
| 5: objects | 138 | 318 | 2 |
| 6: data, enum, sealed | 185 | 271 | 2 |
| 7: arrays, vararg, spread | 190 | 266 | 2 |
| 8: Char, Float, Byte, Short | 190 | 266 | 2 |
| 9: inliner | 226 | 230 | 2 |
| 10: lambdas with receivers | 228 | 228 | 2 |
| 11: extensions | 243 | 213 | 1 |
| 12: operator conventions | 244 | 212 | 1 |
| 13: identity, class literals, references | 271 | 185 | 1 |
| 14: class delegation | 271 | 185 | 1 |
| 15: secondary constructors | 272 | 184 | 1 |
| 16: local functions | 273 | 183 | 1 |
| 17: expect and actual | 273 | 183 | 1 |
| 18: annotation classes | 273 | 183 | 1 |
| 19: value classes | 273 | 183 | 1 |
| 20: unsigned integers | 274 | 182 | 1 |
| definite assignment | 279 | 177 | 1 |
| 21: inline functions | 287 | 169 | 1 |
| 22: reified type parameters | 287 | 169 | 1 |
| 23: labels, do-while | 288 | 168 | 1 |
| 24: delegated properties | 288 | 168 | 1 |
| 25: object expressions | 302 | 154 | 1 |
| 26: destructuring lambda parameters | 304 | 152 | 1 |
| 42-49: coroutines | 382 | 74 | 1 |
| 52: bitwise operators | 385 | 71 | 1 |
| 53: typealias | 385 | 71 | 1 |
| 54: setters, backing fields | 385 | 71 | 1 |
| 55: lateinit, tailrec | 399 | 57 | 1 |
| 56-57: collection supertypes | 399 | 57 | 1 |
| 58: member extension functions | 408 | 48 | 1 |
| 59: reified member type parameters | 413 | 43 | 1 |
| 60: inner and local classes | 425 | 31 | 1 |
| 61: when guards, anonymous functions, local extension functions | 428 | 28 | 1 |
| 62a: @JvmField and @JvmStatic in objects | 428 | 28 | 1 |
| 62b: generic value classes | 428 | 28 | 1 |
| 63: definitely non-nullable types | 428 | 28 | 1 |
| 70a: inline property accessors | 428 | 28 | 1 |
| 70b: where clauses | 429 | 27 | 1 |
| 70c: fun interfaces | 429 | 27 | 1 |
| delegated extension properties | 429 | 27 | 1 |
| qualified super | 429 | 27 | 1 |
| context parameters of top-level functions | 429 | 27 | 1 |
| also supported | 455 | 1 | 1 |

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

`require`, `check`, `requireNotNull`, `checkNotNull`, `error` and `TODO` are built in; kc inlines the others, since milestone 22 also those with reified type parameters.

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

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| kotlinx-coroutines-core/common/test/AbstractCoroutineTest.kt | 91 | (annotation, inheritance, modifier override, named argument, object expression, package) |
| kotlinx-coroutines-core/common/test/AsyncLazyTest.kt | 182 | (annotation, inheritance, is, named argument, package, throw, try) |
| kotlinx-coroutines-core/common/test/AsyncTest.kt | 293 | (annotation, inheritance, is, named argument, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/common/test/AtomicCancellationCommonTest.kt | 155 | (annotation, inheritance, named argument, package, try) |
| kotlinx-coroutines-core/common/test/AwaitCancellationTest.kt | 24 | (annotation, inheritance, is, named argument, package, require, check, error) |
| kotlinx-coroutines-core/common/test/AwaitTest.kt | 381 | (annotation, class delegation, class literal, inheritance, inline-only call, named argument, object expression, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/common/test/BuilderContractsTest.kt | 63 | (annotation, inheritance, inline-only call, package, uninitialized local variable) |
| kotlinx-coroutines-core/common/test/CancellableContinuationHandlersTest.kt | 185 | (annotation, as, inheritance, is, local class, modifier override, named argument, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/common/test/CancellableContinuationTest.kt | 148 | (annotation, inheritance, inline-only call, package, try, uninitialized local variable) |
| kotlinx-coroutines-core/common/test/CancellableResumeOldTest.kt | 289 | (annotation, inheritance, is, modifier lateinit, named argument, package, throw, try, uninitialized local variable) |
| kotlinx-coroutines-core/common/test/CancellableResumeTest.kt | 314 | (annotation, inheritance, is, modifier lateinit, named argument, package, throw, try, uninitialized local variable) |
| kotlinx-coroutines-core/common/test/CancelledParentAttachTest.kt | 113 | (annotation, as, inheritance, inline-only call, is, modifier crossinline, modifier inline, modifier suspend, named argument, package, suspend function type, try) |
| kotlinx-coroutines-core/common/test/CompletableDeferredTest.kt | 213 | (annotation, inheritance, modifier inline, modifier reified, named argument, package, reified member type parameter, try) |
| kotlinx-coroutines-core/common/test/CompletableJobTest.kt | 99 | (annotation, function type with receiver, inheritance, is, modifier suspend, package, throw, try) |
| kotlinx-coroutines-core/common/test/CoroutineDispatcherOperatorFunInvokeTest.kt | 74 | (annotation, as, extension function, inheritance, modifier override, nested class, object expression, package, property getter, require, check, error, throw, try) |
| kotlinx-coroutines-core/common/test/CoroutineExceptionHandlerTest.kt | 44 | (annotation, inheritance, package, throw) |
| kotlinx-coroutines-core/common/test/CoroutineScopeTest.kt | 313 | (annotation, as, callable reference, inheritance, local function, modifier lateinit, modifier suspend, named argument, package, throw, try, uninitialized local variable) |
| kotlinx-coroutines-core/common/test/CoroutinesTest.kt | 330 | (annotation, as, inheritance, is, named argument, package, throw, try) |
| kotlinx-coroutines-core/common/test/DelayDurationTest.kt | 66 | (annotation, inheritance, is, modifier suspend, named argument, package, require, check, error) |
| kotlinx-coroutines-core/common/test/DelayTest.kt | 52 | (annotation, inheritance, is, modifier suspend, named argument, package, require, check, error) |
| kotlinx-coroutines-core/common/test/DispatchedContinuationTest.kt | 75 | (annotation, inheritance, modifier lateinit, named argument, package, property without initializer) |
| kotlinx-coroutines-core/common/test/DurationToMillisTest.kt | 66 | (annotation, package) |
| kotlinx-coroutines-core/common/test/EmptyContext.kt | 30 | (annotation, anonymous function, as, identity equality, inline-only call, modifier suspend, package, suspend function type, try) |
| kotlinx-coroutines-core/common/test/FailedJobTest.kt | 57 | (annotation, inheritance, is, named argument, package, throw) |
| kotlinx-coroutines-core/common/test/ImmediateYieldTest.kt | 54 | (annotation, as, inheritance, modifier override, nested class, package) |
| kotlinx-coroutines-core/common/test/JobExtensionsTest.kt | 92 | (annotation, inheritance, inline-only call, modifier inline, package, throw) |
| kotlinx-coroutines-core/common/test/JobStatesTest.kt | 162 | (annotation, inheritance, is, named argument, package, throw) |
| kotlinx-coroutines-core/common/test/JobTest.kt | 250 | (annotation, arrays, inheritance, inline-only call, local function, modifier override, named argument, nested class, package, property getter, require, check, error, throw, try) |
| kotlinx-coroutines-core/common/test/LaunchLazyTest.kt | 68 | (annotation, inheritance, named argument, package) |
| kotlinx-coroutines-core/common/test/LimitedParallelismSharedTest.kt | 59 | (annotation, inheritance, modifier override, object expression, package, throw, try) |
| kotlinx-coroutines-core/common/test/NonCancellableTest.kt | 134 | (annotation, as, inheritance, package, try) |
| kotlinx-coroutines-core/common/test/ParentCancellationTest.kt | 167 | (annotation, default value, extension function, function type with receiver, inheritance, is, modifier suspend, named argument, package, throw, try) |
| kotlinx-coroutines-core/common/test/ScopedBuildersCancelledStartTest.kt | 57 | (annotation, inheritance, is, named argument, package) |
| kotlinx-coroutines-core/common/test/SupervisorTest.kt | 266 | (annotation, inheritance, is, named argument, package, throw, try) |
| kotlinx-coroutines-core/common/test/UnconfinedCancellationTest.kt | 99 | (annotation, inheritance, named argument, package) |
| kotlinx-coroutines-core/common/test/UnconfinedTest.kt | 110 | (annotation, inheritance, named argument, package, throw, try) |
| kotlinx-coroutines-core/common/test/UndispatchedResultTest.kt | 64 | (annotation, extension function, function type with receiver, inheritance, inline-only call, modifier suspend, named argument, package, suspend function type, throw, try) |
| kotlinx-coroutines-core/common/test/WithContextTest.kt | 375 | (annotation, extension function, inheritance, is, modifier lateinit, modifier override, named argument, nested class, package, property getter, require, check, error, throw, try, uninitialized local variable) |
| kotlinx-coroutines-core/common/test/WithTimeoutDurationTest.kt | 211 | (annotation, inheritance, is, modifier lateinit, modifier override, named argument, nested class, package, require, check, error, throw, try, uninitialized local variable) |
| kotlinx-coroutines-core/common/test/WithTimeoutOrNullDurationTest.kt | 240 | (annotation, inheritance, is, modifier override, named argument, nested class, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/common/test/WithTimeoutOrNullTest.kt | 231 | (annotation, inheritance, is, named argument, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/common/test/WithTimeoutTest.kt | 215 | (annotation, inheritance, is, modifier lateinit, named argument, package, throw, try, uninitialized local variable) |
| kotlinx-coroutines-core/common/test/channels/BasicOperationsTest.kt | 224 | (annotation, inheritance, inline-only call, is, modifier suspend, package, throw, try) |
| kotlinx-coroutines-core/common/test/channels/BroadcastChannelFactoryTest.kt | 34 | (annotation, inheritance, is, package) |
| kotlinx-coroutines-core/common/test/channels/BroadcastTest.kt | 139 | (annotation, inheritance, named argument, package) |
| kotlinx-coroutines-core/common/test/channels/BufferedBroadcastChannelTest.kt | 211 | (annotation, inheritance, is, named argument, package, prefix increment, require, check, error) |
| kotlinx-coroutines-core/common/test/channels/BufferedChannelTest.kt | 250 | (annotation, as, extension function, inheritance, inline-only call, is, modifier suspend, named argument, package, require, check, error, try) |
| kotlinx-coroutines-core/common/test/channels/ChannelBufferOverflowTest.kt | 37 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/common/test/channels/ChannelFactoryTest.kt | 45 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/common/test/channels/ChannelIteratorTest.kt | 26 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/common/test/channels/ChannelPropertiesTest.kt | 54 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/common/test/channels/ChannelReceiveCatchingTest.kt | 143 | (annotation, inheritance, inline-only call, package, require, check, error, unsigned integers) |
| kotlinx-coroutines-core/common/test/channels/ChannelUndeliveredElementFailureTest.kt | 173 | (annotation, extension function, inheritance, is, named argument, package, throw) |
| kotlinx-coroutines-core/common/test/channels/ChannelUndeliveredElementTest.kt | 197 | (annotation, function type with receiver, inheritance, inline-only call, modifier suspend, named argument, nested class, package, property getter, require, check, error, try) |
| kotlinx-coroutines-core/common/test/channels/ChannelsTest.kt | 144 | (annotation, default value, extension function, inheritance, package, qualified this, try) |
| kotlinx-coroutines-core/common/test/channels/ConflatedBroadcastChannelTest.kt | 126 | (annotation, inheritance, modifier inline, named argument, package, try) |
| kotlinx-coroutines-core/common/test/channels/ConflatedChannelTest.kt | 92 | (annotation, inheritance, inline-only call, is, modifier open, modifier protected, package, require, check, error) |
| kotlinx-coroutines-core/common/test/channels/ConsumeTest.kt | 146 | (annotation, class literal, inheritance, local function, modifier suspend, named argument, package, throw, try) |
| kotlinx-coroutines-core/common/test/channels/ProduceConsumeTest.kt | 56 | (annotation, inheritance, modifier suspend, named argument, package) |
| kotlinx-coroutines-core/common/test/channels/ProduceTest.kt | 293 | (annotation, as, inheritance, inline-only call, is, modifier suspend, named argument, package, qualified this, require, check, error, throw, try) |
| kotlinx-coroutines-core/common/test/channels/RendezvousChannelTest.kt | 296 | (annotation, inheritance, inline-only call, is, named argument, package, require, check, error, try) |
| kotlinx-coroutines-core/common/test/channels/SendReceiveStressTest.kt | 46 | (annotation, inheritance, modifier suspend, package, require, check, error) |
| kotlinx-coroutines-core/common/test/channels/TestBroadcastChannelKind.kt | 22 | (annotation, function without body, modifier abstract, modifier enum, modifier open, modifier override, package) |
| kotlinx-coroutines-core/common/test/channels/TestChannelKind.kt | 56 | (annotation, class delegation, default value, inheritance, modifier enum, modifier override, modifier suspend, named argument, package, property getter, require, check, error) |
| kotlinx-coroutines-core/common/test/channels/UnlimitedChannelTest.kt | 40 | (annotation, inheritance, is, package, require, check, error) |
| kotlinx-coroutines-core/common/test/flow/BuildersTest.kt | 39 | (annotation, arrays, inheritance, inline-only call, package) |
| kotlinx-coroutines-core/common/test/flow/FlowInvariantsTest.kt | 277 | (annotation, class literal, default value, extension function, function type with receiver, inheritance, inline-only call, local extension function, local function, modifier override, modifier suspend, object expression, package, require, check, error, suspend function type, throw, try) |
| kotlinx-coroutines-core/common/test/flow/IdFlowTest.kt | 59 | (annotation, extension function, inheritance, is, named argument, package) |
| kotlinx-coroutines-core/common/test/flow/NamedDispatchers.kt | 62 | (arrays, bitwise operators, inheritance, inline-only call, modifier operator, modifier override, named argument, object, object expression, package, prefix increment, require, check, error, try) |
| kotlinx-coroutines-core/common/test/flow/SafeFlowTest.kt | 28 | (annotation, extension function, inheritance, modifier suspend, package) |
| kotlinx-coroutines-core/common/test/flow/VirtualTime.kt | 99 | (annotation, as, class delegation, extension function, function type with receiver, inheritance, inline-only call, modifier inner, modifier override, modifier suspend, named argument, nested class, package, require, check, error) |
| kotlinx-coroutines-core/common/test/flow/channels/ChannelBuildersFlowTest.kt | 137 | (annotation, inheritance, package, throw) |
| kotlinx-coroutines-core/common/test/flow/channels/ChannelFlowTest.kt | 207 | (annotation, extension function, inheritance, is, local extension function, local function, named argument, package, throw) |
| kotlinx-coroutines-core/common/test/flow/channels/FlowCallbackTest.kt | 52 | (annotation, inheritance, package, try) |
| kotlinx-coroutines-core/common/test/flow/internal/FlowScopeTest.kt | 74 | (annotation, inheritance, package, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/AsFlowTest.kt | 34 | (annotation, inheritance, named argument, package) |
| kotlinx-coroutines-core/common/test/flow/operators/BooleanTerminationTest.kt | 106 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/common/test/flow/operators/BufferConflationTest.kt | 143 | (annotation, default value, function type with receiver, inheritance, modifier suspend, named argument, package, require, check, error) |
| kotlinx-coroutines-core/common/test/flow/operators/BufferTest.kt | 194 | (annotation, function type with receiver, inheritance, modifier suspend, named argument, package) |
| kotlinx-coroutines-core/common/test/flow/operators/CancellableTest.kt | 34 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/common/test/flow/operators/CatchTest.kt | 201 | (annotation, as, inheritance, package, prefix increment, throw, try) |
| kotlinx-coroutines-core/common/test/flow/operators/ChunkedTest.kt | 88 | (annotation, class literal, inheritance, modifier suspend, named argument, package) |
| kotlinx-coroutines-core/common/test/flow/operators/CombineParametersTest.kt | 187 | (annotation, arrays, inheritance, inline-only call, named argument, package) |
| kotlinx-coroutines-core/common/test/flow/operators/CombineTest.kt | 312 | (annotation, arrays, as, callable reference, extension function, function without body, inheritance, inline-only call, is, member extension function, modifier abstract, modifier override, named argument, package, suspend function type, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/ConflateTest.kt | 23 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/common/test/flow/operators/DebounceTest.kt | 321 | (annotation, inheritance, modifier inline, modifier reified, package, reified member type parameter, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/DistinctUntilChangedTest.kt | 123 | (annotation, callable reference, function type with receiver, inheritance, named argument, nested class, package, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/DropTest.kt | 57 | (annotation, inheritance, named argument, package, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/DropWhileTest.kt | 48 | (annotation, inheritance, named argument, package, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/FilterTest.kt | 78 | (annotation, inheritance, package, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/FilterTrivialTest.kt | 74 | (annotation, class literal, inheritance, local class, modifier open, package) |
| kotlinx-coroutines-core/common/test/flow/operators/FlatMapBaseTest.kt | 88 | (annotation, extension function, function without body, inheritance, inline-only call, member extension function, modifier abstract, package, suspend function type) |
| kotlinx-coroutines-core/common/test/flow/operators/FlatMapConcatTest.kt | 35 | (annotation, extension function, inheritance, inline-only call, member extension function, modifier override, named argument, package, prefix increment, suspend function type) |
| kotlinx-coroutines-core/common/test/flow/operators/FlatMapLatestTest.kt | 134 | (annotation, inheritance, package, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/FlatMapMergeBaseTest.kt | 90 | (annotation, function without body, inheritance, modifier abstract, package, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/FlatMapMergeFastPathTest.kt | 85 | (annotation, extension function, inheritance, inline-only call, member extension function, modifier override, named argument, package, prefix increment, suspend function type, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/FlatMapMergeTest.kt | 113 | (annotation, extension function, inheritance, inline-only call, member extension function, modifier override, named argument, package, prefix increment, suspend function type, throw, try) |
| kotlinx-coroutines-core/common/test/flow/operators/FlattenConcatTest.kt | 49 | (annotation, extension function, inheritance, inline-only call, member extension function, modifier override, package, prefix increment, suspend function type, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/FlattenMergeTest.kt | 50 | (annotation, extension function, inheritance, inline-only call, member extension function, modifier override, named argument, package, prefix increment, suspend function type) |
| kotlinx-coroutines-core/common/test/flow/operators/FlowContextOptimizationsTest.kt | 113 | (annotation, as, inheritance, modifier companion, modifier data, nested class, package) |
| kotlinx-coroutines-core/common/test/flow/operators/FlowOnTest.kt | 358 | (annotation, callable reference, inheritance, inline-only call, is, modifier inner, nested class, package, suspend function type, throw, try) |
| kotlinx-coroutines-core/common/test/flow/operators/IndexedTest.kt | 42 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/common/test/flow/operators/LintTest.kt | 27 | (annotation, inheritance, inline-only call, package) |
| kotlinx-coroutines-core/common/test/flow/operators/MapNotNullTest.kt | 47 | (annotation, inheritance, package, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/MapTest.kt | 47 | (annotation, inheritance, package, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/MergeTest.kt | 123 | (annotation, arrays, extension function, function without body, inheritance, inline-only call, member extension function, modifier abstract, modifier override, package, spread operator, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/OnCompletionTest.kt | 397 | (annotation, as, inheritance, is, modifier data, modifier override, modifier sealed, nested class, package, throw, try) |
| kotlinx-coroutines-core/common/test/flow/operators/OnEachTest.kt | 47 | (annotation, inheritance, package, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/OnEmptyTest.kt | 77 | (annotation, inheritance, package, throw, try) |
| kotlinx-coroutines-core/common/test/flow/operators/OnStartTest.kt | 33 | (annotation, inheritance, package, try) |
| kotlinx-coroutines-core/common/test/flow/operators/RetryTest.kt | 161 | (annotation, inheritance, is, package, prefix increment, throw, try) |
| kotlinx-coroutines-core/common/test/flow/operators/SampleTest.kt | 301 | (annotation, inheritance, modifier inline, modifier reified, package, reified member type parameter, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/ScanTest.kt | 72 | (annotation, extension function, inheritance, inline-only call, modifier operator, package, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/TakeTest.kt | 144 | (annotation, inheritance, named argument, package, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/TakeWhileTest.kt | 64 | (annotation, inheritance, named argument, package, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/TimeoutTest.kt | 258 | (annotation, callable reference, inheritance, is, modifier inline, modifier reified, package, reified member type parameter, throw, try) |
| kotlinx-coroutines-core/common/test/flow/operators/TransformLatestTest.kt | 157 | (annotation, inheritance, package, throw) |
| kotlinx-coroutines-core/common/test/flow/operators/TransformTest.kt | 17 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/common/test/flow/operators/TransformWhileTest.kt | 67 | (annotation, extension function, inheritance, modifier data, named argument, nested class, package) |
| kotlinx-coroutines-core/common/test/flow/operators/ZipTest.kt | 251 | (annotation, callable reference, inheritance, package, throw, try) |
| kotlinx-coroutines-core/common/test/flow/sharing/ShareInBufferTest.kt | 95 | (annotation, function type with receiver, inheritance, modifier suspend, package) |
| kotlinx-coroutines-core/common/test/flow/sharing/ShareInConflationTest.kt | 159 | (annotation, default value, function type with receiver, inheritance, modifier suspend, named argument, package, require, check, error) |
| kotlinx-coroutines-core/common/test/flow/sharing/ShareInFusionTest.kt | 53 | (annotation, as, inheritance, is, package) |
| kotlinx-coroutines-core/common/test/flow/sharing/ShareInTest.kt | 241 | (annotation, as, extension function, inheritance, inline-only call, is, local function, modifier suspend, named argument, nested class, package, qualified this, require, check, error, suspend function type, throw, try) |
| kotlinx-coroutines-core/common/test/flow/sharing/SharedFlowScenarioTest.kt | 403 | (annotation, callable reference, function type with receiver, inheritance, modifier data, modifier open, modifier override, modifier suspend, modifier vararg, named argument, nested class, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/common/test/flow/sharing/SharedFlowTest.kt | 870 | (annotation, as, default value, extension function, function type with receiver, inheritance, inline-only call, is, local function, modifier data, modifier inline, modifier lateinit, modifier suspend, named argument, nested class, package, property without initializer, require, check, error, throw, try) |
| kotlinx-coroutines-core/common/test/flow/sharing/SharingStartedTest.kt | 180 | (annotation, as, default value, extension function, function type with receiver, inheritance, modifier suspend, named argument, nested class, package) |
| kotlinx-coroutines-core/common/test/flow/sharing/SharingStartedWhileSubscribedTest.kt | 76 | (annotation, inheritance, named argument, package, prefix increment) |
| kotlinx-coroutines-core/common/test/flow/sharing/StateFlowTest.kt | 247 | (annotation, as, inheritance, modifier data, named argument, nested class, package, require, check, error, throw) |
| kotlinx-coroutines-core/common/test/flow/sharing/StateInTest.kt | 111 | (annotation, as, inheritance, inline-only call, is, named argument, package, qualified this, throw, uninitialized local variable) |
| kotlinx-coroutines-core/common/test/flow/terminal/CollectLatestTest.kt | 53 | (annotation, inheritance, is, package, throw, try) |
| kotlinx-coroutines-core/common/test/flow/terminal/CountTest.kt | 44 | (annotation, inheritance, package, throw) |
| kotlinx-coroutines-core/common/test/flow/terminal/FirstTest.kt | 195 | (annotation, inheritance, inline-only call, named argument, package, throw, try) |
| kotlinx-coroutines-core/common/test/flow/terminal/FoldTest.kt | 51 | (annotation, inheritance, package, throw) |
| kotlinx-coroutines-core/common/test/flow/terminal/LastTest.kt | 42 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/common/test/flow/terminal/LaunchInTest.kt | 53 | (annotation, inheritance, is, named argument, package, throw) |
| kotlinx-coroutines-core/common/test/flow/terminal/ReduceTest.kt | 71 | (annotation, inheritance, package, prefix increment, throw) |
| kotlinx-coroutines-core/common/test/flow/terminal/SingleTest.kt | 91 | (annotation, inheritance, package, throw) |
| kotlinx-coroutines-core/common/test/flow/terminal/ToCollectionTest.kt | 28 | (annotation, inheritance, inline-only call, package) |
| kotlinx-coroutines-core/common/test/flow/terminal/ToMapCollectionSamplesTest.kt | 115 | (annotation, destructuring lambda parameter, inheritance, inline-only call, local class, modifier data, modifier override, package) |
| kotlinx-coroutines-core/common/test/selects/SelectBiasTest.kt | 41 | (annotation, arrays, inheritance, package) |
| kotlinx-coroutines-core/common/test/selects/SelectBufferedChannelTest.kt | 414 | (annotation, extension function, inheritance, inline-only call, is, package, suspend function type) |
| kotlinx-coroutines-core/common/test/selects/SelectDeferredTest.kt | 185 | (annotation, inheritance, is, modifier override, named argument, nested class, package, property getter, require, check, error) |
| kotlinx-coroutines-core/common/test/selects/SelectJobTest.kt | 64 | (annotation, inheritance, named argument, package) |
| kotlinx-coroutines-core/common/test/selects/SelectLoopTest.kt | 38 | (annotation, inheritance, is, named argument, package, throw, try) |
| kotlinx-coroutines-core/common/test/selects/SelectMutexTest.kt | 54 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/common/test/selects/SelectOldTest.kt | 149 | (annotation, inheritance, named argument, package, throw, try) |
| kotlinx-coroutines-core/common/test/selects/SelectRendezvousChannelTest.kt | 474 | (annotation, extension function, inheritance, inline-only call, is, modifier suspend, named argument, package, suspend function type) |
| kotlinx-coroutines-core/common/test/selects/SelectTimeoutDurationTest.kt | 89 | (annotation, arrays, inheritance, package, prefix increment) |
| kotlinx-coroutines-core/common/test/selects/SelectTimeoutTest.kt | 86 | (annotation, arrays, inheritance, package, prefix increment) |
| kotlinx-coroutines-core/common/test/selects/SelectUnlimitedChannelTest.kt | 26 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/common/test/sync/MutexTest.kt | 201 | (annotation, inheritance, local function, modifier suspend, package, throw, try) |
| kotlinx-coroutines-core/common/test/sync/SemaphoreTest.kt | 214 | (annotation, inheritance, local function, modifier suspend, named argument, package, throw, try) |
| kotlinx-coroutines-core/concurrent/test/AbstractDispatcherConcurrencyTest.kt | 52 | (annotation, inheritance, inline-only call, modifier abstract, package, prefix increment, property without initializer) |
| kotlinx-coroutines-core/concurrent/test/AtomicCancellationTest.kt | 143 | (annotation, inheritance, named argument, package, try) |
| kotlinx-coroutines-core/concurrent/test/CommonThreadLocalTest.kt | 72 | (annotation, inheritance, inline-only call, package) |
| kotlinx-coroutines-core/concurrent/test/ConcurrentExceptionsStressTest.kt | 64 | (annotation, arrays, as, extension function, inheritance, modifier suspend, named argument, nested class, package, throw) |
| kotlinx-coroutines-core/concurrent/test/ConcurrentTestUtilities.common.kt | 33 | (annotation, extension function, function without body, inline-only call, modifier expect, modifier inline, object, package, try) |
| kotlinx-coroutines-core/concurrent/test/DefaultDispatchersConcurrencyTest.kt | 9 | (inheritance, modifier override, package) |
| kotlinx-coroutines-core/concurrent/test/JobStructuredJoinStressTest.kt | 61 | (annotation, callable reference, inheritance, inline-only call, package, require, check, error, suspend function type, throw, try) |
| kotlinx-coroutines-core/concurrent/test/LimitedParallelismConcurrentTest.kt | 92 | (annotation, function type with receiver, inheritance, local class, modifier crossinline, modifier inline, modifier override, modifier suspend, package) |
| kotlinx-coroutines-core/concurrent/test/MultithreadedDispatcherStressTest.kt | 31 | (annotation, package) |
| kotlinx-coroutines-core/concurrent/test/RunBlockingTest.kt | 207 | (annotation, as, inheritance, inline-only call, named argument, package, try, uninitialized local variable) |
| kotlinx-coroutines-core/concurrent/test/channels/BroadcastChannelSubStressTest.kt | 57 | (annotation, inheritance, named argument, package, require, check, error) |
| kotlinx-coroutines-core/concurrent/test/channels/ChannelCancelUndeliveredElementStressTest.kt | 100 | (annotation, as, inheritance, inline-only call, modifier suspend, package, prefix increment, require, check, error, throw, try) |
| kotlinx-coroutines-core/concurrent/test/channels/ConflatedBroadcastChannelNotifyStressTest.kt | 86 | (annotation, inheritance, inline-only call, modifier suspend, package, prefix increment, try) |
| kotlinx-coroutines-core/concurrent/test/channels/TrySendBlockingTest.kt | 50 | (annotation, inheritance, inline-only call, package) |
| kotlinx-coroutines-core/concurrent/test/flow/CombineStressTest.kt | 50 | (annotation, inheritance, package, throw, try) |
| kotlinx-coroutines-core/concurrent/test/flow/FlowCancellationTest.kt | 60 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/concurrent/test/flow/StateFlowCommonStressTest.kt | 44 | (annotation, do-while, inheritance, package, require, check, error) |
| kotlinx-coroutines-core/concurrent/test/flow/StateFlowUpdateCommonTest.kt | 36 | (annotation, function type with receiver, inheritance, package) |
| kotlinx-coroutines-core/concurrent/test/selects/SelectChannelStressTest.kt | 67 | (annotation, extension function, inheritance, inline-only call, package, suspend function type) |
| kotlinx-coroutines-core/concurrent/test/selects/SelectMutexStressTest.kt | 30 | (annotation, as, inheritance, package) |
| kotlinx-coroutines-core/concurrent/test/sync/MutexStressTest.kt | 135 | (annotation, extension function, inheritance, inline-only call, modifier suspend, named argument, package) |
| kotlinx-coroutines-core/concurrent/test/sync/SemaphoreStressTest.kt | 135 | (annotation, inheritance, inline-only call, named argument, package) |
| kotlinx-coroutines-core/jvm/test/AbstractLincheckTest.kt | 41 | (annotation, class literal, extension function, member extension function, modifier abstract, modifier open, package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/AsyncJvmTest.kt | 46 | (annotation, inheritance, is, package, require, check, error, try) |
| kotlinx-coroutines-core/jvm/test/AwaitJvmTest.kt | 24 | (annotation, inheritance, package, try) |
| kotlinx-coroutines-core/jvm/test/AwaitStressTest.kt | 122 | (annotation, inheritance, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/test/CancellableContinuationJvmTest.kt | 79 | (annotation, extension function, inheritance, is, modifier suspend, nested class, package) |
| kotlinx-coroutines-core/jvm/test/CancellableContinuationResumeCloseStressTest.kt | 61 | (annotation, extension function, inheritance, modifier suspend, named argument, package) |
| kotlinx-coroutines-core/jvm/test/CancelledAwaitStressTest.kt | 52 | (annotation, arrays, inheritance, named argument, package) |
| kotlinx-coroutines-core/jvm/test/ConcurrentTestUtilities.kt | 6 | (annotation, modifier actual, modifier inline, package) |
| kotlinx-coroutines-core/jvm/test/CoroutineScopeTestJvm.kt | 74 | (annotation, as, callable reference, identity equality, inheritance, inline-only call, is, modifier override, modifier vararg, package) |
| kotlinx-coroutines-core/jvm/test/CoroutinesJvmTest.kt | 36 | (annotation, as, inheritance, is, named argument, package, throw) |
| kotlinx-coroutines-core/jvm/test/DebugThreadNameTest.kt | 70 | (annotation, inheritance, named argument, package) |
| kotlinx-coroutines-core/jvm/test/DefaultExecutorStressTest.kt | 62 | (annotation, arrays, inheritance, local function, modifier suspend, package, prefix increment, try) |
| kotlinx-coroutines-core/jvm/test/DelayJvmTest.kt | 68 | (annotation, inheritance, inline-only call, modifier override, nested class, package, property getter) |
| kotlinx-coroutines-core/jvm/test/DispatcherKeyTest.kt | 49 | (annotation, as, class literal, inheritance, modifier companion, modifier override, package) |
| kotlinx-coroutines-core/jvm/test/DispatchersToStringTest.kt | 55 | (annotation, as, package) |
| kotlinx-coroutines-core/jvm/test/EventLoopsTest.kt | 125 | (annotation, inheritance, inline-only call, named argument, nested class, package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/ExecutorAsCoroutineDispatcherDelayTest.kt | 58 | (annotation, as, class delegation, inheritance, modifier inner, modifier override, named argument, nested class, package, super) |
| kotlinx-coroutines-core/jvm/test/ExecutorsTest.kt | 148 | (annotation, inheritance, inline-only call, modifier override, object expression, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/test/FailFastOnStartTest.kt | 110 | (annotation, callable reference, inheritance, inline-only call, is, named argument, package, try) |
| kotlinx-coroutines-core/jvm/test/FailingCoroutinesMachineryTest.kt | 151 | (annotation, arrays, as, bitwise operators, class literal, inheritance, inline-only call, is, modifier companion, modifier override, nested class, object, package, property getter, throw) |
| kotlinx-coroutines-core/jvm/test/IODispatcherTest.kt | 22 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/IntellijIdeaDebuggerEvaluatorCompatibilityTest.kt | 52 | (annotation, is, modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/JobActivationStressTest.kt | 68 | (annotation, as, inheritance, inline-only call, named argument, object expression, package, throw) |
| kotlinx-coroutines-core/jvm/test/JobCancellationExceptionSerializerTest.kt | 65 | (annotation, arrays, as, inheritance, inline-only call, named argument, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/JobChildStressTest.kt | 115 | (annotation, inheritance, inline-only call, package, throw) |
| kotlinx-coroutines-core/jvm/test/JobDisposeStressTest.kt | 77 | (annotation, inheritance, inline-only call, named argument, nested class, package) |
| kotlinx-coroutines-core/jvm/test/JobHandlersUpgradeStressTest.kt | 97 | (annotation, inheritance, inline-only call, named argument, nested class, package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/JobOnCompletionStressTest.kt | 192 | (annotation, function type with receiver, identity equality, inheritance, inline-only call, modifier suspend, named argument, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/JobStressTest.kt | 14 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/JoinStressTest.kt | 90 | (annotation, arrays, as, inheritance, package, prefix increment, throw, try) |
| kotlinx-coroutines-core/jvm/test/LimitedParallelismStressTest.kt | 148 | (annotation, arrays, class literal, function type with receiver, inheritance, modifier companion, modifier crossinline, modifier inline, modifier override, modifier suspend, object expression, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/LimitedParallelismUnhandledExceptionTest.kt | 29 | (annotation, inheritance, inline-only call, is, package, throw) |
| kotlinx-coroutines-core/jvm/test/MemoryFootprintTest.kt | 43 | (annotation, class literal, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/MultithreadedDispatchersJvmTest.kt | 30 | safe-call assignment (annotation, as, inheritance, inline-only call, package, require, check, error, try) |
| kotlinx-coroutines-core/jvm/test/MutexCancellationStressTest.kt | 83 | (annotation, arrays, inheritance, inline-only call, package) |
| kotlinx-coroutines-core/jvm/test/NoParamAssertionsTest.kt | 27 | (annotation, as, callable reference, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/RejectedExecutionTest.kt | 166 | (annotation, inheritance, inline-only call, is, modifier inner, modifier override, modifier suspend, named argument, nested class, package, require, check, error, super, throw, try) |
| kotlinx-coroutines-core/jvm/test/ReusableCancellableContinuationInvariantStressTest.kt | 50 | (annotation, inheritance, modifier crossinline, modifier inline, package, throw) |
| kotlinx-coroutines-core/jvm/test/ReusableCancellableContinuationLeakStressTest.kt | 38 | (annotation, as, extension function, inheritance, is, modifier suspend, named argument, nested class, package) |
| kotlinx-coroutines-core/jvm/test/ReusableCancellableContinuationTest.kt | 218 | (annotation, callable reference, class literal, extension function, identity equality, inheritance, inline-only call, is, modifier inline, modifier suspend, package, suspend function type, try) |
| kotlinx-coroutines-core/jvm/test/ReusableContinuationStressTest.kt | 38 | (annotation, inheritance, modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/RunBlockingJvmTest.kt | 184 | (annotation, inheritance, inline-only call, package, suspend function type, throw, try) |
| kotlinx-coroutines-core/jvm/test/RunInterruptibleStressTest.kt | 53 | (annotation, inheritance, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/test/RunInterruptibleTest.kt | 60 | (annotation, inheritance, named argument, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/TestBaseTest.kt | 24 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/ThreadContextElementRestoreTest.kt | 195 | (annotation, as, inheritance, inline-only call, modifier crossinline, modifier inline, modifier suspend, package, require, check, error, suspend function type) |
| kotlinx-coroutines-core/jvm/test/ThreadContextElementTest.kt | 377 | (annotation, class delegation, default value, destructuring declaration, extension function, inheritance, inline-only call, local function, modifier companion, modifier crossinline, modifier inline, modifier override, named argument, nested class, object expression, package, property getter, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/test/ThreadContextMutableCopiesTest.kt | 158 | (annotation, as, inheritance, inline-only call, modifier companion, modifier override, nested class, package, property getter) |
| kotlinx-coroutines-core/jvm/test/ThreadContextOrderTest.kt | 62 | (annotation, inheritance, modifier override, object expression, package) |
| kotlinx-coroutines-core/jvm/test/ThreadLocalStressTest.kt | 162 | (annotation, callable reference, inheritance, inline-only call, modifier suspend, package, suspend function type, try) |
| kotlinx-coroutines-core/jvm/test/ThreadLocalTest.kt | 230 | (annotation, inheritance, inline-only call, modifier data, named argument, nested class, package, prefix increment) |
| kotlinx-coroutines-core/jvm/test/ThreadLocalsLeaksTest.kt | 106 | (annotation, arrays, inheritance, local function, modifier lateinit, modifier override, named argument, nested class, package, uninitialized local variable) |
| kotlinx-coroutines-core/jvm/test/UnconfinedConcurrentStressTest.kt | 47 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/VirtualTimeSource.kt | 154 | (annotation, as, default value, inheritance, inline-only call, modifier const, modifier inline, modifier override, package, prefix increment, try) |
| kotlinx-coroutines-core/jvm/test/WithDefaultContextTest.kt | 30 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/WithTimeoutChildDispatchStressTest.kt | 29 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/WithTimeoutOrNullJvmTest.kt | 64 | (annotation, inheritance, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/WithTimeoutOrNullThreadDispatchTest.kt | 79 | (annotation, inheritance, inline-only call, modifier override, object expression, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/test/WithTimeoutThreadDispatchTest.kt | 82 | (annotation, inheritance, inline-only call, modifier override, object expression, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/test/channels/ActorLazyTest.kt | 88 | (annotation, as, inheritance, is, named argument, package) |
| kotlinx-coroutines-core/jvm/test/channels/ActorTest.kt | 193 | (annotation, arrays, as, class literal, inheritance, is, modifier companion, named argument, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/test/channels/BroadcastChannelLeakTest.kt | 35 | (annotation, identity equality, inheritance, modifier enum, nested class, package) |
| kotlinx-coroutines-core/jvm/test/channels/BroadcastChannelMultiReceiveStressTest.kt | 166 | (annotation, arrays, class literal, inheritance, local function, modifier companion, modifier suspend, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/test/channels/BufferedChannelStressTest.kt | 61 | (annotation, arrays, class literal, inheritance, modifier companion, package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/channels/CancelledChannelLeakTest.kt | 27 | (annotation, identity equality, inheritance, named argument, nested class, package) |
| kotlinx-coroutines-core/jvm/test/channels/ChannelMemoryLeakStressTest.kt | 21 | (annotation, arrays, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/channels/ChannelSelectStressTest.kt | 78 | (annotation, bitwise operators, extension function, inheritance, inline-only call, package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/channels/ChannelSendReceiveStressTest.kt | 182 | (annotation, arrays, as, class literal, inheritance, inline-only call, modifier companion, modifier suspend, package, prefix increment, try) |
| kotlinx-coroutines-core/jvm/test/channels/ChannelUndeliveredElementSelectOldStressTest.kt | 251 | (annotation, arrays, bitwise operators, class literal, inheritance, modifier companion, modifier const, modifier inline, modifier inner, modifier lateinit, modifier operator, modifier suspend, named argument, nested class, package, property without initializer, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/test/channels/ChannelUndeliveredElementStressTest.kt | 267 | (annotation, arrays, as, bitwise operators, class literal, inheritance, inline-only call, modifier companion, modifier const, modifier inline, modifier inner, modifier lateinit, modifier operator, modifier suspend, named argument, nested class, package, property without initializer, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/test/channels/ConflatedChannelCloseStressTest.kt | 106 | (annotation, inheritance, inline-only call, nested class, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/test/channels/DoubleChannelCloseStressTest.kt | 27 | (annotation, inheritance, named argument, package, try) |
| kotlinx-coroutines-core/jvm/test/channels/InvokeOnCloseStressTest.kt | 61 | (annotation, inheritance, modifier override, modifier suspend, package, property getter) |
| kotlinx-coroutines-core/jvm/test/channels/ProduceConsumeJvmTest.kt | 59 | (annotation, arrays, class literal, inheritance, modifier companion, named argument, package) |
| kotlinx-coroutines-core/jvm/test/channels/SendReceiveJvmStressTest.kt | 45 | (annotation, arrays, class literal, inheritance, modifier companion, package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/channels/SimpleSendReceiveJvmTest.kt | 50 | (annotation, arrays, class literal, inheritance, modifier companion, package) |
| kotlinx-coroutines-core/jvm/test/channels/TickerChannelCommonTest.kt | 164 | (annotation, arrays, class literal, default value, extension function, function without body, inheritance, inline-only call, modifier abstract, modifier companion, modifier enum, modifier operator, modifier override, named argument, nested class, package, prefix increment, qualified this) |
| kotlinx-coroutines-core/jvm/test/channels/TickerChannelTest.kt | 62 | (annotation, inheritance, named argument, package) |
| kotlinx-coroutines-core/jvm/test/examples/example-delay-01.kt | 21 | (inline-only call, package) |
| kotlinx-coroutines-core/jvm/test/examples/example-delay-02.kt | 27 | (inline-only call, package) |
| kotlinx-coroutines-core/jvm/test/examples/example-delay-03.kt | 16 | (inline-only call, package) |
| kotlinx-coroutines-core/jvm/test/examples/example-delay-duration-01.kt | 21 | (inline-only call, package) |
| kotlinx-coroutines-core/jvm/test/examples/example-delay-duration-02.kt | 27 | (inline-only call, package) |
| kotlinx-coroutines-core/jvm/test/examples/example-delay-duration-03.kt | 16 | (inline-only call, package) |
| kotlinx-coroutines-core/jvm/test/examples/example-timeout-duration-01.kt | 30 | (inline-only call, is, package, throw) |
| kotlinx-coroutines-core/jvm/test/examples/test/FlowDelayTest.kt | 56 | (annotation, package) |
| kotlinx-coroutines-core/jvm/test/exceptions/CoroutineExceptionHandlerJvmTest.kt | 75 | (annotation, inheritance, modifier inline, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/FlowSuppressionTest.kt | 71 | (annotation, inheritance, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/JobBasicCancellationTest.kt | 156 | (annotation, inheritance, named argument, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/JobExceptionHandlingTest.kt | 377 | (annotation, inheritance, inline-only call, is, modifier override, named argument, nested class, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/JobExceptionsStressTest.kt | 64 | (annotation, class literal, inheritance, named argument, package, throw) |
| kotlinx-coroutines-core/jvm/test/exceptions/JobNestedExceptionsTest.kt | 116 | (annotation, inheritance, named argument, package, throw) |
| kotlinx-coroutines-core/jvm/test/exceptions/ProduceExceptionsTest.kt | 166 | (annotation, inheritance, is, named argument, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryChannelsTest.kt | 150 | (annotation, extension function, inheritance, modifier inline, modifier suspend, package, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryCopyableThrowableTest.kt | 76 | (annotation, default value, inheritance, inline-only call, local class, modifier override, nested class, package, require, check, error, secondary constructor, throw, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryExceptionConstructorsTest.kt | 91 | (annotation, default value, inheritance, inline-only call, nested class, package, require, check, error, secondary constructor, super, throw, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryNestedScopesTest.kt | 99 | (annotation, extension function, inheritance, modifier suspend, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryNestedTest.kt | 84 | (annotation, extension function, inheritance, modifier suspend, modifier vararg, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryResumeModeTest.kt | 146 | (annotation, inheritance, modifier suspend, package, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoverySelectTest.kt | 68 | (annotation, inheritance, inline-only call, modifier suspend, package, throw) |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryStdlibInterfaceTest.kt | 104 | (annotation, default value, function without body, inheritance, inline-only call, interface, local class, modifier open, modifier override, nested class, package, require, check, error, secondary constructor, throw, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryStdlibInterfaceWithTypeBoundsTest.kt | 108 | (annotation, default value, function without body, inheritance, inline-only call, interface, local class, local function, modifier open, modifier override, nested class, package, require, check, error, secondary constructor, throw, try, where clause) |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryTest.kt | 262 | (annotation, arrays, inheritance, inline-only call, local function, modifier override, modifier suspend, modifier vararg, named argument, nested class, package, require, check, error, secondary constructor, spread operator, throw, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/StackTraceRecoveryWithTimeoutTest.kt | 85 | (annotation, inheritance, modifier suspend, package, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/Stacktraces.kt | 42 | (arrays, as, class literal, extension function, inline-only call, modifier vararg, package, spread operator) |
| kotlinx-coroutines-core/jvm/test/exceptions/SuppressionTests.kt | 83 | (annotation, inheritance, modifier override, named argument, object expression, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/WithContextCancellationStressTest.kt | 104 | (annotation, arrays, as, default value, extension function, inheritance, is, modifier override, named argument, object expression, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/test/exceptions/WithContextExceptionHandlingTest.kt | 278 | (annotation, arrays, as, class literal, default value, function type with receiver, inheritance, modifier companion, modifier enum, modifier override, modifier suspend, nested class, object expression, package, require, check, error, throw, try) |
| kotlinx-coroutines-core/jvm/test/flow/CallbackFlowTest.kt | 125 | (annotation, extension function, inheritance, is, modifier lateinit, nested class, package, prefix increment, property without initializer, throw) |
| kotlinx-coroutines-core/jvm/test/flow/ExceptionTransparencyTest.kt | 74 | (annotation, inheritance, package, try) |
| kotlinx-coroutines-core/jvm/test/flow/FirstJvmTest.kt | 25 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/flow/FlatMapStressTest.kt | 99 | (annotation, inheritance, modifier suspend, named argument, package) |
| kotlinx-coroutines-core/jvm/test/flow/OnCompletionInterceptedReleaseTest.kt | 42 | (annotation, inheritance, modifier override, named argument, nested class, package) |
| kotlinx-coroutines-core/jvm/test/flow/SafeCollectorMemoryLeakTest.kt | 45 | (annotation, extension function, inheritance, modifier inline, package) |
| kotlinx-coroutines-core/jvm/test/flow/SharedFlowStressTest.kt | 83 | (annotation, inheritance, inline-only call, modifier lateinit, package, property without initializer, require, check, error) |
| kotlinx-coroutines-core/jvm/test/flow/SharingReferenceTest.kt | 57 | (annotation, identity equality, inheritance, object expression, package) |
| kotlinx-coroutines-core/jvm/test/flow/SharingStressTest.kt | 193 | (annotation, do-while, extension function, inheritance, modifier lateinit, named argument, nested class, package, property without initializer, throw, try) |
| kotlinx-coroutines-core/jvm/test/flow/StateFlowCancellabilityTest.kt | 53 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/flow/StateFlowStressTest.kt | 79 | (annotation, arrays, callable reference, do-while, extension function, inheritance, inline-only call, modifier lateinit, package, property without initializer, require, check, error) |
| kotlinx-coroutines-core/jvm/test/flow/StateFlowUpdateStressTest.kt | 41 | (annotation, function type with receiver, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-basic-01.kt | 12 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-basic-02.kt | 15 | (modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-basic-03.kt | 16 | (modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-basic-04.kt | 23 | (modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-basic-05.kt | 14 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-basic-06.kt | 13 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-01.kt | 18 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-02.kt | 23 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-03.kt | 23 | (package, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-04.kt | 23 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-05.kt | 21 | (package, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-06.kt | 25 | (package, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-07.kt | 13 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-08.kt | 15 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-09.kt | 27 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-cancel-10.kt | 32 | (package, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-01.kt | 17 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-02.kt | 16 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-03.kt | 15 | (extension function, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-04.kt | 24 | (extension function, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-05.kt | 24 | (extension function, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-06.kt | 26 | (extension function, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-07.kt | 22 | (modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-08.kt | 18 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-09.kt | 25 | (modifier data, modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-channel-10.kt | 29 | (named argument, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-compose-01.kt | 24 | (modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-compose-02.kt | 24 | (modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-compose-03.kt | 27 | (modifier suspend, named argument, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-compose-04.kt | 40 | (annotation, class literal, modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-compose-05.kt | 28 | (modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-compose-06.kt | 28 | (modifier suspend, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-context-01.kt | 19 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-context-02.kt | 17 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-context-03.kt | 18 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-context-04.kt | 20 | (inline-only call, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-context-05.kt | 8 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-context-06.kt | 27 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-context-07.kt | 19 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-context-08.kt | 22 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-context-09.kt | 10 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-context-10.kt | 32 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-context-11.kt | 18 | (named argument, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-exceptions-01.kt | 24 | (annotation, class literal, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-exceptions-02.kt | 18 | (annotation, class literal, package, throw) |
| kotlinx-coroutines-core/jvm/test/guide/example-exceptions-03.kt | 23 | (package, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-exceptions-04.kt | 30 | (annotation, class literal, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-exceptions-05.kt | 29 | (annotation, class literal, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-exceptions-06.kt | 28 | (annotation, class literal, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-01.kt | 8 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-02.kt | 13 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-03.kt | 13 | (inline-only call, modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-04.kt | 24 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-05.kt | 22 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-06.kt | 20 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-07.kt | 10 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-08.kt | 16 | (modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-09.kt | 19 | (modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-10.kt | 22 | (package, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-11.kt | 12 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-12.kt | 19 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-13.kt | 18 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-14.kt | 19 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-15.kt | 21 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-16.kt | 23 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-17.kt | 25 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-18.kt | 25 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-19.kt | 25 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-20.kt | 12 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-21.kt | 15 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-22.kt | 15 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-23.kt | 20 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-24.kt | 20 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-25.kt | 20 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-26.kt | 23 | (package, require, check, error, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-27.kt | 25 | (package, require, check, error, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-28.kt | 23 | (package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-29.kt | 21 | (package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-30.kt | 22 | (package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-31.kt | 15 | (package, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-32.kt | 13 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-33.kt | 17 | (package, throw) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-34.kt | 16 | (package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-35.kt | 15 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-36.kt | 15 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-37.kt | 19 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-38.kt | 12 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-flow-39.kt | 12 | (package) |
| kotlinx-coroutines-core/jvm/test/guide/example-select-01.kt | 40 | (extension function, modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-select-02.kt | 39 | (modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-select-03.kt | 29 | (extension function, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-select-04.kt | 30 | (destructuring lambda parameter, extension function, inline-only call, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-select-05.kt | 50 | (extension function, package) |
| kotlinx-coroutines-core/jvm/test/guide/example-supervision-01.kt | 32 | (inline-only call, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-supervision-02.kt | 26 | (package, throw, try) |
| kotlinx-coroutines-core/jvm/test/guide/example-supervision-03.kt | 19 | (package, throw) |
| kotlinx-coroutines-core/jvm/test/guide/example-sync-01.kt | 31 | (inline-only call, modifier suspend, package, suspend function type) |
| kotlinx-coroutines-core/jvm/test/guide/example-sync-02.kt | 32 | (annotation, inline-only call, modifier suspend, package, suspend function type) |
| kotlinx-coroutines-core/jvm/test/guide/example-sync-03.kt | 32 | (inline-only call, modifier suspend, package, suspend function type) |
| kotlinx-coroutines-core/jvm/test/guide/example-sync-04.kt | 35 | (inline-only call, modifier suspend, package, suspend function type) |
| kotlinx-coroutines-core/jvm/test/guide/example-sync-05.kt | 33 | (inline-only call, modifier suspend, package, suspend function type) |
| kotlinx-coroutines-core/jvm/test/guide/example-sync-06.kt | 36 | (inline-only call, modifier suspend, package, suspend function type) |
| kotlinx-coroutines-core/jvm/test/guide/example-sync-07.kt | 51 | (extension function, inheritance, inline-only call, is, modifier sealed, modifier suspend, object, package, suspend function type) |
| kotlinx-coroutines-core/jvm/test/guide/test/BasicsGuideTest.kt | 57 | (annotation, inline-only call, package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/guide/test/CancellationGuideTest.kt | 94 | (annotation, package) |
| kotlinx-coroutines-core/jvm/test/guide/test/ChannelsGuideTest.kt | 123 | (annotation, destructuring lambda parameter, inline-only call, package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/guide/test/ComposingGuideTest.kt | 56 | (annotation, package) |
| kotlinx-coroutines-core/jvm/test/guide/test/DispatcherGuideTest.kt | 110 | (annotation, inline-only call, package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/guide/test/ExceptionsGuideTest.kt | 89 | (annotation, package) |
| kotlinx-coroutines-core/jvm/test/guide/test/FlowGuideTest.kt | 417 | (annotation, package) |
| kotlinx-coroutines-core/jvm/test/guide/test/SelectGuideTest.kt | 69 | (annotation, package) |
| kotlinx-coroutines-core/jvm/test/guide/test/SharedStateGuideTest.kt | 55 | (annotation, package) |
| kotlinx-coroutines-core/jvm/test/internal/ConcurrentWeakMapCollectionStressTest.kt | 28 | (annotation, arrays, inheritance, modifier data, named argument, nested class, package) |
| kotlinx-coroutines-core/jvm/test/internal/ConcurrentWeakMapOperationStressTest.kt | 70 | (annotation, arrays, destructuring declaration, inheritance, inline-only call, modifier data, named argument, nested class, package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/internal/ConcurrentWeakMapTest.kt | 42 | (annotation, destructuring declaration, inheritance, inline-only call, package) |
| kotlinx-coroutines-core/jvm/test/internal/FastServiceLoaderTest.kt | 17 | (annotation, class literal, inheritance, inline-only call, package) |
| kotlinx-coroutines-core/jvm/test/internal/LockFreeLinkedListLongStressTest.kt | 81 | (annotation, as, bitwise operators, do-while, extension function, inheritance, is, modifier data, named argument, nested class, package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/internal/LockFreeTaskQueueStressTest.kt | 116 | (annotation, arrays, class literal, inheritance, inline-only call, modifier companion, named argument, nested class, package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/internal/LockFreeTaskQueueTest.kt | 73 | (annotation, arrays, class literal, inheritance, modifier companion, package) |
| kotlinx-coroutines-core/jvm/test/internal/OnDemandAllocatingPoolLincheckTest.kt | 54 | (annotation, class literal, extension function, inheritance, member extension function, modifier abstract, modifier override, named argument, package) |
| kotlinx-coroutines-core/jvm/test/internal/ThreadSafeHeapStressTest.kt | 63 | (annotation, inheritance, modifier override, named argument, nested class, package, try) |
| kotlinx-coroutines-core/jvm/test/internal/ThreadSafeHeapTest.kt | 93 | (annotation, arrays, inheritance, is, modifier override, nested class, package, require, check, error, uninitialized local variable) |
| kotlinx-coroutines-core/jvm/test/jdk8/future/AsFutureTest.kt | 121 | (annotation, inheritance, inline-only call, named argument, package, throw, try) |
| kotlinx-coroutines-core/jvm/test/jdk8/future/FutureAsDeferredUnhandledCompletionExceptionTest.kt | 35 | (annotation, inheritance, is, modifier lateinit, package, property without initializer, throw) |
| kotlinx-coroutines-core/jvm/test/jdk8/future/FutureExceptionsTest.kt | 84 | (annotation, default value, inheritance, is, package, try) |
| kotlinx-coroutines-core/jvm/test/jdk8/future/FutureTest.kt | 614 | (annotation, class literal, destructuring declaration, extension function, inheritance, inline-only call, is, modifier inline, modifier override, modifier reified, modifier suspend, modifier vararg, named argument, object expression, package, reified member type parameter, throw, try) |
| kotlinx-coroutines-core/jvm/test/jdk8/stream/ConsumeAsFlowTest.kt | 35 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/jdk8/time/DurationOverflowTest.kt | 76 | (annotation, inheritance, named argument, package, prefix increment) |
| kotlinx-coroutines-core/jvm/test/jdk8/time/FlowDebounceTest.kt | 35 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/jdk8/time/FlowSampleTest.kt | 35 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/jdk8/time/WithTimeoutTest.kt | 65 | (annotation, inheritance, package, try) |
| kotlinx-coroutines-core/jvm/test/knit/ClosedAfterGuideTestExecutor.kt | 36 | (as, inheritance, modifier override, object expression, package, qualified this) |
| kotlinx-coroutines-core/jvm/test/knit/TestUtil.kt | 171 | (arrays, as, default value, extension function, identity equality, inline-only call, is, modifier const, modifier enum, modifier inline, modifier vararg, named argument, package, require, check, error, spread operator, throw, try) |
| kotlinx-coroutines-core/jvm/test/lincheck/ChannelsLincheckTest.kt | 273 | (annotation, as, class literal, default value, destructuring declaration, extension function, identity equality, inheritance, inline-only call, is, member extension function, modifier abstract, modifier open, modifier override, modifier protected, modifier suspend, named argument, package, super, try, when subject variable) |
| kotlinx-coroutines-core/jvm/test/lincheck/LockFreeTaskQueueLincheckTest.kt | 44 | (annotation, class literal, extension function, inheritance, member extension function, modifier abstract, modifier override, modifier protected, named argument, package) |
| kotlinx-coroutines-core/jvm/test/lincheck/MutexLincheckTest.kt | 42 | (annotation, class literal, extension function, extension property, inheritance, member extension function, modifier override, modifier suspend, package) |
| kotlinx-coroutines-core/jvm/test/lincheck/ResizableAtomicArrayLincheckTest.kt | 20 | (annotation, class literal, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/lincheck/SemaphoreLincheckTest.kt | 31 | (annotation, class literal, extension function, inheritance, member extension function, modifier abstract, modifier override, modifier suspend, named argument, package) |
| kotlinx-coroutines-core/jvm/test/scheduling/BlockingCoroutineDispatcherLivenessStressTest.kt | 61 | (annotation, inheritance, package, try) |
| kotlinx-coroutines-core/jvm/test/scheduling/BlockingCoroutineDispatcherMixedStealingStressTest.kt | 77 | (annotation, inheritance, is, package) |
| kotlinx-coroutines-core/jvm/test/scheduling/BlockingCoroutineDispatcherTerminationStressTest.kt | 39 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/scheduling/BlockingCoroutineDispatcherTest.kt | 174 | (annotation, class literal, inheritance, package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/scheduling/BlockingCoroutineDispatcherThreadLimitStressTest.kt | 58 | (annotation, inheritance, package, try) |
| kotlinx-coroutines-core/jvm/test/scheduling/BlockingCoroutineDispatcherWorkSignallingStressTest.kt | 95 | (annotation, extension function, inheritance, package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/scheduling/CoroutineDispatcherTest.kt | 144 | (annotation, inheritance, inline-only call, is, package, require, check, error) |
| kotlinx-coroutines-core/jvm/test/scheduling/CoroutineSchedulerCloseStressTest.kt | 76 | (annotation, arrays, bitwise operators, class literal, extension function, inheritance, modifier companion, modifier enum, modifier lateinit, named argument, nested class, package, property without initializer, try) |
| kotlinx-coroutines-core/jvm/test/scheduling/CoroutineSchedulerLivenessStressTest.kt | 49 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/scheduling/CoroutineSchedulerOversubscriptionTest.kt | 86 | (annotation, extension function, inheritance, package, require, check, error, try) |
| kotlinx-coroutines-core/jvm/test/scheduling/CoroutineSchedulerStressTest.kt | 105 | (annotation, inheritance, modifier inner, modifier override, nested class, package, prefix increment, require, check, error) |
| kotlinx-coroutines-core/jvm/test/scheduling/CoroutineSchedulerTest.kt | 166 | (annotation, arrays, class literal, inheritance, inline-only call, named argument, package, prefix increment, require, check, error, try) |
| kotlinx-coroutines-core/jvm/test/scheduling/DefaultDispatchersTest.kt | 72 | (annotation, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/scheduling/LimitingCoroutineDispatcherStressTest.kt | 52 | (annotation, inheritance, package, try) |
| kotlinx-coroutines-core/jvm/test/scheduling/LimitingDispatcherTest.kt | 41 | (annotation, class literal, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/scheduling/SchedulerTestBase.kt | 116 | (Char, annotation, default value, extension function, inheritance, is, modifier abstract, modifier companion, modifier override, modifier protected, modifier suspend, object expression, package, property getter, qualified this) |
| kotlinx-coroutines-core/jvm/test/scheduling/SharingWorkerClassTest.kt | 43 | (annotation, inheritance, named argument, package, try) |
| kotlinx-coroutines-core/jvm/test/scheduling/TestTimeSource.kt | 11 | (default value, inheritance, modifier override, package) |
| kotlinx-coroutines-core/jvm/test/scheduling/WorkQueueStressTest.kt | 121 | (annotation, arrays, extension function, inheritance, inline-only call, named argument, package) |
| kotlinx-coroutines-core/jvm/test/scheduling/WorkQueueTest.kt | 108 | (annotation, extension function, inheritance, inline-only call, package) |
| kotlinx-coroutines-core/jvm/test/selects/SelectDeadlockStressTest.kt | 59 | (annotation, extension function, inheritance, modifier override, nested class, package) |
| kotlinx-coroutines-core/jvm/test/selects/SelectMemoryLeakStressTest.kt | 56 | (annotation, arrays, inheritance, package) |
| kotlinx-coroutines-core/jvm/test/selects/SelectPhilosophersStressTest.kt | 63 | (annotation, arrays, destructuring lambda parameter, inheritance, inline-only call, modifier suspend, package) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

