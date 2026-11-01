# Survey: kotlinx-datetime v0.8.0

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 69 files, 13870 lines

Directories: `core/common/src`, `core/jvm/src`

### Features left, by files blocked

69 of 69 files use no feature that kc doesn't support.

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

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| core/common/src/Clock.kt | 102 |  |
| core/common/src/DateTimePeriod.kt | 693 |  |
| core/common/src/DateTimeUnit.kt | 302 |  |
| core/common/src/DayOfWeek.kt | 47 |  |
| core/common/src/DeprecatedClock.kt | 23 |  |
| core/common/src/DeprecatedInstant.kt | 23 |  |
| core/common/src/Exceptions.kt | 45 |  |
| core/common/src/Instant.kt | 548 |  |
| core/common/src/LocalDate.kt | 674 |  |
| core/common/src/LocalDateRange.kt | 277 |  |
| core/common/src/LocalDateTime.kt | 548 |  |
| core/common/src/LocalIsoWeekDate.kt | 461 |  |
| core/common/src/LocalTime.kt | 466 |  |
| core/common/src/Month.kt | 79 |  |
| core/common/src/OverloadMarker.kt | 31 |  |
| core/common/src/TimeZone.kt | 357 |  |
| core/common/src/UtcOffset.kt | 283 |  |
| core/common/src/YearMonth.kt | 454 |  |
| core/common/src/YearMonthRange.kt | 277 |  |
| core/common/src/format/DateTimeComponents.kt | 688 |  |
| core/common/src/format/DateTimeFormat.kt | 171 |  |
| core/common/src/format/DateTimeFormatBuilder.kt | 565 |  |
| core/common/src/format/LocalDateFormat.kt | 295 |  |
| core/common/src/format/LocalDateTimeFormat.kt | 83 |  |
| core/common/src/format/LocalTimeFormat.kt | 298 |  |
| core/common/src/format/Unicode.kt | 655 |  |
| core/common/src/format/UtcOffsetFormat.kt | 277 |  |
| core/common/src/format/YearMonthFormat.kt | 292 |  |
| core/common/src/internal/BinaryDataReader.kt | 60 |  |
| core/common/src/internal/dateCalculations.kt | 46 |  |
| core/common/src/internal/format/Builder.kt | 21 |  |
| core/common/src/internal/format/FieldFormatDirective.kt | 219 |  |
| core/common/src/internal/format/FieldSpec.kt | 149 |  |
| core/common/src/internal/format/FormatStructure.kt | 270 |  |
| core/common/src/internal/format/Predicate.kt | 35 |  |
| core/common/src/internal/format/formatter/Formatter.kt | 66 |  |
| core/common/src/internal/format/formatter/FormatterOperation.kt | 130 |  |
| core/common/src/internal/format/parser/NumberConsumer.kt | 179 |  |
| core/common/src/internal/format/parser/ParseResult.kt | 26 |  |
| core/common/src/internal/format/parser/Parser.kt | 276 |  |
| core/common/src/internal/format/parser/ParserOperation.kt | 556 |  |
| core/common/src/internal/math.kt | 287 |  |
| core/common/src/internal/toKotlinCode.kt | 27 |  |
| core/common/src/internal/util.kt | 48 |  |
| core/common/src/serializers/DateTimePeriodSerializers.kt | 193 |  |
| core/common/src/serializers/DateTimeUnitSerializers.kt | 228 |  |
| core/common/src/serializers/DayOfWeekSerializers.kt | 23 |  |
| core/common/src/serializers/InstantSerializers.kt | 63 |  |
| core/common/src/serializers/LocalDateSerializers.kt | 127 |  |
| core/common/src/serializers/LocalDateTimeSerializers.kt | 140 |  |
| core/common/src/serializers/LocalTimeSerializers.kt | 122 |  |
| core/common/src/serializers/MonthSerializers.kt | 28 |  |
| core/common/src/serializers/TimeZoneSerializers.kt | 60 |  |
| core/common/src/serializers/UtcOffsetSerializers.kt | 69 |  |
| core/common/src/serializers/YearMonthSerializers.kt | 109 |  |
| core/jvm/src/Converters.kt | 119 |  |
| core/jvm/src/DayOfWeekJvm.kt | 20 |  |
| core/jvm/src/Instant.kt | 46 |  |
| core/jvm/src/LocalDate.kt | 239 |  |
| core/jvm/src/LocalDateTimeJvm.kt | 187 |  |
| core/jvm/src/LocalTimeJvm.kt | 142 |  |
| core/jvm/src/MonthJvm.kt | 25 |  |
| core/jvm/src/TimeZoneJvm.kt | 134 |  |
| core/jvm/src/UtcOffsetJvm.kt | 113 |  |
| core/jvm/src/YearMonthJvm.kt | 126 |  |
| core/jvm/src/format/Deprecations.kt | 55 |  |
| core/jvm/src/format/format/MyJvmDefaultWithoutCompatibility.kt | 8 |  |
| core/jvm/src/internal/Ser.kt | 74 |  |
| core/jvm/src/internal/mathJvm.kt | 11 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 59 files, 10674 lines

Directories: `core/common/test`, `core/jvm/test`

### Features left, by files blocked

59 of 59 files use no feature that kc doesn't support.

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

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| core/common/test/ClockTimeSourceTest.kt | 129 |  |
| core/common/test/DateTimePeriodTest.kt | 198 |  |
| core/common/test/DateTimeUnitTest.kt | 62 |  |
| core/common/test/DayOfWeekTest.kt | 37 |  |
| core/common/test/InstantTest.kt | 659 |  |
| core/common/test/LocalDateRangeTest.kt | 316 |  |
| core/common/test/LocalDateTest.kt | 411 |  |
| core/common/test/LocalDateTimeTest.kt | 203 |  |
| core/common/test/LocalIsoWeekDateTest.kt | 129 |  |
| core/common/test/LocalTimeTest.kt | 285 |  |
| core/common/test/MonthTest.kt | 31 |  |
| core/common/test/MultiplyAndDivideTest.kt | 83 |  |
| core/common/test/ReadmeTest.kt | 211 |  |
| core/common/test/TimeZoneTest.kt | 321 |  |
| core/common/test/UtcOffsetTest.kt | 179 |  |
| core/common/test/YearMonthRangeTest.kt | 313 |  |
| core/common/test/YearMonthTest.kt | 221 |  |
| core/common/test/assertions.kt | 48 |  |
| core/common/test/format/DateTimeComponentsFormatTest.kt | 492 |  |
| core/common/test/format/DateTimeComponentsTest.kt | 68 |  |
| core/common/test/format/DateTimeFormatTest.kt | 194 |  |
| core/common/test/format/LocalDateFormatTest.kt | 271 |  |
| core/common/test/format/LocalDateTimeFormatTest.kt | 251 |  |
| core/common/test/format/LocalTimeFormatTest.kt | 247 |  |
| core/common/test/format/ParserStructureConcatenationTest.kt | 191 |  |
| core/common/test/format/UtcOffsetFormatTest.kt | 192 |  |
| core/common/test/format/YearMonthFormatTest.kt | 86 |  |
| core/common/test/samples/ClockSamples.kt | 67 |  |
| core/common/test/samples/DateTimePeriodSamples.kt | 178 |  |
| core/common/test/samples/DateTimeUnitSamples.kt | 57 |  |
| core/common/test/samples/DayOfWeekSamples.kt | 52 |  |
| core/common/test/samples/InstantSamples.kt | 340 |  |
| core/common/test/samples/LocalDateRangeSamples.kt | 98 |  |
| core/common/test/samples/LocalDateSamples.kt | 365 |  |
| core/common/test/samples/LocalDateTimeSamples.kt | 269 |  |
| core/common/test/samples/LocalIsoWeekDateSamples.kt | 109 |  |
| core/common/test/samples/LocalTimeSamples.kt | 324 |  |
| core/common/test/samples/MonthSamples.kt | 57 |  |
| core/common/test/samples/TimeZoneSamples.kt | 253 |  |
| core/common/test/samples/UtcOffsetSamples.kt | 169 |  |
| core/common/test/samples/YearMonthRangeSamples.kt | 98 |  |
| core/common/test/samples/YearMonthSamples.kt | 250 |  |
| core/common/test/samples/format/DateTimeComponentsFormatSamples.kt | 48 |  |
| core/common/test/samples/format/DateTimeComponentsSamples.kt | 506 |  |
| core/common/test/samples/format/DateTimeFormatBuilderSamples.kt | 103 |  |
| core/common/test/samples/format/DateTimeFormatSamples.kt | 150 |  |
| core/common/test/samples/format/LocalDateFormatSamples.kt | 121 |  |
| core/common/test/samples/format/LocalDateTimeFormatSamples.kt | 28 |  |
| core/common/test/samples/format/LocalTimeFormatSamples.kt | 58 |  |
| core/common/test/samples/format/UnicodeSamples.kt | 31 |  |
| core/common/test/samples/format/UtcOffsetFormatSamples.kt | 45 |  |
| core/common/test/samples/format/YearMonthFormatSamples.kt | 152 |  |
| core/jvm/test/ConvertersTest.kt | 214 |  |
| core/jvm/test/DeprecationReplacements.kt | 33 |  |
| core/jvm/test/InstantParsing.kt | 35 |  |
| core/jvm/test/JvmSerializationTest.kt | 97 |  |
| core/jvm/test/LocalIsoWeekDateTest.kt | 22 |  |
| core/jvm/test/MaliciousJvmSerializationTest.kt | 206 |  |
| core/jvm/test/UnicodeFormatTest.kt | 311 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

