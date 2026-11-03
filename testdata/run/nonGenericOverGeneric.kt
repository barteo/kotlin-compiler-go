// When no overload is the most specific, one without type parameters of
// its own is more specific than a generic one, and a generic call
// thrown infers a Throwable.

interface A
interface B : A
interface C
class D : B, C

fun B.foo(): Int = 1
fun <T> T.foo(): String where T : A, T : C = "generic"

abstract class Base {
    fun <T> bar(): String = "typed"
    @Suppress("INAPPLICABLE_JVM_NAME")
    @JvmName("barUntyped")
    fun bar(): String = "untyped"
}
class Sub : Base()

@Suppress("UNCHECKED_CAST")
fun <T> fail(msg: String): T = IllegalStateException(msg) as T

fun main() {
    val x: Int = D().foo()
    println(x)
    println(Sub().bar())
    println(Sub().bar<Int>())
    try {
        throw fail("thrown")
    } catch (e: IllegalStateException) {
        println(e.message)
    }
}
