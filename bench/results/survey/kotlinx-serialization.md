# Survey: kotlinx-serialization v1.11.0

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 99 files, 16291 lines

Directories: `core/commonMain/src`, `core/jvmMain/src`, `formats/json/commonMain/src`, `formats/json/jvmMain/src`

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

`require`, `check`, `requireNotNull`, `checkNotNull`, `error` and `TODO` are built in; kc inlines the others, also those with reified type parameters.

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

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| core/commonMain/src/kotlinx/serialization/Annotations.kt | 350 |  |
| core/commonMain/src/kotlinx/serialization/ApiLevels.kt | 51 |  |
| core/commonMain/src/kotlinx/serialization/ContextualSerializer.kt | 69 |  |
| core/commonMain/src/kotlinx/serialization/KSerializer.kt | 194 |  |
| core/commonMain/src/kotlinx/serialization/PolymorphicSerializer.kt | 109 |  |
| core/commonMain/src/kotlinx/serialization/SealedSerializer.kt | 153 |  |
| core/commonMain/src/kotlinx/serialization/SerialFormat.kt | 196 |  |
| core/commonMain/src/kotlinx/serialization/SerializationExceptions.kt | 166 |  |
| core/commonMain/src/kotlinx/serialization/Serializers.kt | 448 |  |
| core/commonMain/src/kotlinx/serialization/SerializersCache.kt | 78 |  |
| core/commonMain/src/kotlinx/serialization/builtins/BuiltinSerializers.kt | 295 |  |
| core/commonMain/src/kotlinx/serialization/builtins/InstantComponentSerializer.kt | 61 |  |
| core/commonMain/src/kotlinx/serialization/builtins/LongAsStringSerializer.kt | 31 |  |
| core/commonMain/src/kotlinx/serialization/descriptors/ContextAware.kt | 108 |  |
| core/commonMain/src/kotlinx/serialization/descriptors/SerialDescriptor.kt | 447 |  |
| core/commonMain/src/kotlinx/serialization/descriptors/SerialDescriptors.kt | 393 |  |
| core/commonMain/src/kotlinx/serialization/descriptors/SerialKinds.kt | 257 |  |
| core/commonMain/src/kotlinx/serialization/encoding/AbstractDecoder.kt | 80 |  |
| core/commonMain/src/kotlinx/serialization/encoding/AbstractEncoder.kt | 92 |  |
| core/commonMain/src/kotlinx/serialization/encoding/ChunkedDecoder.kt | 50 |  |
| core/commonMain/src/kotlinx/serialization/encoding/Decoding.kt | 575 |  |
| core/commonMain/src/kotlinx/serialization/encoding/Encoding.kt | 507 |  |
| core/commonMain/src/kotlinx/serialization/internal/AbstractPolymorphicSerializer.kt | 115 |  |
| core/commonMain/src/kotlinx/serialization/internal/BuiltInSerializers.kt | 70 |  |
| core/commonMain/src/kotlinx/serialization/internal/CachedNames.kt | 20 |  |
| core/commonMain/src/kotlinx/serialization/internal/CollectionDescriptors.kt | 133 |  |
| core/commonMain/src/kotlinx/serialization/internal/CollectionSerializers.kt | 282 |  |
| core/commonMain/src/kotlinx/serialization/internal/ElementMarker.kt | 117 |  |
| core/commonMain/src/kotlinx/serialization/internal/Enums.kt | 147 |  |
| core/commonMain/src/kotlinx/serialization/internal/InlineClassDescriptor.kt | 43 |  |
| core/commonMain/src/kotlinx/serialization/internal/JsonInternalDependencies.kt | 20 |  |
| core/commonMain/src/kotlinx/serialization/internal/NamedCompanion.kt | 15 |  |
| core/commonMain/src/kotlinx/serialization/internal/NoOpEncoder.kt | 33 |  |
| core/commonMain/src/kotlinx/serialization/internal/NothingSerialDescriptor.kt | 33 |  |
| core/commonMain/src/kotlinx/serialization/internal/NullableSerializer.kt | 69 |  |
| core/commonMain/src/kotlinx/serialization/internal/ObjectSerializer.kt | 56 |  |
| core/commonMain/src/kotlinx/serialization/internal/Platform.common.kt | 189 |  |
| core/commonMain/src/kotlinx/serialization/internal/PluginExceptions.kt | 40 |  |
| core/commonMain/src/kotlinx/serialization/internal/PluginGeneratedSerialDescriptor.kt | 132 |  |
| core/commonMain/src/kotlinx/serialization/internal/PluginHelperInterfaces.kt | 36 |  |
| core/commonMain/src/kotlinx/serialization/internal/PrimitiveArraysSerializers.kt | 630 |  |
| core/commonMain/src/kotlinx/serialization/internal/Primitives.kt | 132 |  |
| core/commonMain/src/kotlinx/serialization/internal/SerializationConstructorMarker.kt | 9 |  |
| core/commonMain/src/kotlinx/serialization/internal/Tagged.kt | 343 |  |
| core/commonMain/src/kotlinx/serialization/internal/Tuples.kt | 167 | member extension property |
| core/commonMain/src/kotlinx/serialization/internal/ValueClasses.kt | 62 |  |
| core/commonMain/src/kotlinx/serialization/modules/PolymorphicModuleBuilder.kt | 198 |  |
| core/commonMain/src/kotlinx/serialization/modules/SerializersModule.kt | 244 |  |
| core/commonMain/src/kotlinx/serialization/modules/SerializersModuleBuilders.kt | 262 |  |
| core/commonMain/src/kotlinx/serialization/modules/SerializersModuleCollector.kt | 104 |  |
| core/jvmMain/src/kotlinx/serialization/SerializersJvm.kt | 200 |  |
| core/jvmMain/src/kotlinx/serialization/internal/Caching.kt | 218 |  |
| core/jvmMain/src/kotlinx/serialization/internal/Platform.kt | 224 |  |
| core/jvmMain/src/kotlinx/serialization/internal/SuppressAnimalSniffer.kt | 13 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/Json.kt | 749 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonAnnotations.kt | 109 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonConfiguration.kt | 142 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonContentPolymorphicSerializer.kt | 110 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonDecoder.kt | 92 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonElement.kt | 344 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonElementBuilders.kt | 246 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonElementSerializers.kt | 236 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonEncoder.kt | 89 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonExceptions.kt | 89 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonNamingStrategy.kt | 177 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/JsonTransformingSerializer.kt | 99 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/CharArrayPool.common.kt | 9 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/Composers.kt | 100 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/FormatLanguage.kt | 45 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonElementMarker.kt | 31 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonExceptions.kt | 135 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonIterator.kt | 102 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonNamesMap.kt | 166 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonPath.kt | 150 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonSerializersModuleValidator.kt | 73 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonStreams.kt | 138 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonToStringWriter.kt | 10 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/JsonTreeReader.kt | 118 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/Polymorphic.kt | 113 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/SchemaCache.kt | 52 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/StreamingJsonDecoder.kt | 391 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/StreamingJsonEncoder.kt | 231 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/StringOps.kt | 71 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/SuppressAnimalSniffer.kt | 14 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/TreeJsonDecoder.kt | 350 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/TreeJsonEncoder.kt | 277 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/WriteMode.kt | 52 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/lexer/AbstractJsonLexer.kt | 758 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/lexer/CommentLexers.kt | 220 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/lexer/ReaderJsonLexer.kt | 221 |  |
| formats/json/commonMain/src/kotlinx/serialization/json/internal/lexer/StringJsonLexer.kt | 132 |  |
| formats/json/jvmMain/src/kotlinx/serialization/json/JsonSchemaCache.kt | 10 |  |
| formats/json/jvmMain/src/kotlinx/serialization/json/JvmStreams.kt | 128 |  |
| formats/json/jvmMain/src/kotlinx/serialization/json/internal/ArrayPools.kt | 89 |  |
| formats/json/jvmMain/src/kotlinx/serialization/json/internal/CharsetReader.kt | 125 |  |
| formats/json/jvmMain/src/kotlinx/serialization/json/internal/FormatLanguage.kt | 9 |  |
| formats/json/jvmMain/src/kotlinx/serialization/json/internal/JsonToStringWriter.kt | 137 |  |
| formats/json/jvmMain/src/kotlinx/serialization/json/internal/JvmJsonStreams.kt | 271 |  |
| formats/json/jvmMain/src/kotlinx/serialization/json/internal/createMapForCache.kt | 15 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 212 files, 19489 lines

Directories: `core/commonTest/src`, `core/jvmTest/src`, `formats/json-tests/commonTest/src`, `formats/json-tests/jvmTest/src`

### Features left, by files blocked

212 of 212 files use no feature that kc doesn't support.

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

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| core/commonTest/src/kotlinx/serialization/BasicTypesSerializationTest.kt | 210 |  |
| core/commonTest/src/kotlinx/serialization/CachedSerializersTest.kt | 65 |  |
| core/commonTest/src/kotlinx/serialization/CustomPropertyAccessorsTest.kt | 157 |  |
| core/commonTest/src/kotlinx/serialization/ElementMarkerTest.kt | 98 |  |
| core/commonTest/src/kotlinx/serialization/EnumDescriptorsTest.kt | 101 |  |
| core/commonTest/src/kotlinx/serialization/InheritableSerialInfoTest.kt | 46 |  |
| core/commonTest/src/kotlinx/serialization/InterfaceContextualSerializerTest.kt | 241 |  |
| core/commonTest/src/kotlinx/serialization/MetaSerializableTest.kt | 56 |  |
| core/commonTest/src/kotlinx/serialization/PolymorphismTestData.kt | 37 |  |
| core/commonTest/src/kotlinx/serialization/PrimitiveSerialDescriptorTest.kt | 29 |  |
| core/commonTest/src/kotlinx/serialization/SealedGenericClassesTest.kt | 43 |  |
| core/commonTest/src/kotlinx/serialization/SerialDescriptorAnnotationsTest.kt | 120 |  |
| core/commonTest/src/kotlinx/serialization/SerialDescriptorBuilderTest.kt | 137 |  |
| core/commonTest/src/kotlinx/serialization/SerialDescriptorEqualityTest.kt | 93 |  |
| core/commonTest/src/kotlinx/serialization/SerialDescriptorSpecificationTest.kt | 228 |  |
| core/commonTest/src/kotlinx/serialization/SerializersLookupEnumTest.kt | 88 |  |
| core/commonTest/src/kotlinx/serialization/SerializersLookupInterfaceTest.kt | 51 |  |
| core/commonTest/src/kotlinx/serialization/SerializersLookupNamedCompanionTest.kt | 94 |  |
| core/commonTest/src/kotlinx/serialization/SerializersLookupObjectTest.kt | 64 |  |
| core/commonTest/src/kotlinx/serialization/SerializersModuleTest.kt | 135 |  |
| core/commonTest/src/kotlinx/serialization/TaggedTest.kt | 58 |  |
| core/commonTest/src/kotlinx/serialization/TestId.kt | 23 |  |
| core/commonTest/src/kotlinx/serialization/UmbrellaTypes.kt | 88 |  |
| core/commonTest/src/kotlinx/serialization/WrappedSerialDescriptorTest.kt | 88 |  |
| core/commonTest/src/kotlinx/serialization/features/SchemaTest.kt | 142 |  |
| core/commonTest/src/kotlinx/serialization/features/SealedInterfacesSerializationTest.kt | 100 |  |
| core/commonTest/src/kotlinx/serialization/internal/DummySequentialDecoder.kt | 65 |  |
| core/commonTest/src/kotlinx/serialization/internal/ObjectSerializerTest.kt | 18 |  |
| core/commonTest/src/kotlinx/serialization/internal/TuplesTest.kt | 18 |  |
| core/commonTest/src/kotlinx/serialization/modules/ContextualGenericsTest.kt | 74 |  |
| core/commonTest/src/kotlinx/serialization/modules/ModuleBuildersTest.kt | 381 |  |
| core/commonTest/src/kotlinx/serialization/test/CompilerVersions.kt | 192 |  |
| core/commonTest/src/kotlinx/serialization/test/CurrentPlatform.common.kt | 16 |  |
| core/commonTest/src/kotlinx/serialization/test/TestHelpers.kt | 44 |  |
| core/jvmTest/src/kotlinx/serialization/CachingTest.kt | 80 |  |
| core/jvmTest/src/kotlinx/serialization/InterfaceContextualSerializerTestJvm.kt | 79 |  |
| core/jvmTest/src/kotlinx/serialization/RetentionTest.kt | 18 |  |
| core/jvmTest/src/kotlinx/serialization/SerializationMethodInvocationOrderTest.kt | 228 |  |
| core/jvmTest/src/kotlinx/serialization/SerializeFlatTest.kt | 315 |  |
| core/jvmTest/src/kotlinx/serialization/features/JvmContextualGenericsTest.kt | 27 |  |
| core/jvmTest/src/kotlinx/serialization/features/SerializerJvmSpecificTest.kt | 58 |  |
| core/jvmTest/src/kotlinx/serialization/privateclasstest/PrivateDataOutOfKotlinXSerializationPackageTest.kt | 47 |  |
| core/jvmTest/src/kotlinx/serialization/test/CurrentPlatform.kt | 9 |  |
| core/jvmTest/src/kotlinx/serialization/test/TypeToken.kt | 19 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/ClassWithMultipleMasksTest.kt | 85 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/EncodingCollectionsTest.kt | 27 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/EncodingExtensionsTest.kt | 40 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/EnumSerializationTest.kt | 132 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/GenericOverrideTest.kt | 33 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/GenericSerializersOnFileTest.kt | 62 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/InstantSerializationTest.kt | 88 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/JsonElementPolymorphicErrorTest.kt | 79 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/JsonOverwriteKeyTest.kt | 36 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/JsonPathTest.kt | 186 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/KeepGeneratedSerializerTest.kt | 247 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/NotNullSerializersCompatibilityOnFileTest.kt | 112 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/PolymorphismTestData.kt | 44 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/SerializableClasses.kt | 30 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/SerializableOnPropertyTypeAndTypealiasTest.kt | 93 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/SerializationForNullableTypeOnFileTest.kt | 58 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/SerializerForNullableTypeTest.kt | 139 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/SerializersLookupTest.kt | 340 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/TuplesTest.kt | 69 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/UmbrellaTypes.kt | 107 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/UnknownElementIndexTest.kt | 39 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/builtins/KeyValueSerializersTest.kt | 142 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/BinaryPayloadExampleTest.kt | 79 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/ByteArraySerializerTest.kt | 54 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/CollectionSerializerTest.kt | 39 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/ContextAndPolymorphicTest.kt | 139 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/DefaultPolymorphicSerializerTest.kt | 54 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/DerivedContextualSerializerTest.kt | 49 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/DurationTest.kt | 25 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/EmojiTest.kt | 22 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/GenericCustomSerializerTest.kt | 167 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/InheritanceTest.kt | 115 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/JsonClassDiscriminatorTest.kt | 135 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/JsonCommentsTest.kt | 165 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/JsonEnumsCaseInsensitiveTest.kt | 170 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/JsonNamesTest.kt | 104 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/JsonNamingStrategyExclusionTest.kt | 60 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/JsonNamingStrategyTest.kt | 279 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/LocalClassesTest.kt | 84 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/LongAsStringTest.kt | 32 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/MetaSerializableJsonTest.kt | 72 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/ObjectSerialization.kt | 62 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PartiallyCustomSerializerTest.kt | 31 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PolymorphicDeserializationErrorMessagesTest.kt | 54 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PolymorphicOnClassesTest.kt | 149 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PolymorphicSealedChildTest.kt | 115 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PolymorphismForCustomTest.kt | 63 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PolymorphismTest.kt | 184 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PolymorphismWithAnyTest.kt | 149 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PrimitiveArraySerializersTest.kt | 65 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/PropertyInitializerTest.kt | 105 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/SealedClassesSerializationTest.kt | 264 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/SealedPolymorphismTest.kt | 63 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/SerializableOnTypeUsageTest.kt | 28 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/SerializableWithTest.kt | 77 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/SkipDefaults.kt | 67 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/UseSerializersTest.kt | 34 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/UuidTest.kt | 50 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/inline/EncodeInlineElementTest.kt | 48 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/inline/InlineClassesCompleteTest.kt | 133 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/inline/InlineClassesTest.kt | 214 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/inline/InlineMapQuotedTest.kt | 66 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/inline/UnsignedIntegersTest.kt | 140 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/inline/ValueClassesInSealedHierarchyTest.kt | 78 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/sealed/SealedChild.kt | 8 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/sealed/SealedDiamondTest.kt | 49 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/sealed/SealedInterfacesInlineSerialNameTest.kt | 51 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/sealed/SealedInterfacesJsonSerializationTest.kt | 40 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/features/sealed/SealedParent.kt | 6 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/AbstractJsonImplicitNullsTest.kt | 156 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/BasicTypesSerializationTest.kt | 58 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/DecodeFromJsonElementTest.kt | 59 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonBuildersTest.kt | 147 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonChunkedStringDecoderTest.kt | 74 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonCoerceInputValuesTest.kt | 181 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonConfigurationTest.kt | 31 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonCustomSerializersTest.kt | 354 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonDecodingErrorMessagesTest.kt | 278 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonDecodingRedactedErrorMessagesTest.kt | 47 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonDefaultContextTest.kt | 17 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonElementDecodingTest.kt | 120 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonEncoderDecoderRecursiveTest.kt | 213 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonEncodingErrorMessagesTest.kt | 30 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonExponentTest.kt | 78 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonGenericTest.kt | 55 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonHugeDataSerializationTest.kt | 40 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonIgnoreKeysTest.kt | 53 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonImplicitNullsTest.kt | 13 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonMapKeysTest.kt | 181 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonModesTest.kt | 128 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonNumericKeysTest.kt | 47 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonOptionalTests.kt | 53 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonParserFailureModesTest.kt | 184 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonParserTest.kt | 116 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonPrettyPrintTest.kt | 86 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonReifiedCollectionsTest.kt | 29 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonRootLevelNullTest.kt | 29 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonSealedSubclassTest.kt | 27 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonTestBase.kt | 204 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonTransformingSerializerTest.kt | 126 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonTransientTest.kt | 59 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonTreeAndMapperTest.kt | 96 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonTreeImplicitNullsTest.kt | 14 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonTreeTest.kt | 150 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonUnicodeTest.kt | 74 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonUnionEnumTest.kt | 27 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/JsonUpdateModeTest.kt | 62 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/LenientTest.kt | 96 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/MapLikeSerializerTest.kt | 66 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/SpecialFloatingPointValuesTest.kt | 55 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/TrailingCommaTest.kt | 128 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonClassDiscriminatorModeBaseTest.kt | 157 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonClassDiscriminatorModeTest.kt | 138 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonContentPolymorphicSerializerTest.kt | 103 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonDeserializePolymorphicTwiceTest.kt | 24 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonListPolymorphismTest.kt | 46 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonMapPolymorphismTest.kt | 94 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonNestedPolymorphismTest.kt | 67 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonNullablePolymorphicTest.kt | 33 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonPolymorphicClassDescriptorTest.kt | 26 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonPolymorphicObjectTest.kt | 44 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonPolymorphismExceptionTest.kt | 53 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonProhibitedPolymorphicKindsTest.kt | 98 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonPropertyPolymorphicTest.kt | 63 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/JsonTreeDecoderPolymorphicTest.kt | 43 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/polymorphic/PolymorphicClasses.kt | 62 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonArraySerializerTest.kt | 113 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonNativePrimitivesTest.kt | 29 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonNullSerializerTest.kt | 60 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonObjectSerializerTest.kt | 125 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonPrimitiveSerializerTest.kt | 218 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonSerializerInGenericsTest.kt | 37 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonTreeTest.kt | 151 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/JsonUnquotedLiteralTest.kt | 140 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/json/serializers/Primitives.kt | 22 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/modules/SerialNameCollisionInSealedClassesTest.kt | 61 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/modules/SerialNameCollisionTest.kt | 162 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/ContextualTest.kt | 47 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/CurrentPlatform.common.kt | 15 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/ExceptionsTesting.kt | 146 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/InternalHexConverter.kt | 51 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/JsonHelpers.kt | 9 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/TestHelpers.kt | 57 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/TestId.kt | 12 |  |
| formats/json-tests/commonTest/src/kotlinx/serialization/test/TestingFramework.kt | 100 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/BigDecimalTest.kt | 193 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/JavaCollectionsTest.kt | 112 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/JvmMissingFieldsExceptionTest.kt | 164 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/SerializationCasesTest.kt | 93 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/SerializeJavaClassTest.kt | 46 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/SerializerByTypeCacheTest.kt | 119 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/SerializerForNullableJavaTypeTest.kt | 44 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/StacktraceRecoveryTest.kt | 64 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/UuidPlatformClashTest.kt | 56 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/features/ContextualSerializationOnFileTest.kt | 41 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/features/InternalInheritanceTest.kt | 134 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/features/JsonJvmStreamsTest.kt | 129 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/features/JsonLazySequenceTest.kt | 197 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/features/JsonSequencePathTest.kt | 33 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/features/SerializerByTypeTest.kt | 278 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/json/GsonCompatibilityTest.kt | 56 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/json/JsonChunkedBase64DecoderTest.kt | 85 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/json/JsonConcurrentStressTest.kt | 78 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/json/MissingFieldExceptionWithPathTest.kt | 40 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/json/SpecConformanceTest.kt | 130 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/test/CurrentPlatform.kt | 8 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/test/JsonHelpers.kt | 20 |  |
| formats/json-tests/jvmTest/src/kotlinx/serialization/test/TypeToken.kt | 17 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

