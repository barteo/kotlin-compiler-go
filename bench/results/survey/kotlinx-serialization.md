# Survey: kotlinx-serialization v1.11.0

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 99 files, 16291 lines

Directories: `core/commonMain/src`, `core/jvmMain/src`, `formats/json/commonMain/src`, `formats/json/jvmMain/src`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 99 | 9 |
| 1: packages, annotations | 1 | 98 | 8 |
| 2: interfaces, inheritance | 6 | 93 | 6 |
| 3: type checks | 6 | 93 | 5 |
| 4: exceptions | 6 | 93 | 5 |
| 5: objects | 8 | 91 | 5 |
| 6: data, enum, sealed | 9 | 90 | 4 |
| 7: arrays, vararg, spread | 10 | 89 | 4 |
| 8: Char, Float, Byte, Short | 11 | 88 | 3 |
| 9: inliner | 12 | 87 | 3 |
| 10: lambdas with receivers | 12 | 87 | 3 |
| 11: extensions | 12 | 87 | 2 |
| 12: operator conventions | 12 | 87 | 2 |
| 13: identity, class literals, references | 26 | 73 | 2 |
| 14: class delegation | 28 | 71 | 2 |
| 15: secondary constructors | 30 | 69 | 2 |
| 16: local functions | 30 | 69 | 2 |
| 17: expect and actual | 37 | 62 | 2 |
| 18: annotation classes | 45 | 54 | 2 |
| 19: value classes | 45 | 54 | 2 |
| 20: unsigned integers | 46 | 53 | 2 |
| definite assignment | 46 | 53 | 2 |
| 21: inline functions | 55 | 44 | 1 |
| 22: reified type parameters | 60 | 39 | 1 |
| 23: labels, do-while | 61 | 38 | 1 |
| 24: delegated properties | 65 | 34 | 1 |
| 25: object expressions | 67 | 32 | 1 |
| 26: destructuring lambda parameters | 68 | 31 | 1 |
| 42-49: coroutines | 68 | 31 | 1 |
| 52: bitwise operators | 74 | 25 | 1 |
| 53: typealias | 77 | 22 | 1 |
| 54: setters, backing fields | 77 | 22 | 1 |
| 55: lateinit, tailrec | 78 | 21 | 1 |
| 56-57: collection supertypes | 81 | 18 | 1 |
| 58: member extension functions | 84 | 15 | 1 |
| 59: reified member type parameters | 88 | 11 | 1 |
| 60: inner and local classes | 88 | 11 | 1 |
| 61: when guards, anonymous functions, local extension functions | 89 | 10 | 1 |
| 62a: @JvmField and @JvmStatic in objects | 89 | 10 | 1 |
| 62b: generic value classes | 89 | 10 | 1 |
| 63: definitely non-nullable types | 89 | 10 | 1 |
| 70a: inline property accessors | 89 | 10 | 1 |
| 70b: where clauses | 89 | 10 | 1 |
| 70c: fun interfaces | 90 | 9 | 1 |
| delegated extension properties | 90 | 9 | 1 |
| qualified super | 90 | 9 | 1 |
| context parameters of top-level functions | 90 | 9 | 1 |
| also supported | 98 | 1 | 1 |

### Features left, by files blocked

98 of 99 files use no feature that kc doesn't support.

| Feature | Files | Only blocker | Uses |
|:---|---:|---:|---:|
| member extension property | 1 | 1 | 6 |

### Greedy order of the features left

At each step, the feature that unblocks the most files given the ones before it, or, if none unblocks a file, the one that blocks the most files.

| Step | Feature | Files unblocked |
|---:|:---|---:|
| 1 | member extension property | 99 |

### Inline-only functions called

`require`, `check`, `requireNotNull`, `checkNotNull`, `error` and `TODO` are built in; kc inlines the others, since milestone 22 also those with reified type parameters.

| Function | Files | Calls |
|:---|---:|---:|
| `require` | 14 | 36 |
| `error` | 12 | 29 |
| `let` | 11 | 19 |
| `buildString` | 4 | 5 |
| `copyOf` | 4 | 27 |
| `isFinite` | 4 | 8 |
| `requireNotNull` | 4 | 5 |
| `also` | 3 | 4 |
| `apply` | 3 | 5 |
| `isNotEmpty` | 3 | 4 |
| `orEmpty` | 3 | 3 |
| `List` | 2 | 3 |
| `Sequence` | 2 | 3 |
| `find` | 2 | 2 |
| `run` | 2 | 2 |
| `runCatching` | 2 | 2 |
| `Iterable` | 1 | 2 |
| `UByteArray` | 1 | 1 |
| `UIntArray` | 1 | 1 |
| `ULongArray` | 1 | 1 |
| `UShortArray` | 1 | 1 |
| `assert` | 1 | 1 |
| `buildMap` | 1 | 1 |
| `countTrailingZeroBits` | 1 | 2 |
| `insert` | 1 | 7 |
| `isNotBlank` | 1 | 4 |
| `isNullOrBlank` | 1 | 3 |
| `isNullOrEmpty` | 1 | 1 |
| `reader` | 1 | 1 |
| `replaceFirstChar` | 1 | 1 |
| `synchronized` | 1 | 4 |
| `takeIf` | 1 | 1 |
| `takeUnless` | 1 | 1 |

### Files

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| core/commonMain/src/kotlinx/serialization/Annotations.kt | 350 | (annotation, class literal, default value, modifier annotation, modifier enum, modifier vararg, nested class, package) |
| core/commonMain/src/kotlinx/serialization/ApiLevels.kt | 51 | (annotation, modifier annotation, package) |
| core/commonMain/src/kotlinx/serialization/ContextualSerializer.kt | 69 | (annotation, arrays, inheritance, inline-only call, modifier override, package, secondary constructor) |
| core/commonMain/src/kotlinx/serialization/KSerializer.kt | 194 | (function without body, inheritance, interface, modifier override, package, property without initializer) |
| core/commonMain/src/kotlinx/serialization/PolymorphicSerializer.kt | 109 | (annotation, arrays, class literal, delegated property, extension function, inheritance, modifier override, package, secondary constructor) |
| core/commonMain/src/kotlinx/serialization/SealedSerializer.kt | 153 | (annotation, arrays, class literal, delegated property, destructuring lambda parameter, inheritance, modifier override, package, property without initializer, require, check, error, secondary constructor, super, throw) |
| core/commonMain/src/kotlinx/serialization/SerialFormat.kt | 196 | (arrays, extension function, function without body, inheritance, interface, modifier inline, modifier reified, named argument, package, property without initializer) |
| core/commonMain/src/kotlinx/serialization/SerializationExceptions.kt | 166 | (annotation, inheritance, modifier open, named argument, package, secondary constructor, super) |
| core/commonMain/src/kotlinx/serialization/Serializers.kt | 448 | (annotation, arrays, as, callable reference, class literal, extension function, modifier inline, modifier reified, named argument, package, spread operator, throw, try) |
| core/commonMain/src/kotlinx/serialization/SerializersCache.kt | 78 | (annotation, as, extension function, modifier inline, package) |
| core/commonMain/src/kotlinx/serialization/builtins/BuiltinSerializers.kt | 295 | (Byte and Short, Char, Float, annotation, arrays, as, class literal, extension function, extension property, modifier inline, modifier reified, package, property getter, unsigned integers) |
| core/commonMain/src/kotlinx/serialization/builtins/InstantComponentSerializer.kt | 61 | (annotation, class literal, inheritance, modifier override, named argument, object, package, throw, when subject variable) |
| core/commonMain/src/kotlinx/serialization/builtins/LongAsStringSerializer.kt | 31 | (inheritance, modifier override, object, package) |
| core/commonMain/src/kotlinx/serialization/descriptors/ContextAware.kt | 108 | (annotation, as, class delegation, extension function, extension property, inheritance, inline-only call, is, modifier override, package, property getter) |
| core/commonMain/src/kotlinx/serialization/descriptors/SerialDescriptor.kt | 447 | (annotation, class literal, collection supertype, extension property, function without body, inheritance, inline-only call, interface, modifier override, object expression, package, property getter, property without initializer) |
| core/commonMain/src/kotlinx/serialization/descriptors/SerialDescriptors.kt | 393 | (annotation, arrays, class delegation, class literal, default value, delegated property, extension function, extension property, function type with receiver, identity equality, inheritance, inline-only call, is, modifier inline, modifier override, modifier reified, modifier vararg, package, property getter, require, check, error) |
| core/commonMain/src/kotlinx/serialization/descriptors/SerialKinds.kt | 257 | (annotation, class literal, inheritance, modifier override, modifier sealed, object, package) |
| core/commonMain/src/kotlinx/serialization/encoding/AbstractDecoder.kt | 80 | (Byte and Short, Char, Float, annotation, as, class literal, default value, inheritance, modifier abstract, modifier final, modifier open, modifier override, package, throw) |
| core/commonMain/src/kotlinx/serialization/encoding/AbstractEncoder.kt | 92 | (Byte and Short, Char, Float, annotation, class literal, inheritance, modifier abstract, modifier final, modifier open, modifier override, package, throw) |
| core/commonMain/src/kotlinx/serialization/encoding/ChunkedDecoder.kt | 50 | (annotation, function without body, interface, package) |
| core/commonMain/src/kotlinx/serialization/encoding/Decoding.kt | 575 | (Byte and Short, Char, Float, annotation, class literal, default value, extension function, function type with receiver, function without body, interface, modifier companion, modifier const, modifier crossinline, modifier inline, package, property without initializer) |
| core/commonMain/src/kotlinx/serialization/encoding/Encoding.kt | 507 | (Byte and Short, Char, Float, annotation, as, extension function, function type with receiver, function without body, interface, modifier crossinline, modifier inline, package, property without initializer) |
| core/commonMain/src/kotlinx/serialization/internal/AbstractPolymorphicSerializer.kt | 115 | (annotation, as, class literal, inheritance, labels, modifier abstract, modifier final, modifier open, modifier override, package, property without initializer, require, check, error, throw, when subject variable) |
| core/commonMain/src/kotlinx/serialization/internal/BuiltInSerializers.kt | 70 | (annotation, inheritance, modifier override, object, package, throw) |
| core/commonMain/src/kotlinx/serialization/internal/CachedNames.kt | 20 | (interface, package, property without initializer) |
| core/commonMain/src/kotlinx/serialization/internal/CollectionDescriptors.kt | 133 | (identity equality, inheritance, inline-only call, is, modifier const, modifier override, modifier sealed, package, require, check, error, throw) |
| core/commonMain/src/kotlinx/serialization/internal/CollectionSerializers.kt | 282 | (annotation, arrays, as, class literal, default value, destructuring lambda parameter, extension function, function without body, inheritance, inline-only call, is, member extension function, modifier abstract, modifier final, modifier override, modifier protected, modifier sealed, named argument, package, property without initializer, require, check, error) |
| core/commonMain/src/kotlinx/serialization/internal/ElementMarker.kt | 117 | (annotation, arrays, bitwise operators, class literal, inline-only call, modifier companion, package, property without initializer) |
| core/commonMain/src/kotlinx/serialization/internal/Enums.kt | 147 | (annotation, arrays, class literal, delegated property, identity equality, inheritance, is, modifier override, named argument, package, secondary constructor, throw) |
| core/commonMain/src/kotlinx/serialization/internal/InlineClassDescriptor.kt | 43 | (annotation, arrays, inheritance, modifier override, object expression, package, require, check, error, super) |
| core/commonMain/src/kotlinx/serialization/internal/JsonInternalDependencies.kt | 20 | (annotation, extension function, modifier annotation, package) |
| core/commonMain/src/kotlinx/serialization/internal/NamedCompanion.kt | 15 | (annotation, modifier annotation, package) |
| core/commonMain/src/kotlinx/serialization/internal/NoOpEncoder.kt | 33 | (Byte and Short, Char, Float, annotation, class literal, inheritance, modifier override, object, package) |
| core/commonMain/src/kotlinx/serialization/internal/NothingSerialDescriptor.kt | 33 | (annotation, class literal, identity equality, inheritance, modifier override, object, package, require, check, error, throw) |
| core/commonMain/src/kotlinx/serialization/internal/NullableSerializer.kt | 69 | (annotation, as, class delegation, class literal, identity equality, inheritance, is, modifier override, package, property getter) |
| core/commonMain/src/kotlinx/serialization/internal/ObjectSerializer.kt | 56 | (annotation, arrays, class literal, delegated property, inheritance, modifier override, package, secondary constructor, throw, when subject variable) |
| core/commonMain/src/kotlinx/serialization/internal/Platform.common.kt | 189 | (Char, annotation, arrays, as, bitwise operators, class literal, default value, extension function, function without body, inline-only call, interface, is, modifier const, modifier expect, modifier inline, modifier vararg, object, package, require, check, error, throw, when subject variable) |
| core/commonMain/src/kotlinx/serialization/internal/PluginExceptions.kt | 40 | (annotation, arrays, bitwise operators, class literal, package, throw) |
| core/commonMain/src/kotlinx/serialization/internal/PluginGeneratedSerialDescriptor.kt | 132 | (annotation, arrays, class literal, default value, delegated property, extension function, identity equality, inheritance, inline-only call, is, modifier final, modifier inline, modifier open, modifier override, modifier reified, package, prefix increment) |
| core/commonMain/src/kotlinx/serialization/internal/PluginHelperInterfaces.kt | 36 | (annotation, arrays, function without body, inheritance, interface, modifier vararg, package) |
| core/commonMain/src/kotlinx/serialization/internal/PrimitiveArraysSerializers.kt | 630 | (Byte and Short, Char, Float, annotation, arrays, extension function, inheritance, inline-only call, member extension function, modifier const, modifier override, object, package, unsigned integers) |
| core/commonMain/src/kotlinx/serialization/internal/Primitives.kt | 132 | (Byte and Short, Char, Float, annotation, as, class delegation, class literal, extension function, function without body, identity equality, inheritance, inline-only call, is, modifier expect, modifier override, object, package, require, check, error, throw) |
| core/commonMain/src/kotlinx/serialization/internal/SerializationConstructorMarker.kt | 9 | (annotation, package) |
| core/commonMain/src/kotlinx/serialization/internal/Tagged.kt | 343 | (Byte and Short, Char, Float, annotation, as, class literal, extension function, function without body, inheritance, inline-only call, member extension function, modifier abstract, modifier final, modifier open, modifier override, modifier protected, named argument, package, property getter, throw) |
| core/commonMain/src/kotlinx/serialization/internal/Tuples.kt | 167 | member extension property (annotation, as, class literal, collection supertype, extension property, function without body, identity equality, inheritance, labels, modifier abstract, modifier const, modifier data, modifier override, modifier protected, modifier sealed, nested class, package, property without initializer, throw, when subject variable) |
| core/commonMain/src/kotlinx/serialization/internal/ValueClasses.kt | 62 | (Byte and Short, annotation, inheritance, modifier override, object, package, unsigned integers) |
| core/commonMain/src/kotlinx/serialization/modules/PolymorphicModuleBuilder.kt | 198 | (annotation, as, class literal, default value, destructuring declaration, destructuring lambda parameter, extension function, is, modifier inline, modifier reified, package, reified member type parameter, require, check, error) |
| core/commonMain/src/kotlinx/serialization/modules/SerializersModule.kt | 244 | (annotation, as, class literal, default value, destructuring lambda parameter, extension function, function without body, inheritance, is, modifier abstract, modifier infix, modifier operator, modifier override, modifier sealed, named argument, nested class, object expression, package, property without initializer, qualified this, typealias) |
| core/commonMain/src/kotlinx/serialization/modules/SerializersModuleBuilders.kt | 262 | (annotation, callable reference, class literal, default value, extension function, function type with receiver, identity equality, inheritance, inline-only call, modifier inline, modifier override, modifier reified, package, require, check, error, secondary constructor, throw) |
| core/commonMain/src/kotlinx/serialization/modules/SerializersModuleCollector.kt | 104 | (annotation, function without body, interface, package) |
| core/jvmMain/src/kotlinx/serialization/SerializersJvm.kt | 200 | (annotation, arrays, as, class literal, default value, extension function, inline-only call, is, named argument, package, spread operator, throw, when subject variable) |
| core/jvmMain/src/kotlinx/serialization/internal/Caching.kt | 218 | (annotation, as, inheritance, inline-only call, is, modifier actual, modifier crossinline, modifier inline, modifier override, package, property getter, try) |
| core/jvmMain/src/kotlinx/serialization/internal/Platform.kt | 224 | (Byte and Short, Char, Float, annotation, arrays, as, class literal, extension function, inline-only call, modifier actual, modifier inline, modifier vararg, package, spread operator, throw, try, unsigned integers) |
| core/jvmMain/src/kotlinx/serialization/internal/SuppressAnimalSniffer.kt | 13 | (annotation, modifier annotation, package) |
| formats/json/commonMain/src/kotlinx/serialization/json/Json.kt | 749 | (Char, annotation, class literal, default value, extension function, extension property, function type with receiver, inheritance, modifier companion, modifier const, modifier enum, modifier expect, modifier final, modifier inline, modifier override, modifier reified, modifier sealed, package, property without initializer, qualified this, reified member type parameter, require, check, error, try) |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonAnnotations.kt | 109 | (annotation, modifier annotation, modifier vararg, package) |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonConfiguration.kt | 142 | (annotation, class literal, default value, modifier enum, modifier override, package) |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonContentPolymorphicSerializer.kt | 110 | (annotation, as, class literal, function without body, inheritance, modifier abstract, modifier final, modifier override, modifier protected, package, throw) |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonDecoder.kt | 92 | (annotation, class literal, function without body, inheritance, interface, package, property without initializer) |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonElement.kt | 344 | (Char, Float, annotation, as, class delegation, class literal, collection supertype, default value, destructuring lambda parameter, extension function, extension property, identity equality, inheritance, inline-only call, is, modifier abstract, modifier inline, modifier override, modifier sealed, named argument, object, package, property getter, property without initializer, require, check, error, throw, try, unsigned integers) |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonElementBuilders.kt | 246 | (annotation, callable reference, class literal, extension function, function type with receiver, modifier annotation, modifier inline, package) |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonElementSerializers.kt | 236 | (annotation, as, class delegation, class literal, delegated property, extension function, inheritance, inline-only call, is, modifier override, object, object expression, package, property getter, throw, unsigned integers) |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonEncoder.kt | 89 | (annotation, class literal, function without body, inheritance, interface, package, property without initializer) |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonExceptions.kt | 89 | (annotation, default value, inheritance, modifier abstract, modifier override, modifier sealed, package, property without initializer) |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonNamingStrategy.kt | 177 | (Char, annotation, callable reference, fun interface, function without body, inheritance, inline-only call, modifier companion, modifier override, object expression, package) |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonTransformingSerializer.kt | 99 | (annotation, inheritance, modifier abstract, modifier final, modifier open, modifier override, modifier protected, package) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/CharArrayPool.common.kt | 9 | (arrays, function without body, modifier expect, object, package) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/Composers.kt | 100 | (Byte and Short, Char, Float, annotation, inheritance, modifier open, modifier override, modifier protected, package, super) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/FormatLanguage.kt | 45 | (annotation, modifier annotation, modifier expect, package) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonElementMarker.kt | 31 | (annotation, callable reference, class literal, package) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonExceptions.kt | 135 | (annotation, class literal, default value, extension function, inline-only call, modifier inline, named argument, package) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonIterator.kt | 102 | (annotation, class literal, collection supertype, extension function, inheritance, modifier override, package, require, check, error) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonNamesMap.kt | 166 | (annotation, arrays, class literal, default value, extension function, inline-only call, is, local extension function, local function, modifier inline, named argument, package, qualified this, throw) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonPath.kt | 150 | (annotation, arrays, as, class literal, identity equality, inline-only call, is, modifier override, object, package, prefix increment) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonSerializersModuleValidator.kt | 73 | (annotation, class literal, inheritance, is, modifier override, package, throw) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonStreams.kt | 138 | (Char, annotation, arrays, bitwise operators, default value, function without body, inheritance, inline-only call, interface, modifier abstract, modifier annotation, modifier companion, modifier const, modifier final, modifier inline, modifier override, modifier reified, package, try) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonToStringWriter.kt | 10 | (Char, function without body, inheritance, modifier expect, modifier override, package) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonTreeReader.kt | 118 | (extension function, inline-only call, modifier inline, modifier suspend, named argument, package, prefix increment, require, check, error, when subject variable) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/Polymorphic.kt | 113 | (annotation, as, class literal, extension function, inline-only call, is, modifier inline, named argument, package, require, check, error, throw, try) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/SchemaCache.kt | 52 | (annotation, as, function without body, inline-only call, modifier expect, modifier operator, nested class, package, typealias) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/StreamingJsonDecoder.kt | 391 | (Byte and Short, Char, Float, annotation, as, bitwise operators, class literal, extension function, function type with receiver, inheritance, inline-only call, is, modifier final, modifier inline, modifier open, modifier override, named argument, nested class, package, prefix increment, require, check, error, super, throw, try) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/StreamingJsonEncoder.kt | 231 | (Byte and Short, Char, Float, annotation, arrays, callable reference, class literal, extension property, identity equality, inheritance, inline-only call, is, modifier inline, modifier override, modifier reified, package, property getter, reified member type parameter, secondary constructor, super, throw, unsigned integers) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/StringOps.kt | 71 | (Char, annotation, arrays, bitwise operators, extension function, inline-only call, named argument, package) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/SuppressAnimalSniffer.kt | 14 | (annotation, modifier annotation, package) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/TreeJsonDecoder.kt | 350 | (Byte and Short, Char, Float, annotation, as, callable reference, class literal, default value, extension function, function type with receiver, function without body, identity equality, inheritance, inline-only call, is, modifier abstract, modifier inline, modifier open, modifier override, modifier protected, modifier reified, modifier sealed, named argument, package, property getter, reified member type parameter, require, check, error, super, throw, try) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/TreeJsonEncoder.kt | 277 | (Byte and Short, Char, Float, annotation, class literal, extension function, extension property, function without body, identity equality, inheritance, inline-only call, is, modifier abstract, modifier const, modifier final, modifier inline, modifier lateinit, modifier open, modifier override, modifier protected, modifier reified, modifier sealed, named argument, object expression, package, property getter, property without initializer, require, check, error, super, throw, uninitialized local variable) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/WriteMode.kt | 52 | (Char, annotation, class literal, extension function, is, modifier enum, modifier inline, package, throw) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/lexer/AbstractJsonLexer.kt | 758 | (@JvmField in object, Byte and Short, Char, annotation, arrays, bitwise operators, class literal, default value, extension function, function without body, inline-only call, local function, modifier abstract, modifier const, modifier inline, modifier open, modifier override, modifier protected, object, package, prefix increment, property without initializer, throw, try, when subject variable) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/lexer/CommentLexers.kt | 220 | (Byte and Short, Char, arrays, destructuring declaration, inheritance, modifier override, package, prefix increment) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/lexer/ReaderJsonLexer.kt | 221 | (Byte and Short, Char, annotation, arrays, default value, inheritance, modifier const, modifier open, modifier override, modifier protected, package, prefix increment, when subject variable) |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/lexer/StringJsonLexer.kt | 132 | (Byte and Short, Char, annotation, inheritance, modifier open, modifier override, named argument, package, prefix increment, try) |
| formats/json/jvmMain/src/kotlinx/serialization/json/JsonSchemaCache.kt | 10 | (annotation, extension property, modifier actual, package) |
| formats/json/jvmMain/src/kotlinx/serialization/json/JvmStreams.kt | 128 | (annotation, default value, extension function, inline-only call, modifier inline, modifier reified, package, try) |
| formats/json/jvmMain/src/kotlinx/serialization/json/internal/ArrayPools.kt | 89 | (arrays, inheritance, inline-only call, modifier actual, modifier open, modifier protected, object, package, require, check, error, super) |
| formats/json/jvmMain/src/kotlinx/serialization/json/internal/CharsetReader.kt | 125 | (annotation, arrays, as, inline-only call, package, property without initializer, require, check, error, try) |
| formats/json/jvmMain/src/kotlinx/serialization/json/internal/FormatLanguage.kt | 9 | (annotation, modifier actual, package, typealias) |
| formats/json/jvmMain/src/kotlinx/serialization/json/internal/JsonToStringWriter.kt | 137 | (Char, annotation, arrays, inheritance, inline-only call, modifier actual, modifier override, package, when subject variable) |
| formats/json/jvmMain/src/kotlinx/serialization/json/internal/JvmJsonStreams.kt | 271 | (Char, annotation, arrays, bitwise operators, class literal, inheritance, inline-only call, modifier inline, modifier override, package, require, check, error, throw, when subject variable) |
| formats/json/jvmMain/src/kotlinx/serialization/json/internal/createMapForCache.kt | 15 | (modifier actual, package) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 212 files, 19489 lines

Directories: `core/commonTest/src`, `core/jvmTest/src`, `formats/json-tests/commonTest/src`, `formats/json-tests/jvmTest/src`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 212 | 8 |
| 1: packages, annotations | 5 | 207 | 6 |
| 2: interfaces, inheritance | 9 | 203 | 4 |
| 3: type checks | 10 | 202 | 4 |
| 4: exceptions | 10 | 202 | 4 |
| 5: objects | 16 | 196 | 3 |
| 6: data, enum, sealed | 51 | 161 | 2 |
| 7: arrays, vararg, spread | 52 | 160 | 2 |
| 8: Char, Float, Byte, Short | 57 | 155 | 2 |
| 9: inliner | 63 | 149 | 1 |
| 10: lambdas with receivers | 63 | 149 | 1 |
| 11: extensions | 64 | 148 | 1 |
| 12: operator conventions | 64 | 148 | 1 |
| 13: identity, class literals, references | 134 | 78 | 1 |
| 14: class delegation | 134 | 78 | 1 |
| 15: secondary constructors | 134 | 78 | 1 |
| 16: local functions | 138 | 74 | 1 |
| 17: expect and actual | 144 | 68 | 1 |
| 18: annotation classes | 151 | 61 | 1 |
| 19: value classes | 153 | 59 | 1 |
| 20: unsigned integers | 155 | 57 | 1 |
| definite assignment | 155 | 57 | 1 |
| 21: inline functions | 160 | 52 | 1 |
| 22: reified type parameters | 164 | 48 | 1 |
| 23: labels, do-while | 167 | 45 | 1 |
| 24: delegated properties | 167 | 45 | 1 |
| 25: object expressions | 176 | 36 | 1 |
| 26: destructuring lambda parameters | 179 | 33 | 1 |
| 42-49: coroutines | 180 | 32 | 1 |
| 52: bitwise operators | 181 | 31 | 1 |
| 53: typealias | 182 | 30 | 1 |
| 54: setters, backing fields | 183 | 29 | 1 |
| 55: lateinit, tailrec | 186 | 26 | 1 |
| 56-57: collection supertypes | 186 | 26 | 1 |
| 58: member extension functions | 190 | 22 | 1 |
| 59: reified member type parameters | 202 | 10 | 1 |
| 60: inner and local classes | 207 | 5 | 1 |
| 61: when guards, anonymous functions, local extension functions | 209 | 3 | 1 |
| 62a: @JvmField and @JvmStatic in objects | 209 | 3 | 1 |
| 62b: generic value classes | 211 | 1 | 1 |
| 63: definitely non-nullable types | 211 | 1 | 1 |
| 70a: inline property accessors | 211 | 1 | 1 |
| 70b: where clauses | 211 | 1 | 1 |
| 70c: fun interfaces | 211 | 1 | 1 |
| delegated extension properties | 211 | 1 | 1 |
| qualified super | 211 | 1 | 1 |
| context parameters of top-level functions | 211 | 1 | 1 |
| also supported | 212 | 0 | – |

### Features left, by files blocked

212 of 212 files use no feature that kc doesn't support.

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
| `TODO` | 8 | 28 |
| `error` | 7 | 15 |
| `runCatching` | 6 | 16 |
| `require` | 5 | 7 |
| `apply` | 4 | 5 |
| `with` | 4 | 9 |
| `check` | 3 | 5 |
| `isNotEmpty` | 3 | 5 |
| `let` | 3 | 5 |
| `requireNotNull` | 3 | 3 |
| `takeIf` | 3 | 5 |
| `List` | 2 | 3 |
| `assert` | 2 | 7 |
| `buildList` | 2 | 2 |
| `buildString` | 2 | 2 |
| `run` | 2 | 5 |
| `use` | 2 | 2 |
| `MutableList` | 1 | 1 |
| `byteInputStream` | 1 | 1 |
| `contentDeepEquals` | 1 | 1 |
| `contentDeepToString` | 1 | 1 |
| `format` | 1 | 1 |
| `isNaN` | 1 | 4 |
| `isNullOrBlank` | 1 | 1 |
| `onFailure` | 1 | 1 |
| `orEmpty` | 1 | 1 |
| `printStackTrace` | 1 | 2 |
| `reader` | 1 | 1 |
| `ubyteArrayOf` | 1 | 2 |
| `uintArrayOf` | 1 | 2 |
| `ulongArrayOf` | 1 | 2 |
| `ushortArrayOf` | 1 | 2 |

### Files

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| core/commonTest/src/kotlinx/serialization/BasicTypesSerializationTest.kt | 210 | (Byte and Short, Char, Float, annotation, class literal, inheritance, modifier override, modifier vararg, nested class, package, require, check, error) |
| core/commonTest/src/kotlinx/serialization/CachedSerializersTest.kt | 65 | (annotation, inheritance, modifier abstract, modifier annotation, modifier data, modifier enum, modifier sealed, nested class, object, package) |
| core/commonTest/src/kotlinx/serialization/CustomPropertyAccessorsTest.kt | 157 | (annotation, inheritance, modifier override, nested class, package, property backing field, property getter, property setter, property without initializer) |
| core/commonTest/src/kotlinx/serialization/ElementMarkerTest.kt | 98 | (annotation, class literal, package) |
| core/commonTest/src/kotlinx/serialization/EnumDescriptorsTest.kt | 101 | (annotation, as, modifier annotation, modifier enum, nested class, package) |
| core/commonTest/src/kotlinx/serialization/InheritableSerialInfoTest.kt | 46 | (annotation, inheritance, interface, modifier abstract, modifier annotation, modifier sealed, nested class, package) |
| core/commonTest/src/kotlinx/serialization/InterfaceContextualSerializerTest.kt | 241 | (annotation, as, class literal, extension function, inheritance, interface, modifier data, modifier inline, modifier override, modifier reified, modifier sealed, named argument, nested class, object, object expression, package, property getter, property without initializer, reified member type parameter, require, check, error) |
| core/commonTest/src/kotlinx/serialization/MetaSerializableTest.kt | 56 | (annotation, class literal, modifier annotation, nested class, package) |
| core/commonTest/src/kotlinx/serialization/PolymorphismTestData.kt | 37 | (annotation, as, class literal, identity equality, inheritance, modifier data, modifier open, modifier override, package) |
| core/commonTest/src/kotlinx/serialization/PrimitiveSerialDescriptorTest.kt | 29 | (annotation, package) |
| core/commonTest/src/kotlinx/serialization/SealedGenericClassesTest.kt | 43 | (annotation, inheritance, interface, modifier abstract, modifier data, modifier override, modifier sealed, nested class, package, property without initializer) |
| core/commonTest/src/kotlinx/serialization/SerialDescriptorAnnotationsTest.kt | 120 | (Byte and Short, annotation, default value, extension function, inheritance, inline-only call, local extension function, local function, modifier abstract, modifier annotation, modifier data, modifier enum, modifier sealed, nested class, object, package) |
| core/commonTest/src/kotlinx/serialization/SerialDescriptorBuilderTest.kt | 137 | (annotation, default value, inheritance, modifier override, named argument, nested class, object expression, package, require, check, error) |
| core/commonTest/src/kotlinx/serialization/SerialDescriptorEqualityTest.kt | 93 | (annotation, default value, destructuring declaration, inheritance, modifier open, nested class, package) |
| core/commonTest/src/kotlinx/serialization/SerialDescriptorSpecificationTest.kt | 228 | (Byte and Short, Char, Float, annotation, class delegation, default value, extension function, inheritance, inline-only call, is, local extension function, local function, modifier enum, modifier inline, named argument, nested class, object, package) |
| core/commonTest/src/kotlinx/serialization/SerializersLookupEnumTest.kt | 88 | (annotation, class literal, inheritance, modifier enum, modifier override, nested class, object, package, require, check, error) |
| core/commonTest/src/kotlinx/serialization/SerializersLookupInterfaceTest.kt | 51 | (annotation, class literal, interface, is, modifier sealed, nested class, package) |
| core/commonTest/src/kotlinx/serialization/SerializersLookupNamedCompanionTest.kt | 94 | (annotation, class literal, inheritance, interface, modifier companion, modifier override, modifier sealed, named argument, nested class, object, package, require, check, error) |
| core/commonTest/src/kotlinx/serialization/SerializersLookupObjectTest.kt | 64 | (annotation, class literal, inheritance, modifier override, nested class, object, package, require, check, error) |
| core/commonTest/src/kotlinx/serialization/SerializersModuleTest.kt | 135 | (annotation, arrays, as, class literal, inheritance, modifier abstract, modifier data, modifier enum, modifier sealed, nested class, object, package) |
| core/commonTest/src/kotlinx/serialization/TaggedTest.kt | 58 | (annotation, default value, extension function, inheritance, member extension function, modifier data, modifier override, nested class, package, throw) |
| core/commonTest/src/kotlinx/serialization/TestId.kt | 23 | (annotation, class literal, extension function, modifier annotation, modifier inline, modifier reified, package, throw) |
| core/commonTest/src/kotlinx/serialization/UmbrellaTypes.kt | 88 | (Byte and Short, Char, Float, annotation, arrays, default value, is, modifier data, modifier enum, modifier override, package) |
| core/commonTest/src/kotlinx/serialization/WrappedSerialDescriptorTest.kt | 88 | (Float, annotation, nested class, package) |
| core/commonTest/src/kotlinx/serialization/features/SchemaTest.kt | 142 | (annotation, class literal, default value, extension function, inheritance, inline-only call, is, modifier companion, modifier data, modifier enum, modifier override, named argument, nested class, package, require, check, error) |
| core/commonTest/src/kotlinx/serialization/features/SealedInterfacesSerializationTest.kt | 100 | (Char, annotation, class literal, extension function, inheritance, interface, modifier operator, modifier override, modifier sealed, nested class, package, require, check, error) |
| core/commonTest/src/kotlinx/serialization/internal/DummySequentialDecoder.kt | 65 | (Byte and Short, Char, Float, default value, inheritance, modifier override, package, prefix increment, throw) |
| core/commonTest/src/kotlinx/serialization/internal/ObjectSerializerTest.kt | 18 | (annotation, object, package) |
| core/commonTest/src/kotlinx/serialization/internal/TuplesTest.kt | 18 | (annotation, package) |
| core/commonTest/src/kotlinx/serialization/modules/ContextualGenericsTest.kt | 74 | (annotation, class literal, inheritance, local function, modifier data, modifier open, modifier override, modifier protected, nested class, package) |
| core/commonTest/src/kotlinx/serialization/modules/ModuleBuildersTest.kt | 381 | (annotation, as, class delegation, class literal, default value, extension function, inheritance, inline-only call, is, local function, modifier override, named argument, nested class, object, object expression, package, require, check, error) |
| core/commonTest/src/kotlinx/serialization/test/CompilerVersions.kt | 192 | (annotation, class literal, default value, extension function, is, modifier inline, modifier reified, named argument, package, require, check, error, throw, try) |
| core/commonTest/src/kotlinx/serialization/test/CurrentPlatform.common.kt | 16 | (modifier enum, modifier expect, package, property without initializer) |
| core/commonTest/src/kotlinx/serialization/test/TestHelpers.kt | 44 | (annotation, class literal, extension function, modifier inline, modifier reified, package) |
| core/jvmTest/src/kotlinx/serialization/CachingTest.kt | 80 | (annotation, class literal, modifier inline, modifier reified, nested class, package, reified member type parameter) |
| core/jvmTest/src/kotlinx/serialization/InterfaceContextualSerializerTestJvm.kt | 79 | (annotation, as, class literal, inheritance, interface, modifier override, nested class, object, object expression, package, property getter, property without initializer, require, check, error) |
| core/jvmTest/src/kotlinx/serialization/RetentionTest.kt | 18 | (annotation, callable reference, default value, nested class, package) |
| core/jvmTest/src/kotlinx/serialization/SerializationMethodInvocationOrderTest.kt | 228 | (annotation, inheritance, modifier companion, modifier data, modifier override, nested class, package) |
| core/jvmTest/src/kotlinx/serialization/SerializeFlatTest.kt | 315 | (annotation, class literal, inheritance, inline-only call, modifier companion, modifier data, modifier override, nested class, object, object expression, package, throw) |
| core/jvmTest/src/kotlinx/serialization/features/JvmContextualGenericsTest.kt | 27 | (annotation, class literal, inheritance, named argument, package) |
| core/jvmTest/src/kotlinx/serialization/features/SerializerJvmSpecificTest.kt | 58 | (annotation, class literal, default value, interface, is, modifier abstract, modifier enum, nested class, package) |
| core/jvmTest/src/kotlinx/serialization/privateclasstest/PrivateDataOutOfKotlinXSerializationPackageTest.kt | 47 | (annotation, inline-only call, modifier companion, modifier data, package) |
| core/jvmTest/src/kotlinx/serialization/test/CurrentPlatform.kt | 9 | (modifier actual, package) |
| core/jvmTest/src/kotlinx/serialization/test/TypeToken.kt | 19 | (annotation, as, class literal, inheritance, modifier inline, modifier open, modifier reified, object expression, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/ClassWithMultipleMasksTest.kt | 85 | (annotation, default value, inline-only call, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/EncodingCollectionsTest.kt | 27 | (annotation, inheritance, modifier override, object, package, throw) |
| formats/json-tests/commonTest/src/kotlinx/serialization/EncodingExtensionsTest.kt | 40 | (annotation, class literal, inheritance, modifier override, nested class, object, package, throw) |
| formats/json-tests/commonTest/src/kotlinx/serialization/EnumSerializationTest.kt | 132 | (annotation, class literal, inheritance, modifier data, modifier enum, modifier override, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/GenericOverrideTest.kt | 33 | (annotation, inheritance, modifier data, modifier override, modifier sealed, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/GenericSerializersOnFileTest.kt | 62 | (annotation, as, class literal, inheritance, modifier data, modifier override, named argument, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/InstantSerializationTest.kt | 88 | (annotation, as, class literal, destructuring declaration, inheritance, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/JsonElementPolymorphicErrorTest.kt | 79 | (annotation, class literal, inheritance, modifier abstract, modifier data, modifier override, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/JsonOverwriteKeyTest.kt | 36 | (annotation, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/JsonPathTest.kt | 186 | (annotation, default value, inheritance, inline-only call, modifier data, modifier enum, modifier inline, modifier sealed, named argument, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/KeepGeneratedSerializerTest.kt | 247 | (annotation, class literal, identity equality, inheritance, is, modifier companion, modifier data, modifier enum, modifier inline, modifier open, modifier override, modifier reified, modifier value, nested class, object, package, reified member type parameter) |
| formats/json-tests/commonTest/src/kotlinx/serialization/NotNullSerializersCompatibilityOnFileTest.kt | 112 | (annotation, class literal, inheritance, modifier data, modifier override, named argument, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/PolymorphismTestData.kt | 44 | (annotation, as, class literal, identity equality, inheritance, modifier data, modifier open, modifier override, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/SerializableClasses.kt | 30 | (annotation, inheritance, modifier data, modifier enum, modifier sealed, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/SerializableOnPropertyTypeAndTypealiasTest.kt | 93 | (annotation, class literal, inheritance, modifier data, modifier override, object, package, property getter, typealias) |
| formats/json-tests/commonTest/src/kotlinx/serialization/SerializationForNullableTypeOnFileTest.kt | 58 | (annotation, class literal, inheritance, modifier data, modifier override, named argument, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/SerializerForNullableTypeTest.kt | 139 | (annotation, class literal, default value, inheritance, modifier data, modifier override, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/SerializersLookupTest.kt | 340 | (Char, annotation, arrays, as, class literal, default value, extension function, inheritance, modifier inline, modifier override, modifier reified, nested class, object expression, package, property without initializer, reified member type parameter, require, check, error, unsigned integers) |
| formats/json-tests/commonTest/src/kotlinx/serialization/TuplesTest.kt | 69 | (annotation, callable reference, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/UmbrellaTypes.kt | 107 | (Byte and Short, Char, Float, annotation, arrays, default value, is, modifier data, modifier enum, modifier override, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/UnknownElementIndexTest.kt | 39 | (annotation, class literal, inheritance, modifier data, modifier enum, modifier override, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/builtins/KeyValueSerializersTest.kt | 142 | (Char, annotation, collection supertype, identity equality, inheritance, is, modifier inline, modifier override, modifier reified, nested class, package, reified member type parameter) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/BinaryPayloadExampleTest.kt | 79 | (annotation, arrays, as, class literal, identity equality, inheritance, labels, modifier companion, modifier override, nested class, package, throw, when subject variable) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/ByteArraySerializerTest.kt | 54 | (annotation, arrays, as, class literal, identity equality, modifier override, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/CollectionSerializerTest.kt | 39 | (annotation, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/ContextAndPolymorphicTest.kt | 139 | (annotation, class literal, default value, extension function, inheritance, modifier data, modifier lateinit, modifier override, nested class, object, package, property without initializer, require, check, error) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/DefaultPolymorphicSerializerTest.kt | 54 | (annotation, as, class literal, inheritance, modifier abstract, modifier data, modifier override, nested class, package, property without initializer) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/DerivedContextualSerializerTest.kt | 49 | (annotation, inheritance, is, modifier abstract, modifier override, nested class, object, package, require, check, error) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/DurationTest.kt | 25 | (annotation, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/EmojiTest.kt | 22 | (annotation, inheritance, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/GenericCustomSerializerTest.kt | 167 | (annotation, arrays, as, class literal, identity equality, inheritance, is, labels, modifier data, modifier lateinit, modifier override, modifier sealed, nested class, object, package, throw, try, uninitialized local variable, when subject variable) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/InheritanceTest.kt | 115 | (annotation, identity equality, inheritance, is, modifier abstract, modifier open, modifier override, package, property getter, property without initializer, super) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/JsonClassDiscriminatorTest.kt | 135 | (annotation, class literal, inheritance, interface, modifier abstract, modifier data, modifier sealed, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/JsonCommentsTest.kt | 165 | (annotation, as, destructuring lambda parameter, inheritance, modifier data, named argument, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/JsonEnumsCaseInsensitiveTest.kt | 170 | (annotation, default value, inheritance, local class, modifier data, modifier enum, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/JsonNamesTest.kt | 104 | (annotation, default value, inheritance, modifier data, modifier enum, named argument, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/JsonNamingStrategyExclusionTest.kt | 60 | (annotation, default value, extension function, inheritance, inline-only call, modifier annotation, modifier data, modifier enum, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/JsonNamingStrategyTest.kt | 279 | (annotation, default value, destructuring lambda parameter, inheritance, inline-only call, interface, local function, modifier data, modifier enum, modifier sealed, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/LocalClassesTest.kt | 84 | (annotation, class literal, inheritance, inline-only call, local class, modifier data, modifier override, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/LongAsStringTest.kt | 32 | (annotation, class literal, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/MetaSerializableJsonTest.kt | 72 | (annotation, destructuring declaration, extension function, inheritance, inline-only call, modifier annotation, modifier data, modifier override, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/ObjectSerialization.kt | 62 | (annotation, class literal, inheritance, modifier data, modifier sealed, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PartiallyCustomSerializerTest.kt | 31 | (annotation, class literal, default value, inheritance, modifier companion, modifier data, modifier override, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PolymorphicDeserializationErrorMessagesTest.kt | 54 | (annotation, inheritance, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PolymorphicOnClassesTest.kt | 149 | (annotation, as, callable reference, class literal, extension function, inheritance, inline-only call, interface, is, local function, modifier abstract, modifier open, modifier override, nested class, package, property without initializer, require, check, error) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PolymorphicSealedChildTest.kt | 115 | (annotation, class literal, inheritance, modifier abstract, modifier data, modifier lateinit, modifier sealed, nested class, package, uninitialized local variable) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PolymorphismForCustomTest.kt | 63 | (annotation, as, class literal, inheritance, modifier data, modifier override, nested class, object expression, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PolymorphismTest.kt | 184 | (annotation, class literal, inheritance, modifier data, modifier override, modifier sealed, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PolymorphismWithAnyTest.kt | 149 | (Char, annotation, class literal, inheritance, modifier data, named argument, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PrimitiveArraySerializersTest.kt | 65 | (Char, Float, annotation, arrays, default value, identity equality, inheritance, is, modifier data, modifier override, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PropertyInitializerTest.kt | 105 | (annotation, class literal, default value, inline-only call, modifier const, modifier data, nested class, object, package, property without initializer) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/SealedClassesSerializationTest.kt | 264 | (annotation, arrays, as, class literal, inheritance, modifier abstract, modifier data, modifier sealed, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/SealedPolymorphismTest.kt | 63 | (annotation, class literal, inheritance, modifier data, modifier sealed, named argument, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/SerializableOnTypeUsageTest.kt | 28 | (annotation, class literal, modifier data, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/SerializableWithTest.kt | 77 | (annotation, class literal, inheritance, modifier data, modifier override, object, package, property getter) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/SkipDefaults.kt | 67 | (annotation, default value, inline-only call, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/UseSerializersTest.kt | 34 | (annotation, class literal, default value, modifier data, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/UuidTest.kt | 50 | (annotation, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/inline/EncodeInlineElementTest.kt | 48 | (annotation, class literal, inheritance, labels, modifier data, modifier override, named argument, object, package, unsigned integers, when subject variable) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/inline/InlineClassesCompleteTest.kt | 133 | (annotation, generic value class, modifier data, modifier value, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/inline/InlineClassesTest.kt | 214 | (annotation, class literal, generic value class, inheritance, modifier data, modifier override, modifier value, named argument, object, package, unsigned integers) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/inline/InlineMapQuotedTest.kt | 66 | (annotation, class literal, inheritance, modifier data, modifier override, modifier value, nested class, object, package, unsigned integers) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/inline/UnsignedIntegersTest.kt | 140 | (annotation, as, class literal, identity equality, inheritance, inline-only call, modifier data, modifier override, nested class, package, unsigned integers) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/inline/ValueClassesInSealedHierarchyTest.kt | 78 | (annotation, class literal, inheritance, interface, is, modifier companion, modifier override, modifier sealed, modifier value, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/sealed/SealedChild.kt | 8 | (annotation, inheritance, modifier data, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/sealed/SealedDiamondTest.kt | 49 | (annotation, inheritance, interface, local class, modifier data, modifier enum, modifier sealed, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/sealed/SealedInterfacesInlineSerialNameTest.kt | 51 | (annotation, inheritance, interface, modifier data, modifier sealed, modifier value, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/sealed/SealedInterfacesJsonSerializationTest.kt | 40 | (annotation, inheritance, interface, modifier data, modifier sealed, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/sealed/SealedParent.kt | 6 | (annotation, modifier sealed, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/AbstractJsonImplicitNullsTest.kt | 156 | (annotation, class literal, default value, extension function, function without body, member extension function, modifier abstract, modifier data, modifier protected, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/BasicTypesSerializationTest.kt | 58 | (Char, Float, annotation, inheritance, modifier inline, modifier reified, package, reified member type parameter) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/DecodeFromJsonElementTest.kt | 59 | (annotation, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonBuildersTest.kt | 147 | (annotation, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonChunkedStringDecoderTest.kt | 74 | (annotation, class literal, inheritance, is, modifier data, modifier open, modifier override, object, package, require, check, error) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonCoerceInputValuesTest.kt | 181 | (annotation, default value, destructuring declaration, inheritance, modifier data, named argument, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonConfigurationTest.kt | 31 | (annotation, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonCustomSerializersTest.kt | 354 | (annotation, class literal, default value, inheritance, inline-only call, modifier companion, modifier data, modifier override, named argument, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonDecodingErrorMessagesTest.kt | 278 | (annotation, inheritance, interface, modifier data, modifier sealed, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonDecodingRedactedErrorMessagesTest.kt | 47 | (annotation, inheritance, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonDefaultContextTest.kt | 17 | (annotation, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonElementDecodingTest.kt | 120 | (annotation, class literal, default value, inheritance, local class, modifier data, modifier final, modifier inline, modifier open, modifier override, modifier reified, named argument, nested class, object, package, reified member type parameter) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonEncoderDecoderRecursiveTest.kt | 213 | (annotation, as, class literal, destructuring declaration, inheritance, inline-only call, is, modifier const, modifier data, modifier override, modifier sealed, nested class, object, package, throw) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonEncodingErrorMessagesTest.kt | 30 | (annotation, inheritance, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonExponentTest.kt | 78 | (annotation, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonGenericTest.kt | 55 | (annotation, arrays, inheritance, inline-only call, modifier override, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonHugeDataSerializationTest.kt | 40 | (annotation, inheritance, inline-only call, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonIgnoreKeysTest.kt | 53 | (annotation, inheritance, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonImplicitNullsTest.kt | 13 | (extension function, inheritance, member extension function, modifier override, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonMapKeysTest.kt | 181 | (annotation, class literal, inheritance, modifier companion, modifier data, modifier inline, modifier override, modifier reified, modifier value, nested class, package, reified member type parameter) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonModesTest.kt | 128 | (Float, annotation, class literal, inheritance, local function, modifier data, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonNumericKeysTest.kt | 47 | (annotation, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonOptionalTests.kt | 53 | (annotation, as, class literal, default value, identity equality, inheritance, modifier override, named argument, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonParserFailureModesTest.kt | 184 | (Byte and Short, annotation, default value, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonParserTest.kt | 116 | (annotation, as, inheritance, inline-only call, is, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonPrettyPrintTest.kt | 86 | (annotation, default value, inheritance, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonReifiedCollectionsTest.kt | 29 | (annotation, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonRootLevelNullTest.kt | 29 | (annotation, default value, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonSealedSubclassTest.kt | 27 | (annotation, inheritance, modifier data, modifier sealed, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonTestBase.kt | 204 | (default value, extension function, function type with receiver, inheritance, inline-only call, modifier abstract, modifier companion, modifier enum, modifier inline, modifier inner, modifier open, modifier override, modifier protected, modifier reified, nested class, package, reified member type parameter, require, check, error, throw) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonTransformingSerializerTest.kt | 126 | (annotation, class literal, default value, destructuring lambda parameter, inheritance, inline-only call, is, modifier data, modifier override, nested class, object, package, require, check, error) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonTransientTest.kt | 59 | (annotation, as, class literal, default value, identity equality, inheritance, modifier override, nested class, package, property getter) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonTreeAndMapperTest.kt | 96 | (annotation, as, class literal, inheritance, inline-only call, is, modifier data, modifier override, modifier sealed, nested class, object, package, throw) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonTreeImplicitNullsTest.kt | 14 | (extension function, inheritance, member extension function, modifier override, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonTreeTest.kt | 150 | (Byte and Short, Char, Float, annotation, default value, inheritance, modifier data, modifier inline, modifier reified, nested class, package, reified member type parameter) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonUnicodeTest.kt | 74 | (annotation, inheritance, modifier data, nested class, package, throw, try) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonUnionEnumTest.kt | 27 | (annotation, default value, inheritance, modifier data, modifier enum, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonUpdateModeTest.kt | 62 | (annotation, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/LenientTest.kt | 96 | (annotation, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/MapLikeSerializerTest.kt | 66 | (annotation, inheritance, labels, modifier data, modifier override, nested class, object, package, throw, when subject variable) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/SpecialFloatingPointValuesTest.kt | 55 | (Float, annotation, as, class literal, identity equality, inheritance, inline-only call, modifier data, modifier override, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/TrailingCommaTest.kt | 128 | (annotation, default value, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonClassDiscriminatorModeBaseTest.kt | 157 | (annotation, default value, inheritance, modifier abstract, modifier data, modifier inline, modifier lateinit, modifier override, modifier reified, modifier sealed, nested class, object, package, reified member type parameter, uninitialized local variable) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonClassDiscriminatorModeTest.kt | 138 | (annotation, class literal, inheritance, interface, modifier enum, named argument, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonContentPolymorphicSerializerTest.kt | 103 | (annotation, class literal, inheritance, interface, modifier data, modifier override, modifier sealed, nested class, object, package, property without initializer, throw) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonDeserializePolymorphicTwiceTest.kt | 24 | (annotation, inheritance, modifier data, modifier sealed, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonListPolymorphismTest.kt | 46 | (annotation, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonMapPolymorphismTest.kt | 94 | (annotation, class literal, inheritance, modifier abstract, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonNestedPolymorphismTest.kt | 67 | (annotation, class literal, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonNullablePolymorphicTest.kt | 33 | (annotation, class literal, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonPolymorphicClassDescriptorTest.kt | 26 | (annotation, inheritance, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonPolymorphicObjectTest.kt | 44 | (annotation, class literal, inheritance, modifier data, named argument, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonPolymorphismExceptionTest.kt | 53 | (annotation, class literal, default value, inheritance, modifier abstract, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonProhibitedPolymorphicKindsTest.kt | 98 | (annotation, class literal, function type with receiver, inheritance, modifier enum, modifier sealed, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonPropertyPolymorphicTest.kt | 63 | (annotation, class literal, inheritance, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonTreeDecoderPolymorphicTest.kt | 43 | (annotation, as, default value, inheritance, modifier data, modifier sealed, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/PolymorphicClasses.kt | 62 | (annotation, class literal, default value, inheritance, interface, modifier data, modifier open, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonArraySerializerTest.kt | 113 | (annotation, callable reference, inheritance, is, package, require, check, error) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonNativePrimitivesTest.kt | 29 | (Char, annotation, inheritance, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonNullSerializerTest.kt | 60 | (annotation, inheritance, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonObjectSerializerTest.kt | 125 | (annotation, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonPrimitiveSerializerTest.kt | 218 | (Float, annotation, class literal, destructuring lambda parameter, inheritance, modifier inline, modifier reified, nested class, package, reified member type parameter, unsigned integers) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonSerializerInGenericsTest.kt | 37 | (annotation, inheritance, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonTreeTest.kt | 151 | (annotation, as, inheritance, is, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonUnquotedLiteralTest.kt | 140 | (annotation, inheritance, local function, named argument, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/Primitives.kt | 22 | (annotation, modifier data, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/modules/SerialNameCollisionInSealedClassesTest.kt | 61 | (annotation, default value, inheritance, modifier const, modifier data, modifier sealed, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/modules/SerialNameCollisionTest.kt | 162 | (annotation, class literal, default value, inheritance, interface, modifier abstract, modifier const, modifier data, nested class, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/ContextualTest.kt | 47 | (annotation, class literal, inheritance, modifier data, modifier override, nested class, object, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/CurrentPlatform.common.kt | 15 | (modifier enum, modifier expect, package, property without initializer) |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/ExceptionsTesting.kt | 146 | (class literal, default value, function type with receiver, inline-only call, modifier inline, named argument, package, throw) |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/InternalHexConverter.kt | 51 | (Char, arrays, bitwise operators, default value, inline-only call, modifier const, object, package, require, check, error) |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/JsonHelpers.kt | 9 | (extension function, function without body, modifier expect, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/TestHelpers.kt | 57 | (Char, extension function, inline-only call, modifier inline, named argument, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/TestId.kt | 12 | (annotation, modifier annotation, package) |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/TestingFramework.kt | 100 | (class literal, default value, extension function, function type with receiver, modifier inline, modifier reified, package) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/BigDecimalTest.kt | 193 | (annotation, class literal, default value, inheritance, is, local function, modifier companion, modifier const, modifier data, modifier inline, modifier override, modifier reified, named argument, object, package, reified member type parameter, typealias) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/JavaCollectionsTest.kt | 112 | (annotation, modifier data, named argument, nested class, object expression, package) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/JvmMissingFieldsExceptionTest.kt | 164 | (annotation, arrays, class literal, default value, inheritance, inline-only call, modifier abstract, modifier data, modifier inline, modifier open, modifier sealed, named argument, nested class, package) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/SerializationCasesTest.kt | 93 | (annotation, class literal, inheritance, inline-only call, is, modifier data, modifier enum, modifier override, nested class, object, package) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/SerializeJavaClassTest.kt | 46 | (annotation, class literal, inheritance, inline-only call, modifier data, modifier override, object, package) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/SerializerByTypeCacheTest.kt | 119 | (annotation, arrays, as, class literal, identity equality, inheritance, is, modifier override, nested class, package) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/SerializerForNullableJavaTypeTest.kt | 44 | (annotation, class literal, inheritance, modifier data, modifier override, nested class, package, when subject variable) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/StacktraceRecoveryTest.kt | 64 | (annotation, class literal, inheritance, inline-only call, modifier override, modifier suspend, nested class, package) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/UuidPlatformClashTest.kt | 56 | (annotation, class literal, inheritance, modifier data, modifier override, nested class, object, package) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/features/ContextualSerializationOnFileTest.kt | 41 | (annotation, class literal, default value, modifier data, package) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/features/InternalInheritanceTest.kt | 134 | (annotation, as, default value, identity equality, inheritance, is, modifier open, modifier override, modifier protected, named argument, nested class, package, super) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/features/JsonJvmStreamsTest.kt | 129 | (annotation, arrays, as, class literal, inheritance, interface, modifier data, nested class, package, throw, try) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/features/JsonLazySequenceTest.kt | 197 | (annotation, arrays, default value, extension function, inline-only call, modifier inline, modifier reified, modifier suspend, modifier vararg, package, reified member type parameter, throw) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/features/JsonSequencePathTest.kt | 33 | (annotation, extension function, nested class, package) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/features/SerializerByTypeTest.kt | 278 | (annotation, arrays, as, class literal, inheritance, modifier companion, modifier data, modifier override, nested class, object, object expression, package, require, check, error) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/json/GsonCompatibilityTest.kt | 56 | (Float, annotation, class literal, modifier data, nested class, package, try) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/json/JsonChunkedBase64DecoderTest.kt | 85 | (annotation, arrays, as, class literal, identity equality, inheritance, inline-only call, is, modifier data, modifier override, object, package, require, check, error) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/json/JsonConcurrentStressTest.kt | 78 | (annotation, arrays, inheritance, modifier data, named argument, nested class, package) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/json/MissingFieldExceptionWithPathTest.kt | 40 | (annotation, modifier data, nested class, package, try) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/json/SpecConformanceTest.kt | 130 | (annotation, class literal, inline-only call, modifier data, modifier inline, nested class, package) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/test/CurrentPlatform.kt | 8 | (modifier actual, package) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/test/JsonHelpers.kt | 20 | (extension function, inline-only call, modifier actual, package) |
| formats/json-tests/jvmTest/src/kotlinx/serialization/test/TypeToken.kt | 17 | (annotation, as, class literal, inheritance, modifier inline, modifier open, modifier reified, object expression, package) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

