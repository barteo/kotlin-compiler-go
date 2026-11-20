// A class can't inherit from an annotation class, directly or through
// its supertypes, as kotlinc's EXTENDING_AN_ANNOTATION_CLASS_ERROR with
// Kotlin 2.4's ProhibitExtendingAnnotationClasses.
abstract class A : java.lang.annotation.Retention // ERROR inheriting an annotation class is prohibited
abstract class B : A() // ERROR inheriting an annotation class is prohibited
interface I : java.lang.annotation.Documented // ERROR inheriting an annotation class is prohibited
abstract class C : I // ERROR inheriting an annotation class is prohibited
fun f(): Any = object : I {} // ERROR inheriting an annotation class is prohibited
abstract class Fine : java.lang.annotation.Annotation
