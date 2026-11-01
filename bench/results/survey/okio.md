# Survey: okio parent-3.18.2

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 96 files, 17346 lines

Directories: `okio/src/commonMain`, `okio/src/zlibMain`, `okio/src/systemFileSystemMain`, `okio/src/nonJsMain`, `okio/src/jvmMain`

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

`require`, `check`, `requireNotNull`, `checkNotNull`, `error` and `TODO` are built in; kc inlines the others, also those with reified type parameters.

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

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| okio/src/commonMain/kotlin/okio/Base64.kt | 160 |  |
| okio/src/commonMain/kotlin/okio/Buffer.kt | 441 |  |
| okio/src/commonMain/kotlin/okio/BufferedSink.kt | 327 |  |
| okio/src/commonMain/kotlin/okio/BufferedSource.kt | 604 |  |
| okio/src/commonMain/kotlin/okio/ByteString.kt | 231 |  |
| okio/src/commonMain/kotlin/okio/CommonPlatform.kt | 55 |  |
| okio/src/commonMain/kotlin/okio/ExperimentalFileSystem.kt | 30 |  |
| okio/src/commonMain/kotlin/okio/FileHandle.kt | 443 |  |
| okio/src/commonMain/kotlin/okio/FileMetadata.kt | 156 |  |
| okio/src/commonMain/kotlin/okio/FileSystem.kt | 410 |  |
| okio/src/commonMain/kotlin/okio/ForwardingFileSystem.kt | 251 |  |
| okio/src/commonMain/kotlin/okio/ForwardingSource.kt | 37 |  |
| okio/src/commonMain/kotlin/okio/HashingSink.kt | 80 |  |
| okio/src/commonMain/kotlin/okio/HashingSource.kt | 80 |  |
| okio/src/commonMain/kotlin/okio/Okio.kt | 154 |  |
| okio/src/commonMain/kotlin/okio/Options.kt | 237 |  |
| okio/src/commonMain/kotlin/okio/Path.kt | 309 |  |
| okio/src/commonMain/kotlin/okio/PeekSource.kt | 73 |  |
| okio/src/commonMain/kotlin/okio/RealBufferedSink.kt | 50 |  |
| okio/src/commonMain/kotlin/okio/RealBufferedSource.kt | 70 |  |
| okio/src/commonMain/kotlin/okio/Segment.kt | 186 |  |
| okio/src/commonMain/kotlin/okio/SegmentPool.kt | 36 |  |
| okio/src/commonMain/kotlin/okio/SegmentedByteString.kt | 50 |  |
| okio/src/commonMain/kotlin/okio/Sink.kt | 63 |  |
| okio/src/commonMain/kotlin/okio/Socket.kt | 55 |  |
| okio/src/commonMain/kotlin/okio/Source.kt | 70 |  |
| okio/src/commonMain/kotlin/okio/Timeout.kt | 44 |  |
| okio/src/commonMain/kotlin/okio/TypedOptions.kt | 51 |  |
| okio/src/commonMain/kotlin/okio/Unsigned.kt | 357 |  |
| okio/src/commonMain/kotlin/okio/Utf8.kt | 558 |  |
| okio/src/commonMain/kotlin/okio/Util.kt | 164 |  |
| okio/src/commonMain/kotlin/okio/internal/-Utf16.kt | 35 |  |
| okio/src/commonMain/kotlin/okio/internal/-Utf8.kt | 59 |  |
| okio/src/commonMain/kotlin/okio/internal/Buffer.kt | 1710 | labeled expression |
| okio/src/commonMain/kotlin/okio/internal/BufferedSource.kt | 31 |  |
| okio/src/commonMain/kotlin/okio/internal/ByteString.kt | 384 |  |
| okio/src/commonMain/kotlin/okio/internal/FileSystem.kt | 154 |  |
| okio/src/commonMain/kotlin/okio/internal/Path.kt | 404 |  |
| okio/src/commonMain/kotlin/okio/internal/RealBufferedSink.kt | 218 |  |
| okio/src/commonMain/kotlin/okio/internal/RealBufferedSource.kt | 471 |  |
| okio/src/commonMain/kotlin/okio/internal/SegmentedByteString.kt | 247 |  |
| okio/src/jvmMain/kotlin/okio/-DeprecatedOkio.kt | 147 |  |
| okio/src/jvmMain/kotlin/okio/-DeprecatedUtf8.kt | 40 |  |
| okio/src/jvmMain/kotlin/okio/-JvmPlatform.kt | 55 |  |
| okio/src/jvmMain/kotlin/okio/AsyncTimeout.kt | 513 |  |
| okio/src/jvmMain/kotlin/okio/Buffer.kt | 715 |  |
| okio/src/jvmMain/kotlin/okio/BufferedSink.kt | 113 |  |
| okio/src/jvmMain/kotlin/okio/BufferedSource.kt | 169 |  |
| okio/src/jvmMain/kotlin/okio/ByteString.kt | 376 |  |
| okio/src/jvmMain/kotlin/okio/CipherSink.kt | 147 |  |
| okio/src/jvmMain/kotlin/okio/CipherSource.kt | 119 |  |
| okio/src/jvmMain/kotlin/okio/DeflaterSink.kt | 143 |  |
| okio/src/jvmMain/kotlin/okio/DeprecatedUpgrade.kt | 21 |  |
| okio/src/jvmMain/kotlin/okio/FileSystem.System.kt | 13 |  |
| okio/src/jvmMain/kotlin/okio/FileSystem.kt | 190 |  |
| okio/src/jvmMain/kotlin/okio/ForwardingSink.kt | 48 |  |
| okio/src/jvmMain/kotlin/okio/ForwardingSource.kt | 43 |  |
| okio/src/jvmMain/kotlin/okio/ForwardingTimeout.kt | 59 |  |
| okio/src/jvmMain/kotlin/okio/HashingSink.kt | 148 |  |
| okio/src/jvmMain/kotlin/okio/HashingSource.kt | 159 |  |
| okio/src/jvmMain/kotlin/okio/InflaterSource.kt | 131 |  |
| okio/src/jvmMain/kotlin/okio/JvmFileHandle.kt | 81 |  |
| okio/src/jvmMain/kotlin/okio/JvmOkio.kt | 233 |  |
| okio/src/jvmMain/kotlin/okio/JvmSystemFileSystem.kt | 157 |  |
| okio/src/jvmMain/kotlin/okio/NioFileSystemFileHandle.kt | 84 |  |
| okio/src/jvmMain/kotlin/okio/NioFileSystemWrappingFileSystem.kt | 195 |  |
| okio/src/jvmMain/kotlin/okio/NioSystemFileSystem.kt | 91 |  |
| okio/src/jvmMain/kotlin/okio/Path.kt | 131 |  |
| okio/src/jvmMain/kotlin/okio/Pipe.kt | 261 |  |
| okio/src/jvmMain/kotlin/okio/RealBufferedSink.kt | 166 |  |
| okio/src/jvmMain/kotlin/okio/RealBufferedSource.kt | 206 |  |
| okio/src/jvmMain/kotlin/okio/SegmentPool.kt | 129 |  |
| okio/src/jvmMain/kotlin/okio/SegmentedByteString.kt | 139 |  |
| okio/src/jvmMain/kotlin/okio/Sink.kt | 33 |  |
| okio/src/jvmMain/kotlin/okio/Throttler.kt | 167 |  |
| okio/src/jvmMain/kotlin/okio/Timeout.kt | 357 |  |
| okio/src/jvmMain/kotlin/okio/internal/-Atomic.kt | 31 |  |
| okio/src/jvmMain/kotlin/okio/internal/-JavaIo.kt | 60 |  |
| okio/src/jvmMain/kotlin/okio/internal/-ZlibJvm.kt | 40 |  |
| okio/src/jvmMain/kotlin/okio/internal/DefaultSocket.kt | 175 |  |
| okio/src/jvmMain/kotlin/okio/internal/PipeSocket.kt | 34 |  |
| okio/src/jvmMain/kotlin/okio/internal/ResourceFileSystem.kt | 214 |  |
| okio/src/nonJsMain/kotlin/okio/internal/ByteStringNonJs.kt | 42 |  |
| okio/src/systemFileSystemMain/kotlin/okio/FileSystem.System.kt | 7 |  |
| okio/src/zlibMain/kotlin/okio/Deflater.kt | 31 |  |
| okio/src/zlibMain/kotlin/okio/DeflaterSink.kt | 63 |  |
| okio/src/zlibMain/kotlin/okio/GzipSink.kt | 151 |  |
| okio/src/zlibMain/kotlin/okio/GzipSource.kt | 221 |  |
| okio/src/zlibMain/kotlin/okio/Inflater.kt | 30 |  |
| okio/src/zlibMain/kotlin/okio/InflaterSource.kt | 48 |  |
| okio/src/zlibMain/kotlin/okio/ZipFileSystem.kt | 140 |  |
| okio/src/zlibMain/kotlin/okio/ZlibOkio.kt | 35 |  |
| okio/src/zlibMain/kotlin/okio/internal/-Zlib.kt | 38 |  |
| okio/src/zlibMain/kotlin/okio/internal/CRC32.kt | 23 |  |
| okio/src/zlibMain/kotlin/okio/internal/ZipEntry.kt | 137 |  |
| okio/src/zlibMain/kotlin/okio/internal/ZipFiles.kt | 502 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 99 files, 21371 lines

Directories: `okio/src/commonTest`, `okio/src/hashFunctions`, `okio/src/nonWasmTest`, `okio/src/zlibTest`, `okio/src/jvmTest`

### Features left, by files blocked

99 of 99 files use no feature that kc doesn't support.

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

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| okio/src/commonTest/kotlin/okio/BufferCommonTest.kt | 90 |  |
| okio/src/commonTest/kotlin/okio/BufferedSinkFactory.kt | 34 |  |
| okio/src/commonTest/kotlin/okio/BufferedSourceFactory.kt | 130 |  |
| okio/src/commonTest/kotlin/okio/ByteStringFactory.kt | 51 |  |
| okio/src/commonTest/kotlin/okio/ByteStringHexTest.kt | 71 |  |
| okio/src/commonTest/kotlin/okio/ByteStringMoreTests.kt | 29 |  |
| okio/src/commonTest/kotlin/okio/ByteStringTest.kt | 619 |  |
| okio/src/commonTest/kotlin/okio/CommonBufferTest.kt | 439 |  |
| okio/src/commonTest/kotlin/okio/CommonBufferedSinkTest.kt | 329 |  |
| okio/src/commonTest/kotlin/okio/CommonBufferedSourceTest.kt | 1274 |  |
| okio/src/commonTest/kotlin/okio/CommonOkioKotlinTest.kt | 42 |  |
| okio/src/commonTest/kotlin/okio/CommonOptionsTest.kt | 447 |  |
| okio/src/commonTest/kotlin/okio/CommonRealBufferedSinkTest.kt | 202 |  |
| okio/src/commonTest/kotlin/okio/CommonRealBufferedSourceTest.kt | 183 |  |
| okio/src/commonTest/kotlin/okio/ForwardingSourceTest.kt | 53 |  |
| okio/src/commonTest/kotlin/okio/HashingSinkTest.kt | 119 |  |
| okio/src/commonTest/kotlin/okio/HashingSourceTest.kt | 128 |  |
| okio/src/commonTest/kotlin/okio/HashingTest.kt | 143 |  |
| okio/src/commonTest/kotlin/okio/LimitSourceTest.kt | 166 |  |
| okio/src/commonTest/kotlin/okio/MockSink.kt | 63 |  |
| okio/src/commonTest/kotlin/okio/OkioTesting.kt | 106 |  |
| okio/src/commonTest/kotlin/okio/PathTest.kt | 795 |  |
| okio/src/commonTest/kotlin/okio/TypedOptionsTest.kt | 90 |  |
| okio/src/commonTest/kotlin/okio/UnsafeCursorTest.kt | 92 |  |
| okio/src/commonTest/kotlin/okio/UnsignedTest.kt | 279 |  |
| okio/src/commonTest/kotlin/okio/Utf8KotlinTest.kt | 189 |  |
| okio/src/hashFunctions/kotlin/okio/internal/HashFunction.kt | 27 |  |
| okio/src/hashFunctions/kotlin/okio/internal/Hmac.kt | 75 |  |
| okio/src/hashFunctions/kotlin/okio/internal/Md5.kt | 206 |  |
| okio/src/hashFunctions/kotlin/okio/internal/Sha1.kt | 204 |  |
| okio/src/hashFunctions/kotlin/okio/internal/Sha256.kt | 253 |  |
| okio/src/hashFunctions/kotlin/okio/internal/Sha512.kt | 291 |  |
| okio/src/jvmTest/kotlin/okio/AsyncSocket.kt | 88 |  |
| okio/src/jvmTest/kotlin/okio/AsyncTimeoutTest.kt | 500 |  |
| okio/src/jvmTest/kotlin/okio/AwaitSignalTest.kt | 210 |  |
| okio/src/jvmTest/kotlin/okio/BufferCursorKotlinTest.kt | 118 |  |
| okio/src/jvmTest/kotlin/okio/BufferCursorTest.kt | 445 |  |
| okio/src/jvmTest/kotlin/okio/BufferFactory.kt | 67 |  |
| okio/src/jvmTest/kotlin/okio/BufferKotlinTest.kt | 96 |  |
| okio/src/jvmTest/kotlin/okio/BufferTest.kt | 611 |  |
| okio/src/jvmTest/kotlin/okio/BufferedSinkAppendableTest.kt | 131 |  |
| okio/src/jvmTest/kotlin/okio/BufferedSinkJavaTest.kt | 250 |  |
| okio/src/jvmTest/kotlin/okio/BufferedSinkTest.kt | 375 |  |
| okio/src/jvmTest/kotlin/okio/BufferedSourceJavaTest.kt | 225 |  |
| okio/src/jvmTest/kotlin/okio/BufferedSourceTest.kt | 1707 |  |
| okio/src/jvmTest/kotlin/okio/ByteStringJavaTest.kt | 260 |  |
| okio/src/jvmTest/kotlin/okio/ByteStringKotlinTest.kt | 66 |  |
| okio/src/jvmTest/kotlin/okio/CipherAlgorithm.kt | 57 |  |
| okio/src/jvmTest/kotlin/okio/CipherFactory.kt | 36 |  |
| okio/src/jvmTest/kotlin/okio/CipherSinkTest.kt | 190 |  |
| okio/src/jvmTest/kotlin/okio/CipherSourceTest.kt | 181 |  |
| okio/src/jvmTest/kotlin/okio/ConstantTimeEqualsTimingTest.kt | 82 |  |
| okio/src/jvmTest/kotlin/okio/DeflateKotlinTest.kt | 51 |  |
| okio/src/jvmTest/kotlin/okio/DeflaterSinkTest.kt | 179 |  |
| okio/src/jvmTest/kotlin/okio/FileHandleFileSystemTest.kt | 93 |  |
| okio/src/jvmTest/kotlin/okio/FileLeakTest.kt | 167 |  |
| okio/src/jvmTest/kotlin/okio/FileSystemJavaTest.kt | 130 |  |
| okio/src/jvmTest/kotlin/okio/ForwardingTimeoutKotlinTest.kt | 47 |  |
| okio/src/jvmTest/kotlin/okio/ForwardingTimeoutTest.kt | 46 |  |
| okio/src/jvmTest/kotlin/okio/InflaterSourceTest.kt | 206 |  |
| okio/src/jvmTest/kotlin/okio/JimfsOkioRoundTripTest.kt | 68 |  |
| okio/src/jvmTest/kotlin/okio/JvmSystemFileSystemTest.kt | 90 |  |
| okio/src/jvmTest/kotlin/okio/JvmTest.kt | 50 |  |
| okio/src/jvmTest/kotlin/okio/JvmTesting.kt | 65 |  |
| okio/src/jvmTest/kotlin/okio/LargeStreamsTest.kt | 112 |  |
| okio/src/jvmTest/kotlin/okio/MessageDigestConsistencyTest.kt | 101 |  |
| okio/src/jvmTest/kotlin/okio/NioTest.kt | 142 |  |
| okio/src/jvmTest/kotlin/okio/OkioKotlinTest.kt | 128 |  |
| okio/src/jvmTest/kotlin/okio/OkioTest.kt | 146 |  |
| okio/src/jvmTest/kotlin/okio/PipeKotlinTest.kt | 913 |  |
| okio/src/jvmTest/kotlin/okio/PipeTest.kt | 362 |  |
| okio/src/jvmTest/kotlin/okio/ReadUtf8LineTest.kt | 190 |  |
| okio/src/jvmTest/kotlin/okio/SegmentSharingTest.kt | 175 |  |
| okio/src/jvmTest/kotlin/okio/SocketTest.kt | 305 |  |
| okio/src/jvmTest/kotlin/okio/SocketTimeoutTest.kt | 141 |  |
| okio/src/jvmTest/kotlin/okio/Stopwatch.kt | 34 |  |
| okio/src/jvmTest/kotlin/okio/TestUtil.kt | 298 |  |
| okio/src/jvmTest/kotlin/okio/ThrottlerTakeTest.kt | 144 |  |
| okio/src/jvmTest/kotlin/okio/ThrottlerTest.kt | 158 |  |
| okio/src/jvmTest/kotlin/okio/TimeoutFactory.kt | 33 |  |
| okio/src/jvmTest/kotlin/okio/TimeoutTest.kt | 118 |  |
| okio/src/jvmTest/kotlin/okio/Utf8Test.kt | 360 |  |
| okio/src/jvmTest/kotlin/okio/WaitUntilNotifiedTest.kt | 217 |  |
| okio/src/jvmTest/kotlin/okio/ZipBuilder.kt | 158 |  |
| okio/src/jvmTest/kotlin/okio/ZipFileSystemJavaTest.kt | 42 |  |
| okio/src/jvmTest/kotlin/okio/internal/HmacTest.kt | 78 |  |
| okio/src/jvmTest/kotlin/okio/internal/PriorityQueueTest.kt | 279 |  |
| okio/src/jvmTest/kotlin/okio/internal/ResourceFileSystemTest.kt | 522 |  |
| okio/src/jvmTest/kotlin/okio/internal/SetBitsOrZeroTest.kt | 53 |  |
| okio/src/nonWasmTest/kotlin/okio/FakeFileSystemTest.kt | 505 |  |
| okio/src/nonWasmTest/kotlin/okio/ForwardingFileSystemTest.kt | 188 |  |
| okio/src/nonWasmTest/kotlin/okio/UseTest.kt | 38 |  |
| okio/src/zlibTest/kotlin/okio/GzipKotlinTest.kt | 50 |  |
| okio/src/zlibTest/kotlin/okio/GzipSinkTest.kt | 59 |  |
| okio/src/zlibTest/kotlin/okio/GzipSourceTest.kt | 230 |  |
| okio/src/zlibTest/kotlin/okio/ZipFileSystemGoTest.kt | 50 |  |
| okio/src/zlibTest/kotlin/okio/ZipFileSystemTest.kt | 619 |  |
| okio/src/zlibTest/kotlin/okio/internal/CRC32Test.kt | 56 |  |
| okio/src/zlibTest/kotlin/okio/internal/DatePartsToEpochMillisTest.kt | 261 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

