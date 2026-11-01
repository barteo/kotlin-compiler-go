# Survey: arrow 2.2.3

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 107 files, 26909 lines

Directories: `arrow-libs/core/arrow-annotations/src/commonMain`, `arrow-libs/core/arrow-atomic/src/commonMain`, `arrow-libs/core/arrow-autoclose/src/commonMain`, `arrow-libs/core/arrow-cache4k/src/commonMain`, `arrow-libs/core/arrow-core/src/commonMain`, `arrow-libs/core/arrow-core-high-arity/src/commonMain`, `arrow-libs/core/arrow-eval/src/commonMain`, `arrow-libs/core/arrow-exception-utils/src/commonMain`, `arrow-libs/core/arrow-functions/src/commonMain`, `arrow-libs/core/arrow-platform/src/commonMain`, `arrow-libs/core/arrow-atomic/src/androidAndJvmMain`, `arrow-libs/core/arrow-core/src/androidAndJvmMain`, `arrow-libs/core/arrow-exception-utils/src/androidAndJvmMain`, `arrow-libs/core/arrow-functions/src/androidAndJvmMain`, `arrow-libs/core/arrow-platform/src/jvmMain`, `arrow-libs/fx/arrow-collectors/src/commonMain`, `arrow-libs/fx/arrow-fx-coroutines/src/commonMain`, `arrow-libs/fx/arrow-fx-stm/src/commonMain`, `arrow-libs/fx/arrow-collectors/src/androidAndJvmMain`, `arrow-libs/fx/arrow-fx-coroutines/src/androidAndJvmMain`, `arrow-libs/fx/arrow-fx-stm/src/androidAndJvmMain`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 107 | 9 |
| 1: packages, annotations | 0 | 107 | 7 |
| 2: interfaces, inheritance | 0 | 107 | 6 |
| 3: type checks | 0 | 107 | 5 |
| 4: exceptions | 0 | 107 | 5 |
| 5: objects | 0 | 107 | 5 |
| 6: data, enum, sealed | 0 | 107 | 4 |
| 7: arrays, vararg, spread | 0 | 107 | 4 |
| 8: Char, Float, Byte, Short | 0 | 107 | 4 |
| 9: inliner | 0 | 107 | 4 |
| 10: lambdas with receivers | 0 | 107 | 3 |
| 11: extensions | 3 | 104 | 3 |
| 12: operator conventions | 12 | 95 | 3 |
| 13: identity, class literals, references | 13 | 94 | 2 |
| 14: class delegation | 13 | 94 | 2 |
| 15: secondary constructors | 13 | 94 | 2 |
| 16: local functions | 13 | 94 | 2 |
| 17: expect and actual | 22 | 85 | 2 |
| 18: annotation classes | 25 | 82 | 2 |
| 19: value classes | 26 | 81 | 2 |
| 20: unsigned integers | 26 | 81 | 2 |
| definite assignment | 26 | 81 | 2 |
| 21: inline functions | 32 | 75 | 2 |
| 22: reified type parameters | 32 | 75 | 2 |
| 23: labels, do-while | 32 | 75 | 1 |
| 24: delegated properties | 33 | 74 | 1 |
| 25: object expressions | 33 | 74 | 1 |
| 26: destructuring lambda parameters | 37 | 70 | 1 |
| 42-49: coroutines | 62 | 45 | 1 |
| 52: bitwise operators | 62 | 45 | 1 |
| 53: typealias | 72 | 35 | 1 |
| 54: setters, backing fields | 76 | 31 | 1 |
| 55: lateinit, tailrec | 79 | 28 | 1 |
| 56-57: collection supertypes | 83 | 24 | 1 |
| 58: member extension functions | 86 | 21 | 1 |
| 59: reified member type parameters | 87 | 20 | 1 |
| 60: inner and local classes | 88 | 19 | 1 |
| 61: when guards, anonymous functions, local extension functions | 94 | 13 | 1 |
| 62a: @JvmField and @JvmStatic in objects | 94 | 13 | 1 |
| 62b: generic value classes | 98 | 9 | 2 |
| 63: definitely non-nullable types | 98 | 9 | 2 |
| 70a: inline property accessors | 98 | 9 | 2 |
| 70b: where clauses | 98 | 9 | 2 |
| 70c: fun interfaces | 98 | 9 | 2 |
| delegated extension properties | 98 | 9 | 2 |
| qualified super | 98 | 9 | 2 |
| context parameters of top-level functions | 105 | 2 | 1 |
| also supported | 106 | 1 | 1 |

### Features left, by files blocked

106 of 107 files use no feature that kc doesn't support.

| Feature | Files | Only blocker | Uses |
|:---|---:|---:|---:|
| member extension property | 1 | 1 | 1 |

### Greedy order of the features left

At each step, the feature that unblocks the most files given the ones before it, or, if none unblocks a file, the one that blocks the most files.

| Step | Feature | Files unblocked |
|---:|:---|---:|
| 1 | member extension property | 107 |

### Inline-only functions called

`require`, `check`, `requireNotNull`, `checkNotNull`, `error` and `TODO` are built in; kc inlines the others, since milestone 22 also those with reified type parameters.

| Function | Files | Calls |
|:---|---:|---:|
| `also` | 18 | 41 |
| `let` | 18 | 39 |
| `suspend` | 13 | 658 |
| `recover` | 9 | 39 |
| `getOrElse` | 8 | 15 |
| `isNotEmpty` | 7 | 16 |
| `with` | 7 | 36 |
| `buildList` | 6 | 12 |
| `require` | 6 | 6 |
| `List` | 4 | 21 |
| `takeIf` | 4 | 6 |
| `apply` | 3 | 6 |
| `buildMap` | 3 | 27 |
| `run` | 3 | 18 |
| `Sequence` | 2 | 21 |
| `printStackTrace` | 2 | 2 |
| `runCatching` | 2 | 2 |
| `use` | 2 | 2 |
| `buildSet` | 1 | 1 |
| `check` | 1 | 2 |
| `copyOf` | 1 | 4 |
| `error` | 1 | 2 |
| `find` | 1 | 1 |
| `insert` | 1 | 5 |
| `orEmpty` | 1 | 1 |
| `plusAssign` | 1 | 3 |

### Files

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| arrow-libs/core/arrow-annotations/src/commonMain/kotlin/arrow/optics/optics.kt | 19 | (annotation, arrays, default value, modifier annotation, modifier enum, nested class, package) |
| arrow-libs/core/arrow-annotations/src/commonMain/kotlin/arrow/synthetic.kt | 34 | (annotation, modifier annotation, package) |
| arrow-libs/core/arrow-atomic/src/androidAndJvmMain/kotlin/arrow/atomic/Atomic.kt | 7 | (annotation, modifier actual, package, typealias) |
| arrow-libs/core/arrow-atomic/src/androidAndJvmMain/kotlin/arrow/atomic/AtomicInt.kt | 7 | (annotation, modifier actual, package, typealias) |
| arrow-libs/core/arrow-atomic/src/androidAndJvmMain/kotlin/arrow/atomic/AtomicLong.kt | 7 | (annotation, modifier actual, package, typealias) |
| arrow-libs/core/arrow-atomic/src/commonMain/kotlin/arrow/atomic/Atomic.kt | 103 | (annotation, class literal, do-while, extension function, extension property, function without body, inline-only call, modifier expect, modifier inline, package, property getter, property setter) |
| arrow-libs/core/arrow-atomic/src/commonMain/kotlin/arrow/atomic/AtomicBoolean.kt | 89 | (annotation, class literal, do-while, extension function, inline-only call, modifier inline, package, property getter, property setter) |
| arrow-libs/core/arrow-atomic/src/commonMain/kotlin/arrow/atomic/AtomicInt.kt | 84 | (annotation, class literal, do-while, extension function, extension property, function without body, inline-only call, modifier expect, modifier inline, package, property getter, property setter) |
| arrow-libs/core/arrow-atomic/src/commonMain/kotlin/arrow/atomic/AtomicLong.kt | 84 | (annotation, class literal, do-while, extension function, extension property, function without body, inline-only call, modifier expect, modifier inline, package, property getter, property setter) |
| arrow-libs/core/arrow-autoclose/src/commonMain/kotlin/arrow/AutoCloseScope.kt | 119 | (annotation, class literal, function type with receiver, function without body, inheritance, inline-only call, interface, is, modifier annotation, modifier inline, modifier override, package, throw, try) |
| arrow-libs/core/arrow-autoclose/src/commonMain/kotlin/arrow/ThrowIfFatal.kt | 11 | (annotation, extension function, package) |
| arrow-libs/core/arrow-cache4k/src/commonMain/kotlin/arrow/core/Cache4k.kt | 18 | (annotation, class literal, function type with receiver, generic value class, inheritance, inline-only call, modifier override, modifier value, package) |
| arrow-libs/core/arrow-core-high-arity/src/commonMain/kotlin/arrow/core/Iterable.kt | 1065 | (annotation, class literal, extension function, inline-only call, modifier inline, package) |
| arrow-libs/core/arrow-core-high-arity/src/commonMain/kotlin/arrow/core/NonEmptyList.kt | 252 | (annotation, class literal, extension function, modifier inline, package) |
| arrow-libs/core/arrow-core-high-arity/src/commonMain/kotlin/arrow/core/Sequence.kt | 942 | (annotation, collection supertype, extension function, inheritance, inline-only call, modifier override, object expression, package, qualified this) |
| arrow-libs/core/arrow-core-high-arity/src/commonMain/kotlin/arrow/core/map.kt | 952 | (annotation, as, destructuring lambda parameter, extension function, inline-only call, modifier inline, package, qualified this) |
| arrow-libs/core/arrow-core-high-arity/src/commonMain/kotlin/arrow/core/raise/RaiseAccumulate.kt | 639 | (annotation, class literal, delegated property, extension function, function type with receiver, modifier inline, package) |
| arrow-libs/core/arrow-core/src/androidAndJvmMain/kotlin/arrow/core/ArrowCoreInternalException.kt | 9 | (annotation, inheritance, modifier actual, modifier override, object, package) |
| arrow-libs/core/arrow-core/src/androidAndJvmMain/kotlin/arrow/core/MemoizedDeepRecursiveFunction.kt | 12 | (annotation, default value, generic value class, inheritance, modifier override, modifier value, package) |
| arrow-libs/core/arrow-core/src/androidAndJvmMain/kotlin/arrow/core/SortedMapK.kt | 9 | (extension function, package) |
| arrow-libs/core/arrow-core/src/androidAndJvmMain/kotlin/arrow/core/raise/CancellationExceptionNoTrace.kt | 19 | (annotation, arrays, class literal, inheritance, modifier actual, modifier override, package) |
| arrow-libs/core/arrow-core/src/androidAndJvmMain/kotlin/arrow/core/raise/RaiseCancellationException.kt | 9 | (annotation, modifier actual, modifier sealed, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/ArrowCoreInternalException.kt | 4 | (annotation, inheritance, modifier expect, object, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Comparison.kt | 69 | (Byte and Short, arrays, context function, extension function, inline-only call, modifier inline, modifier operator, modifier vararg, package, spread operator) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Either.kt | 1623 | (annotation, as, callable reference, class literal, extension function, function type with receiver, inheritance, inline-only call, is, modifier companion, modifier const, modifier data, modifier infix, modifier inline, modifier operator, modifier override, modifier reified, modifier sealed, nested class, package, qualified this, reified member type parameter, throw, typealias, when subject variable) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Ior.kt | 531 | (annotation, callable reference, class literal, extension function, inheritance, inline-only call, is, modifier companion, modifier data, modifier inline, modifier operator, modifier override, modifier sealed, nested class, package, qualified this, typealias, when subject variable) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Iterable.kt | 921 | (annotation, callable reference, class literal, destructuring lambda parameter, do-while, extension function, function type with receiver, identity equality, inline-only call, is, modifier infix, modifier inline, modifier operator, modifier tailrec, package, qualified this, uninitialized local variable, when subject variable) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/MemoizedDeepRecursiveFunction.kt | 43 | (annotation, default value, function type with receiver, function without body, generic value class, inheritance, interface, modifier override, modifier suspend, modifier value, named argument, package, when subject variable) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/NonEmptyCollection.kt | 52 | (annotation, as, class literal, collection supertype, function without body, inheritance, interface, modifier annotation, modifier crossinline, modifier inline, modifier operator, modifier override, package, property without initializer) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/NonEmptyList.kt | 501 | (annotation, as, callable reference, class delegation, class literal, collection supertype, destructuring declaration, do-while, extension function, function type with receiver, generic value class, inheritance, inline-only call, is, modifier companion, modifier inline, modifier operator, modifier override, modifier value, modifier vararg, package, prefix increment, property getter, require, check, error, secondary constructor, spread operator, typealias) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/NonEmptySet.kt | 140 | (annotation, as, class delegation, class literal, collection supertype, extension function, function type with receiver, generic value class, inheritance, inline-only call, modifier companion, modifier inline, modifier operator, modifier override, modifier value, modifier vararg, package, qualified this, require, check, error, secondary constructor, spread operator) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Option.kt | 695 | (annotation, callable reference, class literal, extension function, function type with receiver, inheritance, inline-only call, is, modifier companion, modifier data, modifier inline, modifier operator, modifier override, modifier reified, modifier sealed, modifier suspend, object, package, qualified this) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Pair.kt | 13 | (annotation, extension function, modifier operator, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Result.kt | 17 | (annotation, callable reference, class literal, extension function, modifier inline, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Sequence.kt | 760 | (annotation, callable reference, class literal, collection supertype, destructuring lambda parameter, extension function, function type with receiver, inheritance, inline-only call, is, modifier override, object expression, package, qualified this) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Triple.kt | 16 | (annotation, extension function, modifier operator, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Tuple4.kt | 32 | (annotation, extension function, modifier companion, modifier data, modifier operator, modifier override, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Tuple5.kt | 37 | (annotation, modifier companion, modifier data, modifier operator, modifier override, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Tuple6.kt | 41 | (annotation, modifier companion, modifier data, modifier operator, modifier override, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Tuple7.kt | 45 | (annotation, modifier companion, modifier data, modifier operator, modifier override, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Tuple8.kt | 49 | (annotation, modifier companion, modifier data, modifier operator, modifier override, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Tuple9.kt | 53 | (annotation, modifier companion, modifier data, modifier operator, modifier override, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/TupleN.kt | 24 | (annotation, extension function, modifier const, modifier operator, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/map.kt | 518 | (annotation, as, bitwise operators, class literal, collection supertype, destructuring declaration, destructuring lambda parameter, extension function, function type with receiver, inheritance, inline-only call, is, modifier inline, modifier override, modifier reified, package, qualified this) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/predef.kt | 23 | (annotation, as, callable reference, identity equality, modifier inline, object, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/Builders.kt | 415 | (annotation, as, callable reference, class delegation, class literal, default value, destructuring lambda parameter, extension function, function type with receiver, identity equality, inheritance, inline-only call, is, modifier inline, modifier noinline, modifier override, named argument, package, property getter, qualified this, try) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/Effect.kt | 681 | (annotation, class literal, extension function, function type with receiver, modifier inline, modifier noinline, modifier suspend, package, typealias) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/ErrorHandlers.kt | 163 | (annotation, callable reference, class literal, extension function, function type with receiver, inline-only call, modifier crossinline, modifier infix, modifier inline, modifier reified, modifier suspend, package, suspend function type) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/Fold.kt | 301 | (annotation, as, class literal, default value, extension function, function type with receiver, function without body, identity equality, inheritance, inline-only call, is, modifier annotation, modifier const, modifier expect, modifier inline, modifier override, modifier sealed, modifier suspend, package, property without initializer, suspend function type, throw, try) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/Mappers.kt | 43 | (annotation, extension function, inline-only call, modifier inline, modifier suspend, package, suspend function type) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/Raise.kt | 741 | (annotation, callable reference, class literal, destructuring lambda parameter, extension function, function type with receiver, function without body, inline-only call, interface, is, member extension function, modifier annotation, modifier inline, modifier reified, modifier suspend, package, qualified this, throw, try) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/RaiseAccumulate.kt | 1196 | (annotation, as, callable reference, class delegation, class literal, destructuring lambda parameter, extension function, function type with receiver, function without body, inheritance, inline-only call, interface, is, member extension function, modifier abstract, modifier annotation, modifier crossinline, modifier inline, modifier open, modifier operator, modifier override, modifier sealed, modifier value, nested class, package, property without initializer, qualified this, require, check, error, secondary constructor) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/Trace.kt | 45 | (annotation, inline-only call, modifier annotation, modifier value, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/context/Builders.kt | 81 | (annotation, class literal, context function type, default value, modifier inline, modifier noinline, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/context/RaiseAccumulateCombineContext.kt | 290 | (annotation, class literal, context function, context function type, extension function, modifier inline, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/context/RaiseAccumulateContext.kt | 100 | (annotation, class literal, context function, context function type, extension function, inline-only call, modifier inline, package, qualified this, typealias) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/context/RaiseAccumulateNelContext.kt | 301 | (annotation, class literal, context function, context function type, extension function, function type with receiver, modifier inline, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/context/RaiseContext.kt | 152 | (annotation, class literal, context function, context function type, extension function, inline-only call, is, modifier inline, modifier suspend, package, qualified this, typealias) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/string.kt | 6 | (extension function, package) |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/utils.kt | 10 | (extension function, inline-only call, modifier const, package, qualified this, typealias) |
| arrow-libs/core/arrow-eval/src/commonMain/kotlin/arrow/eval/Eval.kt | 613 | (annotation, as, callable reference, default value, delegated property, extension function, function without body, inheritance, inline-only call, is, labels, local function, modifier abstract, modifier companion, modifier crossinline, modifier data, modifier inline, modifier operator, modifier override, modifier sealed, modifier suspend, modifier tailrec, nested class, object expression, package, qualified this, secondary constructor, throw) |
| arrow-libs/core/arrow-eval/src/commonMain/kotlin/arrow/eval/SuspendEval.kt | 237 | (annotation, as, callable reference, function without body, inheritance, inline-only call, interface, is, labels, local function, modifier abstract, modifier companion, modifier crossinline, modifier data, modifier inline, modifier override, modifier sealed, modifier suspend, modifier tailrec, nested class, object expression, package, property without initializer, qualified this, suspend function type, throw) |
| arrow-libs/core/arrow-exception-utils/src/androidAndJvmMain/kotlin/arrow/core/NonFatal.kt | 11 | (annotation, is, modifier actual, package) |
| arrow-libs/core/arrow-exception-utils/src/commonMain/kotlin/arrow/core/NonFatal.kt | 46 | (function without body, modifier expect, package) |
| arrow-libs/core/arrow-exception-utils/src/commonMain/kotlin/arrow/core/ThrowableUtils.kt | 20 | (annotation, class literal, extension function, inline-only call, is, modifier infix, package, qualified this, throw) |
| arrow-libs/core/arrow-exception-utils/src/commonMain/kotlin/arrow/core/nonFatalOrThrow.kt | 41 | (extension function, modifier inline, package, throw) |
| arrow-libs/core/arrow-functions/src/androidAndJvmMain/kotlin/arrow/core/composition-jvm.kt | 256 | (annotation, anonymous function, as, inheritance, is, modifier actual, modifier companion, modifier const, modifier data, modifier infix, modifier operator, modifier override, modifier sealed, modifier tailrec, nested class, package, when subject variable) |
| arrow-libs/core/arrow-functions/src/commonMain/kotlin/arrow/core/composition.kt | 10 | (annotation, anonymous function, modifier expect, modifier infix, package) |
| arrow-libs/core/arrow-functions/src/commonMain/kotlin/arrow/core/currying.kt | 295 | (annotation, anonymous function, inline-only call, package, suspend function type) |
| arrow-libs/core/arrow-functions/src/commonMain/kotlin/arrow/core/memoization.kt | 135 | (Byte and Short, annotation, anonymous function, function without body, inheritance, interface, modifier data, modifier operator, modifier override, package, qualified this) |
| arrow-libs/core/arrow-functions/src/commonMain/kotlin/arrow/core/partials.kt | 1775 | (annotation, anonymous function, inline-only call, package, suspend function type) |
| arrow-libs/core/arrow-platform/src/commonMain/kotlin/arrow/platform/Platform.kt | 16 | (modifier enum, modifier expect, package, property without initializer) |
| arrow-libs/core/arrow-platform/src/commonMain/kotlin/arrow/platform/test/Flaky.kt | 7 | (modifier annotation, modifier expect, package) |
| arrow-libs/core/arrow-platform/src/jvmMain/kotlin/arrow/platform/PlatformJvm.kt | 4 | (annotation, modifier actual, package) |
| arrow-libs/core/arrow-platform/src/jvmMain/kotlin/arrow/platform/test/Flaky.kt | 5 | (modifier actual, package, typealias) |
| arrow-libs/fx/arrow-collectors/src/androidAndJvmMain/kotlin/arrow/collectors/Collectors.kt | 42 | (annotation, callable reference, destructuring lambda parameter, extension function, named argument, package) |
| arrow-libs/fx/arrow-collectors/src/androidAndJvmMain/kotlin/arrow/collectors/JvmCollector.kt | 34 | (annotation, extension function, inline-only call, named argument, package, typealias) |
| arrow-libs/fx/arrow-collectors/src/commonMain/kotlin/arrow/collectors/Collect.kt | 145 | (annotation, as, class literal, default value, extension function, modifier suspend, package, uninitialized local variable) |
| arrow-libs/fx/arrow-collectors/src/commonMain/kotlin/arrow/collectors/Collector.kt | 262 | (annotation, callable reference, default value, destructuring lambda parameter, function without body, inheritance, inline-only call, interface, modifier companion, modifier enum, modifier override, modifier suspend, modifier vararg, named argument, object expression, package, property without initializer, suspend function type, typealias) |
| arrow-libs/fx/arrow-collectors/src/commonMain/kotlin/arrow/collectors/Collectors.kt | 137 | (annotation, as, callable reference, default value, destructuring lambda parameter, modifier data, named argument, object, package, when subject variable) |
| arrow-libs/fx/arrow-collectors/src/commonMain/kotlin/arrow/collectors/Zip.kt | 137 | (destructuring lambda parameter, inline-only call, package, suspend function type) |
| arrow-libs/fx/arrow-fx-coroutines/src/androidAndJvmMain/kotlin/arrow/fx/coroutines/ResourceExtensions.kt | 164 | (annotation, class literal, default value, extension function, inline-only call, modifier actual, modifier suspend, package, suspend function type) |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/Bracket.kt | 279 | (annotation, class literal, extension property, inheritance, inline-only call, is, modifier companion, modifier crossinline, modifier data, modifier inline, modifier override, modifier sealed, modifier suspend, nested class, object, package, property getter, require, check, error, suspend function type, throw, try) |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/CountDownLatch.kt | 47 | (annotation, modifier suspend, package, require, check, error) |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/CyclicBarrier.kt | 153 | (annotation, default value, destructuring declaration, inheritance, interface, is, modifier data, modifier override, modifier sealed, modifier suspend, named argument, nested class, package, property without initializer, require, check, error, throw, try, when subject variable) |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/ParMap.kt | 121 | (annotation, class delegation, class literal, default value, extension function, function type with receiver, inheritance, inline-only call, modifier suspend, package, qualified this) |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/ParZip.kt | 1007 | (annotation, as, class literal, default value, destructuring declaration, function type with receiver, modifier crossinline, modifier inline, modifier suspend, package) |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/ParZipOrAccumulate.kt | 856 | (annotation, class literal, extension function, function type with receiver, modifier crossinline, modifier inline, modifier suspend, package, qualified this) |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/Race2.kt | 123 | (annotation, class literal, default value, function type with receiver, inline-only call, is, modifier crossinline, modifier inline, modifier suspend, package) |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/Race3.kt | 126 | (annotation, class literal, default value, function type with receiver, inheritance, inline-only call, is, modifier crossinline, modifier data, modifier inline, modifier sealed, modifier suspend, nested class, package, qualified this, try) |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/Racing.kt | 162 | (annotation, class delegation, class literal, default value, extension function, function type with receiver, function without body, inheritance, inline-only call, interface, is, modifier annotation, modifier override, modifier suspend, named argument, package, qualified this, try) |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/RaiseRacing.kt | 62 | (annotation, class delegation, default value, extension function, function type with receiver, inheritance, inline-only call, interface, labels, modifier suspend, package, qualified this, typealias) |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/Resource.kt | 567 | (annotation, callable reference, class literal, default value, destructuring declaration, extension function, function type with receiver, function without body, inheritance, inline-only call, interface, is, member extension function, modifier annotation, modifier expect, modifier infix, modifier inline, modifier override, modifier suspend, object, package, property without initializer, suspend function type, throw, typealias) |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/await/AwaitAllScope.kt | 97 | (annotation, class delegation, class literal, default value, extension function, function type with receiver, inheritance, modifier annotation, modifier inner, modifier override, modifier suspend, nested class, package) |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/flow.kt | 283 | (annotation, callable reference, default value, extension function, function type with receiver, inline-only call, modifier crossinline, modifier inline, modifier suspend, package, require, check, error, suspend function type, throw, try) |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/resource/context/Resource.kt | 47 | (annotation, class literal, context function, context function type, default value, extension function, inline-only call, modifier inline, modifier suspend, package, qualified this, suspend function type, typealias) |
| arrow-libs/fx/arrow-fx-stm/src/androidAndJvmMain/kotlin/arrow/fx/stm/internal/RetryException.kt | 6 | (inheritance, modifier actual, modifier override, package) |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/STM.kt | 1705 | member extension property (annotation, anonymous function, class literal, extension function, extension property, function type with receiver, function without body, inline-only call, interface, is, member extension function, modifier infix, modifier inline, modifier noinline, modifier operator, modifier suspend, package, require, check, error, throw, when subject variable) |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/TArray.kt | 129 | (arrays, extension function, identity equality, modifier companion, modifier data, modifier override, modifier suspend, modifier vararg, package) |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/TMVar.kt | 198 | (extension function, inheritance, modifier companion, modifier data, modifier sealed, modifier suspend, nested class, object, package) |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/TMap.kt | 166 | (extension function, modifier companion, modifier data, modifier suspend, package) |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/TQueue.kt | 350 | (annotation, extension function, inheritance, inline-only call, is, modifier companion, modifier crossinline, modifier data, modifier inline, modifier sealed, modifier suspend, nested class, object, package, qualified this, throw) |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/TSemaphore.kt | 107 | (extension function, modifier companion, modifier data, modifier suspend, package, throw) |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/TSet.kt | 96 | (extension function, modifier companion, modifier data, modifier suspend, package) |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/TVar.kt | 234 | (annotation, as, identity equality, inline-only call, is, modifier companion, modifier override, modifier suspend, package, property getter) |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/internal/Hamt.kt | 155 | (annotation, arrays, as, extension function, identity equality, inheritance, inline-only call, is, modifier companion, modifier const, modifier data, modifier override, modifier sealed, modifier suspend, modifier vararg, nested class, package, spread operator, when subject variable) |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/internal/Impl.kt | 219 | (annotation, anonymous function, as, class literal, default value, destructuring declaration, destructuring lambda parameter, extension function, function type with receiver, identity equality, inheritance, inline-only call, labels, member extension function, modifier expect, modifier inline, modifier override, modifier suspend, nested class, object, package, qualified this, throw, try, when subject variable) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 191 files, 11404 lines

Directories: `arrow-libs/core/arrow-core/src/commonTest`, `arrow-libs/core/arrow-core/src/jvmTest`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 191 | 2 |
| 1: packages, annotations | 94 | 97 | 2 |
| 2: interfaces, inheritance | 94 | 97 | 2 |
| 3: type checks | 97 | 94 | 3 |
| 4: exceptions | 99 | 92 | 2 |
| 5: objects | 101 | 90 | 2 |
| 6: data, enum, sealed | 102 | 89 | 2 |
| 7: arrays, vararg, spread | 102 | 89 | 2 |
| 8: Char, Float, Byte, Short | 102 | 89 | 1 |
| 9: inliner | 127 | 64 | 2 |
| 10: lambdas with receivers | 127 | 64 | 2 |
| 11: extensions | 130 | 61 | 2 |
| 12: operator conventions | 130 | 61 | 2 |
| 13: identity, class literals, references | 141 | 50 | 1 |
| 14: class delegation | 141 | 50 | 1 |
| 15: secondary constructors | 141 | 50 | 1 |
| 16: local functions | 147 | 44 | 1 |
| 17: expect and actual | 147 | 44 | 1 |
| 18: annotation classes | 147 | 44 | 1 |
| 19: value classes | 147 | 44 | 1 |
| 20: unsigned integers | 147 | 44 | 1 |
| definite assignment | 147 | 44 | 1 |
| 21: inline functions | 147 | 44 | 1 |
| 22: reified type parameters | 148 | 43 | 1 |
| 23: labels, do-while | 148 | 43 | 1 |
| 24: delegated properties | 150 | 41 | 1 |
| 25: object expressions | 150 | 41 | 1 |
| 26: destructuring lambda parameters | 155 | 36 | 1 |
| 42-49: coroutines | 177 | 14 | 1 |
| 52: bitwise operators | 177 | 14 | 1 |
| 53: typealias | 177 | 14 | 1 |
| 54: setters, backing fields | 177 | 14 | 1 |
| 55: lateinit, tailrec | 177 | 14 | 1 |
| 56-57: collection supertypes | 178 | 13 | 1 |
| 58: member extension functions | 178 | 13 | 1 |
| 59: reified member type parameters | 178 | 13 | 1 |
| 60: inner and local classes | 181 | 10 | 1 |
| 61: when guards, anonymous functions, local extension functions | 190 | 1 | 1 |
| 62a: @JvmField and @JvmStatic in objects | 190 | 1 | 1 |
| 62b: generic value classes | 190 | 1 | 1 |
| 63: definitely non-nullable types | 190 | 1 | 1 |
| 70a: inline property accessors | 190 | 1 | 1 |
| 70b: where clauses | 190 | 1 | 1 |
| 70c: fun interfaces | 190 | 1 | 1 |
| delegated extension properties | 190 | 1 | 1 |
| qualified super | 190 | 1 | 1 |
| context parameters of top-level functions | 191 | 0 | – |
| also supported | 191 | 0 | – |

### Features left, by files blocked

191 of 191 files use no feature that kc doesn't support.

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
| `recover` | 23 | 51 |
| `getOrElse` | 13 | 45 |
| `let` | 12 | 42 |
| `matches` | 7 | 7 |
| `isNotEmpty` | 6 | 9 |
| `suspend` | 5 | 102 |
| `also` | 4 | 7 |
| `runCatching` | 4 | 8 |
| `require` | 3 | 10 |
| `takeIf` | 3 | 5 |
| `List` | 2 | 18 |
| `bufferedReader` | 2 | 4 |
| `getOrThrow` | 2 | 7 |
| `with` | 2 | 2 |
| `Iterable` | 1 | 1 |
| `buildList` | 1 | 1 |
| `buildSet` | 1 | 1 |
| `error` | 1 | 2 |
| `find` | 1 | 2 |
| `firstNotNullOfOrNull` | 1 | 1 |
| `onFailure` | 1 | 2 |
| `toPair` | 1 | 3 |

### Files

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/BooleanTest.kt | 14 | (annotation, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/CollectionsSyntaxTests.kt | 37 | (annotation, destructuring declaration, inline-only call, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/ComparisonKtTest.kt | 210 | (annotation, arrays, callable reference, destructuring declaration, extension function, inheritance, modifier companion, modifier override, named argument, package, spread operator) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/EitherTest.kt | 1043 | (annotation, callable reference, class literal, inheritance, inline-only call, is, local class, local function, modifier data, modifier override, package, throw, try) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/IorTest.kt | 247 | (annotation, callable reference, destructuring lambda parameter, inline-only call, is, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/IterableTest.kt | 632 | (annotation, callable reference, destructuring declaration, destructuring lambda parameter, extension function, inline-only call, is, local class, local extension function, local function, modifier data, named argument, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/KotestInlineUtils.kt | 56 | (annotation, as, class literal, extension function, is, modifier inline, modifier reified, package, qualified this, throw, try) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/ListKTest.kt | 29 | (annotation, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/MapKTest.kt | 842 | (annotation, callable reference, destructuring declaration, destructuring lambda parameter, extension function, is, local class, local extension function, local function, modifier data, named argument, package, when subject variable) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/NonEmptyListTest.kt | 603 | (annotation, callable reference, class delegation, class literal, collection supertype, inheritance, inline-only call, local class, local function, modifier data, modifier override, named argument, nested class, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/NonEmptySetTest.kt | 244 | (annotation, callable reference, class delegation, class literal, collection supertype, inheritance, inline-only call, local function, modifier override, named argument, nested class, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/OptionTest.kt | 370 | (Char, annotation, callable reference, inline-only call, is, local function, modifier vararg, named argument, package, throw) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/ResultTest.kt | 44 | (annotation, inline-only call, local function, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/SequenceKTest.kt | 569 | (Byte and Short, Char, annotation, arrays, callable reference, destructuring declaration, destructuring lambda parameter, inline-only call, is, local class, local function, modifier data, named argument, package, uninitialized local variable) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/TupleTest.kt | 165 | (annotation, extension function, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/extensions/NumberInstancesTest.kt | 56 | (annotation, callable reference, default value, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/EagerEffectSpec.kt | 296 | (annotation, callable reference, inline-only call, local function, package, require, check, error, suspend function type, throw, try) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/EffectSpec.kt | 813 | (Char, annotation, callable reference, class literal, extension function, inline-only call, local function, modifier data, modifier suspend, named argument, package, require, check, error, suspend function type, throw, try) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/IorAccumulateSpec.kt | 257 | (annotation, callable reference, class literal, delegated property, inline-only call, named argument, package, throw, try) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/IorSpec.kt | 319 | (annotation, as, callable reference, destructuring declaration, function without body, inline-only call, is, named argument, package, throw, try) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/MappersSpec.kt | 139 | (annotation, package, throw) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/NullableSpec.kt | 166 | (annotation, as, callable reference, extension function, function type with receiver, inline-only call, local extension function, local function, package, throw) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/OptionSpec.kt | 58 | (annotation, callable reference, inline-only call, local function, package, throw) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/RaiseAccumulateSpec.kt | 146 | (annotation, callable reference, class literal, delegated property, package, try) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/ResultSpec.kt | 139 | (annotation, inline-only call, named argument, package, throw) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/SingletonSpec.kt | 307 | (annotation, inline-only call, package, throw) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/StructuredConcurrencySpec.kt | 241 | (annotation, callable reference, extension function, local extension function, local function, package, require, check, error) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/TraceSpec.kt | 57 | (annotation, callable reference, class literal, package, throw) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/context/RaiseAccumulateContextTest.kt | 46 | (annotation, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/predef.kt | 11 | (callable reference, extension function, modifier suspend, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/test/Generators.kt | 236 | (annotation, anonymous function, as, callable reference, default value, destructuring declaration, destructuring lambda parameter, extension function, inline-only call, modifier suspend, named argument, package, suspend function type, throw) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/test/GeneratorsTest.kt | 139 | (annotation, default value, inline-only call, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/test/Laws.kt | 27 | (callable reference, default value, extension function, inline-only call, interface, modifier data, modifier vararg, named argument, package, property without initializer, suspend function type) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/test/laws/MonoidLaws.kt | 48 | (default value, inheritance, modifier data, modifier override, modifier suspend, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/test/laws/SemigroupLaws.kt | 24 | (default value, inheritance, modifier data, modifier override, modifier suspend, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/test/laws/SemiringLaws.kt | 106 | (default value, inheritance, modifier data, modifier override, modifier suspend, package) |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/usage/Context.kt | 38 | (context function, named argument, package) |
| arrow-libs/core/arrow-core/src/jvmTest/java/arrow/core/DeadlockTest.kt | 55 | (annotation, class literal, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/arrow/core/raise/EffectUsage.kt | 51 | (callable reference, extension function, inheritance, interface, is, modifier data, modifier sealed, modifier suspend, object, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/arrow/core/raise/TraceJvmSpec.kt | 25 | (annotation, class literal, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-effect-error-01.kt | 14 | (inline-only call, object, package, throw) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-effect-error-02.kt | 14 | (object, package, throw) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-effect-error-03.kt | 16 | (object, package, throw) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-effect-error-04.kt | 14 | (callable reference, object, package, throw) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-effect-error-05.kt | 13 | (callable reference, object, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-01.kt | 14 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-02.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-03.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-04.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-05.kt | 15 | (inline-only call, package, throw) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-06.kt | 21 | (inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-07.kt | 15 | (inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-08.kt | 27 | (inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-09.kt | 31 | (inline-only call, is, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-10.kt | 24 | (inheritance, inline-only call, modifier sealed, object, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-11.kt | 35 | (inheritance, inline-only call, is, modifier sealed, object, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-12.kt | 16 | (inheritance, modifier sealed, modifier suspend, object, package, throw) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-13.kt | 13 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-14.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-15.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-16.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-17.kt | 11 | (inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-18.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-19.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-20.kt | 16 | (inline-only call, is, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-21.kt | 15 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-22.kt | 15 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-23.kt | 14 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-24.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-25.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-26.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-27.kt | 9 | (callable reference, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-28.kt | 9 | (callable reference, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-29.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-30.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-31.kt | 12 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-32.kt | 14 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-33.kt | 10 | (inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-34.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-35.kt | 12 | (inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-36.kt | 12 | (Char, inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-37.kt | 22 | (package, throw) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-01.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-02.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-03.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-04.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-05.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-06.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-07.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-08.kt | 13 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-09.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-10.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-11.kt | 12 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-01.kt | 19 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-02.kt | 19 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-03.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-04.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-05.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-06.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-07.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-08.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-09.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-10.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-11.kt | 13 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-12.kt | 12 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-13.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-14.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-15.kt | 12 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-16.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-17.kt | 12 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-01.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-02.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-03.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-04.kt | 12 | (destructuring lambda parameter, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-05.kt | 13 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-06.kt | 16 | (destructuring lambda parameter, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-07.kt | 12 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-08.kt | 16 | (destructuring lambda parameter, inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-nonemptylist-01.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-nonemptylist-02.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-nonemptylist-03.kt | 17 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-nonemptylist-04.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-nonemptylist-05.kt | 17 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-nonemptylist-06.kt | 20 | (modifier data, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-01.kt | 13 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-02.kt | 9 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-03.kt | 17 | (inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-04.kt | 17 | (inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-05.kt | 17 | (is, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-06.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-07.kt | 15 | (is, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-08.kt | 15 | (is, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-09.kt | 17 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-10.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-11.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-12.kt | 12 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-13.kt | 14 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-14.kt | 16 | (arrays, inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-15.kt | 12 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-16.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-17.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-18.kt | 14 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-19.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-20.kt | 14 | (inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-21.kt | 17 | (inline-only call, local function, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-01.kt | 18 | (inline-only call, object, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-02.kt | 52 | (annotation, inheritance, inline-only call, interface, modifier override, modifier sealed, modifier suspend, modifier value, object, package, try) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-03.kt | 16 | (callable reference, extension function, modifier suspend, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-04.kt | 55 | (Char, extension function, inline-only call, modifier suspend, package, throw) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-05.kt | 25 | (callable reference, modifier suspend, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-06.kt | 32 | (callable reference, extension function, modifier suspend, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-07.kt | 31 | (callable reference, extension function, modifier suspend, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-08.kt | 29 | (inline-only call, modifier suspend, named argument, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-09.kt | 34 | (extension function, inline-only call, local extension function, local function, modifier suspend, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-10.kt | 59 | (annotation, callable reference, inheritance, inline-only call, interface, modifier override, modifier sealed, modifier suspend, modifier value, object, package, try) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-11.kt | 21 | (modifier suspend, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-12.kt | 22 | (modifier suspend, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-13.kt | 25 | (inline-only call, modifier suspend, package, suspend function type) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-01.kt | 10 | (extension function, inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-02.kt | 25 | (Char, extension function, inline-only call, modifier suspend, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-03.kt | 29 | (callable reference, extension function, modifier suspend, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-04.kt | 23 | (Char, extension function, inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-05.kt | 19 | (inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-06.kt | 15 | (inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-07.kt | 20 | (extension function, inline-only call, local extension function, local function, package, throw) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-08.kt | 20 | (extension function, inline-only call, local extension function, local function, package, throw) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-09.kt | 21 | (local function, package, throw) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-10.kt | 21 | (local function, package, throw) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-11.kt | 15 | (callable reference, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-12.kt | 10 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-01.kt | 22 | (extension function, local extension function, local function, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-02.kt | 16 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-03.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-04.kt | 14 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-05.kt | 14 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-06.kt | 14 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-07.kt | 14 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-08.kt | 14 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-09.kt | 14 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-10.kt | 13 | (destructuring lambda parameter, inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-11.kt | 13 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-12.kt | 12 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-13.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-14.kt | 11 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-15.kt | 16 | (inline-only call, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-16.kt | 15 | (package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/test/EitherKnitTest.kt | 72 | (annotation, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/test/IterableKnitTest.kt | 72 | (annotation, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/test/OptionKnitTest.kt | 16 | (annotation, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/test/RaiseKnitTest.kt | 52 | (annotation, package) |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/test/SequenceKnitTest.kt | 28 | (annotation, package) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

