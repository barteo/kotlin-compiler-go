// A @Target that names a nested class, its own or inherited, is
// ambiguous with kotlin.annotation.Target, which kotlinc reads it as.
import kotlin.annotation.AnnotationTarget.TYPE

abstract class Base {
    annotation class Target(vararg val allowedTargets: AnnotationTarget)
}

class Derived : Base() {
    @Target(TYPE) // ERROR resolution of the annotation type is ambiguous between 'Base.Target' and the compiler-required annotation 'Target'
    annotation class Ann

    fun foo(x: @Ann String) {}
}

class Own {
    annotation class Target(val target: Int)

    @Target(1) // ERROR resolution of the annotation type is ambiguous between 'Own.Target'
    annotation class Ann
}

@kotlin.annotation.Target(TYPE)
annotation class Qualified
