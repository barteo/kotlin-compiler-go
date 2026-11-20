// A Java override's result of a platform type is nullable where the Java
// method it overrides is @Nullable: JUnknownImpl's foo() returns Any?,
// which JNotNull's Any foo() doesn't take. Kotlin's
// kt13355viaJava.kt test.

abstract class ATest2 : TestNN.JUnknownImpl(), TestNN.JNotNull // ERROR return types of inherited members 'foo' are incompatible

abstract class ATest3 : TestNN.JUnknownVsNotNull()
