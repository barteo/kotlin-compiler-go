# Survey: kotlinpoet 2.4.0

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 39 files, 8696 lines

Directories: `kotlinpoet/src/commonMain`, `kotlinpoet/src/jvmMain`

### Features left, by files blocked

39 of 39 files use no feature that kc doesn't support.

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
| `apply` | 21 | 142 |
| `isNotEmpty` | 15 | 63 |
| `require` | 15 | 70 |
| `check` | 7 | 55 |
| `buildString` | 6 | 7 |
| `also` | 5 | 5 |
| `let` | 5 | 6 |
| `codePointAt` | 4 | 5 |
| `requireNotNull` | 3 | 4 |
| `with` | 3 | 3 |
| `List` | 2 | 2 |
| `error` | 2 | 3 |
| `use` | 2 | 4 |
| `AutoCloseable` | 1 | 1 |
| `bufferedWriter` | 1 | 1 |
| `buildList` | 1 | 2 |
| `checkNotNull` | 1 | 1 |
| `find` | 1 | 1 |
| `format` | 1 | 2 |
| `isNotBlank` | 1 | 1 |
| `matches` | 1 | 1 |
| `outputStream` | 1 | 1 |
| `replaceFirstChar` | 1 | 3 |
| `run` | 1 | 2 |
| `synchronized` | 1 | 1 |
| `thenBy` | 1 | 2 |
| `toBigDecimal` | 1 | 2 |
| `toRegex` | 1 | 1 |

### Files

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/CodePoint.kt | 34 |  |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/DelicateKotlinPoetApi.kt | 33 |  |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/ExperimentalKotlinPoetApi.kt | 27 |  |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/Import.kt | 32 |  |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/KModifier.kt | 80 |  |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/KOperator.kt | 44 |  |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/LineWrapper.kt | 154 |  |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/NameAllocator.kt | 174 |  |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/Util.kt | 373 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/Annotatable.kt | 52 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/AnnotationSpec.kt | 265 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/ClassName.kt | 342 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/CodeBlock.kt | 685 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/CodeBlockHolder.kt | 41 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/CodePoint.jvm.kt | 37 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/CodeWriter.kt | 864 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/ContextParameter.kt | 113 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/ContextReceivable.kt | 50 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/Documentable.kt | 30 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/Dynamic.kt | 29 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/FileSpec.kt | 599 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/FunSpec.kt | 704 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/LambdaTypeName.kt | 177 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/MemberName.kt | 174 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/MemberSpecHolder.kt | 67 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/OriginatingElementsHolder.kt | 47 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/ParameterSpec.kt | 260 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/ParameterizedTypeName.kt | 274 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/PropertySpec.kt | 402 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/Taggable.kt | 153 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/TypeAliasSpec.kt | 153 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/TypeName.kt | 446 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/TypeSpec.kt | 1060 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/TypeSpecHolder.kt | 30 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/TypeVariableName.kt | 314 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/Util.jvm.kt | 68 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/WildcardTypeName.kt | 158 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/jvm/JvmAnnotations.kt | 126 |  |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/tags/TypeAliasTag.kt | 25 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 37 files, 20965 lines

Directories: `kotlinpoet/src/jvmTest/kotlin`

### Features left, by files blocked

37 of 37 files use no feature that kc doesn't support.

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
| `apply` | 4 | 11 |
| `TODO` | 2 | 11 |
| `check` | 2 | 2 |
| `use` | 2 | 2 |
| `MutableList` | 1 | 2 |
| `also` | 1 | 1 |
| `find` | 1 | 1 |
| `synchronized` | 1 | 4 |

### Files

The features each file still needs.

| File | Lines | Blockers left |
|:---|---:|:---|
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/AbstractTypesTest.kt | 228 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/AnnotatedTypeNameTest.kt | 155 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/AnnotationSpecTest.kt | 551 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/Cased/Weird/Sup.kt | 20 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/ClassNameTest.kt | 392 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/CodeBlockTest.kt | 793 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/CrossplatformTest.kt | 241 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/DelegatedConstructorCallTest.kt | 193 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/DocumentableTest.kt | 91 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/ExpectDeclarationsTest.kt | 83 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/ExternalDeclarationsTest.kt | 199 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/FileReadingTest.kt | 70 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/FileSpecTest.kt | 1576 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/FileWritingTest.kt | 328 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/FunSpecTest.kt | 1880 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/KotlinPoetTest.kt | 1734 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/LambdaTypeNameTest.kt | 335 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/LineWrapperTest.kt | 241 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/LineWrappingTest.kt | 293 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/MemberNameTest.kt | 889 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/NameAllocatorTest.kt | 128 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/ParameterSpecTest.kt | 193 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/ParameterizedTypeNameTest.kt | 252 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/PropertySpecTest.kt | 958 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/StringsTest.kt | 97 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TaggableTest.kt | 109 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TestFiler.kt | 126 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TypeAliasSpecTest.kt | 270 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TypeNameKotlinTest.kt | 161 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TypeSpecTest.kt | 6118 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TypeVariableNameTest.kt | 370 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TypesEclipseTest.kt | 139 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TypesTest.kt | 33 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/UtilTest.kt | 212 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/ValueTypeSpecTest.kt | 277 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/WildcardTypeNameTest.kt | 62 |  |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/jvm/JvmAnnotationsTest.kt | 1168 |  |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

