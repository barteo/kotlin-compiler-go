# Survey: kotlinx-datetime v0.8.0

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 69 files, 13870 lines

Directories: `core/common/src`, `core/jvm/src`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 69 | 13 |
| 1: packages, annotations | 1 | 68 | 11 |
| 2: interfaces, inheritance | 1 | 68 | 8 |
| 3: type checks | 2 | 67 | 7 |
| 4: exceptions | 2 | 67 | 6 |
| 5: objects | 5 | 64 | 6 |
| 6: data, enum, sealed | 6 | 63 | 4 |
| 7: arrays, vararg, spread | 6 | 63 | 4 |
| 8: Char, Float, Byte, Short | 6 | 63 | 4 |
| 9: inliner | 6 | 63 | 3 |
| 10: lambdas with receivers | 7 | 62 | 3 |
| 11: extensions | 15 | 54 | 3 |
| 12: operator conventions | 15 | 54 | 3 |
| 13: identity, class literals, references | 17 | 52 | 2 |
| 14: class delegation | 21 | 48 | 2 |
| 15: secondary constructors | 23 | 46 | 1 |
| 16: local functions | 23 | 46 | 1 |
| 17: expect and actual | 33 | 36 | 1 |
| 18: annotation classes | 33 | 36 | 1 |
| 19: value classes | 33 | 36 | 1 |
| 20: unsigned integers | 33 | 36 | 1 |
| definite assignment | 33 | 36 | 1 |
| 21: inline functions | 35 | 34 | 1 |
| 22: reified type parameters | 35 | 34 | 1 |
| 23: labels, do-while | 38 | 31 | 1 |
| 24: delegated properties | 45 | 24 | 1 |
| 25: object expressions | 47 | 22 | 1 |
| 26: destructuring lambda parameters | 48 | 21 | 1 |
| 42-49: coroutines | 48 | 21 | 1 |
| 52: bitwise operators | 52 | 17 | 1 |
| 53: typealias | 56 | 13 | 1 |
| 54: setters, backing fields | 58 | 11 | 1 |
| 55: lateinit, tailrec | 58 | 11 | 1 |
| 56-57: collection supertypes | 60 | 9 | 1 |
| 58: member extension functions | 60 | 9 | 1 |
| 59: reified member type parameters | 60 | 9 | 1 |
| 60: inner and local classes | 61 | 8 | 2 |
| 61: when guards, anonymous functions, local extension functions | 62 | 7 | 1 |
| 62a: @JvmField and @JvmStatic in objects | 62 | 7 | 1 |
| 62b: generic value classes | 63 | 6 | 1 |
| 63: definitely non-nullable types | 63 | 6 | 1 |
| 70a: inline property accessors | 63 | 6 | 1 |
| 70b: where clauses | 63 | 6 | 1 |
| 70c: fun interfaces | 63 | 6 | 1 |
| delegated extension properties | 63 | 6 | 1 |
| qualified super | 63 | 6 | 1 |
| also supported | 69 | 0 | – |

### Features left, by files blocked

69 of 69 files use no feature that kc doesn't support.

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
| `let` | 23 | 59 |
| `require` | 17 | 50 |
| `format` | 14 | 26 |
| `also` | 8 | 16 |
| `apply` | 7 | 7 |
| `isNotEmpty` | 6 | 12 |
| `buildString` | 5 | 8 |
| `mod` | 4 | 7 |
| `with` | 4 | 7 |
| `buildList` | 3 | 6 |
| `deleteAt` | 3 | 5 |
| `error` | 3 | 3 |
| `floorDiv` | 3 | 5 |
| `run` | 3 | 7 |
| `check` | 2 | 8 |
| `onSuccess` | 2 | 2 |
| `Sequence` | 1 | 3 |
| `appendLine` | 1 | 12 |
| `isInfinite` | 1 | 1 |
| `onFailure` | 1 | 1 |
| `sumOf` | 1 | 1 |

### Files

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| core/common/src/Clock.kt | 102 | (annotation, extension function, inheritance, inline-only call, is, modifier override, object expression, package, qualified this, throw) |
| core/common/src/DateTimePeriod.kt | 693 | (Char, annotation, as, bitwise operators, callable reference, class literal, default value, extension function, identity equality, inheritance, inline-only call, is, modifier abstract, modifier companion, modifier inline, modifier open, modifier operator, modifier override, modifier sealed, named argument, package, property without initializer, require, check, error, secondary constructor, throw, try, when subject variable) |
| core/common/src/DateTimeUnit.kt | 302 | (annotation, bitwise operators, class literal, function without body, identity equality, inheritance, is, modifier abstract, modifier companion, modifier operator, modifier override, modifier protected, modifier sealed, named argument, nested class, package, property getter, property without initializer, require, check, error) |
| core/common/src/DayOfWeek.kt | 47 | (annotation, extension property, modifier enum, package, require, check, error) |
| core/common/src/DeprecatedClock.kt | 23 | (annotation, package, typealias) |
| core/common/src/DeprecatedInstant.kt | 23 | (annotation, package, typealias) |
| core/common/src/Exceptions.kt | 45 | (inheritance, modifier companion, modifier const, package, secondary constructor, super) |
| core/common/src/Instant.kt | 548 | (annotation, default value, destructuring lambda parameter, extension function, inline-only call, is, modifier const, named argument, package, qualified this, require, check, error, throw, try) |
| core/common/src/LocalDate.kt | 674 | (annotation, class literal, default value, extension function, function type with receiver, function without body, inheritance, inline-only call, modifier companion, modifier const, modifier expect, modifier operator, modifier override, named argument, object, package, property without initializer, secondary constructor) |
| core/common/src/LocalDateRange.kt | 277 | (annotation, as, callable reference, collection supertype, default value, extension function, inheritance, inline-only call, is, modifier companion, modifier infix, modifier open, modifier override, package, property getter, require, check, error, secondary constructor, throw) |
| core/common/src/LocalDateTime.kt | 548 | (annotation, class literal, default value, extension function, function type with receiver, function without body, inheritance, inline-only call, modifier companion, modifier expect, modifier operator, modifier override, named argument, object, package, property without initializer, secondary constructor) |
| core/common/src/LocalIsoWeekDate.kt | 461 | (Char, bitwise operators, extension function, identity equality, inheritance, inline-only call, is, local extension function, local function, modifier companion, modifier inline, modifier override, named argument, package, prefix increment, secondary constructor, throw, try, uninitialized local variable, when subject variable) |
| core/common/src/LocalTime.kt | 466 | (annotation, class literal, default value, extension function, function type with receiver, function without body, inheritance, inline-only call, modifier companion, modifier expect, modifier operator, modifier override, object, package, property without initializer, secondary constructor) |
| core/common/src/Month.kt | 79 | (annotation, extension property, modifier enum, package, require, check, error) |
| core/common/src/OverloadMarker.kt | 31 | (annotation, modifier companion, package) |
| core/common/src/TimeZone.kt | 357 | (annotation, default value, extension function, function without body, inheritance, modifier companion, modifier expect, modifier open, modifier override, package, property without initializer, secondary constructor, typealias) |
| core/common/src/UtcOffset.kt | 283 | (annotation, class literal, default value, extension function, function type with receiver, function without body, inline-only call, modifier companion, modifier expect, modifier override, object, package, property without initializer) |
| core/common/src/YearMonth.kt | 454 | (annotation, class literal, default value, extension function, extension property, function type with receiver, function without body, inheritance, inline-only call, modifier companion, modifier expect, modifier operator, modifier override, object, package, property without initializer, require, check, error, secondary constructor, throw, try) |
| core/common/src/YearMonthRange.kt | 277 | (annotation, as, callable reference, collection supertype, default value, extension function, inheritance, inline-only call, is, modifier companion, modifier infix, modifier open, modifier override, package, property getter, require, check, error, secondary constructor, throw) |
| core/common/src/format/DateTimeComponents.kt | 688 | (Char, annotation, as, bitwise operators, callable reference, class delegation, default value, delegated property, extension function, function type with receiver, identity equality, inheritance, inline-only call, is, modifier companion, modifier operator, modifier override, nested class, object, package, property getter, property setter, require, check, error, throw, try) |
| core/common/src/format/DateTimeFormat.kt | 171 | (as, callable reference, delegated property, extension function, function without body, inheritance, inline-only call, interface, is, local function, modifier abstract, modifier companion, modifier enum, modifier inline, modifier open, modifier override, modifier sealed, package, property without initializer, throw, try, when subject variable) |
| core/common/src/format/DateTimeFormatBuilder.kt | 565 | (Char, annotation, arrays, as, callable reference, class literal, default value, extension function, function type with receiver, function without body, inheritance, inline-only call, interface, is, labels, modifier annotation, modifier const, modifier expect, modifier override, modifier sealed, modifier vararg, named argument, nested class, package, prefix increment, property without initializer, spread operator, throw, where clause) |
| core/common/src/format/LocalDateFormat.kt | 295 | (Char, as, callable reference, class delegation, default value, delegated property, extension function, function type with receiver, function without body, inheritance, inline-only call, interface, is, modifier companion, modifier override, named argument, nested class, object, package, property getter, property without initializer, require, check, error, secondary constructor, throw, when subject variable) |
| core/common/src/format/LocalDateTimeFormat.kt | 83 | (Char, as, class delegation, default value, delegated property, function type with receiver, function without body, inheritance, inline-only call, interface, modifier companion, modifier override, nested class, package) |
| core/common/src/format/LocalTimeFormat.kt | 298 | (Char, as, callable reference, default value, delegated property, function type with receiver, function without body, inheritance, inline-only call, interface, is, modifier companion, modifier enum, modifier override, named argument, nested class, object, package, property getter, property setter, property without initializer, require, check, error, throw) |
| core/common/src/format/Unicode.kt | 655 | (Char, annotation, default value, extension function, function without body, inheritance, inline-only call, interface, is, local function, modifier abstract, modifier annotation, modifier companion, modifier data, modifier override, modifier sealed, named argument, nested class, package, prefix increment, property without initializer, require, check, error, throw) |
| core/common/src/format/UtcOffsetFormat.kt | 277 | (Char, as, callable reference, default value, delegated property, extension function, function type with receiver, function without body, inheritance, inline-only call, interface, is, local extension function, local function, modifier companion, modifier enum, modifier override, named argument, nested class, object, object expression, package, property getter, property without initializer, require, check, error) |
| core/common/src/format/YearMonthFormat.kt | 292 | (Char, as, callable reference, default value, delegated property, extension function, function type with receiver, function without body, inheritance, inline-only call, interface, is, modifier companion, modifier const, modifier override, named argument, nested class, object, package, property getter, property without initializer, require, check, error, secondary constructor, throw) |
| core/common/src/internal/BinaryDataReader.kt | 60 | (Byte and Short, Char, arrays, bitwise operators, default value, inline-only call, package, prefix increment, unsigned integers) |
| core/common/src/internal/dateCalculations.kt | 46 | (bitwise operators, extension function, modifier const, package) |
| core/common/src/internal/format/Builder.kt | 21 | (is, package) |
| core/common/src/internal/format/FieldFormatDirective.kt | 219 | (callable reference, function without body, inheritance, inline-only call, interface, modifier abstract, modifier final, modifier inner, modifier override, named argument, nested class, package, property without initializer, qualified this, require, check, error) |
| core/common/src/internal/format/FieldSpec.kt | 149 | (default value, function without body, identity equality, inheritance, interface, modifier abstract, modifier override, package, property without initializer, throw) |
| core/common/src/internal/format/FormatStructure.kt | 270 | (callable reference, function without body, inheritance, inline-only call, interface, is, local function, modifier companion, modifier inline, modifier open, modifier override, modifier sealed, named argument, nested class, package, require, check, error, super) |
| core/common/src/internal/format/Predicate.kt | 35 | (function without body, inheritance, interface, modifier override, object, package) |
| core/common/src/internal/format/formatter/Formatter.kt | 66 | (Char, default value, destructuring declaration, function type with receiver, function without body, inheritance, inline-only call, interface, modifier override, named argument, package) |
| core/common/src/internal/format/formatter/FormatterOperation.kt | 130 | (Char, inheritance, inline-only call, modifier override, package, prefix increment, require, check, error) |
| core/common/src/internal/format/parser/NumberConsumer.kt | 179 | (default value, extension function, function without body, inheritance, interface, modifier abstract, modifier open, modifier override, modifier sealed, named argument, nested class, object, package, require, check, error, when subject variable) |
| core/common/src/internal/format/parser/ParseResult.kt | 26 | (annotation, inline-only call, is, modifier companion, modifier inline, modifier value, package, require, check, error) |
| core/common/src/internal/format/parser/Parser.kt | 276 | (annotation, default value, extension function, function without body, generic value class, inheritance, inline-only call, interface, is, labels, local extension function, local function, modifier companion, modifier const, modifier inline, modifier override, modifier value, named argument, nested class, package, throw) |
| core/common/src/internal/format/parser/ParserOperation.kt | 556 | (Char, default value, destructuring declaration, extension function, function without body, identity equality, inheritance, inline-only call, interface, labels, local extension function, local function, modifier companion, modifier enum, modifier inline, modifier override, named argument, nested class, package, prefix increment, property getter, property without initializer, require, check, error, when subject variable) |
| core/common/src/internal/math.kt | 287 | (Char, annotation, arrays, bitwise operators, default value, destructuring declaration, extension function, extension property, function without body, inheritance, inline-only call, is, modifier expect, modifier inline, modifier operator, modifier override, modifier value, package, property getter, require, check, error, throw, try) |
| core/common/src/internal/toKotlinCode.kt | 27 | (Char, extension function, inline-only call, package, qualified this) |
| core/common/src/internal/util.kt | 48 | (Char, extension function, named argument, package) |
| core/common/src/serializers/DateTimePeriodSerializers.kt | 193 | (annotation, class delegation, inheritance, inline-only call, is, labels, modifier override, named argument, object, package, throw, when subject variable) |
| core/common/src/serializers/DateTimeUnitSerializers.kt | 228 | (annotation, arrays, class literal, delegated property, function without body, inheritance, labels, modifier override, named argument, object, package, property getter, throw, when subject variable) |
| core/common/src/serializers/DayOfWeekSerializers.kt | 23 | (annotation, class delegation, inheritance, object, package) |
| core/common/src/serializers/InstantSerializers.kt | 63 | (annotation, class literal, inheritance, inline-only call, modifier abstract, modifier override, package) |
| core/common/src/serializers/LocalDateSerializers.kt | 127 | (Byte and Short, annotation, class delegation, class literal, extension function, inheritance, inline-only call, labels, modifier abstract, modifier override, named argument, object, object expression, package, throw, when subject variable) |
| core/common/src/serializers/LocalDateTimeSerializers.kt | 140 | (Byte and Short, annotation, class delegation, class literal, inheritance, labels, modifier abstract, modifier override, named argument, object, package, throw, when subject variable) |
| core/common/src/serializers/LocalTimeSerializers.kt | 122 | (Byte and Short, annotation, class delegation, class literal, inheritance, labels, modifier abstract, modifier override, named argument, object, package, throw, when subject variable) |
| core/common/src/serializers/MonthSerializers.kt | 28 | (annotation, arrays, class delegation, inheritance, object, package) |
| core/common/src/serializers/TimeZoneSerializers.kt | 60 | (annotation, inheritance, is, modifier override, object, package, throw) |
| core/common/src/serializers/UtcOffsetSerializers.kt | 69 | (annotation, class delegation, inheritance, modifier abstract, modifier override, object, package) |
| core/common/src/serializers/YearMonthSerializers.kt | 109 | (Byte and Short, annotation, class delegation, class literal, inheritance, modifier abstract, modifier override, named argument, object, package, throw, when subject variable) |
| core/jvm/src/Converters.kt | 119 | (annotation, extension function, package) |
| core/jvm/src/DayOfWeekJvm.kt | 20 | (annotation, extension property, package) |
| core/jvm/src/Instant.kt | 46 | (annotation, package) |
| core/jvm/src/LocalDate.kt | 239 | (annotation, callable reference, class literal, extension function, function type with receiver, identity equality, inheritance, inline-only call, is, modifier actual, modifier companion, modifier const, modifier operator, modifier override, named argument, object, package, secondary constructor, throw, try) |
| core/jvm/src/LocalDateTimeJvm.kt | 187 | (annotation, callable reference, class literal, default value, extension function, function type with receiver, identity equality, inheritance, inline-only call, is, modifier actual, modifier companion, modifier const, modifier override, named argument, object, package, secondary constructor, throw, try) |
| core/jvm/src/LocalTimeJvm.kt | 142 | (annotation, callable reference, class literal, default value, extension function, function type with receiver, identity equality, inheritance, inline-only call, is, modifier actual, modifier companion, modifier const, modifier override, named argument, object, package, throw, try) |
| core/jvm/src/MonthJvm.kt | 25 | (annotation, extension property, package) |
| core/jvm/src/TimeZoneJvm.kt | 134 | (annotation, as, callable reference, extension function, extension property, identity equality, inheritance, inline-only call, is, modifier actual, modifier companion, modifier open, modifier override, package, property getter, qualified this, throw, try) |
| core/jvm/src/UtcOffsetJvm.kt | 113 | (annotation, callable reference, class literal, default value, delegated property, extension function, function type with receiver, identity equality, inheritance, inline-only call, is, modifier actual, modifier companion, modifier const, modifier override, named argument, object, package, throw, try) |
| core/jvm/src/YearMonthJvm.kt | 126 | (Char, annotation, callable reference, class literal, delegated property, extension function, function type with receiver, identity equality, inheritance, inline-only call, is, modifier actual, modifier companion, modifier const, modifier operator, modifier override, object, package, throw, try) |
| core/jvm/src/format/Deprecations.kt | 55 | (annotation, modifier companion, named argument, package) |
| core/jvm/src/format/format/MyJvmDefaultWithoutCompatibility.kt | 8 | (modifier actual, package, typealias) |
| core/jvm/src/internal/Ser.kt | 74 | (annotation, as, inheritance, modifier companion, modifier const, modifier override, named argument, package, secondary constructor, throw) |
| core/jvm/src/internal/mathJvm.kt | 11 | (modifier actual, package) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 59 files, 10674 lines

Directories: `core/common/test`, `core/jvm/test`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 59 | 6 |
| 1: packages, annotations | 3 | 56 | 5 |
| 2: interfaces, inheritance | 3 | 56 | 4 |
| 3: type checks | 3 | 56 | 4 |
| 4: exceptions | 7 | 52 | 4 |
| 5: objects | 7 | 52 | 3 |
| 6: data, enum, sealed | 9 | 50 | 2 |
| 7: arrays, vararg, spread | 9 | 50 | 2 |
| 8: Char, Float, Byte, Short | 9 | 50 | 2 |
| 9: inliner | 27 | 32 | 1 |
| 10: lambdas with receivers | 27 | 32 | 1 |
| 11: extensions | 27 | 32 | 1 |
| 12: operator conventions | 27 | 32 | 1 |
| 13: identity, class literals, references | 38 | 21 | 1 |
| 14: class delegation | 38 | 21 | 1 |
| 15: secondary constructors | 38 | 21 | 1 |
| 16: local functions | 46 | 13 | 1 |
| 17: expect and actual | 46 | 13 | 1 |
| 18: annotation classes | 46 | 13 | 1 |
| 19: value classes | 46 | 13 | 1 |
| 20: unsigned integers | 46 | 13 | 1 |
| definite assignment | 47 | 12 | 1 |
| 21: inline functions | 48 | 11 | 1 |
| 22: reified type parameters | 48 | 11 | 1 |
| 23: labels, do-while | 48 | 11 | 1 |
| 24: delegated properties | 48 | 11 | 1 |
| 25: object expressions | 51 | 8 | 1 |
| 26: destructuring lambda parameters | 53 | 6 | 1 |
| 42-49: coroutines | 53 | 6 | 1 |
| 52: bitwise operators | 55 | 4 | 1 |
| 53: typealias | 55 | 4 | 1 |
| 54: setters, backing fields | 55 | 4 | 1 |
| 55: lateinit, tailrec | 55 | 4 | 1 |
| 56-57: collection supertypes | 55 | 4 | 1 |
| 58: member extension functions | 55 | 4 | 1 |
| 59: reified member type parameters | 56 | 3 | 1 |
| 60: inner and local classes | 57 | 2 | 1 |
| 61: when guards, anonymous functions, local extension functions | 59 | 0 | – |
| 62a: @JvmField and @JvmStatic in objects | 59 | 0 | – |
| 62b: generic value classes | 59 | 0 | – |
| 63: definitely non-nullable types | 59 | 0 | – |
| 70a: inline property accessors | 59 | 0 | – |
| 70b: where clauses | 59 | 0 | – |
| 70c: fun interfaces | 59 | 0 | – |
| delegated extension properties | 59 | 0 | – |
| qualified super | 59 | 0 | – |
| also supported | 59 | 0 | – |

### Features left, by files blocked

59 of 59 files use no feature that kc doesn't support.

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
| `check` | 28 | 895 |
| `format` | 26 | 134 |
| `apply` | 11 | 25 |
| `let` | 9 | 35 |
| `with` | 8 | 36 |
| `buildMap` | 7 | 28 |
| `run` | 4 | 6 |
| `isNotEmpty` | 3 | 3 |
| `runCatching` | 3 | 4 |
| `buildList` | 2 | 2 |
| `error` | 2 | 2 |
| `onSuccess` | 2 | 2 |
| `use` | 2 | 2 |
| `HexFormat` | 1 | 1 |
| `buildString` | 1 | 1 |

### Files

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| core/common/test/ClockTimeSourceTest.kt | 129 | (annotation, class literal, destructuring declaration, inheritance, modifier override, named argument, object expression, package) |
| core/common/test/DateTimePeriodTest.kt | 198 | (annotation, as, default value, destructuring declaration, inline-only call, is, local function, modifier inline, modifier reified, named argument, package, reified member type parameter) |
| core/common/test/DateTimeUnitTest.kt | 62 | (annotation, callable reference, package) |
| core/common/test/DayOfWeekTest.kt | 37 | (annotation, package) |
| core/common/test/InstantTest.kt | 659 | (annotation, bitwise operators, callable reference, extension function, extension property, inline-only call, labels, local function, named argument, package, qualified this, require, check, error) |
| core/common/test/LocalDateRangeTest.kt | 316 | (annotation, as, inline-only call, package) |
| core/common/test/LocalDateTest.kt | 411 | (annotation, callable reference, default value, destructuring declaration, destructuring lambda parameter, extension property, inline-only call, local function, named argument, package, require, check, error) |
| core/common/test/LocalDateTimeTest.kt | 203 | (annotation, default value, destructuring declaration, inline-only call, local function, package) |
| core/common/test/LocalIsoWeekDateTest.kt | 129 | (annotation, destructuring declaration, inline-only call, local function, package) |
| core/common/test/LocalTimeTest.kt | 285 | (annotation, arrays, callable reference, default value, destructuring declaration, destructuring lambda parameter, extension function, inline-only call, local function, package) |
| core/common/test/MonthTest.kt | 31 | (annotation, package) |
| core/common/test/MultiplyAndDivideTest.kt | 83 | (annotation, bitwise operators, destructuring declaration, inline-only call, package) |
| core/common/test/ReadmeTest.kt | 211 | (Char, annotation, class literal, inline-only call, named argument, package) |
| core/common/test/TimeZoneTest.kt | 321 | (annotation, arrays, as, destructuring declaration, extension function, inline-only call, local function, named argument, package, require, check, error, throw, try) |
| core/common/test/UtcOffsetTest.kt | 179 | (Char, annotation, arrays, default value, destructuring declaration, extension function, local extension function, local function, modifier companion, named argument, package, require, check, error, spread operator) |
| core/common/test/YearMonthRangeTest.kt | 313 | (annotation, as, inline-only call, package) |
| core/common/test/YearMonthTest.kt | 221 | (Char, annotation, as, class literal, destructuring declaration, local function, package) |
| core/common/test/assertions.kt | 48 | (annotation, default value, modifier const, modifier inline, package) |
| core/common/test/format/DateTimeComponentsFormatTest.kt | 492 | (Char, annotation, callable reference, class literal, default value, destructuring declaration, destructuring lambda parameter, inline-only call, local class, named argument, object, package, uninitialized local variable) |
| core/common/test/format/DateTimeComponentsTest.kt | 68 | (annotation, callable reference, inline-only call, package) |
| core/common/test/format/DateTimeFormatTest.kt | 194 | (Char, annotation, class literal, extension function, package, throw, try) |
| core/common/test/format/LocalDateFormatTest.kt | 271 | (Char, annotation, class literal, destructuring declaration, inline-only call, named argument, package) |
| core/common/test/format/LocalDateTimeFormatTest.kt | 251 | (Char, annotation, class literal, destructuring declaration, inline-only call, package) |
| core/common/test/format/LocalTimeFormatTest.kt | 247 | (Char, annotation, class literal, destructuring declaration, extension function, inline-only call, local extension function, local function, named argument, package, require, check, error) |
| core/common/test/format/ParserStructureConcatenationTest.kt | 191 | (annotation, as, inline-only call, is, named argument, package) |
| core/common/test/format/UtcOffsetFormatTest.kt | 192 | (Char, annotation, destructuring declaration, inline-only call, named argument, package) |
| core/common/test/format/YearMonthFormatTest.kt | 86 | (annotation, class literal, destructuring declaration, inline-only call, package) |
| core/common/test/samples/ClockSamples.kt | 67 | (annotation, inheritance, local function, modifier override, named argument, object expression, package, require, check, error) |
| core/common/test/samples/DateTimePeriodSamples.kt | 178 | (annotation, as, inline-only call, is, named argument, package, require, check, error) |
| core/common/test/samples/DateTimeUnitSamples.kt | 57 | (annotation, named argument, package, require, check, error) |
| core/common/test/samples/DayOfWeekSamples.kt | 52 | (annotation, named argument, package, require, check, error, try) |
| core/common/test/samples/InstantSamples.kt | 340 | (Char, annotation, inline-only call, local function, named argument, package, require, check, error) |
| core/common/test/samples/LocalDateRangeSamples.kt | 98 | (annotation, package, require, check, error) |
| core/common/test/samples/LocalDateSamples.kt | 365 | (Char, annotation, inline-only call, named argument, nested class, package, require, check, error) |
| core/common/test/samples/LocalDateTimeSamples.kt | 269 | (Char, annotation, inline-only call, named argument, nested class, package, require, check, error) |
| core/common/test/samples/LocalIsoWeekDateSamples.kt | 109 | (annotation, package, require, check, error) |
| core/common/test/samples/LocalTimeSamples.kt | 324 | (Char, annotation, inline-only call, named argument, nested class, package, require, check, error) |
| core/common/test/samples/MonthSamples.kt | 57 | (annotation, package, require, check, error, try) |
| core/common/test/samples/TimeZoneSamples.kt | 253 | (Char, annotation, as, default value, destructuring declaration, inline-only call, is, local function, named argument, nested class, package, require, check, error) |
| core/common/test/samples/UtcOffsetSamples.kt | 169 | (Char, annotation, inline-only call, named argument, nested class, package, require, check, error, try) |
| core/common/test/samples/YearMonthRangeSamples.kt | 98 | (annotation, package, require, check, error) |
| core/common/test/samples/YearMonthSamples.kt | 250 | (annotation, inline-only call, nested class, package, require, check, error) |
| core/common/test/samples/format/DateTimeComponentsFormatSamples.kt | 48 | (Char, annotation, inline-only call, named argument, package, require, check, error) |
| core/common/test/samples/format/DateTimeComponentsSamples.kt | 506 | (Char, annotation, inline-only call, named argument, nested class, package, require, check, error, try, uninitialized local variable) |
| core/common/test/samples/format/DateTimeFormatBuilderSamples.kt | 103 | (Char, annotation, inline-only call, named argument, package, require, check, error, try) |
| core/common/test/samples/format/DateTimeFormatSamples.kt | 150 | (Char, annotation, class literal, inline-only call, nested class, package, require, check, error, try) |
| core/common/test/samples/format/LocalDateFormatSamples.kt | 121 | (Char, annotation, inline-only call, named argument, nested class, package, require, check, error) |
| core/common/test/samples/format/LocalDateTimeFormatSamples.kt | 28 | (annotation, inline-only call, named argument, package, require, check, error) |
| core/common/test/samples/format/LocalTimeFormatSamples.kt | 58 | (Char, annotation, inline-only call, named argument, package, require, check, error) |
| core/common/test/samples/format/UnicodeSamples.kt | 31 | (annotation, class literal, inline-only call, package, require, check, error) |
| core/common/test/samples/format/UtcOffsetFormatSamples.kt | 45 | (Char, annotation, inline-only call, named argument, package, require, check, error) |
| core/common/test/samples/format/YearMonthFormatSamples.kt | 152 | (Char, annotation, inline-only call, named argument, nested class, package, require, check, error) |
| core/jvm/test/ConvertersTest.kt | 214 | (annotation, as, callable reference, class literal, inline-only call, local function, package) |
| core/jvm/test/DeprecationReplacements.kt | 33 | (annotation, package) |
| core/jvm/test/InstantParsing.kt | 35 | (annotation, inline-only call, local function, package) |
| core/jvm/test/JvmSerializationTest.kt | 97 | (annotation, arrays, class literal, inline-only call, named argument, package, try) |
| core/jvm/test/LocalIsoWeekDateTest.kt | 22 | (annotation, inline-only call, package) |
| core/jvm/test/MaliciousJvmSerializationTest.kt | 206 | (annotation, class literal, extension function, inline-only call, named argument, nested class, package) |
| core/jvm/test/UnicodeFormatTest.kt | 311 | (annotation, class literal, destructuring declaration, extension function, identity equality, inheritance, inline-only call, is, modifier override, named argument, object expression, package, throw, try) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

