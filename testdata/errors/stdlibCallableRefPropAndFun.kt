// Without an expected type, a reference to a name that is both a property
// and a function is ambiguous.
import kotlin.reflect.KProperty1

class A {
    val foo: Int = 0
    fun foo() {}

    val baz: Int = 0
}

fun A.bar(): String = "A"
val A.bar: Int get() = 1

val x0 = A::foo // ERROR ambiguity
val x1 = A::bar // ERROR ambiguity
val x2: KProperty1<A, Int> = A::foo
val x3: (A) -> Unit = A::foo
val x4 = A::baz
val x5 = String::length

fun main() {}
