# Survey: ktor 3.6.0

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 236 files, 23774 lines

Directories: `ktor-io/common/src`, `ktor-io/jvmAndPosix/src`, `ktor-io/jvm/src`, `ktor-utils/common/src`, `ktor-utils/jvm/src`, `ktor-http/common/src`, `ktor-http/jvm/src`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 236 | 7 |
| 1: packages, annotations | 1 | 235 | 6 |
| 2: interfaces, inheritance | 6 | 230 | 4 |
| 3: type checks | 7 | 229 | 4 |
| 4: exceptions | 7 | 229 | 4 |
| 5: objects | 10 | 226 | 3 |
| 6: data, enum, sealed | 19 | 217 | 3 |
| 7: arrays, vararg, spread | 23 | 213 | 3 |
| 8: Char, Float, Byte, Short | 24 | 212 | 3 |
| 9: inliner | 29 | 207 | 2 |
| 10: lambdas with receivers | 29 | 207 | 2 |
| 11: extensions | 61 | 175 | 2 |
| 12: operator conventions | 63 | 173 | 2 |
| 13: identity, class literals, references | 76 | 160 | 1 |
| 14: class delegation | 76 | 160 | 1 |
| 15: secondary constructors | 84 | 152 | 1 |
| 16: local functions | 85 | 151 | 1 |
| 17: expect and actual | 113 | 123 | 1 |
| 18: annotation classes | 115 | 121 | 1 |
| 19: value classes | 116 | 120 | 1 |
| 20: unsigned integers | 117 | 119 | 1 |
| definite assignment | 120 | 116 | 1 |
| 21: inline functions | 133 | 103 | 1 |
| 22: reified type parameters | 135 | 101 | 1 |
| 23: labels, do-while | 137 | 99 | 1 |
| 24: delegated properties | 141 | 95 | 1 |
| 25: object expressions | 145 | 91 | 1 |
| 26: destructuring lambda parameters | 149 | 87 | 1 |
| 42-49: coroutines | 177 | 59 | 1 |
| 52: bitwise operators | 191 | 45 | 1 |
| 53: typealias | 211 | 25 | 1 |
| 54: setters, backing fields | 214 | 22 | 1 |
| 55: lateinit, tailrec | 215 | 21 | 1 |
| 56-57: collection supertypes | 219 | 17 | 1 |
| 58: member extension functions | 220 | 16 | 1 |
| 59: reified member type parameters | 222 | 14 | 1 |
| 60: inner and local classes | 223 | 13 | 1 |
| 61: when guards, anonymous functions, local extension functions | 230 | 6 | 1 |
| 62a: @JvmField and @JvmStatic in objects | 232 | 4 | 1 |
| 62b: generic value classes | 232 | 4 | 1 |
| 63: definitely non-nullable types | 232 | 4 | 1 |
| 70a: inline property accessors | 232 | 4 | 1 |
| 70b: where clauses | 232 | 4 | 1 |
| 70c: fun interfaces | 232 | 4 | 1 |
| delegated extension properties | 232 | 4 | 1 |
| qualified super | 232 | 4 | 1 |
| also supported | 236 | 0 | – |

### Features left, by files blocked

236 of 236 files use no feature that kc doesn't support.

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

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| ktor-http/common/src/io/ktor/content/Compatibility.kt | 11 | (annotation, package, typealias) |
| ktor-http/common/src/io/ktor/http/ApplicationResponseProperties.kt | 12 | (extension function, package) |
| ktor-http/common/src/io/ktor/http/AsciiBitSet.kt | 42 | (Byte and Short, Char, bitwise operators, modifier companion, modifier operator, package, require, check, error) |
| ktor-http/common/src/io/ktor/http/CacheControl.kt | 135 | (default value, identity equality, inheritance, is, modifier abstract, modifier enum, modifier override, nested class, package) |
| ktor-http/common/src/io/ktor/http/Codecs.kt | 322 | (Byte and Short, Char, arrays, bitwise operators, default value, extension function, inheritance, inline-only call, named argument, package, qualified this, throw) |
| ktor-http/common/src/io/ktor/http/ContentDisposition.kt | 123 | (annotation, default value, inheritance, is, modifier companion, modifier const, modifier override, named argument, object, package, property getter) |
| ktor-http/common/src/io/ktor/http/ContentRange.kt | 40 | (Char, default value, inline-only call, package) |
| ktor-http/common/src/io/ktor/http/ContentTypeMatcher.kt | 19 | (function without body, interface, package) |
| ktor-http/common/src/io/ktor/http/ContentTypes.kt | 526 | (Char, annotation, default value, destructuring declaration, extension function, inheritance, inline-only call, is, modifier companion, modifier const, modifier operator, modifier override, named argument, object, package, secondary constructor, throw, try) |
| ktor-http/common/src/io/ktor/http/Cookie.kt | 301 | (Char, annotation, arrays, class literal, default value, extension function, inheritance, inline-only call, modifier data, modifier enum, modifier inline, modifier override, named argument, object, package, throw) |
| ktor-http/common/src/io/ktor/http/CookieUtils.kt | 348 | (Char, extension function, function type with receiver, inheritance, inline-only call, modifier inline, named argument, package, throw) |
| ktor-http/common/src/io/ktor/http/DateUtils.kt | 73 | (Char, extension function, inline-only call, package, qualified this, require, check, error, try) |
| ktor-http/common/src/io/ktor/http/FileContentType.kt | 111 | (Char, arrays, delegated property, destructuring lambda parameter, extension function, inline-only call, named argument, package, throw, try) |
| ktor-http/common/src/io/ktor/http/HeaderValueWithParameters.kt | 171 | (Char, annotation, class literal, default value, do-while, extension function, inline-only call, modifier abstract, modifier companion, modifier inline, modifier override, modifier protected, named argument, package, qualified this, when subject variable) |
| ktor-http/common/src/io/ktor/http/Headers.kt | 129 | (Char, default value, extension function, function type with receiver, inheritance, inline-only call, interface, modifier companion, modifier inline, modifier override, modifier vararg, object, package, super) |
| ktor-http/common/src/io/ktor/http/HttpHeaderValueParser.kt | 242 | (Char, default value, destructuring declaration, extension function, inline-only call, is, labels, local function, modifier data, modifier override, named argument, package, secondary constructor, when guard) |
| ktor-http/common/src/io/ktor/http/HttpHeaders.kt | 581 | (Char, annotation, arrays, bitwise operators, inheritance, inline-only call, modifier const, named argument, object, package, property getter, throw) |
| ktor-http/common/src/io/ktor/http/HttpMessage.kt | 35 | (interface, package, property without initializer) |
| ktor-http/common/src/io/ktor/http/HttpMessageProperties.kt | 190 | (Char, extension function, inline-only call, package) |
| ktor-http/common/src/io/ktor/http/HttpMethod.kt | 122 | (@JvmField in object, annotation, extension property, modifier companion, modifier data, modifier override, package, property getter) |
| ktor-http/common/src/io/ktor/http/HttpProtocolVersion.kt | 97 | (destructuring declaration, inline-only call, modifier companion, modifier data, modifier override, package, require, check, error, when guard) |
| ktor-http/common/src/io/ktor/http/HttpStatusCode.kt | 196 | (annotation, extension function, inheritance, is, modifier companion, modifier data, modifier override, named argument, package) |
| ktor-http/common/src/io/ktor/http/HttpUrlEncoded.kt | 91 | (annotation, default value, destructuring lambda parameter, extension function, inline-only call, named argument, package) |
| ktor-http/common/src/io/ktor/http/IpParser.kt | 21 | (bitwise operators, package) |
| ktor-http/common/src/io/ktor/http/LinkHeader.kt | 75 | (annotation, inheritance, modifier const, modifier vararg, object, package, property getter, secondary constructor) |
| ktor-http/common/src/io/ktor/http/Mimes.kt | 1019 | (Char, delegated property, destructuring declaration, inline-only call, modifier const, named argument, package, property getter) |
| ktor-http/common/src/io/ktor/http/Parameters.kt | 137 | (default value, extension function, function type with receiver, function without body, inheritance, inline-only call, interface, is, modifier companion, modifier inline, modifier operator, modifier override, modifier vararg, object, package, qualified this, throw) |
| ktor-http/common/src/io/ktor/http/Query.kt | 126 | (Char, annotation, default value, extension function, inheritance, inline-only call, modifier override, named argument, object expression, package, property getter) |
| ktor-http/common/src/io/ktor/http/Ranges.kt | 144 | (Char, arrays, destructuring declaration, extension function, inheritance, inline-only call, is, modifier data, modifier enum, modifier override, modifier sealed, nested class, package, try) |
| ktor-http/common/src/io/ktor/http/RangesSpecifier.kt | 85 | (default value, extension function, inline-only call, is, modifier data, modifier override, named argument, package, require, check, error, secondary constructor) |
| ktor-http/common/src/io/ktor/http/RequestConnectionPoint.kt | 141 | (annotation, interface, package, property without initializer) |
| ktor-http/common/src/io/ktor/http/URLBuilder.kt | 369 | (Char, annotation, default value, extension function, extension property, function type with receiver, inline-only call, modifier companion, modifier const, modifier expect, modifier override, modifier vararg, named argument, package, property backing field, property getter, property setter, property without initializer, require, check, error) |
| ktor-http/common/src/io/ktor/http/URLParser.kt | 307 | (Char, arrays, extension function, inheritance, inline-only call, labels, named argument, package, prefix increment, require, check, error, throw, try) |
| ktor-http/common/src/io/ktor/http/URLProtocol.kt | 90 | (annotation, class literal, extension function, inheritance, inline-only call, modifier companion, modifier data, package, require, check, error) |
| ktor-http/common/src/io/ktor/http/URLUtils.kt | 221 | (Char, annotation, destructuring lambda parameter, extension function, extension property, function type with receiver, inline-only call, named argument, package, property getter, try) |
| ktor-http/common/src/io/ktor/http/Url.kt | 302 | (Char, annotation, arrays, as, class literal, delegated property, extension property, identity equality, inheritance, inline-only call, modifier companion, modifier override, object, package, property getter, require, check, error) |
| ktor-http/common/src/io/ktor/http/UrlDecodedParametersBuilder.kt | 87 | (extension function, inheritance, inline-only call, modifier override, named argument, package) |
| ktor-http/common/src/io/ktor/http/auth/AuthScheme.kt | 68 | (modifier const, object, package) |
| ktor-http/common/src/io/ktor/http/auth/DigestAlgorithm.kt | 190 | (annotation, inline-only call, modifier companion, named argument, package, secondary constructor) |
| ktor-http/common/src/io/ktor/http/auth/HeaderValueEncoding.kt | 35 | (modifier enum, package) |
| ktor-http/common/src/io/ktor/http/auth/HttpAuthHeader.kt | 524 | (Char, annotation, default value, extension function, function without body, inheritance, inline-only call, is, modifier abstract, modifier companion, modifier const, modifier override, modifier sealed, named argument, nested class, object, package, require, check, error, secondary constructor, throw) |
| ktor-http/common/src/io/ktor/http/auth/KeyAlgorithm.kt | 47 | (annotation, modifier companion, modifier value, package) |
| ktor-http/common/src/io/ktor/http/auth/SignatureAlgorithm.kt | 156 | (default value, identity equality, inline-only call, is, modifier companion, modifier override, named argument, package) |
| ktor-http/common/src/io/ktor/http/content/ByteArrayContent.kt | 22 | (arrays, default value, inheritance, modifier override, package) |
| ktor-http/common/src/io/ktor/http/content/CachingOptions.kt | 35 | (default value, extension property, modifier data, package, property getter, property setter) |
| ktor-http/common/src/io/ktor/http/content/ChannelWriterContent.kt | 25 | (default value, function type with receiver, inheritance, modifier override, modifier suspend, package) |
| ktor-http/common/src/io/ktor/http/content/CompressedContent.kt | 100 | (default value, delegated property, extension function, inheritance, inline-only call, is, modifier override, modifier suspend, package, property getter) |
| ktor-http/common/src/io/ktor/http/content/Multipart.kt | 210 | (annotation, default value, delegated property, do-while, extension function, function without body, inheritance, inline-only call, interface, modifier override, modifier sealed, modifier suspend, nested class, object, package, secondary constructor, suspend function type) |
| ktor-http/common/src/io/ktor/http/content/OutgoingContent.kt | 222 | (annotation, arrays, class literal, extension function, function without body, inheritance, inline-only call, is, modifier abstract, modifier final, modifier open, modifier override, modifier sealed, modifier suspend, named argument, nested class, object, package, property getter, when guard) |
| ktor-http/common/src/io/ktor/http/content/TextContent.kt | 31 | (arrays, default value, inheritance, inline-only call, modifier override, package, property getter) |
| ktor-http/common/src/io/ktor/http/content/Versions.kt | 312 | (Char, annotation, extension function, extension property, function without body, inheritance, inline-only call, interface, modifier companion, modifier data, modifier enum, modifier override, package, property getter, property setter, require, check, error, try, uninitialized local variable) |
| ktor-http/common/src/io/ktor/http/header/AcceptEncoding.kt | 126 | (default value, destructuring declaration, inheritance, is, modifier companion, modifier override, modifier vararg, named argument, package, secondary constructor) |
| ktor-http/common/src/io/ktor/http/header/utils.kt | 74 | (Char, inline-only call, local function, package) |
| ktor-http/common/src/io/ktor/http/parsing/Debug.kt | 49 | (default value, extension function, is, package) |
| ktor-http/common/src/io/ktor/http/parsing/GrammarBuilder.kt | 35 | (anonymous function, extension function, function type with receiver, inline-only call, modifier infix, modifier operator, package) |
| ktor-http/common/src/io/ktor/http/parsing/ParseException.kt | 18 | (default value, inheritance, modifier override, package) |
| ktor-http/common/src/io/ktor/http/parsing/Parser.kt | 20 | (function without body, interface, modifier operator, package) |
| ktor-http/common/src/io/ktor/http/parsing/ParserDsl.kt | 62 | (Char, bitwise operators, extension function, function type with receiver, inheritance, inline-only call, interface, is, modifier infix, modifier inline, modifier override, modifier reified, modifier sealed, package, property without initializer) |
| ktor-http/common/src/io/ktor/http/parsing/Primitives.kt | 14 | (Char, bitwise operators, package) |
| ktor-http/common/src/io/ktor/http/parsing/regex/RegexParser.kt | 32 | (destructuring lambda parameter, inheritance, inline-only call, modifier override, package) |
| ktor-http/common/src/io/ktor/http/parsing/regex/RegexParserGenerator.kt | 76 | (Char, default value, extension function, inline-only call, is, named argument, package, require, check, error) |
| ktor-http/common/src/io/ktor/http/websocket/Utils.kt | 20 | (arrays, modifier const, package) |
| ktor-http/jvm/src/io/ktor/http/FileContentTypeJvm.kt | 25 | (extension function, package) |
| ktor-http/jvm/src/io/ktor/http/HttpMessagePropertiesJvm.kt | 60 | (extension function, inline-only call, package, property getter) |
| ktor-http/jvm/src/io/ktor/http/URLBuilderJvm.kt | 25 | (extension function, extension property, inline-only call, modifier actual, modifier operator, package) |
| ktor-http/jvm/src/io/ktor/http/URLUtilsJvm.kt | 74 | (Char, annotation, extension function, inline-only call, named argument, package) |
| ktor-http/jvm/src/io/ktor/http/auth/DigestAlgorithm.jvm.kt | 18 | (extension function, package) |
| ktor-http/jvm/src/io/ktor/http/auth/SignatureAlgorithm.jvm.kt | 18 | (extension function, package) |
| ktor-http/jvm/src/io/ktor/http/content/BlockingBridge.kt | 80 | (annotation, arrays, class literal, default value, extension function, inheritance, inline-only call, modifier const, modifier crossinline, modifier inline, modifier override, modifier suspend, named argument, package, suspend function type) |
| ktor-http/jvm/src/io/ktor/http/content/MultipartJvm.kt | 19 | (annotation, extension property, package) |
| ktor-http/jvm/src/io/ktor/http/content/OutputStreamContent.kt | 45 | (annotation, default value, function type with receiver, inheritance, modifier override, modifier suspend, named argument, package) |
| ktor-http/jvm/src/io/ktor/http/content/URIFileContent.kt | 32 | (default value, inheritance, modifier override, named argument, package, secondary constructor) |
| ktor-http/jvm/src/io/ktor/http/content/VersionsJvm.kt | 26 | (package) |
| ktor-http/jvm/src/io/ktor/http/content/WriterContent.kt | 27 | (default value, function type with receiver, inheritance, inline-only call, modifier override, modifier suspend, named argument, package) |
| ktor-io/common/src/io/ktor/utils/io/Annotations.kt | 112 | (annotation, modifier annotation, package) |
| ktor-io/common/src/io/ktor/utils/io/BufferedByteWriteChannel.kt | 22 | (annotation, function without body, inheritance, interface, package) |
| ktor-io/common/src/io/ktor/utils/io/ByteChannel.kt | 325 | (@JvmStatic property, annotation, callable reference, class literal, default value, function without body, inheritance, inline-only call, interface, is, modifier companion, modifier const, modifier crossinline, modifier data, modifier expect, modifier inline, modifier override, modifier reified, modifier sealed, modifier suspend, nested class, object, package, property getter, property without initializer, reified member type parameter, require, check, error, throw) |
| ktor-io/common/src/io/ktor/utils/io/ByteChannelCtor.kt | 29 | (arrays, default value, inline-only call, named argument, package) |
| ktor-io/common/src/io/ktor/utils/io/ByteChannelScanner.kt | 175 | (annotation, arrays, as, class literal, default value, extension function, modifier suspend, package, prefix increment, require, check, error, throw) |
| ktor-io/common/src/io/ktor/utils/io/ByteChannelUtils.kt | 41 | (annotation, as, class literal, extension function, named argument, package) |
| ktor-io/common/src/io/ktor/utils/io/ByteReadChannel.kt | 60 | (annotation, default value, extension function, function without body, inheritance, interface, modifier companion, modifier override, modifier suspend, object expression, package, property getter, property without initializer) |
| ktor-io/common/src/io/ktor/utils/io/ByteReadChannelOperations.kt | 966 | (Byte and Short, Char, Float, annotation, arrays, class literal, default value, extension function, extension property, function type with receiver, inheritance, inline-only call, local extension function, local function, modifier const, modifier crossinline, modifier inline, modifier override, modifier suspend, named argument, package, property getter, require, check, error, suspend function type, throw, try, uninitialized local variable) |
| ktor-io/common/src/io/ktor/utils/io/ByteWriteChannel.kt | 66 | (annotation, callable reference, extension function, function without body, interface, modifier suspend, package, property getter, property without initializer) |
| ktor-io/common/src/io/ktor/utils/io/ByteWriteChannelOperations.kt | 243 | (Byte and Short, Float, annotation, anonymous function, arrays, callable reference, class literal, default value, extension function, extension property, function type with receiver, inheritance, inline-only call, interface, modifier override, modifier suspend, object expression, package, property without initializer, suspend function type, try) |
| ktor-io/common/src/io/ktor/utils/io/CloseHookByteWriteChannel.kt | 29 | (class delegation, extension function, inheritance, inline-only call, modifier override, modifier suspend, package, property getter, suspend function type) |
| ktor-io/common/src/io/ktor/utils/io/CloseToken.kt | 31 | (annotation, callable reference, class literal, default value, extension function, inline-only call, is, modifier companion, modifier inline, package, throw) |
| ktor-io/common/src/io/ktor/utils/io/CountedByteReadChannel.kt | 71 | (annotation, class literal, extension function, extension property, inheritance, modifier override, modifier suspend, package, property getter, require, check, error) |
| ktor-io/common/src/io/ktor/utils/io/CountedByteWriteChannel.kt | 46 | (annotation, class literal, extension function, inheritance, modifier override, modifier suspend, package, property getter) |
| ktor-io/common/src/io/ktor/utils/io/Deprecation.kt | 23 | (annotation, extension function, modifier const, package) |
| ktor-io/common/src/io/ktor/utils/io/Exceptions.kt | 38 | (default value, inheritance, modifier open, package, typealias) |
| ktor-io/common/src/io/ktor/utils/io/IODispatcher.kt | 9 | (function without body, modifier expect, package) |
| ktor-io/common/src/io/ktor/utils/io/JvmSerializable.kt | 25 | (annotation, arrays, function without body, inheritance, interface, modifier expect, object, package) |
| ktor-io/common/src/io/ktor/utils/io/LineEnding.kt | 30 | (modifier enum, package) |
| ktor-io/common/src/io/ktor/utils/io/LineEndingMode.kt | 76 | (annotation, bitwise operators, modifier companion, modifier operator, modifier override, modifier value, package) |
| ktor-io/common/src/io/ktor/utils/io/SinkByteWriteChannel.kt | 69 | (annotation, class literal, extension function, inheritance, inline-only call, modifier override, modifier suspend, named argument, package, property getter, throw) |
| ktor-io/common/src/io/ktor/utils/io/SourceByteReadChannel.kt | 41 | (annotation, class literal, inheritance, inline-only call, modifier override, modifier suspend, package, property getter, throw) |
| ktor-io/common/src/io/ktor/utils/io/bits/ByteOrder.kt | 73 | (Byte and Short, Float, bitwise operators, extension function, extension property, function without body, modifier expect, modifier inline, package, unsigned integers) |
| ktor-io/common/src/io/ktor/utils/io/charsets/Encoding.kt | 133 | (annotation, arrays, class literal, default value, extension function, extension property, function without body, inheritance, inline-only call, modifier abstract, modifier expect, modifier final, modifier open, modifier override, object, package, property without initializer, require, check, error) |
| ktor-io/common/src/io/ktor/utils/io/core/Buffer.kt | 26 | (annotation, extension function, package, typealias) |
| ktor-io/common/src/io/ktor/utils/io/core/BufferAppend.kt | 25 | (annotation, extension function, package) |
| ktor-io/common/src/io/ktor/utils/io/core/Buffers.kt | 21 | (arrays, default value, extension function, inheritance, package) |
| ktor-io/common/src/io/ktor/utils/io/core/Builder.kt | 24 | (annotation, class literal, function type with receiver, modifier inline, package) |
| ktor-io/common/src/io/ktor/utils/io/core/ByteOrder.kt | 14 | (function without body, modifier companion, modifier enum, modifier expect, package) |
| ktor-io/common/src/io/ktor/utils/io/core/BytePacketBuilder.kt | 35 | (annotation, arrays, class literal, default value, extension function, extension property, package, typealias) |
| ktor-io/common/src/io/ktor/utils/io/core/ByteReadPacket.kt | 102 | (Byte and Short, annotation, arrays, class literal, default value, extension function, extension property, inline-only call, named argument, package, property getter, typealias) |
| ktor-io/common/src/io/ktor/utils/io/core/Closeable.kt | 13 | (annotation, extension function, inheritance, inline-only call, interface, modifier expect, modifier inline, package) |
| ktor-io/common/src/io/ktor/utils/io/core/Copy.kt | 22 | (annotation, extension function, package) |
| ktor-io/common/src/io/ktor/utils/io/core/Input.kt | 15 | (arrays, default value, extension function, extension property, package, property getter, typealias) |
| ktor-io/common/src/io/ktor/utils/io/core/Memory.kt | 19 | (annotation, arrays, bitwise operators, extension function, modifier value, package, typealias) |
| ktor-io/common/src/io/ktor/utils/io/core/Output.kt | 15 | (annotation, package, typealias) |
| ktor-io/common/src/io/ktor/utils/io/core/Packet.kt | 29 | (annotation, extension property, package, property getter) |
| ktor-io/common/src/io/ktor/utils/io/core/Strings.kt | 243 | (Byte and Short, annotation, arrays, bitwise operators, class literal, default value, extension function, function without body, identity equality, modifier expect, named argument, package, require, check, error, throw, try) |
| ktor-io/common/src/io/ktor/utils/io/core/internal/CharArraySequence.kt | 34 | (Char, annotation, arrays, inheritance, modifier final, modifier override, package, require, check, error, throw) |
| ktor-io/common/src/io/ktor/utils/io/core/internal/ChunkBuffer.kt | 13 | (annotation, extension property, package, typealias) |
| ktor-io/common/src/io/ktor/utils/io/core/internal/Numbers.kt | 16 | (annotation, extension function, modifier inline, package, throw) |
| ktor-io/common/src/io/ktor/utils/io/errors/Exceptions.kt | 11 | (annotation, package, typealias) |
| ktor-io/common/src/io/ktor/utils/io/locks/Synchronized.kt | 103 | (annotation, extension function, function without body, inline-only call, modifier expect, modifier inline, modifier open, package) |
| ktor-io/common/src/io/ktor/utils/io/pool/ByteArrayPool.kt | 12 | (arrays, inheritance, modifier const, modifier override, object expression, package) |
| ktor-io/common/src/io/ktor/utils/io/pool/Pool.kt | 188 | (annotation, extension function, function without body, identity equality, inheritance, inline-only call, interface, modifier abstract, modifier expect, modifier final, modifier inline, modifier open, modifier override, modifier protected, package, property getter, property without initializer, require, check, error, try) |
| ktor-io/jvm/src/io/ktor/utils/io/ByteChannel.jvm.kt | 10 | (inline-only call, modifier actual, modifier const, package, property getter) |
| ktor-io/jvm/src/io/ktor/utils/io/ByteReadChannelOperations.jvm.kt | 207 | (annotation, arrays, class literal, default value, extension function, inline-only call, is, modifier inline, modifier noinline, modifier suspend, named argument, package, require, check, error, throw) |
| ktor-io/jvm/src/io/ktor/utils/io/ByteWriteChannelOperations.jvm.kt | 73 | (annotation, class literal, default value, extension function, modifier suspend, package, require, check, error) |
| ktor-io/jvm/src/io/ktor/utils/io/IODispatcher.jvm.kt | 10 | (modifier actual, package) |
| ktor-io/jvm/src/io/ktor/utils/io/JvmSerializable.jvm.kt | 45 | (annotation, arrays, as, class literal, inheritance, modifier actual, modifier companion, modifier const, modifier override, package, secondary constructor, typealias) |
| ktor-io/jvm/src/io/ktor/utils/io/LookAheadSession.kt | 58 | (annotation, class literal, extension function, function type with receiver, modifier suspend, package, typealias) |
| ktor-io/jvm/src/io/ktor/utils/io/bits/ByteOrderJvm.kt | 53 | (Byte and Short, Float, annotation, extension function, modifier actual, modifier inline, package) |
| ktor-io/jvm/src/io/ktor/utils/io/charsets/CharsetJVM.kt | 103 | (annotation, arrays, as, extension function, extension property, inline-only call, is, modifier actual, modifier open, modifier override, package, require, check, error, typealias, uninitialized local variable) |
| ktor-io/jvm/src/io/ktor/utils/io/core/BufferPrimitivesJvm.kt | 22 | (annotation, extension function, package) |
| ktor-io/jvm/src/io/ktor/utils/io/core/ByteOrderJVM.kt | 25 | (identity equality, modifier actual, modifier companion, modifier enum, package) |
| ktor-io/jvm/src/io/ktor/utils/io/core/BytePacketBuilderExtensions.jvm.kt | 13 | (annotation, class literal, extension function, package) |
| ktor-io/jvm/src/io/ktor/utils/io/core/ByteReadPacketExtensions.jvm.kt | 35 | (annotation, class literal, extension function, inline-only call, package) |
| ktor-io/jvm/src/io/ktor/utils/io/core/Closeable.jvm.kt | 7 | (modifier actual, package, typealias) |
| ktor-io/jvm/src/io/ktor/utils/io/core/OutputArraysJVM.kt | 10 | (annotation, extension function, package) |
| ktor-io/jvm/src/io/ktor/utils/io/core/Strings.jvm.kt | 31 | (Float, annotation, class literal, extension function, modifier actual, package) |
| ktor-io/jvm/src/io/ktor/utils/io/core/internal/ChunkBufferJvm.kt | 30 | (annotation, class literal, extension function, package) |
| ktor-io/jvm/src/io/ktor/utils/io/jvm/javaio/Blocking.kt | 162 | (annotation, arrays, bitwise operators, class literal, default value, extension function, inheritance, modifier const, modifier override, named argument, package, throw, try) |
| ktor-io/jvm/src/io/ktor/utils/io/jvm/javaio/Reading.kt | 119 | (annotation, arrays, as, class literal, default value, extension function, inheritance, modifier override, modifier suspend, named argument, package, property getter, throw, try) |
| ktor-io/jvm/src/io/ktor/utils/io/jvm/javaio/Writing.kt | 31 | (annotation, class literal, default value, extension function, modifier suspend, package, require, check, error) |
| ktor-io/jvm/src/io/ktor/utils/io/jvm/nio/Reading.kt | 76 | (annotation, class literal, default value, extension function, inheritance, modifier open, modifier override, package, uninitialized local variable) |
| ktor-io/jvm/src/io/ktor/utils/io/jvm/nio/WriteSuspendSession.kt | 61 | (annotation, class literal, extension function, function type with receiver, modifier crossinline, modifier inline, modifier suspend, package, try) |
| ktor-io/jvm/src/io/ktor/utils/io/locks/Synchronized.kt | 84 | (annotation, extension function, inline-only call, modifier actual, modifier inline, package, try, typealias) |
| ktor-io/jvm/src/io/ktor/utils/io/pool/ByteBufferPools.kt | 47 | (default value, inheritance, inline-only call, modifier const, modifier override, package, require, check, error) |
| ktor-io/jvm/src/io/ktor/utils/io/pool/DefaultPool.kt | 110 | (arrays, bitwise operators, function without body, inline-only call, local function, modifier abstract, modifier actual, modifier const, modifier final, modifier open, modifier override, modifier protected, package, prefix increment, require, check, error) |
| ktor-io/jvm/src/io/ktor/utils/io/streams/Streams.kt | 55 | (annotation, class literal, default value, extension function, function type with receiver, inline-only call, package) |
| ktor-io/jvmAndPosix/src/io/ktor/utils/io/ByteReadChannelSource.kt | 41 | (annotation, class literal, extension function, inheritance, modifier override, package) |
| ktor-io/jvmAndPosix/src/io/ktor/utils/io/ByteWriteChannelSink.kt | 49 | (annotation, as, class literal, extension function, inheritance, modifier override, package) |
| ktor-utils/common/src/io/ktor/util/Attributes.kt | 154 | (annotation, as, default value, extension function, function without body, inline-only call, interface, modifier data, modifier expect, modifier inline, modifier operator, modifier override, modifier reified, package, property without initializer, require, check, error, throw, typealias) |
| ktor-utils/common/src/io/ktor/util/Base64.kt | 90 | (annotation, arrays, extension function, inline-only call, package) |
| ktor-utils/common/src/io/ktor/util/ByteChannels.kt | 89 | (annotation, class literal, extension function, inline-only call, modifier const, named argument, package, qualified this, throw, try) |
| ktor-utils/common/src/io/ktor/util/Bytes.kt | 18 | (Byte and Short, annotation, arrays, bitwise operators, extension function, package) |
| ktor-utils/common/src/io/ktor/util/CaseInsensitiveMap.kt | 447 | (annotation, arrays, as, bitwise operators, collection supertype, destructuring lambda parameter, identity equality, inheritance, inline-only call, is, modifier companion, modifier const, modifier inner, modifier override, named argument, nested class, object expression, package, property getter, qualified this, throw) |
| ktor-utils/common/src/io/ktor/util/CaseInsensitiveSet.kt | 67 | (annotation, collection supertype, inheritance, modifier override, package, property getter, secondary constructor) |
| ktor-utils/common/src/io/ktor/util/Charset.kt | 19 | (Char, arrays, extension function, package) |
| ktor-utils/common/src/io/ktor/util/Collections.kt | 19 | (extension function, function without body, modifier expect, package) |
| ktor-utils/common/src/io/ktor/util/ContentEncoder.kt | 88 | (class delegation, function without body, inheritance, interface, modifier expect, modifier override, object, package, property without initializer) |
| ktor-utils/common/src/io/ktor/util/CoroutinesUtils.kt | 32 | (annotation, default value, extension function, package) |
| ktor-utils/common/src/io/ktor/util/Crypto.kt | 175 | (annotation, arrays, bitwise operators, default value, extension function, function without body, interface, modifier const, modifier expect, modifier operator, modifier suspend, named argument, package) |
| ktor-utils/common/src/io/ktor/util/DelegatingMutableSet.kt | 58 | (collection supertype, extension function, function type with receiver, inheritance, is, member extension function, modifier open, modifier override, object expression, package) |
| ktor-utils/common/src/io/ktor/util/Encoders.kt | 67 | (default value, function without body, inheritance, interface, modifier override, object, package) |
| ktor-utils/common/src/io/ktor/util/Hash.kt | 9 | (modifier vararg, object, package) |
| ktor-utils/common/src/io/ktor/util/HashFunction.kt | 439 | (Byte and Short, annotation, arrays, bitwise operators, default value, extension function, function without body, inheritance, interface, modifier companion, modifier infix, modifier override, package) |
| ktor-utils/common/src/io/ktor/util/NonceManager.kt | 48 | (function without body, inheritance, interface, modifier override, modifier suspend, object, package) |
| ktor-utils/common/src/io/ktor/util/PlatformUtils.kt | 45 | (annotation, extension property, inheritance, is, modifier data, modifier enum, modifier expect, modifier sealed, nested class, object, package, property without initializer, when subject variable) |
| ktor-utils/common/src/io/ktor/util/Ranges.kt | 13 | (extension function, modifier operator, package) |
| ktor-utils/common/src/io/ktor/util/StackFrames.kt | 21 | (function without body, interface, modifier expect, package, property without initializer) |
| ktor-utils/common/src/io/ktor/util/StringValues.kt | 675 | (annotation, arrays, as, bitwise operators, collection supertype, default value, destructuring declaration, destructuring lambda parameter, extension function, function type with receiver, function without body, identity equality, inheritance, inline-only call, interface, is, modifier companion, modifier final, modifier inline, modifier open, modifier operator, modifier override, modifier protected, modifier vararg, nested class, object expression, package, property getter, property without initializer) |
| ktor-utils/common/src/io/ktor/util/Text.kt | 129 | (Char, as, extension function, inline-only call, modifier inline, modifier override, named argument, package, property without initializer, qualified this, when subject variable) |
| ktor-utils/common/src/io/ktor/util/Throwable.kt | 22 | (annotation, extension property, package, property getter) |
| ktor-utils/common/src/io/ktor/util/annotations/Annotations.kt | 21 | (annotation, modifier annotation, package) |
| ktor-utils/common/src/io/ktor/util/cio/Channels.kt | 35 | (default value, inheritance, modifier open, package) |
| ktor-utils/common/src/io/ktor/util/cio/Readers.kt | 41 | (annotation, arrays, class literal, default value, extension function, function type with receiver, inline-only call, modifier inline, modifier suspend, package, throw, try) |
| ktor-utils/common/src/io/ktor/util/collections/CollectionUtils.kt | 26 | (annotation, default value, modifier vararg, package, spread operator) |
| ktor-utils/common/src/io/ktor/util/collections/ConcurrentMap.kt | 55 | (collection supertype, default value, function without body, inheritance, modifier const, modifier expect, modifier override, package, property without initializer) |
| ktor-utils/common/src/io/ktor/util/collections/ConcurrentSet.kt | 66 | (annotation, collection supertype, inheritance, inline-only call, modifier override, object expression, package, property getter, require, check, error) |
| ktor-utils/common/src/io/ktor/util/collections/CopyOnWriteHashMap.kt | 84 | (annotation, do-while, identity equality, inline-only call, modifier operator, package) |
| ktor-utils/common/src/io/ktor/util/collections/MapDelegates.kt | 101 | (annotation, callable reference, extension function, function without body, inheritance, inline-only call, interface, modifier operator, modifier override, package, property without initializer) |
| ktor-utils/common/src/io/ktor/util/collections/TreeLike.kt | 53 | (annotation, as, interface, package, property without initializer) |
| ktor-utils/common/src/io/ktor/util/converters/ConversionService.kt | 146 | (Byte and Short, Char, Float, annotation, as, class literal, default value, function without body, inheritance, inline-only call, interface, is, modifier expect, modifier open, modifier override, object, package, throw, unsigned integers, when subject variable) |
| ktor-utils/common/src/io/ktor/util/converters/DataConversion.kt | 135 | (annotation, as, class literal, function type with receiver, inheritance, inline-only call, modifier inline, modifier noinline, modifier override, modifier reified, nested class, package, reified member type parameter, throw) |
| ktor-utils/common/src/io/ktor/util/date/Date.kt | 185 | (annotation, default value, extension function, function without body, inheritance, inline-only call, modifier companion, modifier data, modifier enum, modifier expect, modifier operator, modifier override, package, require, check, error) |
| ktor-utils/common/src/io/ktor/util/date/GMTDateParser.kt | 141 | (Char, extension function, inheritance, inline-only call, modifier companion, modifier const, modifier lateinit, package, property without initializer, require, check, error, throw, try) |
| ktor-utils/common/src/io/ktor/util/debug/ContextUtils.kt | 53 | (inline-only call, modifier suspend, package, suspend function type) |
| ktor-utils/common/src/io/ktor/util/debug/IntellijIdeaDebugDetector.kt | 13 | (modifier expect, object, package, property without initializer) |
| ktor-utils/common/src/io/ktor/util/debug/plugins/PluginName.kt | 34 | (inheritance, modifier companion, modifier data, modifier override, package) |
| ktor-utils/common/src/io/ktor/util/debug/plugins/PluginsTrace.kt | 48 | (default value, inheritance, modifier companion, modifier data, modifier enum, modifier override, nested class, package) |
| ktor-utils/common/src/io/ktor/util/internal/ExceptionUtils.kt | 12 | (extension function, function without body, modifier expect, package) |
| ktor-utils/common/src/io/ktor/util/internal/LockFreeLinkedList.kt | 833 | (annotation, as, class literal, extension function, function without body, identity equality, inheritance, inline-only call, is, modifier abstract, modifier const, modifier crossinline, modifier final, modifier inline, modifier open, modifier override, modifier protected, modifier reified, nested class, object expression, package, property getter, property without initializer, qualified this, reified member type parameter, require, check, error, throw, typealias) |
| ktor-utils/common/src/io/ktor/util/logging/KtorSimpleLogger.kt | 8 | (annotation, function without body, modifier expect, package) |
| ktor-utils/common/src/io/ktor/util/logging/Logger.kt | 68 | (class literal, extension function, extension property, function without body, interface, modifier expect, modifier inline, package, property without initializer, require, check, error) |
| ktor-utils/common/src/io/ktor/util/network/NetworkAddress.kt | 53 | (annotation, extension property, function without body, inheritance, modifier abstract, modifier expect, package, property without initializer) |
| ktor-utils/common/src/io/ktor/util/pipeline/DebugPipelineContext.kt | 84 | (annotation, do-while, inheritance, modifier override, modifier suspend, package) |
| ktor-utils/common/src/io/ktor/util/pipeline/PhaseContent.kt | 79 | (annotation, as, inline-only call, is, modifier companion, modifier override, package, require, check, error, secondary constructor) |
| ktor-utils/common/src/io/ktor/util/pipeline/Pipeline.kt | 537 | (annotation, as, delegated property, extension function, function type with receiver, function without body, identity equality, inline-only call, is, labels, modifier expect, modifier inline, modifier noinline, modifier open, modifier override, modifier reified, modifier suspend, modifier vararg, named argument, package, property getter, require, check, error, secondary constructor, spread operator, super, throw, typealias, when guard) |
| ktor-utils/common/src/io/ktor/util/pipeline/PipelineContext.kt | 69 | (annotation, default value, function without body, inheritance, modifier abstract, modifier expect, modifier suspend, package, property without initializer) |
| ktor-utils/common/src/io/ktor/util/pipeline/PipelinePhase.kt | 23 | (inheritance, modifier override, package) |
| ktor-utils/common/src/io/ktor/util/pipeline/PipelinePhaseRelation.kt | 27 | (inheritance, modifier data, modifier sealed, nested class, object, package) |
| ktor-utils/common/src/io/ktor/util/pipeline/StackTraceRecover.kt | 22 | (annotation, extension function, function without body, modifier expect, package, try) |
| ktor-utils/common/src/io/ktor/util/pipeline/StackWalkingFailed.kt | 16 | (object, package, require, check, error) |
| ktor-utils/common/src/io/ktor/util/pipeline/StackWalkingFailedFrame.kt | 33 | (callable reference, class literal, inheritance, modifier override, object, package, property getter) |
| ktor-utils/common/src/io/ktor/util/pipeline/SuspendFunctionGun.kt | 208 | (arrays, as, do-while, identity equality, inheritance, inline-only call, is, modifier override, modifier suspend, named argument, object expression, package, prefix increment, property getter, require, check, error, try, when subject variable) |
| ktor-utils/common/src/io/ktor/util/reflect/Type.kt | 126 | (annotation, as, class literal, default value, extension function, extension property, function without body, identity equality, inline-only call, interface, is, modifier expect, modifier inline, modifier override, modifier reified, named argument, package, property without initializer, require, check, error, secondary constructor, try) |
| ktor-utils/jvm/src/io/ktor/util/AttributesJvm.kt | 69 | (annotation, as, inheritance, inline-only call, modifier abstract, modifier actual, modifier final, modifier operator, modifier override, modifier protected, package, property getter, property without initializer) |
| ktor-utils/jvm/src/io/ktor/util/BufferViewJvm.kt | 47 | (annotation, extension function, package) |
| ktor-utils/jvm/src/io/ktor/util/Cache.kt | 60 | (Float, annotation, inheritance, inline-only call, modifier const, modifier override, package, super) |
| ktor-utils/jvm/src/io/ktor/util/CollectionsJvm.kt | 14 | (extension function, modifier actual, package) |
| ktor-utils/jvm/src/io/ktor/util/ContentEncodersJvm.kt | 23 | (class delegation, inheritance, modifier actual, modifier override, object, package) |
| ktor-utils/jvm/src/io/ktor/util/CryptoJvm.kt | 133 | (annotation, arrays, as, inheritance, inline-only call, is, modifier actual, modifier override, modifier suspend, modifier value, package) |
| ktor-utils/jvm/src/io/ktor/util/Deflater.kt | 136 | (Byte and Short, annotation, arrays, class literal, default value, extension function, inline-only call, modifier const, modifier suspend, named argument, package, qualified this, require, check, error, throw, try) |
| ktor-utils/jvm/src/io/ktor/util/EncodersJvm.kt | 184 | (annotation, bitwise operators, class literal, default value, extension function, inheritance, inline-only call, modifier const, modifier infix, modifier override, modifier suspend, named argument, object, object expression, package, require, check, error, throw, try) |
| ktor-utils/jvm/src/io/ktor/util/InputJvm.kt | 33 | (arrays, extension function, inheritance, modifier override, object expression, package, qualified this) |
| ktor-utils/jvm/src/io/ktor/util/NIO.kt | 74 | (arrays, default value, extension function, inline-only call, package, qualified this) |
| ktor-utils/jvm/src/io/ktor/util/NioPath.kt | 59 | (extension function, package, require, check, error, throw) |
| ktor-utils/jvm/src/io/ktor/util/Nonce.kt | 190 | (annotation, arrays, class literal, default value, delegated property, inline-only call, modifier const, named argument, package, require, check, error, try) |
| ktor-utils/jvm/src/io/ktor/util/Path.kt | 101 | (Char, extension function, package, require, check, error, throw) |
| ktor-utils/jvm/src/io/ktor/util/PlatformUtilsJvm.kt | 13 | (extension property, inline-only call, modifier actual, modifier const, package, property getter) |
| ktor-utils/jvm/src/io/ktor/util/StackFramesJvm.kt | 27 | (annotation, modifier actual, package, typealias) |
| ktor-utils/jvm/src/io/ktor/util/StatelessHmacNonceManager.kt | 157 | (Char, annotation, arrays, default value, destructuring declaration, identity equality, inheritance, inline-only call, modifier companion, modifier override, modifier suspend, named argument, package, secondary constructor, suspend function type) |
| ktor-utils/jvm/src/io/ktor/util/cio/ByteBufferPool.kt | 26 | (modifier const, package) |
| ktor-utils/jvm/src/io/ktor/util/cio/FileChannels.kt | 114 | (annotation, class literal, default value, extension function, inline-only call, modifier suspend, named argument, package, qualified this, require, check, error) |
| ktor-utils/jvm/src/io/ktor/util/cio/FileChannelsAtNioPath.kt | 39 | (annotation, default value, extension function, inline-only call, named argument, package, qualified this, require, check, error) |
| ktor-utils/jvm/src/io/ktor/util/cio/InputStreamAdapters.kt | 33 | (annotation, default value, extension function, package) |
| ktor-utils/jvm/src/io/ktor/util/cio/OutputStreamAdapters.kt | 27 | (default value, extension function, inline-only call, package) |
| ktor-utils/jvm/src/io/ktor/util/cio/ReadersJvm.kt | 26 | (annotation, extension function, inline-only call, modifier inline, modifier suspend, package, throw) |
| ktor-utils/jvm/src/io/ktor/util/collections/ConcurrentMapJvm.kt | 67 | (is, local function, modifier actual, modifier override, package, uninitialized local variable) |
| ktor-utils/jvm/src/io/ktor/util/converters/ConversionServiceJvm.kt | 62 | (annotation, as, class literal, inline-only call, is, modifier actual, package, throw) |
| ktor-utils/jvm/src/io/ktor/util/date/DateJvm.kt | 161 | (annotation, arrays, extension function, inline-only call, modifier actual, named argument, package, uninitialized local variable) |
| ktor-utils/jvm/src/io/ktor/util/debug/IntellijIdeaDebugDetectorJvm.kt | 19 | (delegated property, modifier actual, object, package, try) |
| ktor-utils/jvm/src/io/ktor/util/internal/ExceptionUtilsJvm.kt | 14 | (extension function, modifier actual, package) |
| ktor-utils/jvm/src/io/ktor/util/logging/KtorSimpleLoggerJvm.kt | 269 | (annotation, delegated property, inheritance, modifier actual, modifier override, modifier vararg, object expression, package, require, check, error, spread operator, try) |
| ktor-utils/jvm/src/io/ktor/util/logging/LoggerJvm.kt | 10 | (extension property, modifier actual, package, typealias) |
| ktor-utils/jvm/src/io/ktor/util/network/NetworkAddressJvm.kt | 34 | (as, extension property, modifier actual, package, property getter, typealias) |
| ktor-utils/jvm/src/io/ktor/util/pipeline/PipelineContext.jvm.kt | 8 | (modifier actual, package) |
| ktor-utils/jvm/src/io/ktor/util/pipeline/PipelineJvm.kt | 15 | (annotation, as, modifier actual, package) |
| ktor-utils/jvm/src/io/ktor/util/pipeline/StackTraceRecoverJvm.kt | 7 | (extension function, modifier actual, package) |
| ktor-utils/jvm/src/io/ktor/util/reflect/ServiceLoader.kt | 47 | (annotation, class literal, modifier inline, modifier reified, package) |
| ktor-utils/jvm/src/io/ktor/util/reflect/TypeInfoJvm.kt | 44 | (annotation, class literal, extension function, extension property, modifier actual, named argument, package, property getter, typealias) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 78 files, 9697 lines

Directories: `ktor-io/common/test`, `ktor-io/jvm/test`, `ktor-utils/common/test`, `ktor-utils/jvm/test`, `ktor-http/common/test`, `ktor-http/jvm/test`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 78 | 3 |
| 1: packages, annotations | 15 | 63 | 2 |
| 2: interfaces, inheritance | 15 | 63 | 2 |
| 3: type checks | 16 | 62 | 2 |
| 4: exceptions | 16 | 62 | 2 |
| 5: objects | 16 | 62 | 2 |
| 6: data, enum, sealed | 26 | 52 | 2 |
| 7: arrays, vararg, spread | 38 | 40 | 2 |
| 8: Char, Float, Byte, Short | 39 | 39 | 2 |
| 9: inliner | 47 | 31 | 1 |
| 10: lambdas with receivers | 48 | 30 | 1 |
| 11: extensions | 50 | 28 | 1 |
| 12: operator conventions | 50 | 28 | 1 |
| 13: identity, class literals, references | 62 | 16 | 1 |
| 14: class delegation | 62 | 16 | 1 |
| 15: secondary constructors | 62 | 16 | 1 |
| 16: local functions | 64 | 14 | 1 |
| 17: expect and actual | 64 | 14 | 1 |
| 18: annotation classes | 64 | 14 | 1 |
| 19: value classes | 64 | 14 | 1 |
| 20: unsigned integers | 65 | 13 | 1 |
| definite assignment | 66 | 12 | 1 |
| 21: inline functions | 66 | 12 | 1 |
| 22: reified type parameters | 66 | 12 | 1 |
| 23: labels, do-while | 66 | 12 | 1 |
| 24: delegated properties | 66 | 12 | 1 |
| 25: object expressions | 67 | 11 | 1 |
| 26: destructuring lambda parameters | 69 | 9 | 1 |
| 42-49: coroutines | 74 | 4 | 1 |
| 52: bitwise operators | 75 | 3 | 1 |
| 53: typealias | 75 | 3 | 1 |
| 54: setters, backing fields | 75 | 3 | 1 |
| 55: lateinit, tailrec | 76 | 2 | 1 |
| 56-57: collection supertypes | 76 | 2 | 1 |
| 58: member extension functions | 76 | 2 | 1 |
| 59: reified member type parameters | 76 | 2 | 1 |
| 60: inner and local classes | 78 | 0 | – |
| 61: when guards, anonymous functions, local extension functions | 78 | 0 | – |
| 62a: @JvmField and @JvmStatic in objects | 78 | 0 | – |
| 62b: generic value classes | 78 | 0 | – |
| 63: definitely non-nullable types | 78 | 0 | – |
| 70a: inline property accessors | 78 | 0 | – |
| 70b: where clauses | 78 | 0 | – |
| 70c: fun interfaces | 78 | 0 | – |
| delegated extension properties | 78 | 0 | – |
| qualified super | 78 | 0 | – |
| also supported | 78 | 0 | – |

### Features left, by files blocked

78 of 78 files use no feature that kc doesn't support.

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

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| ktor-http/common/test/io/ktor/tests/http/AuthAlgorithmsTest.kt | 108 | (annotation, named argument, package) |
| ktor-http/common/test/io/ktor/tests/http/AuthHeaderParseTest.kt | 89 | (annotation, is, package) |
| ktor-http/common/test/io/ktor/tests/http/CodecTest.kt | 164 | (Char, annotation, arrays, named argument, package, try) |
| ktor-http/common/test/io/ktor/tests/http/CommonHeadersTest.kt | 482 | (Char, annotation, inline-only call, named argument, package) |
| ktor-http/common/test/io/ktor/tests/http/ContentDispositionTest.kt | 41 | (annotation, package) |
| ktor-http/common/test/io/ktor/tests/http/ContentTypeLookupTest.kt | 122 | (annotation, package) |
| ktor-http/common/test/io/ktor/tests/http/ContentTypeMatchTest.kt | 73 | (annotation, package) |
| ktor-http/common/test/io/ktor/tests/http/ContentTypeTest.kt | 166 | (annotation, class literal, inline-only call, package) |
| ktor-http/common/test/io/ktor/tests/http/CookieDateParserTest.kt | 114 | (annotation, class literal, destructuring declaration, package) |
| ktor-http/common/test/io/ktor/tests/http/HttpProtocolVersionTest.kt | 46 | (annotation, package) |
| ktor-http/common/test/io/ktor/tests/http/HttpStatusCodeTest.kt | 35 | (annotation, package) |
| ktor-http/common/test/io/ktor/tests/http/MimesTest.kt | 32 | (annotation, package) |
| ktor-http/common/test/io/ktor/tests/http/ParseClientCookiesHeaderTest.kt | 80 | (annotation, named argument, package) |
| ktor-http/common/test/io/ktor/tests/http/QueryParametersTest.kt | 117 | (annotation, default value, function type with receiver, named argument, package) |
| ktor-http/common/test/io/ktor/tests/http/RangesTest.kt | 278 | (annotation, extension function, modifier vararg, package) |
| ktor-http/common/test/io/ktor/tests/http/RenderSetCookieTest.kt | 51 | (annotation, named argument, package) |
| ktor-http/common/test/io/ktor/tests/http/URLBuilderTest.kt | 517 | (Char, annotation, destructuring lambda parameter, inline-only call, named argument, package) |
| ktor-http/common/test/io/ktor/tests/http/URLParserTest.kt | 113 | (annotation, destructuring declaration, package) |
| ktor-http/common/test/io/ktor/tests/http/UrlDecodedParametersBuilderTest.kt | 135 | (annotation, package) |
| ktor-http/common/test/io/ktor/tests/http/UrlEncodeTest.kt | 110 | (annotation, package) |
| ktor-http/common/test/io/ktor/tests/http/UrlTest.kt | 475 | (Char, annotation, inline-only call, local function, package) |
| ktor-http/common/test/io/ktor/tests/http/header/AcceptEncodingTest.kt | 136 | (annotation, named argument, package) |
| ktor-http/jvm/test/io/ktor/tests/http/DigestAlgorithmJvmTest.kt | 23 | (annotation, arrays, package) |
| ktor-http/jvm/test/io/ktor/tests/http/SerializableTest.kt | 21 | (annotation, package) |
| ktor-http/jvm/test/io/ktor/tests/http/URLBuilderTest.kt | 120 | (annotation, inline-only call, package) |
| ktor-http/jvm/test/io/ktor/tests/http/content/BlockingBridgeTest.kt | 90 | (annotation, arrays, inline-only call, named argument, package, try) |
| ktor-http/jvm/test/io/ktor/tests/http/content/MultipartJvmTest.kt | 25 | (annotation, arrays, package) |
| ktor-io/common/test/ByteChannelTest.kt | 149 | (annotation, arrays, named argument, try) |
| ktor-io/common/test/ByteReadChannelOperationsTest.kt | 364 | (Char, annotation, arrays, class literal, extension function, inline-only call, modifier suspend, named argument) |
| ktor-io/common/test/CharsetDecoderTest.kt | 76 | (annotation, arrays, try) |
| ktor-io/common/test/CoroutinesTest.kt | 34 | (annotation, class literal, inline-only call) |
| ktor-io/common/test/ReadLineTest.kt | 448 | (Char, annotation, arrays, class literal, default value, extension function, inheritance, inline-only call, local class, modifier data, modifier override, modifier suspend, modifier vararg, named argument, object expression, throw) |
| ktor-io/common/test/ReadUtf8LineTest.kt | 102 | (annotation, class literal, inline-only call) |
| ktor-io/common/test/SinkByteWriteChannelTest.kt | 76 | (annotation, class delegation, inheritance, modifier override, object expression) |
| ktor-io/common/test/StringsTest.kt | 244 | (Char, annotation, arrays, inline-only call, named argument) |
| ktor-io/common/test/StringsToByteArrayTest.kt | 45 | (annotation, arrays) |
| ktor-io/common/test/WriterReaderTest.kt | 130 | (Byte and Short, annotation, class literal, inline-only call, modifier const, named argument) |
| ktor-io/jvm/test/ByteChannelConcurrentTest.kt | 72 | (annotation, arrays, class literal, inline-only call, is) |
| ktor-io/jvm/test/ByteReadChannelOperationsJvmTest.kt | 117 | (annotation, arrays) |
| ktor-io/jvm/test/ByteReadChannelReadTest.jvm.kt | 21 | (annotation, arrays) |
| ktor-io/jvm/test/ByteReadPacketExtensionsJvmTest.kt | 180 | (annotation, arrays, inline-only call) |
| ktor-io/jvm/test/LookAheadSessionTest.kt | 33 | (annotation, inline-only call) |
| ktor-io/jvm/test/StringsToByteArrayJvmTest.kt | 24 | (annotation, arrays) |
| ktor-io/jvm/test/io/ktor/utils/io/jvm/javaio/ByteChannelInputStreamTest.kt | 71 | (annotation, class literal, inline-only call, named argument, package, try) |
| ktor-io/jvm/test/io/ktor/utils/io/jvm/javaio/ByteChannelOutputStreamTest.kt | 300 | (annotation, arrays, bitwise operators, class literal, inline-only call, named argument, package, try) |
| ktor-io/jvm/test/io/ktor/utils/io/jvm/javaio/RawSourceChannelTest.kt | 51 | (annotation, arrays, inline-only call, package) |
| ktor-io/jvm/test/io/ktor/utils/io/jvm/nio/ReadingTest.kt | 37 | (annotation, package) |
| ktor-utils/common/test/io/ktor/util/AttributesTest.kt | 43 | (annotation, inline-only call, local function, modifier data, object, package) |
| ktor-utils/common/test/io/ktor/util/Base64Test.kt | 88 | (Byte and Short, Char, annotation, arrays, destructuring lambda parameter, package) |
| ktor-utils/common/test/io/ktor/util/CaseInsensitiveMapTest.kt | 111 | (annotation, package) |
| ktor-utils/common/test/io/ktor/util/ChannelTest.kt | 157 | (annotation, arrays, class literal, package) |
| ktor-utils/common/test/io/ktor/util/GMTDateParserTest.kt | 52 | (annotation, arrays, destructuring declaration, package) |
| ktor-utils/common/test/io/ktor/util/GMTDateTest.kt | 57 | (annotation, named argument, package) |
| ktor-utils/common/test/io/ktor/util/HashFunctionTest.kt | 28 | (annotation, arrays, package) |
| ktor-utils/common/test/io/ktor/util/NonceTest.kt | 40 | (Char, annotation, extension function, package) |
| ktor-utils/common/test/io/ktor/util/PipelineContractsTest.kt | 327 | (annotation, extension function, inheritance, inline-only call, local class, package, throw, try) |
| ktor-utils/common/test/io/ktor/util/PipelinePhasesTest.kt | 91 | (annotation, package) |
| ktor-utils/common/test/io/ktor/util/PipelineTest.kt | 435 | (annotation, extension function, inline-only call, modifier suspend, package, throw, try) |
| ktor-utils/common/test/io/ktor/util/StringValuesBuilderTest.kt | 74 | (annotation, named argument, package) |
| ktor-utils/common/test/io/ktor/util/StringValuesTest.kt | 157 | (annotation, named argument, package) |
| ktor-utils/common/test/io/ktor/util/SuspendFunctionGunTest.kt | 40 | (annotation, package) |
| ktor-utils/common/test/io/ktor/util/collections/ConcurrentSetTest.kt | 119 | (annotation, package) |
| ktor-utils/common/test/io/ktor/util/converters/DataConversionTest.kt | 134 | (Byte and Short, Char, annotation, class literal, destructuring declaration, inline-only call, modifier data, nested class, package, unsigned integers) |
| ktor-utils/common/test/io/ktor/util/reflect/TypeInfoTest.kt | 68 | (annotation, class literal, named argument, package) |
| ktor-utils/jvm/test/io/ktor/tests/utils/CacheTest.kt | 109 | (annotation, named argument, package) |
| ktor-utils/jvm/test/io/ktor/tests/utils/DeflaterReadChannelTest.kt | 184 | (Char, annotation, arrays, as, callable reference, class literal, extension function, inheritance, inline-only call, local function, modifier override, named argument, package, try, uninitialized local variable) |
| ktor-utils/jvm/test/io/ktor/tests/utils/DropLeadingTopDirsTest.kt | 70 | (annotation, package) |
| ktor-utils/jvm/test/io/ktor/tests/utils/EncodersJvmTest.kt | 53 | (annotation, arrays, extension function, modifier suspend, named argument, package) |
| ktor-utils/jvm/test/io/ktor/tests/utils/FileChannelTest.kt | 152 | (annotation, arrays, inline-only call, modifier lateinit, named argument, package, property without initializer) |
| ktor-utils/jvm/test/io/ktor/tests/utils/HexFunctionsTest.kt | 48 | (annotation, arrays, package) |
| ktor-utils/jvm/test/io/ktor/tests/utils/InputJvmTest.kt | 59 | (annotation, arrays, class literal, inline-only call, package) |
| ktor-utils/jvm/test/io/ktor/tests/utils/NonceSmokeTest.kt | 68 | (annotation, class literal, inline-only call, package, suspend function type, try) |
| ktor-utils/jvm/test/io/ktor/tests/utils/StackWalkingFailedTest.kt | 66 | (annotation, arrays, callable reference, class literal, inline-only call, package, require, check, error) |
| ktor-utils/jvm/test/io/ktor/tests/utils/StatelessHmacNonceManagerTest.kt | 48 | (annotation, arrays, package) |
| ktor-utils/jvm/test/io/ktor/tests/utils/converters/DataConversionTest.kt | 80 | (annotation, modifier enum, nested class, package) |
| ktor-utils/jvm/test/io/ktor/util/GMTDateCalendarTest.kt | 215 | (annotation, destructuring declaration, inline-only call, package) |
| ktor-utils/jvm/test/io/ktor/util/HashFunctionConsistencyTest.kt | 56 | (annotation, arrays, callable reference, inline-only call, named argument, package) |
| ktor-utils/jvm/test/io/ktor/util/PipelineContractsJvmTest.kt | 131 | (annotation, identity equality, inheritance, inline-only call, modifier override, modifier suspend, package, suspend function type, throw, try) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

