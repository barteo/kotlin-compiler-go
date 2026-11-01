# Survey: arrow 2.2.3

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 107 files, 26909 lines

Directories: `arrow-libs/core/arrow-annotations/src/commonMain`, `arrow-libs/core/arrow-atomic/src/commonMain`, `arrow-libs/core/arrow-autoclose/src/commonMain`, `arrow-libs/core/arrow-cache4k/src/commonMain`, `arrow-libs/core/arrow-core/src/commonMain`, `arrow-libs/core/arrow-core-high-arity/src/commonMain`, `arrow-libs/core/arrow-eval/src/commonMain`, `arrow-libs/core/arrow-exception-utils/src/commonMain`, `arrow-libs/core/arrow-functions/src/commonMain`, `arrow-libs/core/arrow-platform/src/commonMain`, `arrow-libs/core/arrow-atomic/src/androidAndJvmMain`, `arrow-libs/core/arrow-core/src/androidAndJvmMain`, `arrow-libs/core/arrow-exception-utils/src/androidAndJvmMain`, `arrow-libs/core/arrow-functions/src/androidAndJvmMain`, `arrow-libs/core/arrow-platform/src/jvmMain`, `arrow-libs/fx/arrow-collectors/src/commonMain`, `arrow-libs/fx/arrow-fx-coroutines/src/commonMain`, `arrow-libs/fx/arrow-fx-stm/src/commonMain`, `arrow-libs/fx/arrow-collectors/src/androidAndJvmMain`, `arrow-libs/fx/arrow-fx-coroutines/src/androidAndJvmMain`, `arrow-libs/fx/arrow-fx-stm/src/androidAndJvmMain`

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

`require`, `check`, `requireNotNull`, `checkNotNull`, `error` and `TODO` are built in; kc inlines the others, also those with reified type parameters.

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

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| arrow-libs/core/arrow-annotations/src/commonMain/kotlin/arrow/optics/optics.kt | 19 |  |
| arrow-libs/core/arrow-annotations/src/commonMain/kotlin/arrow/synthetic.kt | 34 |  |
| arrow-libs/core/arrow-atomic/src/androidAndJvmMain/kotlin/arrow/atomic/Atomic.kt | 7 |  |
| arrow-libs/core/arrow-atomic/src/androidAndJvmMain/kotlin/arrow/atomic/AtomicInt.kt | 7 |  |
| arrow-libs/core/arrow-atomic/src/androidAndJvmMain/kotlin/arrow/atomic/AtomicLong.kt | 7 |  |
| arrow-libs/core/arrow-atomic/src/commonMain/kotlin/arrow/atomic/Atomic.kt | 103 |  |
| arrow-libs/core/arrow-atomic/src/commonMain/kotlin/arrow/atomic/AtomicBoolean.kt | 89 |  |
| arrow-libs/core/arrow-atomic/src/commonMain/kotlin/arrow/atomic/AtomicInt.kt | 84 |  |
| arrow-libs/core/arrow-atomic/src/commonMain/kotlin/arrow/atomic/AtomicLong.kt | 84 |  |
| arrow-libs/core/arrow-autoclose/src/commonMain/kotlin/arrow/AutoCloseScope.kt | 119 |  |
| arrow-libs/core/arrow-autoclose/src/commonMain/kotlin/arrow/ThrowIfFatal.kt | 11 |  |
| arrow-libs/core/arrow-cache4k/src/commonMain/kotlin/arrow/core/Cache4k.kt | 18 |  |
| arrow-libs/core/arrow-core-high-arity/src/commonMain/kotlin/arrow/core/Iterable.kt | 1065 |  |
| arrow-libs/core/arrow-core-high-arity/src/commonMain/kotlin/arrow/core/NonEmptyList.kt | 252 |  |
| arrow-libs/core/arrow-core-high-arity/src/commonMain/kotlin/arrow/core/Sequence.kt | 942 |  |
| arrow-libs/core/arrow-core-high-arity/src/commonMain/kotlin/arrow/core/map.kt | 952 |  |
| arrow-libs/core/arrow-core-high-arity/src/commonMain/kotlin/arrow/core/raise/RaiseAccumulate.kt | 639 |  |
| arrow-libs/core/arrow-core/src/androidAndJvmMain/kotlin/arrow/core/ArrowCoreInternalException.kt | 9 |  |
| arrow-libs/core/arrow-core/src/androidAndJvmMain/kotlin/arrow/core/MemoizedDeepRecursiveFunction.kt | 12 |  |
| arrow-libs/core/arrow-core/src/androidAndJvmMain/kotlin/arrow/core/SortedMapK.kt | 9 |  |
| arrow-libs/core/arrow-core/src/androidAndJvmMain/kotlin/arrow/core/raise/CancellationExceptionNoTrace.kt | 19 |  |
| arrow-libs/core/arrow-core/src/androidAndJvmMain/kotlin/arrow/core/raise/RaiseCancellationException.kt | 9 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/ArrowCoreInternalException.kt | 4 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Comparison.kt | 69 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Either.kt | 1623 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Ior.kt | 531 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Iterable.kt | 921 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/MemoizedDeepRecursiveFunction.kt | 43 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/NonEmptyCollection.kt | 52 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/NonEmptyList.kt | 501 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/NonEmptySet.kt | 140 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Option.kt | 695 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Pair.kt | 13 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Result.kt | 17 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Sequence.kt | 760 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Triple.kt | 16 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Tuple4.kt | 32 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Tuple5.kt | 37 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Tuple6.kt | 41 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Tuple7.kt | 45 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Tuple8.kt | 49 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/Tuple9.kt | 53 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/TupleN.kt | 24 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/map.kt | 518 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/predef.kt | 23 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/Builders.kt | 415 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/Effect.kt | 681 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/ErrorHandlers.kt | 163 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/Fold.kt | 301 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/Mappers.kt | 43 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/Raise.kt | 741 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/RaiseAccumulate.kt | 1196 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/Trace.kt | 45 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/context/Builders.kt | 81 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/context/RaiseAccumulateCombineContext.kt | 290 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/context/RaiseAccumulateContext.kt | 100 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/context/RaiseAccumulateNelContext.kt | 301 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/raise/context/RaiseContext.kt | 152 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/string.kt | 6 |  |
| arrow-libs/core/arrow-core/src/commonMain/kotlin/arrow/core/utils.kt | 10 |  |
| arrow-libs/core/arrow-eval/src/commonMain/kotlin/arrow/eval/Eval.kt | 613 |  |
| arrow-libs/core/arrow-eval/src/commonMain/kotlin/arrow/eval/SuspendEval.kt | 237 |  |
| arrow-libs/core/arrow-exception-utils/src/androidAndJvmMain/kotlin/arrow/core/NonFatal.kt | 11 |  |
| arrow-libs/core/arrow-exception-utils/src/commonMain/kotlin/arrow/core/NonFatal.kt | 46 |  |
| arrow-libs/core/arrow-exception-utils/src/commonMain/kotlin/arrow/core/ThrowableUtils.kt | 20 |  |
| arrow-libs/core/arrow-exception-utils/src/commonMain/kotlin/arrow/core/nonFatalOrThrow.kt | 41 |  |
| arrow-libs/core/arrow-functions/src/androidAndJvmMain/kotlin/arrow/core/composition-jvm.kt | 256 |  |
| arrow-libs/core/arrow-functions/src/commonMain/kotlin/arrow/core/composition.kt | 10 |  |
| arrow-libs/core/arrow-functions/src/commonMain/kotlin/arrow/core/currying.kt | 295 |  |
| arrow-libs/core/arrow-functions/src/commonMain/kotlin/arrow/core/memoization.kt | 135 |  |
| arrow-libs/core/arrow-functions/src/commonMain/kotlin/arrow/core/partials.kt | 1775 |  |
| arrow-libs/core/arrow-platform/src/commonMain/kotlin/arrow/platform/Platform.kt | 16 |  |
| arrow-libs/core/arrow-platform/src/commonMain/kotlin/arrow/platform/test/Flaky.kt | 7 |  |
| arrow-libs/core/arrow-platform/src/jvmMain/kotlin/arrow/platform/PlatformJvm.kt | 4 |  |
| arrow-libs/core/arrow-platform/src/jvmMain/kotlin/arrow/platform/test/Flaky.kt | 5 |  |
| arrow-libs/fx/arrow-collectors/src/androidAndJvmMain/kotlin/arrow/collectors/Collectors.kt | 42 |  |
| arrow-libs/fx/arrow-collectors/src/androidAndJvmMain/kotlin/arrow/collectors/JvmCollector.kt | 34 |  |
| arrow-libs/fx/arrow-collectors/src/commonMain/kotlin/arrow/collectors/Collect.kt | 145 |  |
| arrow-libs/fx/arrow-collectors/src/commonMain/kotlin/arrow/collectors/Collector.kt | 262 |  |
| arrow-libs/fx/arrow-collectors/src/commonMain/kotlin/arrow/collectors/Collectors.kt | 137 |  |
| arrow-libs/fx/arrow-collectors/src/commonMain/kotlin/arrow/collectors/Zip.kt | 137 |  |
| arrow-libs/fx/arrow-fx-coroutines/src/androidAndJvmMain/kotlin/arrow/fx/coroutines/ResourceExtensions.kt | 164 |  |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/Bracket.kt | 279 |  |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/CountDownLatch.kt | 47 |  |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/CyclicBarrier.kt | 153 |  |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/ParMap.kt | 121 |  |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/ParZip.kt | 1007 |  |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/ParZipOrAccumulate.kt | 856 |  |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/Race2.kt | 123 |  |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/Race3.kt | 126 |  |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/Racing.kt | 162 |  |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/RaiseRacing.kt | 62 |  |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/Resource.kt | 567 |  |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/await/AwaitAllScope.kt | 97 |  |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/flow.kt | 283 |  |
| arrow-libs/fx/arrow-fx-coroutines/src/commonMain/kotlin/arrow/fx/coroutines/resource/context/Resource.kt | 47 |  |
| arrow-libs/fx/arrow-fx-stm/src/androidAndJvmMain/kotlin/arrow/fx/stm/internal/RetryException.kt | 6 |  |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/STM.kt | 1705 | member extension property |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/TArray.kt | 129 |  |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/TMVar.kt | 198 |  |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/TMap.kt | 166 |  |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/TQueue.kt | 350 |  |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/TSemaphore.kt | 107 |  |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/TSet.kt | 96 |  |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/TVar.kt | 234 |  |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/internal/Hamt.kt | 155 |  |
| arrow-libs/fx/arrow-fx-stm/src/commonMain/kotlin/arrow/fx/stm/internal/Impl.kt | 219 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 191 files, 11404 lines

Directories: `arrow-libs/core/arrow-core/src/commonTest`, `arrow-libs/core/arrow-core/src/jvmTest`

### Features left, by files blocked

191 of 191 files use no feature that kc doesn't support.

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

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/BooleanTest.kt | 14 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/CollectionsSyntaxTests.kt | 37 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/ComparisonKtTest.kt | 210 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/EitherTest.kt | 1043 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/IorTest.kt | 247 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/IterableTest.kt | 632 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/KotestInlineUtils.kt | 56 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/ListKTest.kt | 29 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/MapKTest.kt | 842 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/NonEmptyListTest.kt | 603 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/NonEmptySetTest.kt | 244 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/OptionTest.kt | 370 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/ResultTest.kt | 44 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/SequenceKTest.kt | 569 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/TupleTest.kt | 165 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/extensions/NumberInstancesTest.kt | 56 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/EagerEffectSpec.kt | 296 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/EffectSpec.kt | 813 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/IorAccumulateSpec.kt | 257 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/IorSpec.kt | 319 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/MappersSpec.kt | 139 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/NullableSpec.kt | 166 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/OptionSpec.kt | 58 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/RaiseAccumulateSpec.kt | 146 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/ResultSpec.kt | 139 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/SingletonSpec.kt | 307 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/StructuredConcurrencySpec.kt | 241 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/TraceSpec.kt | 57 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/context/RaiseAccumulateContextTest.kt | 46 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/raise/predef.kt | 11 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/test/Generators.kt | 236 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/test/GeneratorsTest.kt | 139 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/test/Laws.kt | 27 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/test/laws/MonoidLaws.kt | 48 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/test/laws/SemigroupLaws.kt | 24 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/test/laws/SemiringLaws.kt | 106 |  |
| arrow-libs/core/arrow-core/src/commonTest/kotlin/arrow/core/usage/Context.kt | 38 |  |
| arrow-libs/core/arrow-core/src/jvmTest/java/arrow/core/DeadlockTest.kt | 55 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/arrow/core/raise/EffectUsage.kt | 51 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/arrow/core/raise/TraceJvmSpec.kt | 25 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-effect-error-01.kt | 14 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-effect-error-02.kt | 14 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-effect-error-03.kt | 16 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-effect-error-04.kt | 14 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-effect-error-05.kt | 13 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-01.kt | 14 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-02.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-03.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-04.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-05.kt | 15 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-06.kt | 21 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-07.kt | 15 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-08.kt | 27 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-09.kt | 31 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-10.kt | 24 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-11.kt | 35 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-12.kt | 16 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-13.kt | 13 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-14.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-15.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-16.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-17.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-18.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-19.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-20.kt | 16 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-21.kt | 15 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-22.kt | 15 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-23.kt | 14 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-24.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-25.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-26.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-27.kt | 9 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-28.kt | 9 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-29.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-30.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-31.kt | 12 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-32.kt | 14 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-33.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-34.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-35.kt | 12 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-36.kt | 12 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-either-37.kt | 22 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-01.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-02.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-03.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-04.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-05.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-06.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-07.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-08.kt | 13 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-09.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-10.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-ior-11.kt | 12 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-01.kt | 19 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-02.kt | 19 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-03.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-04.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-05.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-06.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-07.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-08.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-09.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-10.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-11.kt | 13 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-12.kt | 12 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-13.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-14.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-15.kt | 12 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-16.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-iterable-17.kt | 12 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-01.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-02.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-03.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-04.kt | 12 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-05.kt | 13 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-06.kt | 16 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-07.kt | 12 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-map-08.kt | 16 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-nonemptylist-01.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-nonemptylist-02.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-nonemptylist-03.kt | 17 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-nonemptylist-04.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-nonemptylist-05.kt | 17 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-nonemptylist-06.kt | 20 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-01.kt | 13 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-02.kt | 9 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-03.kt | 17 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-04.kt | 17 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-05.kt | 17 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-06.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-07.kt | 15 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-08.kt | 15 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-09.kt | 17 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-10.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-11.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-12.kt | 12 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-13.kt | 14 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-14.kt | 16 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-15.kt | 12 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-16.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-17.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-18.kt | 14 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-19.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-20.kt | 14 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-option-21.kt | 17 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-01.kt | 18 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-02.kt | 52 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-03.kt | 16 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-04.kt | 55 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-05.kt | 25 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-06.kt | 32 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-07.kt | 31 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-08.kt | 29 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-09.kt | 34 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-10.kt | 59 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-11.kt | 21 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-12.kt | 22 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-13.kt | 25 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-01.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-02.kt | 25 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-03.kt | 29 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-04.kt | 23 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-05.kt | 19 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-06.kt | 15 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-07.kt | 20 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-08.kt | 20 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-09.kt | 21 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-10.kt | 21 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-11.kt | 15 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-raise-dsl-12.kt | 10 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-01.kt | 22 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-02.kt | 16 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-03.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-04.kt | 14 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-05.kt | 14 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-06.kt | 14 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-07.kt | 14 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-08.kt | 14 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-09.kt | 14 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-10.kt | 13 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-11.kt | 13 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-12.kt | 12 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-13.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-14.kt | 11 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-15.kt | 16 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/example-sequence-16.kt | 15 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/test/EitherKnitTest.kt | 72 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/test/IterableKnitTest.kt | 72 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/test/OptionKnitTest.kt | 16 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/test/RaiseKnitTest.kt | 52 |  |
| arrow-libs/core/arrow-core/src/jvmTest/kotlin/examples/test/SequenceKnitTest.kt | 28 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

