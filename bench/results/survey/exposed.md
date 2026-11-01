# Survey: exposed 1.5.1

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 169 files, 32485 lines

Directories: `exposed-core/src/main`, `exposed-dao/src/main`, `exposed-jdbc/src/main`

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

`require`, `check`, `requireNotNull`, `checkNotNull`, `error` and `TODO` are built in; kc inlines the others, also those with reified type parameters.

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

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/AbstractQuery.kt | 294 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Alias.kt | 329 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Annotations.kt | 44 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Column.kt | 235 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/ColumnDiff.kt | 54 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/ColumnType.kt | 1812 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/CompositeColumn.kt | 68 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Constraints.kt | 349 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/DatabaseApi.kt | 71 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/DatabaseConfig.kt | 249 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Exceptions.kt | 79 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/ExplainQuery.kt | 49 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Expression.kt | 187 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Function.kt | 746 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/FunctionBuilder.kt | 274 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/LiteralOp.kt | 102 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Op.kt | 649 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/OpBuilder.kt | 917 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/QueryParameter.kt | 121 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/ResultRow.kt | 330 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/SQLExpressionBuilder.kt | 1194 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/SQLLog.kt | 67 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Schema.kt | 69 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/SchemaUtilityApi.kt | 658 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Sequence.kt | 82 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/SortOrder.kt | 34 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Table.kt | 2245 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/TableDepthGraph.kt | 63 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Transaction.kt | 117 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/Version.kt | 60 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/WindowFunction.kt | 468 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/WindowFunctionBuilder.kt | 61 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/dao/id/CompositeID.kt | 62 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/dao/id/EntityID.kt | 44 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/dao/id/IdTable.kt | 192 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/dao/id/java/UUIDTable.kt | 22 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/datetime/DurationColumnType.kt | 40 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/datetime/InstantColumnType.kt | 143 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/datetime/LocalDateColumnType.kt | 110 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/datetime/LocalDateTimeColumnType.kt | 180 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/datetime/LocalTimeColumnType.kt | 122 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/datetime/OffsetDateTimeColumnType.kt | 146 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/functions/array/ArrayFunctions.kt | 51 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/functions/math/MathFunctions.kt | 98 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/functions/math/TrigonometricalFunctions.kt | 96 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/functions/vector/VectorFunctions.kt | 46 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/java/UUIDColumnType.kt | 35 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/ops/AllAnyOps.kt | 83 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/ops/InListOps.kt | 186 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/ops/InTableOp.kt | 31 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/BatchInsertStatement.kt | 127 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/BatchReplaceStatement.kt | 31 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/BatchUpdateStatement.kt | 67 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/BatchUpsertStatement.kt | 51 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/DeleteStatement.kt | 54 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/InsertSelectStatement.kt | 33 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/InsertStatement.kt | 136 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/MergeStatement.kt | 162 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/MultiRowValuesInsertStatement.kt | 61 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/ReplaceStatement.kt | 50 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/ReturningStatement.kt | 27 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/SQLServerBatchInsertStatement.kt | 61 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/Statement.kt | 149 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/StatementBuilder.kt | 474 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/StatementInterceptor.kt | 64 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/UpdateBuilder.kt | 120 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/UpdateStatement.kt | 88 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/UpsertStatement.kt | 140 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/api/ExposedBlob.kt | 46 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/api/ExposedDatabaseMetadata.kt | 122 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/api/ExposedMetadataUtils.kt | 142 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/api/ExposedSavepoint.kt | 8 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/api/IdentifierManagerApi.kt | 198 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/api/PreparedStatementApi.kt | 40 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/statements/api/ResultApi.kt | 103 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/DatabasesManager.kt | 66 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/ThreadLocalTransactionsStack.kt | 116 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/TransactionInterface.kt | 19 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/TransactionManagerApi.kt | 19 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/TransactionManagersContainer.kt | 102 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/TransactionScope.kt | 53 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/Transactions.kt | 57 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/TransactionsHolder.kt | 99 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/suspend/TransactionContextElement.kt | 83 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/transactions/suspend/TransactionContextHolder.kt | 22 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/utils/CachableMapWithDefault.kt | 16 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/utils/CachableMapWithSuspendableDefault.kt | 19 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/ColumnMetadata.kt | 29 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/DataTypeProvider.kt | 166 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/DatabaseDialect.kt | 212 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/ForUpdateOption.kt | 135 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/FunctionProvider.kt | 1038 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/H2.kt | 496 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/Keywords.kt | 506 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/MariaDBDialect.kt | 169 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/MysqlDialect.kt | 453 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/OracleDialect.kt | 574 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/PostgreSQL.kt | 537 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/PrimaryKeyMetadata.kt | 11 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/Redshift.kt | 364 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/SQLServerDialect.kt | 507 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/SQLiteDialect.kt | 393 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/SchemaMetadata.kt | 11 |  |
| exposed-core/src/main/kotlin/org/jetbrains/exposed/v1/core/vendors/VendorDialect.kt | 159 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/CompositeEntity.kt | 30 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/DaoEntityID.kt | 15 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/DaoEntityIDFactory.kt | 15 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/Entity.kt | 486 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/EntityBatchUpdate.kt | 62 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/EntityCache.kt | 355 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/EntityClass.kt | 1214 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/EntityHook.kt | 130 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/EntityLifecycleInterceptor.kt | 143 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/InnerTableLink.kt | 142 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/IntEntity.kt | 29 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/LinkedIdentityHashSet.kt | 75 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/LongEntity.kt | 29 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/References.kt | 432 | safe-call assignment |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/UIntEntity.kt | 29 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/ULongEntity.kt | 29 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/UuidEntity.kt | 31 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/View.kt | 29 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/exceptions/EntityNotFoundException.kt | 11 |  |
| exposed-dao/src/main/kotlin/org/jetbrains/exposed/v1/dao/java/UUIDEntity.kt | 30 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/Database.kt | 303 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/ExplainBlockingExecutable.kt | 63 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/ExposedConnectionImpl.kt | 11 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/IterableEx.kt | 186 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/JdbcTransaction.kt | 353 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/MetadataQueries.kt | 23 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/Queries.kt | 851 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/Query.kt | 377 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/SchemaUtils.kt | 529 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/SetOperations.kt | 312 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/BatchInsertBlockingExecutable.kt | 85 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/BatchUpdateBlockingExecutable.kt | 16 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/BatchUpsertBlockingExecutable.kt | 23 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/BlockingExecutable.kt | 167 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/DeleteBlockingExecutable.kt | 16 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/InsertBlockingExecutable.kt | 278 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/InsertSelectBlockingExecutable.kt | 14 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/MergeBlockingExecutable.kt | 17 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/ReturningBlockingExecutable.kt | 43 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/StatementIterator.kt | 32 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/UpdateBlockingExecutable.kt | 19 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/UpsertBlockingExecutable.kt | 23 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/api/ExposedConnection.kt | 69 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/api/JdbcExposedDatabaseMetadata.kt | 107 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/api/JdbcPreparedStatementApi.kt | 59 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/jdbc/JdbcConnectionImpl.kt | 169 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/jdbc/JdbcDatabaseMetadataImpl.kt | 779 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/jdbc/JdbcIdentifierManager.kt | 29 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/jdbc/JdbcPreparedStatementImpl.kt | 113 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/jdbc/JdbcResult.kt | 60 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/statements/jdbc/JdbcSavepoint.kt | 9 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/transactions/JdbcTransactionInterface.kt | 63 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/transactions/JdbcTransactionManager.kt | 95 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/transactions/TransactionManager.kt | 297 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/transactions/Transactions.kt | 464 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/transactions/experimental/Suspended.kt | 77 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/DatabaseDialectMetadata.kt | 201 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/H2DialectMetadata.kt | 15 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/MariaDBDialectMetadata.kt | 6 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/MysqlDialectMetadata.kt | 22 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/OracleDialectMetadata.kt | 6 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/PostgreSQLDialectMetadata.kt | 13 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/RedshiftDialectMetadata.kt | 8 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/SQLServerDialectMetadata.kt | 6 |  |
| exposed-jdbc/src/main/kotlin/org/jetbrains/exposed/v1/jdbc/vendors/SQLiteDialectMetadata.kt | 12 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 124 files, 31013 lines

Directories: `exposed-tests/src`

### Features left, by files blocked

124 of 124 files use no feature that kc doesn't support.

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

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| exposed-tests/src/main/kotlin/org/jetbrains/exposed/v1/tests/DatabaseTestsBase.kt | 221 |  |
| exposed-tests/src/main/kotlin/org/jetbrains/exposed/v1/tests/LogDbInTestName.kt | 17 |  |
| exposed-tests/src/main/kotlin/org/jetbrains/exposed/v1/tests/TestDB.kt | 161 |  |
| exposed-tests/src/main/kotlin/org/jetbrains/exposed/v1/tests/TestUtils.kt | 58 |  |
| exposed-tests/src/main/kotlin/org/jetbrains/exposed/v1/tests/shared/Assert.kt | 68 |  |
| exposed-tests/src/main/kotlin/org/jetbrains/exposed/v1/tests/shared/ForeignKeyTables.kt | 27 |  |
| exposed-tests/src/main/kotlin/org/jetbrains/exposed/v1/tests/shared/MiscTable.kt | 125 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/ResultRowCacheTest.kt | 209 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/VersionTests.kt | 64 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/demo/dao/SamplesDao.kt | 90 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/demo/sql/SamplesSQL.kt | 141 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/h2/ConnectionPoolTests.kt | 75 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/h2/DatabaseMetadataConcurrencyTest.kt | 104 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/h2/EntityReferenceCacheTest.kt | 546 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/h2/H2Tests.kt | 155 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/h2/MultiDatabaseEntityTest.kt | 224 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/h2/MultiDatabaseTest.kt | 303 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/mysql/MysqlTests.kt | 182 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/oracle/OracleTests.kt | 59 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/postgresql/ConnectionPoolTests.kt | 202 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/postgresql/PostgresqlTests.kt | 113 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/redshift/RedshiftRegistrationTests.kt | 61 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/redshift/RedshiftTests.kt | 431 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/AliasesTests.kt | 411 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ConnectionExceptions.kt | 181 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ConnectionTests.kt | 203 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ConnectionTimeoutTest.kt | 85 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/CoroutineTests.kt | 353 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/CustomTransactionManagerTest.kt | 219 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/DDLTests.kt | 1454 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/DataSourceStub.kt | 41 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/IdentifierManagerConcurrencyTest.kt | 78 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/NestedTransactionsTest.kt | 209 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ParameterizationTests.kt | 173 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/QueryTimeoutTest.kt | 164 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/RollbackTransactionTest.kt | 124 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/SchemaTests.kt | 242 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/StatementInterceptorTests.kt | 341 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/SuspendTransactionTests.kt | 76 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ThreadLocalManagerTest.kt | 105 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/TransactionCoroutineContextTests.kt | 58 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/TransactionExecTests.kt | 281 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/TransactionIsolationTest.kt | 210 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/TransactionStackCorruptionTest.kt | 253 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/UserCreatedTransactionsTests.kt | 187 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/ColumnCommentTests.kt | 161 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/ColumnDefinitionTests.kt | 210 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/CreateDatabaseTest.kt | 86 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/CreateIndexTests.kt | 372 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/CreateMissingTablesAndColumnsTests.kt | 887 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/CreateTableTests.kt | 802 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/EnumerationTests.kt | 241 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/ReplaceColumnTests.kt | 27 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/SequencesTests.kt | 452 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/ddl/WhereConditionsTests.kt | 51 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/AdjustQueryTests.kt | 211 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ArithmeticTests.kt | 46 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ColumnCaseSensitivityTests.kt | 111 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ColumnWithTransformTest.kt | 448 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ConditionsTests.kt | 581 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/CountTests.kt | 103 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/DMLTestData.kt | 224 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/DeleteTests.kt | 178 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/DistinctOnTests.kt | 126 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/DualTableTest.kt | 20 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ExistsTests.kt | 96 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ExplainTests.kt | 198 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/FetchBatchedResultsTests.kt | 144 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/GroupByTests.kt | 297 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/InsertSelectTests.kt | 77 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/InsertTests.kt | 1030 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/JoinTests.kt | 230 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/LateralJoinTests.kt | 114 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/LikeTests.kt | 73 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/MergeBaseTest.kt | 91 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/MergeSelectTest.kt | 254 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/MergeTableTest.kt | 467 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/OrderByTests.kt | 209 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ReplaceTests.kt | 252 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/ReturningTests.kt | 238 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/SelectTests.kt | 673 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/UnionTests.kt | 274 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/UpdateTests.kt | 242 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/dml/UpsertTests.kt | 1010 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/CompositeIdTableEntityTest.kt | 822 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/EntityBugsRegressionTest.kt | 165 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/EntityCacheRefreshTests.kt | 221 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/EntityCacheTests.kt | 383 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/EntityFieldWithTransformTest.kt | 175 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/EntityHookTest.kt | 366 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/EntityTests.kt | 1785 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/EntityWithBlobTests.kt | 58 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/ForeignIdEntityTest.kt | 99 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/ImmutableEntityTest.kt | 246 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/JavaUUIDTableEntityTest.kt | 198 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/LongIdTableEntityTest.kt | 150 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/NonAutoIncEntities.kt | 135 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/OrderedReferenceTest.kt | 247 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/SelfReferenceTest.kt | 108 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/UIntIdTableEntityTest.kt | 151 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/ULongIdTableEntityTest.kt | 151 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/UuidTableEntityTest.kt | 246 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/ViaTest.kt | 384 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/entities/WarmUpLinkedReferencesTests.kt | 64 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/functions/FunctionsTestBase.kt | 33 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/functions/FunctionsTests.kt | 650 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/functions/MathFunctionTests.kt | 199 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/functions/StatisticsFunctionTests.kt | 114 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/functions/TrigonometricalFunctionTests.kt | 121 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/functions/WindowFunctionsTests.kt | 422 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/ArrayColumnTypeTests.kt | 404 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/BinaryColumnTypeTests.kt | 44 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/BlobColumnTypeTests.kt | 163 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/BooleanColumnTypeTests.kt | 106 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/CharColumnType.kt | 60 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/DoubleColumnTypeTests.kt | 52 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/JavaUUIDColumnTypeTests.kt | 73 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/MultiArrayColumnTypeTests.kt | 335 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/NumericColumnTypesTests.kt | 333 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/UnsignedColumnTypeTests.kt | 368 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/UuidColumnTypeTests.kt | 119 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/shared/types/VectorColumnTypeTests.kt | 500 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/sqlite/ForeignKeyConstraintTests.kt | 299 |  |
| exposed-tests/src/test/kotlin/org/jetbrains/exposed/v1/tests/sqlite/MultipleDatabaseBugTest.kt | 74 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

