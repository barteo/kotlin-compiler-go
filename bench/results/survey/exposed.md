# Survey: exposed 1.5.1

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 169 files, 32485 lines

Directories: `exposed-core/src/main`, `exposed-dao/src/main`, `exposed-jdbc/src/main`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 169 | 10 |
| 1: packages, annotations | 1 | 168 | 8 |
| 2: interfaces, inheritance | 22 | 147 | 7 |
| 3: type checks | 22 | 147 | 6 |
| 4: exceptions | 23 | 146 | 5 |
| 5: objects | 24 | 145 | 5 |
| 6: data, enum, sealed | 38 | 131 | 4 |
| 7: arrays, vararg, spread | 42 | 127 | 4 |
| 8: Char, Float, Byte, Short | 44 | 125 | 4 |
| 9: inliner | 53 | 116 | 3 |
| 10: lambdas with receivers | 54 | 115 | 3 |
| 11: extensions | 58 | 111 | 2 |
| 12: operator conventions | 61 | 108 | 2 |
| 13: identity, class literals, references | 76 | 93 | 2 |
| 14: class delegation | 76 | 93 | 2 |
| 15: secondary constructors | 79 | 90 | 2 |
| 16: local functions | 80 | 89 | 2 |
| 17: expect and actual | 80 | 89 | 2 |
| 18: annotation classes | 81 | 88 | 2 |
| 19: value classes | 81 | 88 | 2 |
| 20: unsigned integers | 83 | 86 | 2 |
| definite assignment | 83 | 86 | 2 |
| 21: inline functions | 83 | 86 | 2 |
| 22: reified type parameters | 83 | 86 | 2 |
| 23: labels, do-while | 83 | 86 | 2 |
| 24: delegated properties | 85 | 84 | 1 |
| 25: object expressions | 88 | 81 | 1 |
| 26: destructuring lambda parameters | 99 | 70 | 1 |
| 42-49: coroutines | 104 | 65 | 1 |
| 52: bitwise operators | 106 | 63 | 1 |
| 53: typealias | 107 | 62 | 1 |
| 54: setters, backing fields | 113 | 56 | 1 |
| 55: lateinit, tailrec | 113 | 56 | 1 |
| 56-57: collection supertypes | 116 | 53 | 1 |
| 58: member extension functions | 132 | 37 | 1 |
| 59: reified member type parameters | 132 | 37 | 1 |
| 60: inner and local classes | 134 | 35 | 1 |
| 61: when guards, anonymous functions, local extension functions | 144 | 25 | 1 |
| 62a: @JvmField and @JvmStatic in objects | 144 | 25 | 1 |
| 62b: generic value classes | 144 | 25 | 1 |
| 63: definitely non-nullable types | 162 | 7 | 1 |
| 70a: inline property accessors | 162 | 7 | 1 |
| 70b: where clauses | 165 | 4 | 1 |
| 70c: fun interfaces | 165 | 4 | 1 |
| delegated extension properties | 167 | 2 | 1 |
| qualified super | 167 | 2 | 1 |
| context parameters of top-level functions | 167 | 2 | 1 |
| also supported | 168 | 1 | 1 |

### Features left, by files blocked

168 of 169 files use no feature that kc doesn't support.

| Feature | Files | Only blocker | Uses |
|:---|---:|---:|---:|
| safe-call assignment | 1 | 1 | 1 |

### Greedy order of the features left

At each step, the feature that unblocks the most files given the ones before it, or, if none unblocks a file, the one that blocks the most files.

| Step | Feature | Files unblocked |
|---:|:---|---:|
| 1 | safe-call assignment | 169 |

### Inline-only functions called

`require`, `check`, `requireNotNull`, `checkNotNull`, `error` and `TODO` are built in; kc inlines the others, since milestone 22 also those with reified type parameters.

| Function | Files | Calls |
|:---|---:|---:|
| `let` | 56 | 213 |
| `error` | 38 | 124 |
| `isNotEmpty` | 37 | 95 |
| `List` | 31 | 78 |
| `apply` | 30 | 96 |
| `takeIf` | 26 | 46 |
| `require` | 22 | 61 |
| `also` | 21 | 50 |
| `with` | 21 | 48 |
| `orEmpty` | 17 | 28 |
| `run` | 17 | 24 |
| `buildString` | 14 | 33 |
| `insert` | 12 | 24 |
| `find` | 11 | 39 |
| `check` | 8 | 17 |
| `Iterable` | 6 | 11 |
| `format` | 6 | 22 |
| `Sequence` | 5 | 5 |
| `isNullOrEmpty` | 4 | 5 |
| `matches` | 4 | 22 |
| `requireNotNull` | 3 | 6 |
| `takeUnless` | 3 | 4 |
| `toBoolean` | 3 | 3 |
| `use` | 3 | 5 |
| `buildList` | 2 | 2 |
| `isNotBlank` | 2 | 3 |
| `mod` | 2 | 10 |
| `synchronized` | 2 | 2 |
| `AutoCloseable` | 1 | 1 |
| `assert` | 1 | 1 |
| `checkNotNull` | 1 | 1 |
| `codePointCount` | 1 | 3 |
| `findLast` | 1 | 1 |
| `firstNotNullOfOrNull` | 1 | 1 |
| `floorDiv` | 1 | 1 |
| `fromBits` | 1 | 1 |
| `ifBlank` | 1 | 1 |
| `inputStream` | 1 | 3 |
| `isNaN` | 1 | 2 |
| `isNullOrBlank` | 1 | 3 |
| `suspend` | 1 | 1 |
| `toBigDecimal` | 1 | 5 |
| `toRawBits` | 1 | 1 |

### Files

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/AbstractQuery.kt | 294 | (annotation, as, class literal, default value, destructuring lambda parameter, inheritance, inline-only call, modifier abstract, modifier enum, modifier open, modifier override, modifier protected, modifier vararg, named argument, nested class, package, property without initializer, require, check, error) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Alias.kt | 329 | (annotation, as, default value, definitely non-nullable type, extension function, extension property, inheritance, inline-only call, interface, is, modifier infix, modifier operator, modifier override, named argument, object expression, package, property getter, property without initializer, qualified this, require, check, error, throw) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Annotations.kt | 44 | (annotation, modifier annotation, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Column.kt | 235 | (Char, annotation, as, class literal, default value, definitely non-nullable type, function type with receiver, identity equality, inheritance, inline-only call, is, modifier override, named argument, package, property getter, qualified this, require, check, error) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/ColumnDiff.kt | 54 | (default value, modifier companion, modifier data, named argument, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/ColumnType.kt | 1812 | (Byte and Short, Char, Float, annotation, arrays, as, bitwise operators, callable reference, class delegation, class literal, default value, definitely non-nullable type, delegated property, destructuring lambda parameter, extension function, extension property, function without body, identity equality, inheritance, inline-only call, interface, is, modifier abstract, modifier companion, modifier enum, modifier open, modifier override, modifier protected, named argument, object, object expression, package, property getter, property setter, property without initializer, require, check, error, secondary constructor, super, throw, unsigned integers, when guard, when subject variable) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/CompositeColumn.kt | 68 | (class literal, default value, destructuring declaration, function without body, inheritance, modifier abstract, modifier override, modifier protected, package, require, check, error) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Constraints.kt | 349 | (Char, annotation, class literal, default value, extension function, extension property, function without body, identity equality, inheritance, inline-only call, interface, is, modifier companion, modifier data, modifier enum, modifier operator, modifier override, named argument, package, property getter, property without initializer, require, check, error, secondary constructor, typealias) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/DatabaseApi.kt | 71 | (annotation, class literal, default value, inline-only call, modifier abstract, modifier companion, modifier override, modifier protected, package, property without initializer) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/DatabaseConfig.kt | 249 | (annotation, class literal, default value, function type with receiver, inheritance, inline-only call, interface, modifier companion, modifier open, modifier operator, modifier override, nested class, package, property getter, property without initializer, require, check, error) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Exceptions.kt | 79 | (annotation, as, extension function, inheritance, inline-only call, modifier override, package, super, throw, try) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/ExplainQuery.kt | 49 | (annotation, arrays, inheritance, modifier companion, modifier open, modifier override, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Expression.kt | 187 | (Char, annotation, arrays, as, callable reference, class literal, default value, definitely non-nullable type, delegated property, extension function, function type with receiver, function without body, identity equality, inheritance, inline-only call, is, local function, modifier abstract, modifier companion, modifier inline, modifier operator, modifier override, modifier vararg, package, property without initializer, qualified this, throw) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Function.kt | 746 | (Char, annotation, as, class literal, default value, definitely non-nullable type, destructuring declaration, function without body, inheritance, inline-only call, is, modifier abstract, modifier open, modifier override, modifier sealed, modifier vararg, named argument, nested class, package, property getter, property without initializer, require, check, error, when subject variable) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/FunctionBuilder.kt | 274 | (annotation, arrays, as, default value, definitely non-nullable type, extension function, is, modifier infix, modifier operator, modifier vararg, named argument, package, spread operator) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/LiteralOp.kt | 102 | (Byte and Short, Float, annotation, arrays, as, class literal, default value, definitely non-nullable type, extension function, inheritance, inline-only call, is, modifier inline, modifier override, modifier reified, named argument, package, unsigned integers) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Op.kt | 649 | (Char, annotation, as, default value, definitely non-nullable type, extension function, inheritance, inline-only call, interface, is, modifier abstract, modifier companion, modifier data, modifier infix, modifier inline, modifier operator, modifier override, modifier sealed, named argument, nested class, object, package, require, check, error, super, where clause) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/OpBuilder.kt | 917 | (annotation, arrays, as, callable reference, class literal, default value, extension function, inline-only call, is, modifier infix, modifier inline, modifier operator, modifier reified, named argument, package, qualified this, when guard, when subject variable, where clause) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/QueryParameter.kt | 121 | (Byte and Short, Float, annotation, arrays, as, class literal, default value, definitely non-nullable type, destructuring lambda parameter, extension function, inheritance, inline-only call, modifier inline, modifier override, modifier reified, named argument, package, unsigned integers) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/ResultRow.kt | 330 | (annotation, arrays, as, class literal, destructuring declaration, extension function, identity equality, inline-only call, is, modifier companion, modifier operator, modifier override, named argument, nested class, object, package, require, check, error, secondary constructor, try) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/SQLExpressionBuilder.kt | 1194 | (annotation, arrays, as, default value, definitely non-nullable type, extension function, inheritance, inline-only call, interface, is, member extension function, modifier infix, modifier operator, modifier vararg, named argument, object, package, qualified this, spread operator, when subject variable, where clause) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/SQLLog.kt | 67 | (annotation, class literal, function without body, inheritance, interface, modifier override, object, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Schema.kt | 69 | (annotation, class literal, default value, extension function, inline-only call, modifier data, package, property getter, property without initializer, throw) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/SchemaUtilityApi.kt | 658 | (Byte and Short, Char, annotation, as, class literal, destructuring declaration, destructuring lambda parameter, extension function, inheritance, inline-only call, is, local extension function, local function, modifier abstract, modifier companion, modifier const, modifier inline, modifier protected, modifier vararg, named argument, object, package, qualified this, require, check, error, unsigned integers, when guard, when subject variable) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Sequence.kt | 82 | (annotation, class literal, default value, inline-only call, modifier override, package, property getter, property without initializer, throw) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/SortOrder.kt | 34 | (modifier enum, named argument, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Table.kt | 2245 | (Byte and Short, Char, Float, annotation, arrays, as, callable reference, class literal, default value, definitely non-nullable type, delegated property, destructuring lambda parameter, extension function, function type with receiver, function without body, identity equality, inheritance, inline-only call, interface, is, modifier abstract, modifier enum, modifier infix, modifier inline, modifier inner, modifier open, modifier override, modifier reified, modifier vararg, named argument, nested class, object, package, property getter, property without initializer, qualified this, reified member type parameter, require, check, error, secondary constructor, spread operator, throw, typealias, unsigned integers, when subject variable) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/TableDepthGraph.kt | 63 | (callable reference, inline-only call, local function, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Transaction.kt | 117 | (Char, annotation, as, class literal, delegated property, inheritance, inline-only call, is, modifier abstract, modifier open, modifier protected, modifier vararg, package, property backing field, property getter) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Version.kt | 60 | (annotation, as, class literal, default value, identity equality, inline-only call, modifier companion, modifier override, package, throw) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/WindowFunction.kt | 468 | (annotation, as, default value, definitely non-nullable type, destructuring lambda parameter, extension function, function without body, inheritance, inline-only call, interface, modifier companion, modifier enum, modifier open, modifier override, modifier sealed, modifier vararg, object, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/WindowFunctionBuilder.kt | 61 | (default value, extension function, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/dao/id/CompositeID.kt | 62 | (annotation, as, inline-only call, is, modifier companion, modifier operator, modifier override, package, require, check, error) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/dao/id/EntityID.kt | 44 | (annotation, as, is, modifier open, modifier override, modifier protected, package, property getter, require, check, error, secondary constructor) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/dao/id/IdTable.kt | 192 | (annotation, as, class literal, default value, function without body, inheritance, inline-only call, interface, is, modifier abstract, modifier final, modifier open, modifier override, modifier protected, object, object expression, package, property getter, property without initializer, require, check, error, secondary constructor, unsigned integers) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/dao/id/java/UUIDTable.kt | 22 | (default value, inheritance, modifier final, modifier open, modifier override, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/datetime/DurationColumnType.kt | 40 | (definitely non-nullable type, function without body, inheritance, is, modifier abstract, modifier override, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/datetime/InstantColumnType.kt | 143 | (Char, annotation, class literal, definitely non-nullable type, delegated property, extension function, function without body, inheritance, inline-only call, is, modifier abstract, modifier override, named argument, package, super, throw, try, when guard, when subject variable) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/datetime/LocalDateColumnType.kt | 110 | (class literal, definitely non-nullable type, extension function, function without body, inheritance, inline-only call, is, modifier abstract, modifier override, package, super) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/datetime/LocalDateTimeColumnType.kt | 180 | (Char, annotation, as, class literal, default value, definitely non-nullable type, delegated property, extension function, function without body, inheritance, inline-only call, is, modifier abstract, modifier override, named argument, package, super, try, when subject variable) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/datetime/LocalTimeColumnType.kt | 122 | (Char, annotation, class literal, definitely non-nullable type, delegated property, function without body, inheritance, inline-only call, is, modifier abstract, modifier override, package, super) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/datetime/OffsetDateTimeColumnType.kt | 146 | (Char, class literal, definitely non-nullable type, delegated property, function without body, inheritance, inline-only call, is, modifier abstract, modifier override, package, require, check, error, super, when guard, when subject variable) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/functions/array/ArrayFunctions.kt | 51 | (definitely non-nullable type, inheritance, modifier override, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/functions/math/MathFunctions.kt | 98 | (arrays, default value, inheritance, inline-only call, is, named argument, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/functions/math/TrigonometricalFunctions.kt | 96 | (arrays, inheritance, named argument, object, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/functions/vector/VectorFunctions.kt | 46 | (inheritance, modifier enum, modifier override, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/java/UUIDColumnType.kt | 35 | (arrays, class literal, extension function, inheritance, inline-only call, is, modifier override, package, require, check, error, when guard) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/ops/AllAnyOps.kt | 83 | (Char, extension function, function without body, inheritance, member extension function, modifier abstract, modifier override, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/ops/InListOps.kt | 186 | (Char, default value, destructuring lambda parameter, extension function, function without body, inheritance, is, member extension function, modifier abstract, modifier open, modifier override, modifier protected, named argument, package, property without initializer, super) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/ops/InTableOp.kt | 31 | (Char, default value, inheritance, modifier override, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/BatchInsertStatement.kt | 127 | (annotation, as, callable reference, class literal, default value, extension function, inheritance, inline-only call, is, modifier open, modifier operator, modifier override, package, property backing field, property getter, require, check, error, super, throw) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/BatchReplaceStatement.kt | 31 | (default value, inheritance, modifier open, modifier override, named argument, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/BatchUpdateStatement.kt | 67 | (annotation, as, class literal, delegated property, destructuring lambda parameter, inheritance, inline-only call, is, modifier open, modifier override, named argument, package, require, check, error, super, throw) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/BatchUpsertStatement.kt | 51 | (default value, inheritance, inline-only call, is, modifier open, modifier override, modifier vararg, named argument, package, super) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/DeleteStatement.kt | 54 | (as, class literal, default value, inheritance, inline-only call, is, modifier open, modifier override, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/InsertSelectStatement.kt | 33 | (default value, inheritance, inline-only call, modifier open, modifier override, package, require, check, error) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/InsertStatement.kt | 136 | (annotation, class literal, default value, delegated property, destructuring lambda parameter, extension function, inheritance, inline-only call, modifier infix, modifier open, modifier operator, modifier override, modifier protected, named argument, package, property backing field, property getter, qualified this, require, check, error, throw) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/MergeStatement.kt | 162 | (default value, destructuring lambda parameter, inheritance, inline-only call, is, modifier abstract, modifier data, modifier enum, modifier open, modifier override, modifier protected, named argument, nested class, package, super) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/MultiRowValuesInsertStatement.kt | 61 | (annotation, class literal, default value, destructuring lambda parameter, inheritance, inline-only call, is, modifier const, modifier open, modifier override, named argument, package, super, throw) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/ReplaceStatement.kt | 50 | (inheritance, modifier open, modifier override, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/ReturningStatement.kt | 27 | (inheritance, modifier open, modifier override, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/SQLServerBatchInsertStatement.kt | 61 | (annotation, class literal, default value, destructuring lambda parameter, inheritance, inline-only call, modifier const, modifier open, modifier override, named argument, package, super, throw) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/Statement.kt | 149 | (Char, as, default value, destructuring declaration, extension function, function without body, inheritance, inline-only call, modifier abstract, modifier data, modifier enum, modifier override, modifier sealed, nested class, object, package, prefix increment, when guard, when subject variable) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/StatementBuilder.kt | 474 | (annotation, default value, extension function, function type with receiver, inheritance, inline-only call, interface, is, member extension function, modifier vararg, named argument, object, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/StatementInterceptor.kt | 64 | (inheritance, interface, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/UpdateBuilder.kt | 120 | (annotation, as, class literal, destructuring lambda parameter, extension function, inheritance, inline-only call, is, modifier abstract, modifier open, modifier operator, modifier protected, package, require, check, error) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/UpdateStatement.kt | 88 | (annotation, as, class literal, default value, extension function, inheritance, inline-only call, is, modifier open, modifier override, package, property getter, require, check, error, when guard) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/UpsertStatement.kt | 140 | (annotation, as, class literal, definitely non-nullable type, destructuring lambda parameter, extension function, function type with receiver, inheritance, inline-only call, interface, is, modifier companion, modifier open, modifier override, modifier sealed, modifier vararg, named argument, package, qualified this, super, when subject variable) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/api/ExposedBlob.kt | 46 | (arrays, extension function, identity equality, inline-only call, is, modifier override, named argument, package, property getter, secondary constructor, try) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/api/ExposedDatabaseMetadata.kt | 122 | (annotation, function without body, inline-only call, is, modifier abstract, modifier protected, named argument, package, property without initializer) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/api/ExposedMetadataUtils.kt | 142 | (Char, annotation, class literal, default value, extension function, function type with receiver, inline-only call, is, object, package, require, check, error) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/api/ExposedSavepoint.kt | 8 | (annotation, modifier abstract, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/api/IdentifierManagerApi.kt | 198 | (Char, annotation, class literal, default value, delegated property, extension function, function without body, inheritance, inline-only call, modifier abstract, modifier enum, modifier inline, modifier override, modifier protected, nested class, object expression, package, property getter, property without initializer) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/api/PreparedStatementApi.kt | 40 | (arrays, as, function without body, interface, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/api/ResultApi.kt | 103 | (annotation, as, function without body, inheritance, inline-only call, interface, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/DatabasesManager.kt | 66 | (annotation, function without body, inheritance, interface, modifier abstract, modifier override, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/ThreadLocalTransactionsStack.kt | 116 | (annotation, class literal, inheritance, inline-only call, modifier override, object, package, property getter, require, check, error) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/TransactionInterface.kt | 19 | (interface, package, property without initializer) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/TransactionManagerApi.kt | 19 | (interface, package, property without initializer) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/TransactionManagersContainer.kt | 102 | (annotation, function without body, inheritance, inline-only call, interface, modifier abstract, modifier override, modifier protected, package, throw) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/TransactionScope.kt | 53 | (annotation, as, class literal, default value, function type with receiver, inheritance, inline-only call, modifier override, package, require, check, error) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/Transactions.kt | 57 | (annotation, package, require, check, error, try) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/TransactionsHolder.kt | 99 | (annotation, class literal, function without body, inline-only call, interface, object, package, property getter, property without initializer) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/suspend/TransactionContextElement.kt | 83 | (annotation, inheritance, modifier companion, modifier override, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/suspend/TransactionContextHolder.kt | 22 | (annotation, inheritance, interface, modifier data, modifier override, package, property without initializer) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/utils/CachableMapWithDefault.kt | 16 | (annotation, default value, function without body, inheritance, interface, modifier override, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/utils/CachableMapWithSuspendableDefault.kt | 19 | (annotation, default value, function without body, inheritance, inline-only call, interface, modifier override, modifier suspend, package, suspend function type) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/ColumnMetadata.kt | 29 | (default value, modifier data, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/DataTypeProvider.kt | 166 | (annotation, as, function without body, inline-only call, is, modifier abstract, modifier open, package, throw) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/DatabaseDialect.kt | 212 | (Char, annotation, class literal, extension function, function without body, inline-only call, interface, is, modifier companion, modifier vararg, named argument, package, property getter, property without initializer, qualified this, try) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/ForUpdateOption.kt | 135 | (annotation, default value, inheritance, inline-only call, interface, modifier abstract, modifier companion, modifier data, modifier enum, modifier final, modifier open, modifier override, modifier sealed, modifier vararg, named argument, nested class, object, package, property without initializer, require, check, error) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/FunctionProvider.kt | 1038 | (annotation, default value, destructuring declaration, destructuring lambda parameter, extension function, function without body, inline-only call, interface, is, local extension function, local function, member extension function, modifier abstract, modifier open, modifier protected, modifier vararg, named argument, nested class, package, qualified this, throw) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/H2.kt | 496 | (Char, annotation, as, class literal, delegated property, destructuring lambda parameter, extension function, extension property, inheritance, inline-only call, is, modifier companion, modifier enum, modifier open, modifier override, named argument, nested class, object, package, property getter, require, check, error, super) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/Keywords.kt | 506 | (package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/MariaDBDialect.kt | 169 | (annotation, as, class literal, delegated property, inheritance, inline-only call, is, modifier companion, modifier const, modifier open, modifier override, object, package, super, throw) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/MysqlDialect.kt | 453 | (annotation, arrays, as, class literal, delegated property, destructuring lambda parameter, extension function, inheritance, inline-only call, is, member extension function, modifier companion, modifier enum, modifier open, modifier override, modifier protected, modifier vararg, named argument, nested class, object, package, property getter, require, check, error, super, throw, when guard) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/OracleDialect.kt | 574 | (annotation, as, bitwise operators, class literal, destructuring declaration, destructuring lambda parameter, extension function, inheritance, inline-only call, modifier companion, modifier open, modifier override, modifier vararg, named argument, object, package, require, check, error, super, throw, try) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/PostgreSQL.kt | 537 | (Char, annotation, arrays, as, class literal, default value, destructuring lambda parameter, extension function, inheritance, inline-only call, is, member extension function, modifier companion, modifier const, modifier open, modifier override, modifier vararg, named argument, object, package, super, when guard) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/PrimaryKeyMetadata.kt | 11 | (modifier data, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/Redshift.kt | 364 | (annotation, class literal, destructuring lambda parameter, inheritance, inline-only call, modifier companion, modifier const, modifier open, modifier override, modifier vararg, named argument, object, package, require, check, error, super, throw) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/SQLServerDialect.kt | 507 | (Char, annotation, arrays, class literal, destructuring lambda parameter, inheritance, inline-only call, is, modifier companion, modifier open, modifier override, modifier vararg, named argument, object, package, require, check, error, super) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/SQLiteDialect.kt | 393 | (Char, annotation, arrays, class literal, destructuring lambda parameter, inheritance, inline-only call, modifier companion, modifier const, modifier open, modifier override, modifier vararg, named argument, object, package, super) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/SchemaMetadata.kt | 11 | (modifier data, package) |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/VendorDialect.kt | 159 | (Char, annotation, class literal, extension function, inheritance, inline-only call, is, modifier abstract, modifier open, modifier override, modifier protected, modifier vararg, named argument, nested class, package, property getter, property without initializer) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/CompositeEntity.kt | 30 | (default value, inheritance, modifier abstract, package) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/DaoEntityID.kt | 15 | (inheritance, modifier override, package) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/DaoEntityIDFactory.kt | 15 | (inheritance, modifier override, package) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/Entity.kt | 486 | (annotation, as, default value, delegated property, destructuring declaration, destructuring lambda parameter, extension function, inheritance, inline-only call, is, modifier infix, modifier open, modifier operator, modifier override, modifier protected, modifier value, package, property getter, qualified this, require, check, error, throw, when guard) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/EntityBatchUpdate.kt | 62 | (as, inline-only call, modifier operator, package, require, check, error) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/EntityCache.kt | 355 | (annotation, as, callable reference, class literal, default value, delegated extension property, delegated property, destructuring declaration, destructuring lambda parameter, extension function, extension property, inheritance, inline-only call, is, modifier companion, modifier inner, modifier override, nested class, package, property backing field, property setter, require, check, error, throw, try) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/EntityClass.kt | 1214 | (annotation, arrays, as, class literal, default value, delegated property, destructuring lambda parameter, extension function, function type with receiver, identity equality, inheritance, inline-only call, is, modifier abstract, modifier final, modifier infix, modifier inline, modifier open, modifier operator, modifier override, modifier protected, modifier reified, named argument, package, qualified this, reified member type parameter, require, check, error, super, throw, try, when guard) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/EntityHook.kt | 130 | (annotation, as, class literal, delegated extension property, delegated property, extension function, extension property, inline-only call, modifier data, modifier enum, object, package, try) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/EntityLifecycleInterceptor.kt | 143 | (annotation, class literal, delegated property, extension function, inheritance, is, modifier override, named argument, package, try, when subject variable) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/InnerTableLink.kt | 142 | (annotation, arrays, as, bitwise operators, default value, delegated property, destructuring declaration, inheritance, inline-only call, modifier infix, modifier operator, modifier override, named argument, package, require, check, error, spread operator) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/IntEntity.kt | 29 | (default value, inheritance, modifier abstract, package) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/LinkedIdentityHashSet.kt | 75 | (collection supertype, inheritance, inline-only call, modifier override, object expression, package, property getter, qualified this, require, check, error) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/LongEntity.kt | 29 | (default value, inheritance, modifier abstract, package) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/References.kt | 432 | safe-call assignment (annotation, arrays, as, class literal, default value, destructuring lambda parameter, extension function, inheritance, inline-only call, is, local extension function, local function, modifier infix, modifier open, modifier operator, modifier override, modifier vararg, named argument, package, qualified this, require, check, error, spread operator, throw, when subject variable) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/UIntEntity.kt | 29 | (default value, inheritance, modifier abstract, package, unsigned integers) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/ULongEntity.kt | 29 | (default value, inheritance, modifier abstract, package, unsigned integers) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/UuidEntity.kt | 31 | (annotation, class literal, default value, inheritance, modifier abstract, package) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/View.kt | 29 | (inheritance, inline-only call, modifier operator, modifier override, modifier vararg, package, spread operator) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/exceptions/EntityNotFoundException.kt | 11 | (inheritance, package) |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/java/UUIDEntity.kt | 30 | (default value, inheritance, modifier abstract, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/Database.kt | 303 | (Char, annotation, class literal, default value, delegated property, destructuring lambda parameter, extension property, function type with receiver, inheritance, inline-only call, interface, is, modifier companion, modifier override, named argument, package, property getter, require, check, error, try) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/ExplainBlockingExecutable.kt | 63 | (collection supertype, default value, extension function, function type with receiver, inheritance, inline-only call, member extension function, modifier open, modifier override, nested class, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/ExposedConnectionImpl.kt | 11 | (inheritance, modifier override, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/IterableEx.kt | 186 | (annotation, collection supertype, default value, extension function, function without body, inheritance, inline-only call, interface, is, modifier infix, modifier operator, modifier override, modifier vararg, object expression, package, property getter, property without initializer, require, check, error, secondary constructor, spread operator, throw) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/JdbcTransaction.kt | 353 | (annotation, as, class delegation, class literal, default value, destructuring lambda parameter, extension function, function type with receiver, inheritance, inline-only call, is, member extension function, modifier companion, modifier final, modifier open, modifier override, modifier suspend, modifier vararg, named argument, object expression, package, property backing field, property getter, property setter, property without initializer, require, check, error, spread operator, super) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/MetadataQueries.kt | 23 | (extension function, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/Queries.kt | 851 | (annotation, as, class literal, default value, extension function, function type with receiver, inline-only call, local extension function, local function, modifier vararg, named argument, package, throw, try) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/Query.kt | 377 | (annotation, arrays, as, class literal, collection supertype, default value, extension function, function type with receiver, inheritance, inline-only call, is, local extension function, local function, member extension function, modifier companion, modifier inline, modifier inner, modifier open, modifier override, modifier protected, modifier vararg, named argument, nested class, object expression, package, property getter, qualified this, require, check, error, throw, try) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/SchemaUtils.kt | 529 | (annotation, arrays, class literal, default value, destructuring declaration, extension function, inheritance, inline-only call, is, modifier vararg, named argument, object, object expression, package, spread operator, throw, try) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/SetOperations.kt | 312 | (annotation, as, class literal, default value, destructuring lambda parameter, extension function, inheritance, inline-only call, is, member extension function, modifier companion, modifier inner, modifier open, modifier override, modifier protected, modifier sealed, modifier vararg, named argument, nested class, package, property getter, require, check, error, super, throw, try) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/BatchInsertBlockingExecutable.kt | 85 | (annotation, arrays, as, class literal, extension function, inheritance, inline-only call, is, member extension function, modifier open, modifier override, package, super) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/BatchUpdateBlockingExecutable.kt | 16 | (extension function, inheritance, member extension function, modifier open, modifier override, package, qualified this) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/BatchUpsertBlockingExecutable.kt | 23 | (inheritance, modifier open, modifier override, package, super) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/BlockingExecutable.kt | 167 | (annotation, as, class literal, extension function, function without body, inline-only call, interface, is, member extension function, modifier override, object, package, property getter, property without initializer, require, check, error, throw, try) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/DeleteBlockingExecutable.kt | 16 | (extension function, inheritance, member extension function, modifier open, modifier override, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/InsertBlockingExecutable.kt | 278 | (annotation, arrays, as, class literal, destructuring declaration, destructuring lambda parameter, extension function, inheritance, inline-only call, is, member extension function, modifier open, modifier override, modifier protected, named argument, package, property getter, qualified this, require, check, error, throw, try) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/InsertSelectBlockingExecutable.kt | 14 | (extension function, inheritance, member extension function, modifier open, modifier override, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/MergeBlockingExecutable.kt | 17 | (extension function, inheritance, member extension function, modifier open, modifier override, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/ReturningBlockingExecutable.kt | 43 | (annotation, class literal, collection supertype, extension function, inheritance, inline-only call, member extension function, modifier inner, modifier open, modifier override, nested class, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/StatementIterator.kt | 32 | (collection supertype, function without body, inheritance, modifier abstract, modifier operator, modifier override, modifier protected, package, property backing field, property setter, property without initializer, throw) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/UpdateBlockingExecutable.kt | 19 | (annotation, class literal, extension function, inheritance, member extension function, modifier open, modifier override, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/UpsertBlockingExecutable.kt | 23 | (inheritance, modifier open, modifier override, package, super) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/api/ExposedConnection.kt | 69 | (arrays, function type with receiver, function without body, interface, package, property without initializer) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/api/JdbcExposedDatabaseMetadata.kt | 107 | (function without body, inheritance, modifier abstract, modifier vararg, package, property without initializer) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/api/JdbcPreparedStatementApi.kt | 59 | (function without body, inheritance, interface, package, property without initializer) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/jdbc/JdbcConnectionImpl.kt | 169 | (Char, arrays, as, class delegation, delegated property, extension function, function type with receiver, inheritance, inline-only call, member extension function, modifier override, object expression, package, property backing field, property getter, property setter, require, check, error, super, try) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/jdbc/JdbcDatabaseMetadataImpl.kt | 779 | (annotation, arrays, callable reference, class literal, delegated property, destructuring declaration, destructuring lambda parameter, extension function, function type with receiver, inheritance, inline-only call, is, modifier companion, modifier override, modifier vararg, named argument, package, property backing field, property getter, property without initializer, require, check, error, try) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/jdbc/JdbcIdentifierManager.kt | 29 | (Char, annotation, inheritance, modifier override, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/jdbc/JdbcPreparedStatementImpl.kt | 113 | (arrays, inheritance, inline-only call, is, modifier override, package, property getter, property setter) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/jdbc/JdbcResult.kt | 60 | (as, extension property, inheritance, modifier override, package, qualified this, require, check, error) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/jdbc/JdbcSavepoint.kt | 9 | (inheritance, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/transactions/JdbcTransactionInterface.kt | 63 | (annotation, extension function, extension property, function without body, inheritance, interface, modifier inline, modifier override, modifier suspend, package, property getter, property without initializer, qualified this, try) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/transactions/JdbcTransactionManager.kt | 95 | (annotation, as, class literal, default value, extension function, extension property, function without body, inheritance, interface, is, modifier suspend, package, property getter, property without initializer, require, check, error) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/transactions/TransactionManager.kt | 297 | (annotation, as, class literal, default value, inheritance, inline-only call, modifier companion, modifier override, named argument, nested class, object expression, package, property backing field, property getter, property setter, qualified this, require, check, error, throw, try) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/transactions/Transactions.kt | 464 | (annotation, as, class literal, default value, function type with receiver, inline-only call, modifier inline, modifier suspend, named argument, package, throw, try) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/transactions/experimental/Suspended.kt | 77 | (annotation, default value, extension function, function type with receiver, modifier suspend, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/DatabaseDialectMetadata.kt | 201 | (annotation, class literal, destructuring declaration, extension function, inline-only call, member extension function, modifier abstract, modifier open, modifier protected, modifier vararg, package, property getter, spread operator) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/H2DialectMetadata.kt | 15 | (inheritance, inline-only call, modifier open, modifier override, modifier vararg, package, spread operator, super) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/MariaDBDialectMetadata.kt | 6 | (inheritance, modifier open, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/MysqlDialectMetadata.kt | 22 | (annotation, class literal, extension function, inheritance, member extension function, modifier open, modifier override, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/OracleDialectMetadata.kt | 6 | (inheritance, modifier open, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/PostgreSQLDialectMetadata.kt | 13 | (inheritance, modifier open, modifier override, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/RedshiftDialectMetadata.kt | 8 | (inheritance, modifier open, modifier override, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/SQLServerDialectMetadata.kt | 6 | (inheritance, modifier open, package) |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/SQLiteDialectMetadata.kt | 12 | (inheritance, modifier open, modifier override, package) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 124 files, 31013 lines

Directories: `exposed-tests/src`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 124 | 10 |
| 1: packages, annotations | 1 | 123 | 8 |
| 2: interfaces, inheritance | 2 | 122 | 6 |
| 3: type checks | 2 | 122 | 6 |
| 4: exceptions | 4 | 120 | 5 |
| 5: objects | 5 | 119 | 4 |
| 6: data, enum, sealed | 12 | 112 | 3 |
| 7: arrays, vararg, spread | 12 | 112 | 3 |
| 8: Char, Float, Byte, Short | 12 | 112 | 3 |
| 9: inliner | 22 | 102 | 2 |
| 10: lambdas with receivers | 24 | 100 | 2 |
| 11: extensions | 28 | 96 | 2 |
| 12: operator conventions | 28 | 96 | 2 |
| 13: identity, class literals, references | 35 | 89 | 2 |
| 14: class delegation | 36 | 88 | 2 |
| 15: secondary constructors | 36 | 88 | 2 |
| 16: local functions | 36 | 88 | 2 |
| 17: expect and actual | 36 | 88 | 2 |
| 18: annotation classes | 36 | 88 | 2 |
| 19: value classes | 36 | 88 | 1 |
| 20: unsigned integers | 36 | 88 | 1 |
| definite assignment | 36 | 88 | 1 |
| 21: inline functions | 36 | 88 | 1 |
| 22: reified type parameters | 37 | 87 | 1 |
| 23: labels, do-while | 37 | 87 | 1 |
| 24: delegated properties | 59 | 65 | 1 |
| 25: object expressions | 85 | 39 | 1 |
| 26: destructuring lambda parameters | 90 | 34 | 1 |
| 42-49: coroutines | 93 | 31 | 1 |
| 52: bitwise operators | 104 | 20 | 1 |
| 53: typealias | 104 | 20 | 1 |
| 54: setters, backing fields | 104 | 20 | 1 |
| 55: lateinit, tailrec | 108 | 16 | 1 |
| 56-57: collection supertypes | 108 | 16 | 1 |
| 58: member extension functions | 108 | 16 | 1 |
| 59: reified member type parameters | 108 | 16 | 1 |
| 60: inner and local classes | 110 | 14 | 1 |
| 61: when guards, anonymous functions, local extension functions | 124 | 0 | – |
| 62a: @JvmField and @JvmStatic in objects | 124 | 0 | – |
| 62b: generic value classes | 124 | 0 | – |
| 63: definitely non-nullable types | 124 | 0 | – |
| 70a: inline property accessors | 124 | 0 | – |
| 70b: where clauses | 124 | 0 | – |
| 70c: fun interfaces | 124 | 0 | – |
| delegated extension properties | 124 | 0 | – |
| qualified super | 124 | 0 | – |
| context parameters of top-level functions | 124 | 0 | – |
| also supported | 124 | 0 | – |

### Features left, by files blocked

124 of 124 files use no feature that kc doesn't support.

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
| `insert` | 68 | 400 |
| `List` | 17 | 27 |
| `let` | 16 | 52 |
| `apply` | 12 | 33 |
| `error` | 11 | 19 |
| `find` | 10 | 39 |
| `with` | 10 | 32 |
| `also` | 8 | 19 |
| `assert` | 8 | 15 |
| `orEmpty` | 7 | 31 |
| `isNotEmpty` | 6 | 14 |
| `toBigDecimal` | 6 | 15 |
| `check` | 4 | 10 |
| `run` | 4 | 4 |
| `sumOf` | 3 | 3 |
| `takeIf` | 3 | 5 |
| `buildString` | 2 | 2 |
| `isNullOrEmpty` | 2 | 2 |
| `requireNotNull` | 2 | 2 |
| `Sequence` | 1 | 1 |
| `bufferedReader` | 1 | 1 |
| `getOrElse` | 1 | 1 |
| `isNotBlank` | 1 | 1 |
| `onFailure` | 1 | 1 |
| `suspend` | 1 | 1 |

### Files

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| exposed-tests/src/main/kotlin/org/jetbrains/exposed/v1/tests/DatabaseTestsBase.kt | 221 | (annotation, arrays, default value, delegated property, function type with receiver, inheritance, is, modifier abstract, modifier companion, modifier lateinit, modifier override, modifier protected, modifier vararg, named argument, object, package, property without initializer, spread operator, try) |
| exposed-tests/src/main/kotlin/org/jetbrains/exposed/v1/tests/LogDbInTestName.kt | 17 | (annotation, arrays, modifier abstract, modifier companion, modifier lateinit, package, property without initializer) |
| exposed-tests/src/main/kotlin/org/jetbrains/exposed/v1/tests/TestDB.kt | 161 | (annotation, as, class literal, default value, function type with receiver, inline-only call, modifier companion, modifier enum, named argument, package, try) |
| exposed-tests/src/main/kotlin/org/jetbrains/exposed/v1/tests/TestUtils.kt | 58 | (annotation, as, bitwise operators, class literal, extension function, inline-only call, modifier const, modifier inline, modifier reified, modifier vararg, package, property getter) |
| exposed-tests/src/main/kotlin/org/jetbrains/exposed/v1/tests/shared/Assert.kt | 68 | (extension function, extension property, inline-only call, modifier inline, modifier reified, modifier vararg, named argument, package) |
| exposed-tests/src/main/kotlin/org/jetbrains/exposed/v1/tests/shared/ForeignKeyTables.kt | 27 | (inheritance, modifier const, modifier override, named argument, object, package) |
| exposed-tests/src/main/kotlin/org/jetbrains/exposed/v1/tests/shared/MiscTable.kt | 125 | (Byte and Short, Float, annotation, class literal, delegated property, extension function, inheritance, modifier enum, modifier open, nested class, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/ResultRowCacheTest.kt | 209 | (annotation, arrays, as, class literal, identity equality, inheritance, modifier override, nested class, object, package, require, check, error) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/VersionTests.kt | 64 | (annotation, class literal, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/demo/dao/SamplesDao.kt | 90 | (annotation, delegated property, inheritance, inline-only call, modifier companion, named argument, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/demo/sql/SamplesSQL.kt | 141 | (annotation, bitwise operators, inheritance, inline-only call, modifier override, named argument, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/h2/ConnectionPoolTests.kt | 75 | (annotation, delegated property, inheritance, inline-only call, modifier companion, named argument, nested class, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/h2/DatabaseMetadataConcurrencyTest.kt | 104 | (annotation, inheritance, modifier companion, modifier const, named argument, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/h2/EntityReferenceCacheTest.kt | 546 | (annotation, callable reference, delegated property, inheritance, inline-only call, modifier companion, modifier vararg, named argument, nested class, object, package, spread operator, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/h2/H2Tests.kt | 155 | (annotation, class delegation, class literal, inheritance, inline-only call, is, modifier override, nested class, object, object expression, package, require, check, error, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/h2/MultiDatabaseEntityTest.kt | 224 | (annotation, class literal, delegated property, inline-only call, named argument, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/h2/MultiDatabaseTest.kt | 303 | (annotation, class literal, delegated property, inline-only call, named argument, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/mysql/MysqlTests.kt | 182 | (annotation, arrays, as, bitwise operators, extension function, inheritance, inline-only call, local extension function, local function, modifier override, named argument, nested class, object expression, package, super) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/oracle/OracleTests.kt | 59 | (annotation, inheritance, inline-only call, modifier override, named argument, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/postgresql/ConnectionPoolTests.kt | 202 | (annotation, delegated property, extension function, inheritance, inline-only call, named argument, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/postgresql/PostgresqlTests.kt | 113 | (annotation, arrays, extension function, inheritance, inline-only call, local extension function, local function, modifier override, named argument, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/redshift/RedshiftRegistrationTests.kt | 61 | (annotation, arrays, as, class literal, package, require, check, error) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/redshift/RedshiftTests.kt | 431 | (annotation, arrays, function type with receiver, inheritance, inline-only call, modifier override, named argument, object expression, package, require, check, error, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/AliasesTests.kt | 411 | (annotation, arrays, as, bitwise operators, extension function, inheritance, inline-only call, local extension function, local function, named argument, object expression, package, spread operator) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ConnectionExceptions.kt | 181 | (annotation, callable reference, class delegation, inheritance, modifier abstract, modifier override, named argument, nested class, package, super, throw, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ConnectionTests.kt | 203 | (Char, annotation, as, inheritance, inline-only call, modifier suspend, named argument, object, object expression, package, require, check, error, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ConnectionTimeoutTest.kt | 85 | (annotation, class literal, inheritance, is, modifier override, named argument, nested class, package, throw, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/CoroutineTests.kt | 353 | (annotation, destructuring declaration, inheritance, inline-only call, is, local function, modifier companion, modifier suspend, named argument, nested class, object, package, throw, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/CustomTransactionManagerTest.kt | 219 | (annotation, class literal, default value, inheritance, inline-only call, modifier lateinit, modifier override, named argument, nested class, object expression, package, property getter, throw, try, uninitialized local variable) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/DDLTests.kt | 1454 | (annotation, arrays, as, bitwise operators, callable reference, class literal, default value, delegated property, destructuring declaration, extension function, inheritance, inline-only call, is, local class, local extension function, local function, modifier abstract, modifier override, named argument, nested class, object, object expression, package, require, check, error, unsigned integers) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/DataSourceStub.kt | 41 | (inheritance, modifier open, modifier override, package, throw) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/IdentifierManagerConcurrencyTest.kt | 78 | (annotation, inheritance, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/NestedTransactionsTest.kt | 209 | (annotation, delegated property, inheritance, inline-only call, is, named argument, package, require, check, error, throw, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ParameterizationTests.kt | 173 | (annotation, class literal, delegated property, destructuring declaration, inheritance, named argument, object, object expression, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/QueryTimeoutTest.kt | 164 | (annotation, inheritance, is, named argument, package, throw, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/RollbackTransactionTest.kt | 124 | (annotation, inheritance, inline-only call, named argument, package, require, check, error, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/SchemaTests.kt | 242 | (annotation, bitwise operators, inheritance, inline-only call, is, named argument, object expression, package, require, check, error, try, unsigned integers) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/StatementInterceptorTests.kt | 341 | (annotation, as, inheritance, inline-only call, is, modifier companion, modifier const, modifier data, modifier override, named argument, nested class, object, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/SuspendTransactionTests.kt | 76 | (annotation, inheritance, inline-only call, modifier suspend, named argument, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ThreadLocalManagerTest.kt | 105 | (annotation, inheritance, inline-only call, modifier lateinit, named argument, object, package, uninitialized local variable) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/TransactionCoroutineContextTests.kt | 58 | (annotation, inheritance, inline-only call, modifier override, object, object expression, package, throw) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/TransactionExecTests.kt | 281 | (annotation, as, class literal, destructuring declaration, extension function, inheritance, inline-only call, modifier override, named argument, object, object expression, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/TransactionIsolationTest.kt | 210 | (annotation, destructuring declaration, extension function, inheritance, inline-only call, named argument, package, throw) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/TransactionStackCorruptionTest.kt | 253 | (annotation, class literal, inheritance, inline-only call, modifier override, named argument, nested class, object, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/UserCreatedTransactionsTests.kt | 187 | (annotation, class literal, inheritance, inline-only call, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/ColumnCommentTests.kt | 161 | (annotation, as, inheritance, inline-only call, is, modifier override, named argument, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/ColumnDefinitionTests.kt | 210 | (annotation, as, destructuring declaration, extension function, inheritance, inline-only call, local class, local extension function, local function, modifier override, named argument, object expression, package, super) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/CreateDatabaseTest.kt | 86 | (annotation, inheritance, named argument, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/CreateIndexTests.kt | 372 | (annotation, arrays, bitwise operators, extension function, inheritance, inline-only call, is, modifier override, modifier value, named argument, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/CreateMissingTablesAndColumnsTests.kt | 887 | (Float, annotation, arrays, class literal, delegated property, destructuring lambda parameter, inheritance, inline-only call, is, modifier enum, modifier override, named argument, nested class, object, object expression, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/CreateTableTests.kt | 802 | (annotation, class literal, extension function, inheritance, inline-only call, is, modifier override, named argument, object, object expression, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/EnumerationTests.kt | 241 | (annotation, as, class literal, delegated property, inheritance, inline-only call, is, local class, modifier enum, modifier override, nested class, object, object expression, package, require, check, error, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/ReplaceColumnTests.kt | 27 | (annotation, as, inheritance, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/SequencesTests.kt | 452 | (annotation, arrays, class literal, delegated property, inheritance, inline-only call, modifier companion, modifier override, named argument, nested class, object, object expression, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/WhereConditionsTests.kt | 51 | (annotation, inheritance, inline-only call, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/AdjustQueryTests.kt | 211 | (annotation, bitwise operators, extension function, inheritance, inline-only call, local extension function, local function, package, require, check, error) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ArithmeticTests.kt | 46 | (annotation, inheritance, named argument, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ColumnCaseSensitivityTests.kt | 111 | (annotation, inheritance, inline-only call, is, modifier override, named argument, object, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ColumnWithTransformTest.kt | 448 | (annotation, as, delegated property, extension function, inheritance, inline-only call, local class, local extension function, local function, modifier companion, modifier data, modifier override, named argument, nested class, object, object expression, package, super) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ConditionsTests.kt | 581 | (annotation, inheritance, inline-only call, local class, local function, named argument, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/CountTests.kt | 103 | (annotation, inheritance, inline-only call, is, named argument, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/DMLTestData.kt | 224 | (annotation, bitwise operators, default value, delegated property, extension function, function type with receiver, inheritance, inline-only call, modifier companion, modifier const, modifier override, named argument, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/DeleteTests.kt | 178 | (annotation, inheritance, inline-only call, is, named argument, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/DistinctOnTests.kt | 126 | (annotation, arrays, inheritance, inline-only call, named argument, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/DualTableTest.kt | 20 | (annotation, inheritance, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ExistsTests.kt | 96 | (annotation, bitwise operators, inheritance, is, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ExplainTests.kt | 198 | (Char, annotation, bitwise operators, extension function, function type with receiver, inheritance, inline-only call, is, local extension function, local function, named argument, object, package, throw) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/FetchBatchedResultsTests.kt | 144 | (annotation, class literal, inheritance, inline-only call, named argument, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/GroupByTests.kt | 297 | (annotation, extension function, inheritance, inline-only call, is, local extension function, local function, modifier vararg, named argument, object expression, package, require, check, error, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/InsertSelectTests.kt | 77 | (annotation, as, inheritance, inline-only call, named argument, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/InsertTests.kt | 1030 | (annotation, as, class literal, delegated property, destructuring lambda parameter, extension function, inheritance, inline-only call, is, local extension function, local function, modifier companion, modifier override, named argument, nested class, object, object expression, package, qualified this, require, check, error, super, try, when subject variable) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/JoinTests.kt | 230 | (annotation, bitwise operators, inheritance, inline-only call, modifier override, object expression, package, require, check, error) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/LateralJoinTests.kt | 114 | (annotation, function type with receiver, inheritance, inline-only call, named argument, object, package, require, check, error) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/LikeTests.kt | 73 | (Char, annotation, as, inheritance, inline-only call, modifier override, named argument, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/MergeBaseTest.kt | 91 | (default value, function type with receiver, inheritance, inline-only call, modifier abstract, modifier protected, named argument, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/MergeSelectTest.kt | 254 | (annotation, bitwise operators, inheritance, named argument, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/MergeTableTest.kt | 467 | (annotation, bitwise operators, inheritance, inline-only call, modifier override, named argument, object expression, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/OrderByTests.kt | 209 | (annotation, destructuring lambda parameter, inheritance, inline-only call, is, local function, modifier override, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ReplaceTests.kt | 252 | (annotation, arrays, destructuring declaration, destructuring lambda parameter, inheritance, inline-only call, modifier override, named argument, object, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ReturningTests.kt | 238 | (annotation, delegated property, destructuring lambda parameter, inheritance, inline-only call, modifier companion, named argument, nested class, object, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/SelectTests.kt | 673 | (Char, annotation, arrays, bitwise operators, extension function, inheritance, inline-only call, local extension function, local function, named argument, object expression, package, require, check, error) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/UnionTests.kt | 274 | (annotation, inheritance, inline-only call, is, named argument, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/UpdateTests.kt | 242 | (annotation, inheritance, inline-only call, named argument, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/UpsertTests.kt | 1010 | (Byte and Short, Char, Float, annotation, arrays, bitwise operators, delegated property, destructuring declaration, destructuring lambda parameter, extension function, inheritance, inline-only call, local class, local extension function, local function, modifier override, named argument, object, object expression, package, unsigned integers) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/CompositeIdTableEntityTest.kt | 822 | (annotation, arrays, as, bitwise operators, callable reference, delegated property, destructuring lambda parameter, extension function, inheritance, inline-only call, modifier companion, modifier override, named argument, nested class, object, object expression, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/EntityBugsRegressionTest.kt | 165 | (annotation, default value, delegated property, inheritance, inline-only call, modifier abstract, modifier companion, modifier final, modifier open, modifier override, modifier value, nested class, object, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/EntityCacheRefreshTests.kt | 221 | (annotation, delegated property, inheritance, inline-only call, modifier companion, modifier value, named argument, nested class, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/EntityCacheTests.kt | 383 | (annotation, delegated property, inheritance, inline-only call, modifier companion, modifier override, modifier value, named argument, nested class, object, package, require, check, error, throw, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/EntityFieldWithTransformTest.kt | 175 | (annotation, delegated property, inheritance, inline-only call, modifier companion, modifier value, named argument, nested class, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/EntityHookTest.kt | 366 | (annotation, arrays, delegated property, destructuring declaration, function type with receiver, inheritance, inline-only call, modifier companion, named argument, nested class, object, package, spread operator) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/EntityTests.kt | 1785 | (Char, annotation, arrays, as, callable reference, delegated property, destructuring declaration, destructuring lambda parameter, extension function, function type with receiver, inheritance, inline-only call, is, local extension function, local function, modifier abstract, modifier companion, modifier enum, modifier open, modifier override, modifier value, named argument, nested class, object, object expression, package, spread operator, unsigned integers) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/EntityWithBlobTests.kt | 58 | (annotation, arrays, delegated property, inheritance, modifier companion, modifier override, nested class, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/ForeignIdEntityTest.kt | 99 | (annotation, delegated property, inheritance, modifier companion, modifier override, named argument, nested class, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/ImmutableEntityTest.kt | 246 | (annotation, class literal, delegated property, inheritance, inline-only call, modifier companion, modifier override, named argument, nested class, object, package, throw, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/JavaUUIDTableEntityTest.kt | 198 | (annotation, callable reference, delegated property, inheritance, inline-only call, modifier companion, named argument, nested class, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/LongIdTableEntityTest.kt | 150 | (annotation, callable reference, delegated property, inheritance, inline-only call, modifier companion, nested class, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/NonAutoIncEntities.kt | 135 | (annotation, delegated property, function type with receiver, inheritance, inline-only call, modifier abstract, modifier companion, modifier const, modifier override, nested class, object, package, super) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/OrderedReferenceTest.kt | 247 | (annotation, callable reference, delegated property, destructuring lambda parameter, function type with receiver, inheritance, inline-only call, local function, modifier companion, modifier override, modifier value, nested class, object, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/SelfReferenceTest.kt | 108 | (annotation, inheritance, modifier override, named argument, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/UIntIdTableEntityTest.kt | 151 | (annotation, callable reference, delegated property, inheritance, inline-only call, modifier companion, nested class, object, package, unsigned integers) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/ULongIdTableEntityTest.kt | 151 | (annotation, callable reference, delegated property, inheritance, inline-only call, modifier companion, nested class, object, package, unsigned integers) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/UuidTableEntityTest.kt | 246 | (annotation, callable reference, delegated property, inheritance, inline-only call, modifier companion, named argument, nested class, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/ViaTest.kt | 384 | (annotation, arrays, as, callable reference, delegated property, extension function, inheritance, inline-only call, interface, local function, modifier companion, modifier override, named argument, nested class, object, package, property without initializer, spread operator) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/WarmUpLinkedReferencesTests.kt | 64 | (annotation, delegated property, inheritance, modifier companion, modifier value, nested class, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/functions/FunctionsTestBase.kt | 33 | (default value, extension function, function type with receiver, inheritance, inline-only call, is, modifier abstract, modifier protected, named argument, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/functions/FunctionsTests.kt | 650 | (annotation, as, bitwise operators, extension function, inheritance, inline-only call, is, local extension function, local function, modifier infix, named argument, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/functions/MathFunctionTests.kt | 199 | (annotation, inheritance, named argument, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/functions/StatisticsFunctionTests.kt | 114 | (annotation, default value, extension function, function type with receiver, inheritance, inline-only call, named argument, object, package, throw) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/functions/TrigonometricalFunctionTests.kt | 121 | (annotation, inheritance, named argument, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/functions/WindowFunctionsTests.kt | 422 | (annotation, extension function, inheritance, inline-only call, modifier vararg, named argument, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/ArrayColumnTypeTests.kt | 404 | (annotation, arrays, bitwise operators, default value, delegated property, extension function, function type with receiver, inheritance, inline-only call, is, modifier companion, modifier vararg, named argument, nested class, object, object expression, package, spread operator, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/BinaryColumnTypeTests.kt | 44 | (annotation, arrays, inheritance, inline-only call, modifier override, named argument, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/BlobColumnTypeTests.kt | 163 | (annotation, arrays, inheritance, inline-only call, is, named argument, object, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/BooleanColumnTypeTests.kt | 106 | (Char, annotation, default value, extension function, inheritance, inline-only call, modifier override, nested class, object, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/CharColumnType.kt | 60 | (Char, annotation, inheritance, named argument, object, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/DoubleColumnTypeTests.kt | 52 | (annotation, inheritance, inline-only call, object, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/JavaUUIDColumnTypeTests.kt | 73 | (annotation, inheritance, inline-only call, named argument, object expression, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/MultiArrayColumnTypeTests.kt | 335 | (annotation, default value, delegated property, extension function, inheritance, inline-only call, modifier companion, modifier inline, modifier reified, named argument, nested class, object, object expression, package) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/NumericColumnTypesTests.kt | 333 | (Byte and Short, Float, annotation, class literal, inheritance, inline-only call, is, named argument, object expression, package, unsigned integers) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/UnsignedColumnTypeTests.kt | 368 | (Byte and Short, Char, annotation, class literal, destructuring declaration, inheritance, inline-only call, is, named argument, object, object expression, package, try, unsigned integers) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/UuidColumnTypeTests.kt | 119 | (annotation, destructuring declaration, inheritance, inline-only call, named argument, object expression, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/VectorColumnTypeTests.kt | 500 | (Float, annotation, arrays, default value, delegated property, function type with receiver, inheritance, inline-only call, modifier companion, modifier override, named argument, nested class, object, object expression, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/sqlite/ForeignKeyConstraintTests.kt | 299 | (annotation, extension function, inheritance, inline-only call, modifier override, named argument, object expression, package, try) |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/sqlite/MultipleDatabaseBugTest.kt | 74 | (annotation, package) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

