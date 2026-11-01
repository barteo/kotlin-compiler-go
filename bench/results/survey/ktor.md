# Survey: ktor 3.6.0

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 236 files, 23774 lines

Directories: `ktor-io/common/src`, `ktor-io/jvmAndPosix/src`, `ktor-io/jvm/src`, `ktor-utils/common/src`, `ktor-utils/jvm/src`, `ktor-http/common/src`, `ktor-http/jvm/src`

### Features left, by files blocked

236 of 236 files use no feature that kc doesn't support.

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
| `let` | 33 | 63 |
| `apply` | 22 | 36 |
| `require` | 20 | 36 |
| `List` | 15 | 23 |
| `error` | 15 | 29 |
| `isNotEmpty` | 15 | 25 |
| `also` | 13 | 17 |
| `check` | 13 | 37 |
| `buildString` | 10 | 18 |
| `use` | 8 | 10 |
| `writer` | 8 | 11 |
| `suspend` | 7 | 16 |
| `takeIf` | 7 | 13 |
| `charset` | 6 | 12 |
| `runCatching` | 5 | 6 |
| `synchronized` | 4 | 6 |
| `toBoolean` | 4 | 4 |
| `buffered` | 3 | 3 |
| `find` | 3 | 7 |
| `isNotBlank` | 3 | 4 |
| `reader` | 3 | 5 |
| `toRegex` | 3 | 8 |
| `with` | 3 | 5 |
| `MutableList` | 2 | 2 |
| `AutoCloseable` | 1 | 1 |
| `bufferedWriter` | 1 | 2 |
| `compareByDescending` | 1 | 1 |
| `copyOf` | 1 | 1 |
| `floorDiv` | 1 | 2 |
| `format` | 1 | 1 |
| `getOrElse` | 1 | 2 |
| `inputStream` | 1 | 1 |
| `matches` | 1 | 3 |
| `onFailure` | 1 | 1 |
| `requireNotNull` | 1 | 1 |
| `sumOf` | 1 | 1 |
| `takeUnless` | 1 | 1 |
| `thenBy` | 1 | 1 |
| `thenByDescending` | 1 | 1 |

### Files

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| ktor-http/common/src/io/ktor/content/Compatibility.kt | 11 |  |
| ktor-http/common/src/io/ktor/http/ApplicationResponseProperties.kt | 12 |  |
| ktor-http/common/src/io/ktor/http/AsciiBitSet.kt | 42 |  |
| ktor-http/common/src/io/ktor/http/CacheControl.kt | 135 |  |
| ktor-http/common/src/io/ktor/http/Codecs.kt | 322 |  |
| ktor-http/common/src/io/ktor/http/ContentDisposition.kt | 123 |  |
| ktor-http/common/src/io/ktor/http/ContentRange.kt | 40 |  |
| ktor-http/common/src/io/ktor/http/ContentTypeMatcher.kt | 19 |  |
| ktor-http/common/src/io/ktor/http/ContentTypes.kt | 526 |  |
| ktor-http/common/src/io/ktor/http/Cookie.kt | 301 |  |
| ktor-http/common/src/io/ktor/http/CookieUtils.kt | 348 |  |
| ktor-http/common/src/io/ktor/http/DateUtils.kt | 73 |  |
| ktor-http/common/src/io/ktor/http/FileContentType.kt | 111 |  |
| ktor-http/common/src/io/ktor/http/HeaderValueWithParameters.kt | 171 |  |
| ktor-http/common/src/io/ktor/http/Headers.kt | 129 |  |
| ktor-http/common/src/io/ktor/http/HttpHeaderValueParser.kt | 242 |  |
| ktor-http/common/src/io/ktor/http/HttpHeaders.kt | 581 |  |
| ktor-http/common/src/io/ktor/http/HttpMessage.kt | 35 |  |
| ktor-http/common/src/io/ktor/http/HttpMessageProperties.kt | 190 |  |
| ktor-http/common/src/io/ktor/http/HttpMethod.kt | 122 |  |
| ktor-http/common/src/io/ktor/http/HttpProtocolVersion.kt | 97 |  |
| ktor-http/common/src/io/ktor/http/HttpStatusCode.kt | 196 |  |
| ktor-http/common/src/io/ktor/http/HttpUrlEncoded.kt | 91 |  |
| ktor-http/common/src/io/ktor/http/IpParser.kt | 21 |  |
| ktor-http/common/src/io/ktor/http/LinkHeader.kt | 75 |  |
| ktor-http/common/src/io/ktor/http/Mimes.kt | 1019 |  |
| ktor-http/common/src/io/ktor/http/Parameters.kt | 137 |  |
| ktor-http/common/src/io/ktor/http/Query.kt | 126 |  |
| ktor-http/common/src/io/ktor/http/Ranges.kt | 144 |  |
| ktor-http/common/src/io/ktor/http/RangesSpecifier.kt | 85 |  |
| ktor-http/common/src/io/ktor/http/RequestConnectionPoint.kt | 141 |  |
| ktor-http/common/src/io/ktor/http/URLBuilder.kt | 369 |  |
| ktor-http/common/src/io/ktor/http/URLParser.kt | 307 |  |
| ktor-http/common/src/io/ktor/http/URLProtocol.kt | 90 |  |
| ktor-http/common/src/io/ktor/http/URLUtils.kt | 221 |  |
| ktor-http/common/src/io/ktor/http/Url.kt | 302 |  |
| ktor-http/common/src/io/ktor/http/UrlDecodedParametersBuilder.kt | 87 |  |
| ktor-http/common/src/io/ktor/http/auth/AuthScheme.kt | 68 |  |
| ktor-http/common/src/io/ktor/http/auth/DigestAlgorithm.kt | 190 |  |
| ktor-http/common/src/io/ktor/http/auth/HeaderValueEncoding.kt | 35 |  |
| ktor-http/common/src/io/ktor/http/auth/HttpAuthHeader.kt | 524 |  |
| ktor-http/common/src/io/ktor/http/auth/KeyAlgorithm.kt | 47 |  |
| ktor-http/common/src/io/ktor/http/auth/SignatureAlgorithm.kt | 156 |  |
| ktor-http/common/src/io/ktor/http/content/ByteArrayContent.kt | 22 |  |
| ktor-http/common/src/io/ktor/http/content/CachingOptions.kt | 35 |  |
| ktor-http/common/src/io/ktor/http/content/ChannelWriterContent.kt | 25 |  |
| ktor-http/common/src/io/ktor/http/content/CompressedContent.kt | 100 |  |
| ktor-http/common/src/io/ktor/http/content/Multipart.kt | 210 |  |
| ktor-http/common/src/io/ktor/http/content/OutgoingContent.kt | 222 |  |
| ktor-http/common/src/io/ktor/http/content/TextContent.kt | 31 |  |
| ktor-http/common/src/io/ktor/http/content/Versions.kt | 312 |  |
| ktor-http/common/src/io/ktor/http/header/AcceptEncoding.kt | 126 |  |
| ktor-http/common/src/io/ktor/http/header/utils.kt | 74 |  |
| ktor-http/common/src/io/ktor/http/parsing/Debug.kt | 49 |  |
| ktor-http/common/src/io/ktor/http/parsing/GrammarBuilder.kt | 35 |  |
| ktor-http/common/src/io/ktor/http/parsing/ParseException.kt | 18 |  |
| ktor-http/common/src/io/ktor/http/parsing/Parser.kt | 20 |  |
| ktor-http/common/src/io/ktor/http/parsing/ParserDsl.kt | 62 |  |
| ktor-http/common/src/io/ktor/http/parsing/Primitives.kt | 14 |  |
| ktor-http/common/src/io/ktor/http/parsing/regex/RegexParser.kt | 32 |  |
| ktor-http/common/src/io/ktor/http/parsing/regex/RegexParserGenerator.kt | 76 |  |
| ktor-http/common/src/io/ktor/http/websocket/Utils.kt | 20 |  |
| ktor-http/jvm/src/io/ktor/http/FileContentTypeJvm.kt | 25 |  |
| ktor-http/jvm/src/io/ktor/http/HttpMessagePropertiesJvm.kt | 60 |  |
| ktor-http/jvm/src/io/ktor/http/URLBuilderJvm.kt | 25 |  |
| ktor-http/jvm/src/io/ktor/http/URLUtilsJvm.kt | 74 |  |
| ktor-http/jvm/src/io/ktor/http/auth/DigestAlgorithm.jvm.kt | 18 |  |
| ktor-http/jvm/src/io/ktor/http/auth/SignatureAlgorithm.jvm.kt | 18 |  |
| ktor-http/jvm/src/io/ktor/http/content/BlockingBridge.kt | 80 |  |
| ktor-http/jvm/src/io/ktor/http/content/MultipartJvm.kt | 19 |  |
| ktor-http/jvm/src/io/ktor/http/content/OutputStreamContent.kt | 45 |  |
| ktor-http/jvm/src/io/ktor/http/content/URIFileContent.kt | 32 |  |
| ktor-http/jvm/src/io/ktor/http/content/VersionsJvm.kt | 26 |  |
| ktor-http/jvm/src/io/ktor/http/content/WriterContent.kt | 27 |  |
| ktor-io/common/src/io/ktor/utils/io/Annotations.kt | 112 |  |
| ktor-io/common/src/io/ktor/utils/io/BufferedByteWriteChannel.kt | 22 |  |
| ktor-io/common/src/io/ktor/utils/io/ByteChannel.kt | 325 |  |
| ktor-io/common/src/io/ktor/utils/io/ByteChannelCtor.kt | 29 |  |
| ktor-io/common/src/io/ktor/utils/io/ByteChannelScanner.kt | 175 |  |
| ktor-io/common/src/io/ktor/utils/io/ByteChannelUtils.kt | 41 |  |
| ktor-io/common/src/io/ktor/utils/io/ByteReadChannel.kt | 60 |  |
| ktor-io/common/src/io/ktor/utils/io/ByteReadChannelOperations.kt | 966 |  |
| ktor-io/common/src/io/ktor/utils/io/ByteWriteChannel.kt | 66 |  |
| ktor-io/common/src/io/ktor/utils/io/ByteWriteChannelOperations.kt | 243 |  |
| ktor-io/common/src/io/ktor/utils/io/CloseHookByteWriteChannel.kt | 29 |  |
| ktor-io/common/src/io/ktor/utils/io/CloseToken.kt | 31 |  |
| ktor-io/common/src/io/ktor/utils/io/CountedByteReadChannel.kt | 71 |  |
| ktor-io/common/src/io/ktor/utils/io/CountedByteWriteChannel.kt | 46 |  |
| ktor-io/common/src/io/ktor/utils/io/Deprecation.kt | 23 |  |
| ktor-io/common/src/io/ktor/utils/io/Exceptions.kt | 38 |  |
| ktor-io/common/src/io/ktor/utils/io/IODispatcher.kt | 9 |  |
| ktor-io/common/src/io/ktor/utils/io/JvmSerializable.kt | 25 |  |
| ktor-io/common/src/io/ktor/utils/io/LineEnding.kt | 30 |  |
| ktor-io/common/src/io/ktor/utils/io/LineEndingMode.kt | 76 |  |
| ktor-io/common/src/io/ktor/utils/io/SinkByteWriteChannel.kt | 69 |  |
| ktor-io/common/src/io/ktor/utils/io/SourceByteReadChannel.kt | 41 |  |
| ktor-io/common/src/io/ktor/utils/io/bits/ByteOrder.kt | 73 |  |
| ktor-io/common/src/io/ktor/utils/io/charsets/Encoding.kt | 133 |  |
| ktor-io/common/src/io/ktor/utils/io/core/Buffer.kt | 26 |  |
| ktor-io/common/src/io/ktor/utils/io/core/BufferAppend.kt | 25 |  |
| ktor-io/common/src/io/ktor/utils/io/core/Buffers.kt | 21 |  |
| ktor-io/common/src/io/ktor/utils/io/core/Builder.kt | 24 |  |
| ktor-io/common/src/io/ktor/utils/io/core/ByteOrder.kt | 14 |  |
| ktor-io/common/src/io/ktor/utils/io/core/BytePacketBuilder.kt | 35 |  |
| ktor-io/common/src/io/ktor/utils/io/core/ByteReadPacket.kt | 102 |  |
| ktor-io/common/src/io/ktor/utils/io/core/Closeable.kt | 13 |  |
| ktor-io/common/src/io/ktor/utils/io/core/Copy.kt | 22 |  |
| ktor-io/common/src/io/ktor/utils/io/core/Input.kt | 15 |  |
| ktor-io/common/src/io/ktor/utils/io/core/Memory.kt | 19 |  |
| ktor-io/common/src/io/ktor/utils/io/core/Output.kt | 15 |  |
| ktor-io/common/src/io/ktor/utils/io/core/Packet.kt | 29 |  |
| ktor-io/common/src/io/ktor/utils/io/core/Strings.kt | 243 |  |
| ktor-io/common/src/io/ktor/utils/io/core/internal/CharArraySequence.kt | 34 |  |
| ktor-io/common/src/io/ktor/utils/io/core/internal/ChunkBuffer.kt | 13 |  |
| ktor-io/common/src/io/ktor/utils/io/core/internal/Numbers.kt | 16 |  |
| ktor-io/common/src/io/ktor/utils/io/errors/Exceptions.kt | 11 |  |
| ktor-io/common/src/io/ktor/utils/io/locks/Synchronized.kt | 103 |  |
| ktor-io/common/src/io/ktor/utils/io/pool/ByteArrayPool.kt | 12 |  |
| ktor-io/common/src/io/ktor/utils/io/pool/Pool.kt | 188 |  |
| ktor-io/jvm/src/io/ktor/utils/io/ByteChannel.jvm.kt | 10 |  |
| ktor-io/jvm/src/io/ktor/utils/io/ByteReadChannelOperations.jvm.kt | 207 |  |
| ktor-io/jvm/src/io/ktor/utils/io/ByteWriteChannelOperations.jvm.kt | 73 |  |
| ktor-io/jvm/src/io/ktor/utils/io/IODispatcher.jvm.kt | 10 |  |
| ktor-io/jvm/src/io/ktor/utils/io/JvmSerializable.jvm.kt | 45 |  |
| ktor-io/jvm/src/io/ktor/utils/io/LookAheadSession.kt | 58 |  |
| ktor-io/jvm/src/io/ktor/utils/io/bits/ByteOrderJvm.kt | 53 |  |
| ktor-io/jvm/src/io/ktor/utils/io/charsets/CharsetJVM.kt | 103 |  |
| ktor-io/jvm/src/io/ktor/utils/io/core/BufferPrimitivesJvm.kt | 22 |  |
| ktor-io/jvm/src/io/ktor/utils/io/core/ByteOrderJVM.kt | 25 |  |
| ktor-io/jvm/src/io/ktor/utils/io/core/BytePacketBuilderExtensions.jvm.kt | 13 |  |
| ktor-io/jvm/src/io/ktor/utils/io/core/ByteReadPacketExtensions.jvm.kt | 35 |  |
| ktor-io/jvm/src/io/ktor/utils/io/core/Closeable.jvm.kt | 7 |  |
| ktor-io/jvm/src/io/ktor/utils/io/core/OutputArraysJVM.kt | 10 |  |
| ktor-io/jvm/src/io/ktor/utils/io/core/Strings.jvm.kt | 31 |  |
| ktor-io/jvm/src/io/ktor/utils/io/core/internal/ChunkBufferJvm.kt | 30 |  |
| ktor-io/jvm/src/io/ktor/utils/io/jvm/javaio/Blocking.kt | 162 |  |
| ktor-io/jvm/src/io/ktor/utils/io/jvm/javaio/Reading.kt | 119 |  |
| ktor-io/jvm/src/io/ktor/utils/io/jvm/javaio/Writing.kt | 31 |  |
| ktor-io/jvm/src/io/ktor/utils/io/jvm/nio/Reading.kt | 76 |  |
| ktor-io/jvm/src/io/ktor/utils/io/jvm/nio/WriteSuspendSession.kt | 61 |  |
| ktor-io/jvm/src/io/ktor/utils/io/locks/Synchronized.kt | 84 |  |
| ktor-io/jvm/src/io/ktor/utils/io/pool/ByteBufferPools.kt | 47 |  |
| ktor-io/jvm/src/io/ktor/utils/io/pool/DefaultPool.kt | 110 |  |
| ktor-io/jvm/src/io/ktor/utils/io/streams/Streams.kt | 55 |  |
| ktor-io/jvmAndPosix/src/io/ktor/utils/io/ByteReadChannelSource.kt | 41 |  |
| ktor-io/jvmAndPosix/src/io/ktor/utils/io/ByteWriteChannelSink.kt | 49 |  |
| ktor-utils/common/src/io/ktor/util/Attributes.kt | 154 |  |
| ktor-utils/common/src/io/ktor/util/Base64.kt | 90 |  |
| ktor-utils/common/src/io/ktor/util/ByteChannels.kt | 89 |  |
| ktor-utils/common/src/io/ktor/util/Bytes.kt | 18 |  |
| ktor-utils/common/src/io/ktor/util/CaseInsensitiveMap.kt | 447 |  |
| ktor-utils/common/src/io/ktor/util/CaseInsensitiveSet.kt | 67 |  |
| ktor-utils/common/src/io/ktor/util/Charset.kt | 19 |  |
| ktor-utils/common/src/io/ktor/util/Collections.kt | 19 |  |
| ktor-utils/common/src/io/ktor/util/ContentEncoder.kt | 88 |  |
| ktor-utils/common/src/io/ktor/util/CoroutinesUtils.kt | 32 |  |
| ktor-utils/common/src/io/ktor/util/Crypto.kt | 175 |  |
| ktor-utils/common/src/io/ktor/util/DelegatingMutableSet.kt | 58 |  |
| ktor-utils/common/src/io/ktor/util/Encoders.kt | 67 |  |
| ktor-utils/common/src/io/ktor/util/Hash.kt | 9 |  |
| ktor-utils/common/src/io/ktor/util/HashFunction.kt | 439 |  |
| ktor-utils/common/src/io/ktor/util/NonceManager.kt | 48 |  |
| ktor-utils/common/src/io/ktor/util/PlatformUtils.kt | 45 |  |
| ktor-utils/common/src/io/ktor/util/Ranges.kt | 13 |  |
| ktor-utils/common/src/io/ktor/util/StackFrames.kt | 21 |  |
| ktor-utils/common/src/io/ktor/util/StringValues.kt | 675 |  |
| ktor-utils/common/src/io/ktor/util/Text.kt | 129 |  |
| ktor-utils/common/src/io/ktor/util/Throwable.kt | 22 |  |
| ktor-utils/common/src/io/ktor/util/annotations/Annotations.kt | 21 |  |
| ktor-utils/common/src/io/ktor/util/cio/Channels.kt | 35 |  |
| ktor-utils/common/src/io/ktor/util/cio/Readers.kt | 41 |  |
| ktor-utils/common/src/io/ktor/util/collections/CollectionUtils.kt | 26 |  |
| ktor-utils/common/src/io/ktor/util/collections/ConcurrentMap.kt | 55 |  |
| ktor-utils/common/src/io/ktor/util/collections/ConcurrentSet.kt | 66 |  |
| ktor-utils/common/src/io/ktor/util/collections/CopyOnWriteHashMap.kt | 84 |  |
| ktor-utils/common/src/io/ktor/util/collections/MapDelegates.kt | 101 |  |
| ktor-utils/common/src/io/ktor/util/collections/TreeLike.kt | 53 |  |
| ktor-utils/common/src/io/ktor/util/converters/ConversionService.kt | 146 |  |
| ktor-utils/common/src/io/ktor/util/converters/DataConversion.kt | 135 |  |
| ktor-utils/common/src/io/ktor/util/date/Date.kt | 185 |  |
| ktor-utils/common/src/io/ktor/util/date/GMTDateParser.kt | 141 |  |
| ktor-utils/common/src/io/ktor/util/debug/ContextUtils.kt | 53 |  |
| ktor-utils/common/src/io/ktor/util/debug/IntellijIdeaDebugDetector.kt | 13 |  |
| ktor-utils/common/src/io/ktor/util/debug/plugins/PluginName.kt | 34 |  |
| ktor-utils/common/src/io/ktor/util/debug/plugins/PluginsTrace.kt | 48 |  |
| ktor-utils/common/src/io/ktor/util/internal/ExceptionUtils.kt | 12 |  |
| ktor-utils/common/src/io/ktor/util/internal/LockFreeLinkedList.kt | 833 |  |
| ktor-utils/common/src/io/ktor/util/logging/KtorSimpleLogger.kt | 8 |  |
| ktor-utils/common/src/io/ktor/util/logging/Logger.kt | 68 |  |
| ktor-utils/common/src/io/ktor/util/network/NetworkAddress.kt | 53 |  |
| ktor-utils/common/src/io/ktor/util/pipeline/DebugPipelineContext.kt | 84 |  |
| ktor-utils/common/src/io/ktor/util/pipeline/PhaseContent.kt | 79 |  |
| ktor-utils/common/src/io/ktor/util/pipeline/Pipeline.kt | 537 |  |
| ktor-utils/common/src/io/ktor/util/pipeline/PipelineContext.kt | 69 |  |
| ktor-utils/common/src/io/ktor/util/pipeline/PipelinePhase.kt | 23 |  |
| ktor-utils/common/src/io/ktor/util/pipeline/PipelinePhaseRelation.kt | 27 |  |
| ktor-utils/common/src/io/ktor/util/pipeline/StackTraceRecover.kt | 22 |  |
| ktor-utils/common/src/io/ktor/util/pipeline/StackWalkingFailed.kt | 16 |  |
| ktor-utils/common/src/io/ktor/util/pipeline/StackWalkingFailedFrame.kt | 33 |  |
| ktor-utils/common/src/io/ktor/util/pipeline/SuspendFunctionGun.kt | 208 |  |
| ktor-utils/common/src/io/ktor/util/reflect/Type.kt | 126 |  |
| ktor-utils/jvm/src/io/ktor/util/AttributesJvm.kt | 69 |  |
| ktor-utils/jvm/src/io/ktor/util/BufferViewJvm.kt | 47 |  |
| ktor-utils/jvm/src/io/ktor/util/Cache.kt | 60 |  |
| ktor-utils/jvm/src/io/ktor/util/CollectionsJvm.kt | 14 |  |
| ktor-utils/jvm/src/io/ktor/util/ContentEncodersJvm.kt | 23 |  |
| ktor-utils/jvm/src/io/ktor/util/CryptoJvm.kt | 133 |  |
| ktor-utils/jvm/src/io/ktor/util/Deflater.kt | 136 |  |
| ktor-utils/jvm/src/io/ktor/util/EncodersJvm.kt | 184 |  |
| ktor-utils/jvm/src/io/ktor/util/InputJvm.kt | 33 |  |
| ktor-utils/jvm/src/io/ktor/util/NIO.kt | 74 |  |
| ktor-utils/jvm/src/io/ktor/util/NioPath.kt | 59 |  |
| ktor-utils/jvm/src/io/ktor/util/Nonce.kt | 190 |  |
| ktor-utils/jvm/src/io/ktor/util/Path.kt | 101 |  |
| ktor-utils/jvm/src/io/ktor/util/PlatformUtilsJvm.kt | 13 |  |
| ktor-utils/jvm/src/io/ktor/util/StackFramesJvm.kt | 27 |  |
| ktor-utils/jvm/src/io/ktor/util/StatelessHmacNonceManager.kt | 157 |  |
| ktor-utils/jvm/src/io/ktor/util/cio/ByteBufferPool.kt | 26 |  |
| ktor-utils/jvm/src/io/ktor/util/cio/FileChannels.kt | 114 |  |
| ktor-utils/jvm/src/io/ktor/util/cio/FileChannelsAtNioPath.kt | 39 |  |
| ktor-utils/jvm/src/io/ktor/util/cio/InputStreamAdapters.kt | 33 |  |
| ktor-utils/jvm/src/io/ktor/util/cio/OutputStreamAdapters.kt | 27 |  |
| ktor-utils/jvm/src/io/ktor/util/cio/ReadersJvm.kt | 26 |  |
| ktor-utils/jvm/src/io/ktor/util/collections/ConcurrentMapJvm.kt | 67 |  |
| ktor-utils/jvm/src/io/ktor/util/converters/ConversionServiceJvm.kt | 62 |  |
| ktor-utils/jvm/src/io/ktor/util/date/DateJvm.kt | 161 |  |
| ktor-utils/jvm/src/io/ktor/util/debug/IntellijIdeaDebugDetectorJvm.kt | 19 |  |
| ktor-utils/jvm/src/io/ktor/util/internal/ExceptionUtilsJvm.kt | 14 |  |
| ktor-utils/jvm/src/io/ktor/util/logging/KtorSimpleLoggerJvm.kt | 269 |  |
| ktor-utils/jvm/src/io/ktor/util/logging/LoggerJvm.kt | 10 |  |
| ktor-utils/jvm/src/io/ktor/util/network/NetworkAddressJvm.kt | 34 |  |
| ktor-utils/jvm/src/io/ktor/util/pipeline/PipelineContext.jvm.kt | 8 |  |
| ktor-utils/jvm/src/io/ktor/util/pipeline/PipelineJvm.kt | 15 |  |
| ktor-utils/jvm/src/io/ktor/util/pipeline/StackTraceRecoverJvm.kt | 7 |  |
| ktor-utils/jvm/src/io/ktor/util/reflect/ServiceLoader.kt | 47 |  |
| ktor-utils/jvm/src/io/ktor/util/reflect/TypeInfoJvm.kt | 44 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 78 files, 9697 lines

Directories: `ktor-io/common/test`, `ktor-io/jvm/test`, `ktor-utils/common/test`, `ktor-utils/jvm/test`, `ktor-http/common/test`, `ktor-http/jvm/test`

### Features left, by files blocked

78 of 78 files use no feature that kc doesn't support.

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
| `apply` | 8 | 38 |
| `writer` | 7 | 24 |
| `reader` | 5 | 12 |
| `also` | 4 | 10 |
| `buildString` | 4 | 34 |
| `let` | 4 | 11 |
| `runCatching` | 3 | 5 |
| `suspend` | 3 | 3 |
| `use` | 3 | 8 |
| `with` | 3 | 5 |
| `copyOfRange` | 2 | 8 |
| `inputStream` | 2 | 8 |
| `List` | 1 | 1 |
| `charset` | 1 | 6 |
| `error` | 1 | 1 |
| `getOrElse` | 1 | 1 |
| `orEmpty` | 1 | 2 |
| `printStackTrace` | 1 | 1 |
| `replaceFirstChar` | 1 | 1 |
| `run` | 1 | 1 |

### Files

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| ktor-http/common/test/io/ktor/tests/http/AuthAlgorithmsTest.kt | 108 |  |
| ktor-http/common/test/io/ktor/tests/http/AuthHeaderParseTest.kt | 89 |  |
| ktor-http/common/test/io/ktor/tests/http/CodecTest.kt | 164 |  |
| ktor-http/common/test/io/ktor/tests/http/CommonHeadersTest.kt | 482 |  |
| ktor-http/common/test/io/ktor/tests/http/ContentDispositionTest.kt | 41 |  |
| ktor-http/common/test/io/ktor/tests/http/ContentTypeLookupTest.kt | 122 |  |
| ktor-http/common/test/io/ktor/tests/http/ContentTypeMatchTest.kt | 73 |  |
| ktor-http/common/test/io/ktor/tests/http/ContentTypeTest.kt | 166 |  |
| ktor-http/common/test/io/ktor/tests/http/CookieDateParserTest.kt | 114 |  |
| ktor-http/common/test/io/ktor/tests/http/HttpProtocolVersionTest.kt | 46 |  |
| ktor-http/common/test/io/ktor/tests/http/HttpStatusCodeTest.kt | 35 |  |
| ktor-http/common/test/io/ktor/tests/http/MimesTest.kt | 32 |  |
| ktor-http/common/test/io/ktor/tests/http/ParseClientCookiesHeaderTest.kt | 80 |  |
| ktor-http/common/test/io/ktor/tests/http/QueryParametersTest.kt | 117 |  |
| ktor-http/common/test/io/ktor/tests/http/RangesTest.kt | 278 |  |
| ktor-http/common/test/io/ktor/tests/http/RenderSetCookieTest.kt | 51 |  |
| ktor-http/common/test/io/ktor/tests/http/URLBuilderTest.kt | 517 |  |
| ktor-http/common/test/io/ktor/tests/http/URLParserTest.kt | 113 |  |
| ktor-http/common/test/io/ktor/tests/http/UrlDecodedParametersBuilderTest.kt | 135 |  |
| ktor-http/common/test/io/ktor/tests/http/UrlEncodeTest.kt | 110 |  |
| ktor-http/common/test/io/ktor/tests/http/UrlTest.kt | 475 |  |
| ktor-http/common/test/io/ktor/tests/http/header/AcceptEncodingTest.kt | 136 |  |
| ktor-http/jvm/test/io/ktor/tests/http/DigestAlgorithmJvmTest.kt | 23 |  |
| ktor-http/jvm/test/io/ktor/tests/http/SerializableTest.kt | 21 |  |
| ktor-http/jvm/test/io/ktor/tests/http/URLBuilderTest.kt | 120 |  |
| ktor-http/jvm/test/io/ktor/tests/http/content/BlockingBridgeTest.kt | 90 |  |
| ktor-http/jvm/test/io/ktor/tests/http/content/MultipartJvmTest.kt | 25 |  |
| ktor-io/common/test/ByteChannelTest.kt | 149 |  |
| ktor-io/common/test/ByteReadChannelOperationsTest.kt | 364 |  |
| ktor-io/common/test/CharsetDecoderTest.kt | 76 |  |
| ktor-io/common/test/CoroutinesTest.kt | 34 |  |
| ktor-io/common/test/ReadLineTest.kt | 448 |  |
| ktor-io/common/test/ReadUtf8LineTest.kt | 102 |  |
| ktor-io/common/test/SinkByteWriteChannelTest.kt | 76 |  |
| ktor-io/common/test/StringsTest.kt | 244 |  |
| ktor-io/common/test/StringsToByteArrayTest.kt | 45 |  |
| ktor-io/common/test/WriterReaderTest.kt | 130 |  |
| ktor-io/jvm/test/ByteChannelConcurrentTest.kt | 72 |  |
| ktor-io/jvm/test/ByteReadChannelOperationsJvmTest.kt | 117 |  |
| ktor-io/jvm/test/ByteReadChannelReadTest.jvm.kt | 21 |  |
| ktor-io/jvm/test/ByteReadPacketExtensionsJvmTest.kt | 180 |  |
| ktor-io/jvm/test/LookAheadSessionTest.kt | 33 |  |
| ktor-io/jvm/test/StringsToByteArrayJvmTest.kt | 24 |  |
| ktor-io/jvm/test/io/ktor/utils/io/jvm/javaio/ByteChannelInputStreamTest.kt | 71 |  |
| ktor-io/jvm/test/io/ktor/utils/io/jvm/javaio/ByteChannelOutputStreamTest.kt | 300 |  |
| ktor-io/jvm/test/io/ktor/utils/io/jvm/javaio/RawSourceChannelTest.kt | 51 |  |
| ktor-io/jvm/test/io/ktor/utils/io/jvm/nio/ReadingTest.kt | 37 |  |
| ktor-utils/common/test/io/ktor/util/AttributesTest.kt | 43 |  |
| ktor-utils/common/test/io/ktor/util/Base64Test.kt | 88 |  |
| ktor-utils/common/test/io/ktor/util/CaseInsensitiveMapTest.kt | 111 |  |
| ktor-utils/common/test/io/ktor/util/ChannelTest.kt | 157 |  |
| ktor-utils/common/test/io/ktor/util/GMTDateParserTest.kt | 52 |  |
| ktor-utils/common/test/io/ktor/util/GMTDateTest.kt | 57 |  |
| ktor-utils/common/test/io/ktor/util/HashFunctionTest.kt | 28 |  |
| ktor-utils/common/test/io/ktor/util/NonceTest.kt | 40 |  |
| ktor-utils/common/test/io/ktor/util/PipelineContractsTest.kt | 327 |  |
| ktor-utils/common/test/io/ktor/util/PipelinePhasesTest.kt | 91 |  |
| ktor-utils/common/test/io/ktor/util/PipelineTest.kt | 435 |  |
| ktor-utils/common/test/io/ktor/util/StringValuesBuilderTest.kt | 74 |  |
| ktor-utils/common/test/io/ktor/util/StringValuesTest.kt | 157 |  |
| ktor-utils/common/test/io/ktor/util/SuspendFunctionGunTest.kt | 40 |  |
| ktor-utils/common/test/io/ktor/util/collections/ConcurrentSetTest.kt | 119 |  |
| ktor-utils/common/test/io/ktor/util/converters/DataConversionTest.kt | 134 |  |
| ktor-utils/common/test/io/ktor/util/reflect/TypeInfoTest.kt | 68 |  |
| ktor-utils/jvm/test/io/ktor/tests/utils/CacheTest.kt | 109 |  |
| ktor-utils/jvm/test/io/ktor/tests/utils/DeflaterReadChannelTest.kt | 184 |  |
| ktor-utils/jvm/test/io/ktor/tests/utils/DropLeadingTopDirsTest.kt | 70 |  |
| ktor-utils/jvm/test/io/ktor/tests/utils/EncodersJvmTest.kt | 53 |  |
| ktor-utils/jvm/test/io/ktor/tests/utils/FileChannelTest.kt | 152 |  |
| ktor-utils/jvm/test/io/ktor/tests/utils/HexFunctionsTest.kt | 48 |  |
| ktor-utils/jvm/test/io/ktor/tests/utils/InputJvmTest.kt | 59 |  |
| ktor-utils/jvm/test/io/ktor/tests/utils/NonceSmokeTest.kt | 68 |  |
| ktor-utils/jvm/test/io/ktor/tests/utils/StackWalkingFailedTest.kt | 66 |  |
| ktor-utils/jvm/test/io/ktor/tests/utils/StatelessHmacNonceManagerTest.kt | 48 |  |
| ktor-utils/jvm/test/io/ktor/tests/utils/converters/DataConversionTest.kt | 80 |  |
| ktor-utils/jvm/test/io/ktor/util/GMTDateCalendarTest.kt | 215 |  |
| ktor-utils/jvm/test/io/ktor/util/HashFunctionConsistencyTest.kt | 56 |  |
| ktor-utils/jvm/test/io/ktor/util/PipelineContractsJvmTest.kt | 131 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

