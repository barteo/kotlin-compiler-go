# kotlinpoet on kc's library

Written by `bench/kotlinpoet.sh`. square/kotlinpoet 2.4.0; openjdk version "21.0.12.1" 2026-08-18.

## kotlinpoet's Gradle build

In the kc build, kc compiles `:kotlinpoet`'s JVM library, and kotlinc compiles its tests and the interop modules against kc's classes, reading their Kotlin metadata.

| Task | kotlinc (control) | kc |
|:---|:---|:---|
| `:kotlinpoet:compileKotlinJvm` | ran | ran |
| `:kotlinpoet:compileTestKotlinJvm` | ran | ran |
| `:kotlinpoet:compileJvmTestJava` | ran | ran |
| `:kotlinpoet:jvmTest` | ran | ran |
| `:kotlinpoet:jvmApiCheck` | ran | ran |
| `:interop:kotlin-metadata:compileKotlin` | ran | ran |
| `:interop:kotlin-metadata:compileTestKotlin` | ran | ran |
| `:interop:kotlin-metadata:test` | ran | ran |
| `:interop:ksp:compileKotlin` | ran | ran |
| `:interop:ksp:test-processor:compileKotlin` | ran | ran |
| `:interop:ksp:test-processor:compileTestKotlin` | ran | ran |
| `:interop:ksp:test-processor:test` | ran | ran |

| Tests | Build | Passed | Failed | Skipped |
|:---|:---|---:|---:|---:|
| `:kotlinpoet` | kotlinc (control) | 966 | 0 | 40 |
| `:kotlinpoet` | kc | 966 | 0 | 40 |
| `:interop:kotlin-metadata` | kotlinc (control) | 131 | 0 | 27 |
| `:interop:kotlin-metadata` | kc | 131 | 0 | 27 |
| `:interop:ksp:test-processor` | kotlinc (control) | 19 | 0 | 0 |
| `:interop:ksp:test-processor` | kc | 19 | 0 | 0 |

## kotlinc's tests on kc's classes

The control build's JVM tests, compiled by kotlinc against kotlinc's build of the library, run twice with JUnit 4 and `java -Xverify:all`: on kotlinc's library classes, and with kc's in their place. This needs no metadata from kc, only binary compatibility.

| Library classes | Passed | Failed | Skipped |
|:---|---:|---:|---:|
| kotlinc (control) | 957 | 0 | 6 |
| kc | 957 | 0 | 6 |

Every test that passes on kotlinc's classes passes on kc's.

### Test classes

Passed, failed and skipped tests.

| Class | kotlinc | kc |
|:---|:---|:---|
| `AnnotatedTypeNameTest` | 8 / 0 / 2 | 8 / 0 / 2 |
| `AnnotationSpecTest` | 15 / 0 / 0 | 15 / 0 / 0 |
| `ClassNameTest` | 27 / 0 / 0 | 27 / 0 / 0 |
| `CodeBlockTest` | 68 / 0 / 0 | 68 / 0 / 0 |
| `CrossplatformTest` | 9 / 0 / 0 | 9 / 0 / 0 |
| `DelegatedConstructorCallTest` | 12 / 0 / 0 | 12 / 0 / 0 |
| `DocumentableTest` | 1 / 0 / 0 | 1 / 0 / 0 |
| `ExpectDeclarationsTest` | 4 / 0 / 0 | 4 / 0 / 0 |
| `ExternalDeclarationsTest` | 10 / 0 / 0 | 10 / 0 / 0 |
| `FileReadingTest` | 4 / 0 / 0 | 4 / 0 / 0 |
| `FileSpecTest` | 56 / 0 / 1 | 56 / 0 / 1 |
| `FileWritingTest` | 14 / 0 / 0 | 14 / 0 / 0 |
| `FunSpecTest` | 91 / 0 / 0 | 91 / 0 / 0 |
| `JavaAnnotationSpecTest` | 2 / 0 / 0 | 2 / 0 / 0 |
| `KotlinPoetTest` | 50 / 0 / 0 | 50 / 0 / 0 |
| `LambdaTypeNameTest` | 21 / 0 / 0 | 21 / 0 / 0 |
| `LineWrapperTest` | 14 / 0 / 0 | 14 / 0 / 0 |
| `LineWrappingTest` | 8 / 0 / 0 | 8 / 0 / 0 |
| `MemberNameTest` | 29 / 0 / 0 | 29 / 0 / 0 |
| `NameAllocatorTest` | 12 / 0 / 0 | 12 / 0 / 0 |
| `ParameterSpecTest` | 11 / 0 / 0 | 11 / 0 / 0 |
| `ParameterizedTypeNameTest` | 24 / 0 / 0 | 24 / 0 / 0 |
| `PropertySpecTest` | 48 / 0 / 0 | 48 / 0 / 0 |
| `StringsTest` | 5 / 0 / 0 | 5 / 0 / 0 |
| `TaggableTest` | 42 / 0 / 0 | 42 / 0 / 0 |
| `TypeAliasSpecTest` | 16 / 0 / 0 | 16 / 0 / 0 |
| `TypeNameKotlinTest` | 19 / 0 / 0 | 19 / 0 / 0 |
| `TypeSpecTest` | 234 / 0 / 1 | 234 / 0 / 1 |
| `TypeVariableNameTest` | 23 / 0 / 0 | 23 / 0 / 0 |
| `TypesEclipseTest` | 0 / 0 / 1 | 0 / 0 / 1 |
| `TypesTest` | 0 / 0 / 1 | 0 / 0 / 1 |
| `UtilTest` | 13 / 0 / 0 | 13 / 0 / 0 |
| `ValueTypeSpecTest` | 20 / 0 / 0 | 20 / 0 / 0 |
| `WildcardTypeNameTest` | 3 / 0 / 0 | 3 / 0 / 0 |
| `jvm.JvmAnnotationsTest` | 44 / 0 / 0 | 44 / 0 / 0 |
