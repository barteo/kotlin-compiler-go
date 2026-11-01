# Survey: kotlinpoet 2.4.0

Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.

## main: 39 files, 8696 lines

Directories: `kotlinpoet/src/commonMain`, `kotlinpoet/src/jvmMain`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 39 | 11 |
| 1: packages, annotations | 1 | 38 | 10 |
| 2: interfaces, inheritance | 1 | 38 | 8 |
| 3: type checks | 1 | 38 | 8 |
| 4: exceptions | 1 | 38 | 7 |
| 5: objects | 1 | 38 | 6 |
| 6: data, enum, sealed | 2 | 37 | 5 |
| 7: arrays, vararg, spread | 4 | 35 | 4 |
| 8: Char, Float, Byte, Short | 4 | 35 | 4 |
| 9: inliner | 10 | 29 | 3 |
| 10: lambdas with receivers | 10 | 29 | 3 |
| 11: extensions | 11 | 28 | 2 |
| 12: operator conventions | 11 | 28 | 2 |
| 13: identity, class literals, references | 15 | 24 | 2 |
| 14: class delegation | 18 | 21 | 1 |
| 15: secondary constructors | 20 | 19 | 1 |
| 16: local functions | 21 | 18 | 1 |
| 17: expect and actual | 23 | 16 | 1 |
| 18: annotation classes | 25 | 14 | 1 |
| 19: value classes | 29 | 10 | 2 |
| 20: unsigned integers | 29 | 10 | 2 |
| definite assignment | 29 | 10 | 2 |
| 21: inline functions | 30 | 9 | 1 |
| 22: reified type parameters | 32 | 7 | 1 |
| 23: labels, do-while | 33 | 6 | 1 |
| 24: delegated properties | 34 | 5 | 1 |
| 25: object expressions | 36 | 3 | 1 |
| 26: destructuring lambda parameters | 39 | 0 | – |
| 42-49: coroutines | 39 | 0 | – |
| 52: bitwise operators | 39 | 0 | – |
| 53: typealias | 39 | 0 | – |
| 54: setters, backing fields | 39 | 0 | – |
| 55: lateinit, tailrec | 39 | 0 | – |
| 56-57: collection supertypes | 39 | 0 | – |
| 58: member extension functions | 39 | 0 | – |
| 59: reified member type parameters | 39 | 0 | – |
| 60: inner and local classes | 39 | 0 | – |
| 61: when guards, anonymous functions, local extension functions | 39 | 0 | – |
| 62a: @JvmField and @JvmStatic in objects | 39 | 0 | – |
| 62b: generic value classes | 39 | 0 | – |
| 63: definitely non-nullable types | 39 | 0 | – |
| 70a: inline property accessors | 39 | 0 | – |
| 70b: where clauses | 39 | 0 | – |
| 70c: fun interfaces | 39 | 0 | – |
| delegated extension properties | 39 | 0 | – |
| qualified super | 39 | 0 | – |
| context parameters of top-level functions | 39 | 0 | – |
| also supported | 39 | 0 | – |

### Features left, by files blocked

39 of 39 files use no feature that kc doesn't support.

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

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/CodePoint.kt | 34 | (annotation, extension function, function without body, inline-only call, modifier expect, modifier value, package) |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/DelicateKotlinPoetApi.kt | 33 | (annotation, modifier annotation, package) |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/ExperimentalKotlinPoetApi.kt | 27 | (annotation, modifier annotation, package) |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/Import.kt | 32 | (annotation, default value, inheritance, inline-only call, modifier data, modifier override, package) |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/KModifier.kt | 80 | (modifier enum, modifier vararg, nested class, package, require, check, error) |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/KOperator.kt | 44 | (modifier enum, package) |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/LineWrapper.kt | 154 | (Char, arrays, default value, inheritance, inline-only call, modifier companion, modifier override, package, property getter, require, check, error) |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/NameAllocator.kt | 174 | (Char, annotation, default value, inline-only call, modifier operator, named argument, package, require, check, error, secondary constructor, throw) |
| kotlinpoet/src/commonMain/kotlin/com/squareup/kotlinpoet/Util.kt | 373 | (Char, callable reference, default value, extension function, extension property, function without body, inheritance, inline-only call, modifier const, modifier expect, modifier inline, modifier override, modifier reified, modifier vararg, object, package, property getter, property without initializer, require, check, error) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/Annotatable.kt | 52 | (annotation, as, inline-only call, interface, nested class, package, property without initializer) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/AnnotationSpec.kt | 265 | (Char, Float, annotation, arrays, as, class delegation, class literal, default value, delegated property, identity equality, inheritance, inline-only call, is, modifier companion, modifier enum, modifier override, modifier vararg, named argument, nested class, package, property getter, require, check, error, spread operator, throw, try) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/ClassName.kt | 342 | (Char, annotation, as, default value, extension function, inheritance, inline-only call, local function, modifier companion, modifier override, modifier vararg, package, require, check, error, secondary constructor, spread operator, super, throw) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/CodeBlock.kt | 685 | (Char, Float, annotation, arrays, default value, do-while, extension function, extension property, function type with receiver, identity equality, inline-only call, is, local function, modifier companion, modifier const, modifier inline, modifier override, modifier vararg, named argument, nested class, package, property getter, qualified this, require, check, error, spread operator, throw, uninitialized local variable) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/CodeBlockHolder.kt | 41 | (function without body, interface, modifier vararg, nested class, package, property without initializer) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/CodePoint.jvm.kt | 37 | (extension function, inline-only call, modifier actual, package) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/CodeWriter.kt | 864 | (Char, annotation, as, callable reference, default value, destructuring declaration, destructuring lambda parameter, extension function, function type with receiver, identity equality, inheritance, inline-only call, is, labels, modifier companion, modifier const, modifier inline, modifier override, modifier vararg, named argument, package, qualified this, require, check, error, spread operator, try, when subject variable) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/ContextParameter.kt | 113 | (annotation, as, extension function, identity equality, inheritance, inline-only call, interface, modifier override, modifier value, named argument, nested class, package, property without initializer, secondary constructor) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/ContextReceivable.kt | 50 | (annotation, as, extension function, inheritance, inline-only call, interface, modifier override, modifier value, modifier vararg, nested class, package, property without initializer) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/Documentable.kt | 30 | (annotation, as, inline-only call, interface, modifier vararg, nested class, package, property without initializer, spread operator) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/Dynamic.kt | 29 | (inheritance, inline-only call, modifier override, object, package, throw) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/FileSpec.kt | 599 | (Char, annotation, arrays, callable reference, class delegation, class literal, default value, destructuring declaration, identity equality, inheritance, inline-only call, is, modifier companion, modifier const, modifier override, modifier vararg, named argument, nested class, object expression, package, property getter, qualified this, require, check, error, spread operator, super, throw, try) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/FunSpec.kt | 704 | (annotation, arrays, as, class delegation, class literal, default value, destructuring declaration, extension function, extension property, identity equality, inheritance, inline-only call, is, modifier companion, modifier const, modifier override, modifier vararg, named argument, nested class, package, property getter, require, check, error, spread operator, super, throw) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/LambdaTypeName.kt | 177 | (annotation, arrays, as, class literal, default value, inheritance, inline-only call, is, modifier companion, modifier override, modifier vararg, named argument, package, require, check, error, super) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/MemberName.kt | 174 | (Char, annotation, default value, extension function, inline-only call, modifier companion, modifier data, modifier inline, modifier override, named argument, package, secondary constructor) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/MemberSpecHolder.kt | 67 | (annotation, as, callable reference, function without body, inline-only call, interface, modifier vararg, nested class, package, property without initializer, spread operator) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/OriginatingElementsHolder.kt | 47 | (annotation, as, extension function, inheritance, inline-only call, interface, modifier override, modifier value, nested class, package, property without initializer) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/ParameterSpec.kt | 260 | (annotation, callable reference, class delegation, default value, extension function, identity equality, inheritance, inline-only call, modifier companion, modifier override, modifier vararg, nested class, package, secondary constructor, spread operator, super, throw) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/ParameterizedTypeName.kt | 274 | (annotation, arrays, as, class literal, default value, destructuring lambda parameter, extension function, inheritance, inline-only call, is, modifier companion, modifier override, modifier vararg, named argument, package, require, check, error, super, throw) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/PropertySpec.kt | 402 | (annotation, class delegation, class literal, default value, identity equality, inheritance, inline-only call, is, modifier companion, modifier override, modifier vararg, named argument, nested class, package, require, check, error, spread operator, super, throw) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/Taggable.kt | 153 | (annotation, as, class literal, extension function, inheritance, inline-only call, interface, modifier companion, modifier inline, modifier operator, modifier override, modifier reified, modifier value, nested class, package, property getter, property without initializer) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/TypeAliasSpec.kt | 153 | (annotation, callable reference, class delegation, default value, identity equality, inheritance, inline-only call, modifier companion, modifier override, modifier vararg, nested class, package, require, check, error, spread operator, super) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/TypeName.kt | 446 | (Byte and Short, Char, Float, annotation, as, class delegation, class literal, default value, delegated property, extension function, function without body, identity equality, inheritance, inline-only call, is, modifier abstract, modifier companion, modifier final, modifier inline, modifier open, modifier override, modifier reified, modifier sealed, modifier vararg, named argument, object expression, package, property getter, super, throw) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/TypeSpec.kt | 1060 | (annotation, callable reference, class delegation, class literal, default value, destructuring declaration, destructuring lambda parameter, identity equality, inheritance, inline-only call, local function, modifier companion, modifier enum, modifier override, modifier vararg, named argument, nested class, package, property getter, require, check, error, spread operator, super, throw, try) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/TypeSpecHolder.kt | 30 | (annotation, as, function without body, inline-only call, interface, nested class, package, property without initializer) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/TypeVariableName.kt | 314 | (annotation, as, callable reference, default value, extension function, inheritance, inline-only call, modifier companion, modifier operator, modifier override, modifier vararg, named argument, package, require, check, error, super) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/Util.jvm.kt | 68 | (Char, default value, extension function, extension property, inline-only call, is, modifier actual, package, property getter) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/WildcardTypeName.kt | 158 | (annotation, as, default value, extension function, inheritance, modifier companion, modifier override, named argument, package, require, check, error, super) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/jvm/JvmAnnotations.kt | 126 | (annotation, callable reference, class literal, default value, extension function, inline-only call, modifier vararg, named argument, package, require, check, error) |
| kotlinpoet/src/jvmMain/kotlin/com/squareup/kotlinpoet/tags/TypeAliasTag.kt | 25 | (package) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

## tests: 37 files, 20965 lines

Directories: `kotlinpoet/src/jvmTest/kotlin`

### Milestone by milestone

The features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.

| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |
|:---|---:|---:|---:|
| (before) | 0 | 37 | 6 |
| 1: packages, annotations | 2 | 35 | 5 |
| 2: interfaces, inheritance | 3 | 34 | 5 |
| 3: type checks | 3 | 34 | 5 |
| 4: exceptions | 3 | 34 | 4 |
| 5: objects | 4 | 33 | 3 |
| 6: data, enum, sealed | 6 | 31 | 3 |
| 7: arrays, vararg, spread | 6 | 31 | 2 |
| 8: Char, Float, Byte, Short | 6 | 31 | 2 |
| 9: inliner | 7 | 30 | 1 |
| 10: lambdas with receivers | 7 | 30 | 1 |
| 11: extensions | 7 | 30 | 1 |
| 12: operator conventions | 7 | 30 | 1 |
| 13: identity, class literals, references | 26 | 11 | 1 |
| 14: class delegation | 26 | 11 | 1 |
| 15: secondary constructors | 26 | 11 | 1 |
| 16: local functions | 27 | 10 | 1 |
| 17: expect and actual | 27 | 10 | 1 |
| 18: annotation classes | 30 | 7 | 1 |
| 19: value classes | 30 | 7 | 1 |
| 20: unsigned integers | 30 | 7 | 1 |
| definite assignment | 30 | 7 | 1 |
| 21: inline functions | 30 | 7 | 1 |
| 22: reified type parameters | 30 | 7 | 1 |
| 23: labels, do-while | 30 | 7 | 1 |
| 24: delegated properties | 30 | 7 | 1 |
| 25: object expressions | 32 | 5 | 1 |
| 26: destructuring lambda parameters | 32 | 5 | 1 |
| 42-49: coroutines | 32 | 5 | 1 |
| 52: bitwise operators | 32 | 5 | 1 |
| 53: typealias | 32 | 5 | 1 |
| 54: setters, backing fields | 32 | 5 | 1 |
| 55: lateinit, tailrec | 32 | 5 | 1 |
| 56-57: collection supertypes | 32 | 5 | 1 |
| 58: member extension functions | 32 | 5 | 1 |
| 59: reified member type parameters | 32 | 5 | 1 |
| 60: inner and local classes | 35 | 2 | 1 |
| 61: when guards, anonymous functions, local extension functions | 35 | 2 | 1 |
| 62a: @JvmField and @JvmStatic in objects | 35 | 2 | 1 |
| 62b: generic value classes | 35 | 2 | 1 |
| 63: definitely non-nullable types | 35 | 2 | 1 |
| 70a: inline property accessors | 35 | 2 | 1 |
| 70b: where clauses | 37 | 0 | – |
| 70c: fun interfaces | 37 | 0 | – |
| delegated extension properties | 37 | 0 | – |
| qualified super | 37 | 0 | – |
| context parameters of top-level functions | 37 | 0 | – |
| also supported | 37 | 0 | – |

### Features left, by files blocked

37 of 37 files use no feature that kc doesn't support.

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
| `apply` | 4 | 11 |
| `TODO` | 2 | 11 |
| `check` | 2 | 2 |
| `use` | 2 | 2 |
| `MutableList` | 1 | 2 |
| `also` | 1 | 1 |
| `find` | 1 | 1 |
| `synchronized` | 1 | 4 |

### Files

The features each file still needs, then, in parentheses, the ones the milestones above added.

| File | Lines | Blockers left |
|:---|---:|:---|
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/AbstractTypesTest.kt | 228 | (annotation, arrays, as, class literal, inheritance, inline-only call, modifier abstract, modifier override, modifier protected, nested class, package, property without initializer, throw, try, where clause) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/AnnotatedTypeNameTest.kt | 155 | (Byte and Short, annotation, class literal, modifier annotation, named argument, nested class, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/AnnotationSpecTest.kt | 551 | (Byte and Short, Char, Float, annotation, arrays, class literal, default value, is, modifier annotation, modifier enum, modifier inner, modifier override, modifier vararg, named argument, nested class, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/Cased/Weird/Sup.kt | 20 | (object, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/ClassNameTest.kt | 392 | (Byte and Short, Char, Float, annotation, arrays, class literal, named argument, nested class, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/CodeBlockTest.kt | 793 | (Char, Float, annotation, as, class literal, destructuring declaration, inline-only call, named argument, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/CrossplatformTest.kt | 241 | (annotation, class literal, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/DelegatedConstructorCallTest.kt | 193 | (annotation, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/DocumentableTest.kt | 91 | (annotation, class literal, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/ExpectDeclarationsTest.kt | 83 | (annotation, class literal, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/ExternalDeclarationsTest.kt | 199 | (annotation, class literal, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/FileReadingTest.kt | 70 | (annotation, arrays, callable reference, inline-only call, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/FileSpecTest.kt | 1576 | (Byte and Short, Char, annotation, class literal, inline-only call, is, named argument, nested class, package, require, check, error) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/FileWritingTest.kt | 328 | (annotation, arrays, class literal, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/FunSpecTest.kt | 1880 | (Char, annotation, arrays, as, class literal, function without body, inheritance, inline-only call, interface, modifier abstract, modifier annotation, modifier companion, modifier lateinit, modifier open, modifier override, modifier protected, named argument, nested class, package, property without initializer, throw, where clause) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/KotlinPoetTest.kt | 1734 | (Char, Float, annotation, arrays, as, class literal, named argument, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/LambdaTypeNameTest.kt | 335 | (annotation, arrays, class literal, modifier annotation, modifier inner, named argument, nested class, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/LineWrapperTest.kt | 241 | (annotation, named argument, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/LineWrappingTest.kt | 293 | (annotation, class literal, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/MemberNameTest.kt | 889 | (annotation, class literal, named argument, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/NameAllocatorTest.kt | 128 | (annotation, named argument, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/ParameterSpecTest.kt | 193 | (Char, annotation, class literal, named argument, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/ParameterizedTypeNameTest.kt | 252 | (annotation, arrays, as, callable reference, class literal, interface, named argument, nested class, object, package, property without initializer, throw) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/PropertySpecTest.kt | 958 | (Char, annotation, class literal, modifier annotation, named argument, nested class, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/StringsTest.kt | 97 | (annotation, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TaggableTest.kt | 109 | (annotation, arrays, class literal, extension function, inline-only call, is, modifier companion, package, require, check, error) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TestFiler.kt | 126 | (arrays, inheritance, inline-only call, modifier inner, modifier override, modifier vararg, nested class, package, require, check, error, throw) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TypeAliasSpecTest.kt | 270 | (annotation, class literal, modifier annotation, named argument, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TypeNameKotlinTest.kt | 161 | (annotation, arrays, class literal, named argument, nested class, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TypeSpecTest.kt | 6118 | (Char, annotation, arrays, as, class literal, inheritance, inline-only call, modifier companion, modifier const, modifier override, named argument, object expression, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TypeVariableNameTest.kt | 370 | (annotation, arrays, class literal, local function, named argument, nested class, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TypesEclipseTest.kt | 139 | (annotation, class literal, inheritance, modifier override, nested class, object expression, package, property getter, require, check, error, super, throw, try) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/TypesTest.kt | 33 | (annotation, inheritance, modifier override, package, property getter) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/UtilTest.kt | 212 | (Char, annotation, arrays, inline-only call, named argument, package, spread operator) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/ValueTypeSpecTest.kt | 277 | (annotation, arrays, class literal, interface, modifier companion, nested class, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/WildcardTypeNameTest.kt | 62 | (annotation, class literal, named argument, package) |
| kotlinpoet/src/jvmTest/kotlin/com/squareup/kotlinpoet/jvm/JvmAnnotationsTest.kt | 1168 | (annotation, class literal, inline-only call, named argument, package) |

### Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors. 0 of them name a feature that the detector also found in the file.

### Cross-check with the checker

The checker, run on the library files together, reports unsupported features that the detector didn't find in 0 files.

