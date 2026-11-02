// The names of annotations, and the declarations of annotation classes.

@Missing class A // ERROR unresolved reference: Missing
@kotlin.Missing(1) class B // ERROR unresolved reference: kotlin.Missing
class Foo
@Foo class C // ERROR Foo is not an annotation class
class D<T> {
    @T fun f() = 0 // ERROR not a class
}
class E(@E(1) val x: Int) // ERROR E is not an annotation class

class Outer {
    annotation class Nested
    @Nested fun f() = 0
    @Nested companion object {
        annotation class InCompanion
    }
    @InCompanion val x = 0
}

@Target(AnnotationTarget.EXPRESSION)
@Retention(AnnotationRetention.BINARY) // ERROR expression annotations with retention other than SOURCE are prohibited
annotation class Expr

@Target(AnnotationTarget.EXPRESSION)
@Retention(AnnotationRetention.SOURCE)
annotation class SourceExpr

fun local() {
    annotation class L // ERROR annotation class cannot be local
    @Missing val y = 0 // ERROR unresolved reference: Missing
}

// An annotation class's parameters can't lead back to it.
annotation class Cyc(val c: Cyc) // ERROR type of this annotation parameter refers back to its annotation class
annotation class CycArray(val c: Array<CycArray>)

// Containers of repeatable annotations.
@java.lang.annotation.Repeatable(NoValue::class) // ERROR must have a 'value' property
annotation class R1
annotation class NoValue
@java.lang.annotation.Repeatable(Holder::class)
annotation class R2
annotation class Holder(val value: Array<R2>, val other: Int = 0)
@Repeatable // ERROR repeatable annotation cannot have a nested class named 'Container'
annotation class R3 {
    class Container
}

// Setter use-site targets need a mutable property.
annotation class Ann
class Props {
    @set:Ann val fixed = 1 // ERROR '@set:' annotations could be applied only to mutable properties
    @set:Ann var free = 1
}

// @all: applies an annotation to the property, its field and its accessors,
// so repeating it there repeats the annotation.
@Target(AnnotationTarget.PROPERTY, AnnotationTarget.FIELD)
annotation class Once
class AllTargets {
    @all:Once @field:Once val v = 0 // ERROR this annotation is not repeatable
}
