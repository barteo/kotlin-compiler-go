// A @Target argument of another enum than AnnotationTarget is a mismatch,
// and the annotation class then has no targets, as in K2.

abstract class Base {
    enum class AnnotationTarget { TYPE, CLASS }
}

class Derived : Base() {
    @Target(AnnotationTarget.TYPE) // ERROR type mismatch
    annotation class Ann

    @Target(kotlin.annotation.AnnotationTarget.TYPE)
    annotation class Ok

    fun foo(x: @Ann String, y: @Ok String) {} // ERROR this annotation is not applicable to target 'type usage'

    @Ann // ERROR this annotation is not applicable to target
    class C
}
