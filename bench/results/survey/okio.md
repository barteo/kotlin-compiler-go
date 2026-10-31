# Survey: okio parent-3.18.2

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 96 files, 17346 lines

Directories: `okio/src/commonMain`, `okio/src/zlibMain`, `okio/src/systemFileSystemMain`, `okio/src/nonJsMain`, `okio/src/jvmMain`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 96 | 9 |
| 1: packages, annotations | 1 | 95 | 7 |
| 2: interfaces, inheritance | 3 | 93 | 5 |
| 3: type checks | 3 | 93 | 5 |
| 4: exceptions | 3 | 93 | 4 |
| 5: objects | 3 | 93 | 4 |
| 6: data, enum, sealed | 5 | 91 | 3 |
| 7: arrays, vararg, spread | 8 | 88 | 3 |
| 8: Char, Float, Byte, Short | 8 | 88 | 3 |
| 9: inliner | 9 | 87 | 2 |
| 10: lambdas with receivers | 9 | 87 | 2 |
| 11: extensions | 12 | 84 | 2 |
| 12: operator conventions | 12 | 84 | 2 |
| 13: identity, class literals, references | 25 | 71 | 2 |
| 14: class delegation | 25 | 71 | 2 |
| 15: secondary constructors | 25 | 71 | 2 |
| 16: local functions | 25 | 71 | 1 |
| 17: expect and actual | 53 | 43 | 2 |
| 18: annotation classes | 54 | 42 | 2 |
| 19: value classes | 54 | 42 | 2 |
| 20: unsigned integers | 54 | 42 | 2 |
| definite assignment | 55 | 41 | 2 |
| 21: inline functions | 66 | 30 | 1 |
| 22: reified type parameters | 66 | 30 | 1 |
| 23: labels, do-while | 66 | 30 | 1 |
| 24: delegated properties | 67 | 29 | 1 |
| 25: object expressions | 68 | 28 | 1 |
| 26: destructuring lambda parameters | 68 | 28 | 1 |
| 42-49: coroutines | 69 | 27 | 1 |
| 52: bitwise operators | 80 | 16 | 1 |
| 53: typealias | 82 | 14 | 1 |
| 54: setters, backing fields | 82 | 14 | 1 |
| 55: lateinit, tailrec | 83 | 13 | 1 |
| 56-57: collection supertypes | 85 | 11 | 1 |
| 58: member extension functions | 85 | 11 | 1 |
| 59: reified member type parameters | 85 | 11 | 1 |
| 60: inner and local classes | 86 | 10 | 1 |
| 61: when guards, anonymous functions, local extension functions | 86 | 10 | 1 |
| 62a: @JvmField and @JvmStatic in objects | 91 | 5 | 1 |
| 62b: generic value classes | 91 | 5 | 1 |
| 63: definitely non-nullable types | 91 | 5 | 1 |
| 70a: inline property accessors | 94 | 2 | 2 |
| 70b: where clauses | 94 | 2 | 2 |
| 70c: fun interfaces | 94 | 2 | 2 |
| delegated extension properties | 94 | 2 | 2 |
| qualified super | 94 | 2 | 2 |
| also supported | 95 | 1 | 1 |

### Features left, by files blocked

95 of 96 files use no feature that kc doesn't support.

| Feature | Files | Only blocker | Uses |
|:---|---:|---:|---:|
| labeled expression | 1 | 1 | 1 |

### Greedy order of the features left

At each step, the feature that unblocks the most files given the ones before it, or, if none unblocks a file, the one that blocks the most files.

| Step | Feature | Files unblocked |
|---:|:---|---:|
| 1 | labeled expression | 96 |

### Inline-only functions called

`require`, `check`, `requireNotNull`, `checkNotNull`, `error` and `TODO` are built in; kc inlines the others, since milestone 22 also those with reified type parameters.

| Function | Files | Calls |
|:---|---:|---:|
| `require` | 31 | 100 |
| `check` | 14 | 76 |
| `apply` | 5 | 8 |
| `copyOf` | 5 | 9 |
| `use` | 5 | 12 |
| `run` | 4 | 5 |
| `List` | 3 | 5 |
| `Sequence` | 2 | 2 |
| `also` | 2 | 2 |
| `copyOfRange` | 2 | 3 |
| `inputStream` | 2 | 2 |
| `isNotEmpty` | 2 | 2 |
| `let` | 2 | 5 |
| `MutableList` | 1 | 1 |
| `buildList` | 1 | 3 |
| `checkNotNull` | 1 | 3 |
| `countLeadingZeroBits` | 1 | 1 |
| `outputStream` | 1 | 2 |
| `takeIf` | 1 | 1 |

### Files

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| okio/src/commonMain/kotlin/okio/Base64.kt | 160 | (Char, annotation, arrays, bitwise operators, default value, extension function, inline-only call, package, uninitialized local variable) |
| okio/src/commonMain/kotlin/okio/Buffer.kt | 441 | (Byte and Short, arrays, default value, function without body, inheritance, modifier expect, modifier operator, modifier override, nested class, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/BufferedSink.kt | 327 | (arrays, function without body, inheritance, interface, modifier expect, modifier override, modifier sealed, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/BufferedSource.kt | 604 | (Byte and Short, arrays, function without body, inheritance, interface, modifier expect, modifier sealed, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/ByteString.kt | 231 | (Byte and Short, annotation, arrays, default value, extension function, function without body, inheritance, modifier companion, modifier expect, modifier open, modifier operator, modifier override, modifier vararg, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/CommonPlatform.kt | 55 | (annotation, arrays, class literal, extension function, function without body, inheritance, interface, modifier expect, modifier inline, modifier open, package, secondary constructor) |
| okio/src/commonMain/kotlin/okio/ExperimentalFileSystem.kt | 30 | (annotation, modifier annotation, package) |
| okio/src/commonMain/kotlin/okio/FileHandle.kt | 443 | (annotation, arrays, class literal, default value, function without body, identity equality, inheritance, is, modifier abstract, modifier final, modifier override, modifier protected, named argument, nested class, package, require, check, error) |
| okio/src/commonMain/kotlin/okio/FileMetadata.kt | 156 | (default value, inline-only call, modifier override, named argument, package) |
| okio/src/commonMain/kotlin/okio/FileSystem.kt | 410 | (annotation, class literal, default value, function type with receiver, function without body, inheritance, modifier abstract, modifier companion, modifier expect, modifier inline, modifier open, modifier override, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/ForwardingFileSystem.kt | 251 | (annotation, class literal, inheritance, inline-only call, modifier abstract, modifier open, modifier override, named argument, package) |
| okio/src/commonMain/kotlin/okio/ForwardingSource.kt | 37 | (annotation, class literal, function without body, inheritance, modifier abstract, modifier expect, modifier override, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/HashingSink.kt | 80 | (function without body, inheritance, modifier companion, modifier expect, modifier override, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/HashingSource.kt | 80 | (function without body, inheritance, modifier companion, modifier expect, modifier override, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/Okio.kt | 154 | (annotation, as, default value, extension function, inheritance, inline-only call, modifier inline, modifier override, package, require, check, error, super, throw, try) |
| okio/src/commonMain/kotlin/okio/Options.kt | 237 | (annotation, arrays, bitwise operators, collection supertype, default value, extension property, inheritance, inline-only call, modifier companion, modifier override, modifier vararg, named argument, package, property getter, require, check, error) |
| okio/src/commonMain/kotlin/okio/Path.kt | 309 | (Char, annotation, class literal, default value, extension function, function without body, inheritance, modifier companion, modifier expect, modifier operator, modifier override, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/PeekSource.kt | 73 | (identity equality, inheritance, modifier override, package, require, check, error) |
| okio/src/commonMain/kotlin/okio/RealBufferedSink.kt | 50 | (arrays, function without body, inheritance, modifier expect, modifier override, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/RealBufferedSource.kt | 70 | (Byte and Short, arrays, function without body, inheritance, modifier expect, modifier override, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/Segment.kt | 186 | (annotation, arrays, identity equality, inline-only call, modifier companion, modifier const, named argument, package, property without initializer, require, check, error, secondary constructor, throw, uninitialized local variable) |
| okio/src/commonMain/kotlin/okio/SegmentPool.kt | 36 | (function without body, modifier expect, object, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/SegmentedByteString.kt | 50 | (arrays, inheritance, modifier expect, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/Sink.kt | 63 | (annotation, class literal, function without body, inheritance, interface, modifier expect, modifier override, package) |
| okio/src/commonMain/kotlin/okio/Socket.kt | 55 | (function without body, interface, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/Source.kt | 70 | (annotation, class literal, function without body, inheritance, interface, modifier override, package) |
| okio/src/commonMain/kotlin/okio/Timeout.kt | 44 | (modifier companion, modifier expect, modifier open, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/TypedOptions.kt | 51 | (annotation, arrays, collection supertype, inheritance, modifier companion, modifier inline, modifier override, package, property getter, require, check, error, spread operator) |
| okio/src/commonMain/kotlin/okio/Unsigned.kt | 357 | (annotation, class literal, extension function, modifier inline, package, unsigned integers) |
| okio/src/commonMain/kotlin/okio/Utf8.kt | 558 | (Byte and Short, Char, annotation, arrays, bitwise operators, default value, extension function, modifier const, modifier inline, package, require, check, error) |
| okio/src/commonMain/kotlin/okio/Util.kt | 164 | (Byte and Short, Char, annotation, arrays, bitwise operators, extension function, modifier infix, modifier inline, package, throw) |
| okio/src/commonMain/kotlin/okio/internal/-Utf16.kt | 35 | (bitwise operators, extension property, inline property accessor, modifier inline, package, property without initializer) |
| okio/src/commonMain/kotlin/okio/internal/-Utf8.kt | 59 | (Char, arrays, default value, extension function, inline-only call, package, throw) |
| okio/src/commonMain/kotlin/okio/internal/Buffer.kt | 1710 | labeled expression (Byte and Short, Char, annotation, arrays, as, bitwise operators, default value, do-while, extension function, identity equality, inline-only call, is, labels, modifier const, modifier inline, modifier value, named argument, package, prefix increment, require, check, error, throw, uninitialized local variable) |
| okio/src/commonMain/kotlin/okio/internal/BufferedSource.kt | 31 | (annotation, extension function, modifier inline, package, when subject variable) |
| okio/src/commonMain/kotlin/okio/internal/ByteString.kt | 384 | (Char, annotation, arrays, bitwise operators, extension function, function without body, identity equality, inline-only call, is, labels, modifier expect, modifier inline, named argument, package, require, check, error, throw) |
| okio/src/commonMain/kotlin/okio/internal/FileSystem.kt | 154 | (annotation, class literal, extension function, inline-only call, modifier suspend, named argument, package, qualified this, throw, try) |
| okio/src/commonMain/kotlin/okio/internal/Path.kt | 404 | (Byte and Short, Char, annotation, extension function, extension property, inline-only call, is, modifier inline, named argument, package, property getter, require, check, error, throw, uninitialized local variable, when subject variable) |
| okio/src/commonMain/kotlin/okio/internal/RealBufferedSink.kt | 218 | (annotation, arrays, extension function, modifier inline, package, require, check, error, throw, try) |
| okio/src/commonMain/kotlin/okio/internal/RealBufferedSource.kt | 471 | (Byte and Short, Char, annotation, arrays, bitwise operators, default value, extension function, modifier inline, named argument, package, require, check, error, throw, try) |
| okio/src/commonMain/kotlin/okio/internal/SegmentedByteString.kt | 247 | (Byte and Short, annotation, arrays, bitwise operators, extension function, identity equality, inline-only call, is, modifier inline, named argument, package, require, check, error) |
| okio/src/jvmMain/kotlin/okio/-DeprecatedOkio.kt | 147 | (annotation, modifier vararg, named argument, object, package, spread operator) |
| okio/src/jvmMain/kotlin/okio/-DeprecatedUtf8.kt | 40 | (annotation, named argument, object, package) |
| okio/src/jvmMain/kotlin/okio/-JvmPlatform.kt | 55 | (arrays, extension function, modifier actual, modifier inline, package, typealias) |
| okio/src/jvmMain/kotlin/okio/AsyncTimeout.kt | 513 | (annotation, arrays, bitwise operators, class literal, default value, extension function, identity equality, inheritance, modifier companion, modifier const, modifier inline, modifier open, modifier operator, modifier override, modifier protected, nested class, object expression, package, qualified this, require, check, error, super, throw, try, uninitialized local variable) |
| okio/src/jvmMain/kotlin/okio/Buffer.kt | 715 | (Byte and Short, Char, annotation, arrays, bitwise operators, class literal, default value, identity equality, inheritance, inline-only call, modifier actual, modifier operator, modifier override, named argument, nested class, object expression, package, prefix increment, qualified this, require, check, error, throw, try) |
| okio/src/jvmMain/kotlin/okio/BufferedSink.kt | 113 | (annotation, arrays, class literal, function without body, inheritance, interface, modifier actual, modifier override, modifier sealed, named argument, package, property without initializer) |
| okio/src/jvmMain/kotlin/okio/BufferedSource.kt | 169 | (Byte and Short, annotation, arrays, class literal, function without body, inheritance, interface, modifier actual, modifier sealed, named argument, package, property without initializer) |
| okio/src/jvmMain/kotlin/okio/ByteString.kt | 376 | (@JvmField in object, Byte and Short, annotation, arrays, class literal, default value, extension function, inline-only call, local function, modifier actual, modifier companion, modifier const, modifier open, modifier operator, modifier override, modifier vararg, named argument, package, require, check, error, throw, try, uninitialized local variable) |
| okio/src/jvmMain/kotlin/okio/CipherSink.kt | 147 | (annotation, class literal, inheritance, modifier override, package, require, check, error, throw, try) |
| okio/src/jvmMain/kotlin/okio/CipherSource.kt | 119 | (annotation, class literal, inheritance, modifier override, package, require, check, error) |
| okio/src/jvmMain/kotlin/okio/DeflaterSink.kt | 143 | (annotation, class literal, local function, modifier actual, modifier override, package, throw, try) |
| okio/src/jvmMain/kotlin/okio/DeprecatedUpgrade.kt | 21 | (annotation, package) |
| okio/src/jvmMain/kotlin/okio/FileSystem.System.kt | 13 | (annotation, extension property, modifier actual, modifier inline, package, property getter, property without initializer) |
| okio/src/jvmMain/kotlin/okio/FileSystem.kt | 190 | (@JvmField in object, annotation, class literal, extension function, function type with receiver, function without body, inheritance, inline-only call, modifier abstract, modifier actual, modifier companion, modifier inline, modifier open, modifier override, named argument, package, try) |
| okio/src/jvmMain/kotlin/okio/ForwardingSink.kt | 48 | (annotation, class literal, inheritance, modifier abstract, modifier override, named argument, package) |
| okio/src/jvmMain/kotlin/okio/ForwardingSource.kt | 43 | (annotation, class literal, local function, modifier abstract, modifier actual, modifier override, named argument, package) |
| okio/src/jvmMain/kotlin/okio/ForwardingTimeout.kt | 59 | (annotation, class literal, inheritance, modifier open, modifier override, package) |
| okio/src/jvmMain/kotlin/okio/HashingSink.kt | 148 | (annotation, arrays, class literal, inheritance, inline-only call, modifier actual, modifier companion, modifier override, named argument, package, property getter, property without initializer, secondary constructor, super, throw, try) |
| okio/src/jvmMain/kotlin/okio/HashingSource.kt | 159 | (annotation, arrays, class literal, inheritance, inline-only call, modifier actual, modifier companion, modifier override, named argument, package, property getter, property without initializer, secondary constructor, super, throw, try) |
| okio/src/jvmMain/kotlin/okio/InflaterSource.kt | 131 | (annotation, class literal, local function, modifier actual, modifier override, package, require, check, error, throw, try) |
| okio/src/jvmMain/kotlin/okio/JvmFileHandle.kt | 81 | (annotation, arrays, inheritance, modifier override, package) |
| okio/src/jvmMain/kotlin/okio/JvmOkio.kt | 233 | (annotation, arrays, class literal, default value, extension function, inheritance, inline-only call, modifier open, modifier override, modifier vararg, named argument, package, require, check, error, spread operator, throw, try) |
| okio/src/jvmMain/kotlin/okio/JvmSystemFileSystem.kt | 157 | (extension function, inheritance, modifier open, modifier override, named argument, package, require, check, error, throw) |
| okio/src/jvmMain/kotlin/okio/NioFileSystemFileHandle.kt | 84 | (annotation, arrays, inheritance, modifier override, package) |
| okio/src/jvmMain/kotlin/okio/NioFileSystemWrappingFileSystem.kt | 195 | (arrays, class literal, extension function, inheritance, inline-only call, modifier override, named argument, package, require, check, error, spread operator, throw, try) |
| okio/src/jvmMain/kotlin/okio/NioSystemFileSystem.kt | 91 | (class literal, extension function, inheritance, inline-only call, modifier open, modifier override, modifier protected, named argument, package, throw, try) |
| okio/src/jvmMain/kotlin/okio/Path.kt | 131 | (@JvmField in object, Char, annotation, default value, extension function, local function, modifier actual, modifier companion, modifier operator, modifier override, named argument, package, uninitialized local variable) |
| okio/src/jvmMain/kotlin/okio/Pipe.kt | 261 | (annotation, class literal, extension function, function type with receiver, inheritance, inline-only call, modifier inline, modifier lateinit, modifier override, named argument, object expression, package, qualified this, require, check, error, throw, try, uninitialized local variable) |
| okio/src/jvmMain/kotlin/okio/RealBufferedSink.kt | 166 | (Char, annotation, arrays, inheritance, inline property accessor, inline-only call, local function, modifier actual, modifier inline, modifier override, object expression, package, qualified this, require, check, error, throw, uninitialized local variable) |
| okio/src/jvmMain/kotlin/okio/RealBufferedSource.kt | 206 | (Byte and Short, annotation, arrays, bitwise operators, inheritance, inline property accessor, local function, modifier actual, modifier inline, modifier override, named argument, object expression, package, qualified this, require, check, error, throw, uninitialized local variable) |
| okio/src/jvmMain/kotlin/okio/SegmentPool.kt | 129 | (@JvmStatic in object, annotation, arrays, bitwise operators, identity equality, modifier actual, named argument, object, package, property getter, require, check, error) |
| okio/src/jvmMain/kotlin/okio/SegmentedByteString.kt | 139 | (Byte and Short, annotation, arrays, as, class literal, inline-only call, local function, modifier actual, modifier override, named argument, package, throw, try) |
| okio/src/jvmMain/kotlin/okio/Sink.kt | 33 | (annotation, class literal, function without body, inheritance, interface, modifier actual, modifier override, package) |
| okio/src/jvmMain/kotlin/okio/Throttler.kt | 167 | (annotation, class literal, default value, extension function, inheritance, modifier override, named argument, object expression, package, require, check, error, secondary constructor, super, throw, try) |
| okio/src/jvmMain/kotlin/okio/Timeout.kt | 357 | (@JvmField in object, annotation, as, class literal, extension function, identity equality, inheritance, modifier actual, modifier companion, modifier inline, modifier open, modifier override, object expression, package, require, check, error, throw, try) |
| okio/src/jvmMain/kotlin/okio/internal/-Atomic.kt | 31 | (bitwise operators, extension function, package) |
| okio/src/jvmMain/kotlin/okio/internal/-JavaIo.kt | 60 | (extension property, inheritance, modifier override, package, property without initializer, throw, try) |
| okio/src/jvmMain/kotlin/okio/internal/-ZlibJvm.kt | 40 | (arrays, modifier actual, package, typealias) |
| okio/src/jvmMain/kotlin/okio/internal/DefaultSocket.kt | 175 | (bitwise operators, inheritance, modifier const, modifier inner, modifier override, nested class, package, require, check, error, throw, try) |
| okio/src/jvmMain/kotlin/okio/internal/PipeSocket.kt | 34 | (inheritance, modifier override, package, property getter) |
| okio/src/jvmMain/kotlin/okio/internal/ResourceFileSystem.kt | 214 | (Char, default value, delegated property, destructuring declaration, extension function, inheritance, inline-only call, is, modifier companion, modifier override, named argument, package, throw, try) |
| okio/src/nonJsMain/kotlin/okio/internal/ByteStringNonJs.kt | 42 | (Char, annotation, arrays, bitwise operators, extension function, modifier actual, modifier inline, package, require, check, error, throw) |
| okio/src/systemFileSystemMain/kotlin/okio/FileSystem.System.kt | 7 | (extension property, modifier expect, package, property without initializer) |
| okio/src/zlibMain/kotlin/okio/Deflater.kt | 31 | (function without body, modifier expect, package, secondary constructor) |
| okio/src/zlibMain/kotlin/okio/DeflaterSink.kt | 63 | (annotation, default value, extension function, function without body, inheritance, modifier expect, modifier inline, modifier override, package, secondary constructor) |
| okio/src/zlibMain/kotlin/okio/GzipSink.kt | 151 | (annotation, class literal, extension function, inheritance, inline-only call, modifier inline, modifier override, named argument, package, require, check, error, throw, try) |
| okio/src/zlibMain/kotlin/okio/GzipSource.kt | 221 | (Byte and Short, Char, annotation, bitwise operators, class literal, extension function, inheritance, modifier const, modifier inline, modifier override, package, require, check, error, throw) |
| okio/src/zlibMain/kotlin/okio/Inflater.kt | 30 | (function without body, modifier expect, package, secondary constructor) |
| okio/src/zlibMain/kotlin/okio/InflaterSource.kt | 48 | (annotation, default value, extension function, function without body, inheritance, modifier expect, modifier inline, modifier override, package, secondary constructor) |
| okio/src/zlibMain/kotlin/okio/ZipFileSystem.kt | 140 | (annotation, class literal, inheritance, inline-only call, modifier companion, modifier override, named argument, package, throw) |
| okio/src/zlibMain/kotlin/okio/ZlibOkio.kt | 35 | (annotation, class literal, extension function, package) |
| okio/src/zlibMain/kotlin/okio/internal/-Zlib.kt | 38 | (function without body, modifier expect, package, property without initializer) |
| okio/src/zlibMain/kotlin/okio/internal/CRC32.kt | 23 | (arrays, function without body, modifier expect, package) |
| okio/src/zlibMain/kotlin/okio/internal/ZipEntry.kt | 137 | (default value, named argument, package, property getter) |
| okio/src/zlibMain/kotlin/okio/internal/ZipFiles.kt | 502 | (Char, annotation, bitwise operators, class literal, default value, extension function, extension property, inline-only call, modifier const, named argument, package, property getter, require, check, error, throw, try, uninitialized local variable) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 99 files, 21371 lines

Directories: `okio/src/commonTest`, `okio/src/hashFunctions`, `okio/src/nonWasmTest`, `okio/src/zlibTest`, `okio/src/jvmTest`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 99 | 6 |
| 1: packages, annotations | 7 | 92 | 4 |
| 2: interfaces, inheritance | 7 | 92 | 3 |
| 3: type checks | 10 | 89 | 3 |
| 4: exceptions | 11 | 88 | 3 |
| 5: objects | 13 | 86 | 2 |
| 6: data, enum, sealed | 21 | 78 | 2 |
| 7: arrays, vararg, spread | 28 | 71 | 2 |
| 8: Char, Float, Byte, Short | 36 | 63 | 1 |
| 9: inliner | 54 | 45 | 1 |
| 10: lambdas with receivers | 55 | 44 | 1 |
| 11: extensions | 60 | 39 | 1 |
| 12: operator conventions | 60 | 39 | 1 |
| 13: identity, class literals, references | 64 | 35 | 1 |
| 14: class delegation | 64 | 35 | 1 |
| 15: secondary constructors | 65 | 34 | 1 |
| 16: local functions | 65 | 34 | 1 |
| 17: expect and actual | 66 | 33 | 1 |
| 18: annotation classes | 66 | 33 | 1 |
| 19: value classes | 66 | 33 | 1 |
| 20: unsigned integers | 67 | 32 | 1 |
| definite assignment | 68 | 31 | 1 |
| 21: inline functions | 69 | 30 | 1 |
| 22: reified type parameters | 69 | 30 | 1 |
| 23: labels, do-while | 70 | 29 | 1 |
| 24: delegated properties | 70 | 29 | 1 |
| 25: object expressions | 83 | 16 | 1 |
| 26: destructuring lambda parameters | 83 | 16 | 1 |
| 42-49: coroutines | 83 | 16 | 1 |
| 52: bitwise operators | 94 | 5 | 1 |
| 53: typealias | 94 | 5 | 1 |
| 54: setters, backing fields | 94 | 5 | 1 |
| 55: lateinit, tailrec | 97 | 2 | 2 |
| 56-57: collection supertypes | 97 | 2 | 2 |
| 58: member extension functions | 97 | 2 | 2 |
| 59: reified member type parameters | 97 | 2 | 2 |
| 60: inner and local classes | 98 | 1 | 2 |
| 61: when guards, anonymous functions, local extension functions | 98 | 1 | 2 |
| 62a: @JvmField and @JvmStatic in objects | 99 | 0 | – |
| 62b: generic value classes | 99 | 0 | – |
| 63: definitely non-nullable types | 99 | 0 | – |
| 70a: inline property accessors | 99 | 0 | – |
| 70b: where clauses | 99 | 0 | – |
| 70c: fun interfaces | 99 | 0 | – |
| delegated extension properties | 99 | 0 | – |
| qualified super | 99 | 0 | – |
| also supported | 99 | 0 | – |

### Features left, by files blocked

99 of 99 files use no feature that kc doesn't support.

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
| `apply` | 12 | 36 |
| `use` | 11 | 66 |
| `outputStream` | 8 | 15 |
| `require` | 7 | 31 |
| `inputStream` | 6 | 14 |
| `List` | 5 | 9 |
| `also` | 5 | 9 |
| `with` | 3 | 3 |
| `synchronized` | 2 | 7 |
| `buildString` | 1 | 1 |
| `charset` | 1 | 1 |
| `check` | 1 | 1 |
| `codePointAt` | 1 | 1 |
| `copyOf` | 1 | 2 |
| `error` | 1 | 1 |
| `format` | 1 | 1 |
| `isNotEmpty` | 1 | 2 |
| `let` | 1 | 3 |

### Files

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| okio/src/commonTest/kotlin/okio/BufferCommonTest.kt | 90 | (annotation, inline-only call, package) |
| okio/src/commonTest/kotlin/okio/BufferedSinkFactory.kt | 34 | (as, function without body, modifier abstract, modifier enum, modifier override, package) |
| okio/src/commonTest/kotlin/okio/BufferedSourceFactory.kt | 130 | (as, class delegation, function without body, inheritance, modifier abstract, modifier enum, modifier override, nested class, object expression, package, property getter, property without initializer) |
| okio/src/commonTest/kotlin/okio/ByteStringFactory.kt | 51 | (function without body, inline-only call, modifier abstract, modifier enum, modifier override, package, spread operator) |
| okio/src/commonTest/kotlin/okio/ByteStringHexTest.kt | 71 | (annotation, named argument, package) |
| okio/src/commonTest/kotlin/okio/ByteStringMoreTests.kt | 29 | (annotation, arrays, package) |
| okio/src/commonTest/kotlin/okio/ByteStringTest.kt | 619 | (Char, annotation, arrays, identity equality, inline-only call, named argument, package, spread operator, try) |
| okio/src/commonTest/kotlin/okio/CommonBufferTest.kt | 439 | (Char, annotation, arrays, inline-only call, modifier vararg, package) |
| okio/src/commonTest/kotlin/okio/CommonBufferedSinkTest.kt | 329 | (Char, annotation, as, bitwise operators, class delegation, inheritance, modifier override, object expression, package, throw) |
| okio/src/commonTest/kotlin/okio/CommonBufferedSourceTest.kt | 1274 | (Byte and Short, Char, annotation, arrays, bitwise operators, identity equality, is, package, property without initializer, require, check, error) |
| okio/src/commonTest/kotlin/okio/CommonOkioKotlinTest.kt | 42 | (annotation, as, package) |
| okio/src/commonTest/kotlin/okio/CommonOptionsTest.kt | 447 | (annotation, arrays, default value, extension function, modifier vararg, package, spread operator, try) |
| okio/src/commonTest/kotlin/okio/CommonRealBufferedSinkTest.kt | 202 | (Char, annotation, arrays, as, package, try) |
| okio/src/commonTest/kotlin/okio/CommonRealBufferedSourceTest.kt | 183 | (Char, annotation, arrays, as, class delegation, inheritance, modifier override, object expression, package, require, check, error) |
| okio/src/commonTest/kotlin/okio/ForwardingSourceTest.kt | 53 | (annotation, inheritance, modifier override, object expression, package) |
| okio/src/commonTest/kotlin/okio/HashingSinkTest.kt | 119 | (annotation, package) |
| okio/src/commonTest/kotlin/okio/HashingSourceTest.kt | 128 | (Char, annotation, package, try) |
| okio/src/commonTest/kotlin/okio/HashingTest.kt | 143 | (annotation, modifier companion, package) |
| okio/src/commonTest/kotlin/okio/LimitSourceTest.kt | 166 | (annotation, named argument, package) |
| okio/src/commonTest/kotlin/okio/MockSink.kt | 63 | (inheritance, modifier override, modifier vararg, package, throw) |
| okio/src/commonTest/kotlin/okio/OkioTesting.kt | 106 | (Char, arrays, default value, extension function, function without body, identity equality, inline-only call, modifier expect, modifier vararg, package, uninitialized local variable) |
| okio/src/commonTest/kotlin/okio/PathTest.kt | 795 | (Char, annotation, named argument, package) |
| okio/src/commonTest/kotlin/okio/TypedOptionsTest.kt | 90 | (annotation, package) |
| okio/src/commonTest/kotlin/okio/UnsafeCursorTest.kt | 92 | (Char, annotation, class literal, package, try) |
| okio/src/commonTest/kotlin/okio/UnsignedTest.kt | 279 | (annotation, package, unsigned integers) |
| okio/src/commonTest/kotlin/okio/Utf8KotlinTest.kt | 189 | (Char, annotation, arrays, modifier vararg, package, spread operator) |
| okio/src/hashFunctions/kotlin/okio/internal/HashFunction.kt | 27 | (arrays, default value, function without body, interface, package) |
| okio/src/hashFunctions/kotlin/okio/internal/Hmac.kt | 75 | (Byte and Short, arrays, bitwise operators, inheritance, inline-only call, modifier companion, modifier const, modifier override, named argument, package, throw) |
| okio/src/hashFunctions/kotlin/okio/internal/Md5.kt | 206 | (arrays, bitwise operators, inheritance, modifier companion, modifier override, package) |
| okio/src/hashFunctions/kotlin/okio/internal/Sha1.kt | 204 | (arrays, bitwise operators, inheritance, modifier override, package) |
| okio/src/hashFunctions/kotlin/okio/internal/Sha256.kt | 253 | (arrays, bitwise operators, inheritance, modifier companion, modifier override, package) |
| okio/src/hashFunctions/kotlin/okio/internal/Sha512.kt | 291 | (arrays, bitwise operators, inheritance, modifier companion, modifier override, package) |
| okio/src/jvmTest/kotlin/okio/AsyncSocket.kt | 88 | (inheritance, interface, is, modifier data, modifier override, modifier sealed, named argument, nested class, object, package) |
| okio/src/jvmTest/kotlin/okio/AsyncTimeoutTest.kt | 500 | (annotation, arrays, inheritance, modifier inner, modifier override, modifier vararg, nested class, object expression, package, spread operator, throw, try) |
| okio/src/jvmTest/kotlin/okio/AwaitSignalTest.kt | 210 | (annotation, extension function, package, try) |
| okio/src/jvmTest/kotlin/okio/BufferCursorKotlinTest.kt | 118 | (Char, annotation, inline-only call, package) |
| okio/src/jvmTest/kotlin/okio/BufferCursorTest.kt | 445 | (Char, annotation, arrays, do-while, identity equality, inline-only call, package, spread operator, try) |
| okio/src/jvmTest/kotlin/okio/BufferFactory.kt | 67 | (annotation, arrays, class literal, function without body, modifier abstract, modifier enum, modifier override, package) |
| okio/src/jvmTest/kotlin/okio/BufferKotlinTest.kt | 96 | (Char, annotation, inline-only call, named argument, package) |
| okio/src/jvmTest/kotlin/okio/BufferTest.kt | 611 | (Char, annotation, arrays, inline-only call, modifier vararg, package, try) |
| okio/src/jvmTest/kotlin/okio/BufferedSinkAppendableTest.kt | 131 | (Char, annotation, default value, package) |
| okio/src/jvmTest/kotlin/okio/BufferedSinkJavaTest.kt | 250 | (Char, annotation, arrays, as, inline-only call, package, try) |
| okio/src/jvmTest/kotlin/okio/BufferedSinkTest.kt | 375 | (Char, annotation, arrays, as, bitwise operators, function without body, inheritance, inline-only call, modifier abstract, modifier enum, modifier override, nested class, object expression, package, throw, try) |
| okio/src/jvmTest/kotlin/okio/BufferedSourceJavaTest.kt | 225 | (Char, annotation, arrays, as, inheritance, inline-only call, modifier override, object expression, package, require, check, error, super, try) |
| okio/src/jvmTest/kotlin/okio/BufferedSourceTest.kt | 1707 | (Byte and Short, Char, annotation, arrays, as, bitwise operators, function without body, identity equality, inheritance, inline-only call, is, modifier abstract, modifier enum, modifier override, named argument, nested class, object expression, package, property without initializer, require, check, error, super, try) |
| okio/src/jvmTest/kotlin/okio/ByteStringJavaTest.kt | 260 | (Char, annotation, arrays, as, function without body, modifier abstract, modifier companion, modifier enum, modifier override, nested class, package) |
| okio/src/jvmTest/kotlin/okio/ByteStringKotlinTest.kt | 66 | (annotation, arrays, named argument, package) |
| okio/src/jvmTest/kotlin/okio/CipherAlgorithm.kt | 57 | (Char, default value, modifier enum, package) |
| okio/src/jvmTest/kotlin/okio/CipherFactory.kt | 36 | (function type with receiver, inline-only call, package, property getter) |
| okio/src/jvmTest/kotlin/okio/CipherSinkTest.kt | 190 | (annotation, arrays, inline-only call, package) |
| okio/src/jvmTest/kotlin/okio/CipherSourceTest.kt | 181 | (annotation, arrays, extension function, inheritance, inline-only call, modifier override, nested class, package) |
| okio/src/jvmTest/kotlin/okio/ConstantTimeEqualsTimingTest.kt | 82 | (annotation, arrays, inline-only call, modifier inline, named argument, package) |
| okio/src/jvmTest/kotlin/okio/DeflateKotlinTest.kt | 51 | (annotation, as, package) |
| okio/src/jvmTest/kotlin/okio/DeflaterSinkTest.kt | 179 | (annotation, arrays, inline-only call, package, try) |
| okio/src/jvmTest/kotlin/okio/FileHandleFileSystemTest.kt | 93 | (inheritance, inline-only call, modifier override, named argument, nested class, package) |
| okio/src/jvmTest/kotlin/okio/FileLeakTest.kt | 167 | (annotation, arrays, class literal, extension function, function type with receiver, inline-only call, modifier const, modifier inline, modifier lateinit, named argument, package, property without initializer, try) |
| okio/src/jvmTest/kotlin/okio/FileSystemJavaTest.kt | 130 | (annotation, as, inheritance, modifier override, named argument, object expression, package) |
| okio/src/jvmTest/kotlin/okio/ForwardingTimeoutKotlinTest.kt | 47 | (annotation, package) |
| okio/src/jvmTest/kotlin/okio/ForwardingTimeoutTest.kt | 46 | (annotation, package) |
| okio/src/jvmTest/kotlin/okio/InflaterSourceTest.kt | 206 | (annotation, inline-only call, modifier lateinit, package, property without initializer, try) |
| okio/src/jvmTest/kotlin/okio/JimfsOkioRoundTripTest.kt | 68 | (annotation, inline-only call, package) |
| okio/src/jvmTest/kotlin/okio/JvmSystemFileSystemTest.kt | 90 | (annotation, inheritance, named argument, package, try) |
| okio/src/jvmTest/kotlin/okio/JvmTest.kt | 50 | (annotation, as, package) |
| okio/src/jvmTest/kotlin/okio/JvmTesting.kt | 65 | (modifier actual, named argument, package, try) |
| okio/src/jvmTest/kotlin/okio/LargeStreamsTest.kt | 112 | (annotation, as, inheritance, inline-only call, modifier companion, modifier const, object expression, package, try) |
| okio/src/jvmTest/kotlin/okio/MessageDigestConsistencyTest.kt | 101 | (annotation, arrays, inline-only call, named argument, package) |
| okio/src/jvmTest/kotlin/okio/NioTest.kt | 142 | (annotation, arrays, as, is, package) |
| okio/src/jvmTest/kotlin/okio/OkioKotlinTest.kt | 128 | (annotation, arrays, inheritance, modifier override, named argument, object expression, package) |
| okio/src/jvmTest/kotlin/okio/OkioTest.kt | 146 | (annotation, arrays, package, try) |
| okio/src/jvmTest/kotlin/okio/PipeKotlinTest.kt | 913 | (annotation, arrays, as, inheritance, inline-only call, modifier companion, modifier override, nested class, object expression, package, qualified this, require, check, error, super, throw, try) |
| okio/src/jvmTest/kotlin/okio/PipeTest.kt | 362 | (annotation, arrays, package, try) |
| okio/src/jvmTest/kotlin/okio/ReadUtf8LineTest.kt | 190 | (annotation, arrays, function without body, inheritance, modifier abstract, modifier enum, modifier override, nested class, object expression, package, super, try) |
| okio/src/jvmTest/kotlin/okio/SegmentSharingTest.kt | 175 | (Char, annotation, inline-only call, package) |
| okio/src/jvmTest/kotlin/okio/SocketTest.kt | 305 | (annotation, arrays, as, default value, function without body, modifier abstract, modifier enum, modifier lateinit, modifier override, named argument, nested class, package, property without initializer, try) |
| okio/src/jvmTest/kotlin/okio/SocketTimeoutTest.kt | 141 | (annotation, arrays, inheritance, modifier companion, modifier const, modifier override, object expression, package, require, check, error, throw, try) |
| okio/src/jvmTest/kotlin/okio/Stopwatch.kt | 34 | (package) |
| okio/src/jvmTest/kotlin/okio/TestUtil.kt | 298 | (@JvmField in object, @JvmStatic in object, Byte and Short, annotation, arrays, as, bitwise operators, class literal, extension function, identity equality, inheritance, inline-only call, modifier const, modifier override, modifier vararg, object, object expression, package, spread operator, throw, uninitialized local variable) |
| okio/src/jvmTest/kotlin/okio/ThrottlerTakeTest.kt | 144 | (annotation, named argument, package) |
| okio/src/jvmTest/kotlin/okio/ThrottlerTest.kt | 158 | (annotation, inline-only call, named argument, package) |
| okio/src/jvmTest/kotlin/okio/TimeoutFactory.kt | 33 | (function without body, modifier abstract, modifier enum, modifier override, package) |
| okio/src/jvmTest/kotlin/okio/TimeoutTest.kt | 118 | (annotation, modifier companion, package) |
| okio/src/jvmTest/kotlin/okio/Utf8Test.kt | 360 | (Char, annotation, arrays, as, inline-only call, modifier vararg, package, spread operator) |
| okio/src/jvmTest/kotlin/okio/WaitUntilNotifiedTest.kt | 217 | (annotation, as, extension function, inline-only call, package, qualified this, try) |
| okio/src/jvmTest/kotlin/okio/ZipBuilder.kt | 158 | (annotation, default value, inline-only call, nested class, package, require, check, error) |
| okio/src/jvmTest/kotlin/okio/ZipFileSystemJavaTest.kt | 42 | (annotation, inline-only call, package) |
| okio/src/jvmTest/kotlin/okio/internal/HmacTest.kt | 78 | (annotation, arrays, callable reference, inline-only call, modifier enum, package) |
| okio/src/jvmTest/kotlin/okio/internal/PriorityQueueTest.kt | 279 | (annotation, bitwise operators, default value, extension function, inline-only call, modifier operator, package) |
| okio/src/jvmTest/kotlin/okio/internal/ResourceFileSystemTest.kt | 522 | (annotation, arrays, as, callable reference, class literal, extension function, extension property, inheritance, modifier override, named argument, object expression, package, property getter, super, throw, try) |
| okio/src/jvmTest/kotlin/okio/internal/SetBitsOrZeroTest.kt | 53 | (annotation, bitwise operators, package) |
| okio/src/nonWasmTest/kotlin/okio/FakeFileSystemTest.kt | 505 | (annotation, class literal, inheritance, inline-only call, modifier abstract, modifier data, named argument, nested class, package, secondary constructor, try) |
| okio/src/nonWasmTest/kotlin/okio/ForwardingFileSystemTest.kt | 188 | (annotation, delegated property, inheritance, inline-only call, modifier override, named argument, object expression, package, throw) |
| okio/src/nonWasmTest/kotlin/okio/UseTest.kt | 38 | (annotation, inheritance, inline-only call, local function, modifier override, named argument, object expression, package) |
| okio/src/zlibTest/kotlin/okio/GzipKotlinTest.kt | 50 | (annotation, arrays, as, inline-only call, package) |
| okio/src/zlibTest/kotlin/okio/GzipSinkTest.kt | 59 | (annotation, package, try) |
| okio/src/zlibTest/kotlin/okio/GzipSourceTest.kt | 230 | (Byte and Short, Char, annotation, arrays, inheritance, modifier override, nested class, package, spread operator, try) |
| okio/src/zlibTest/kotlin/okio/ZipFileSystemGoTest.kt | 50 | (annotation, package) |
| okio/src/zlibTest/kotlin/okio/ZipFileSystemTest.kt | 619 | (annotation, extension function, inline-only call, package) |
| okio/src/zlibTest/kotlin/okio/internal/CRC32Test.kt | 56 | (annotation, package) |
| okio/src/zlibTest/kotlin/okio/internal/DatePartsToEpochMillisTest.kt | 261 | (annotation, default value, named argument, nested class, package, require, check, error) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

