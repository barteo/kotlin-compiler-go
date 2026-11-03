// Calls resolving on nullable, Nothing? and type-parameter receivers.

fun String?.a() = "a:" + this
fun Int?.b() = "b:" + this

interface A { fun foo(): Any = "A" }
interface B { fun bar(): String = "B" }
class AB : A, B

fun <T> both(t: T): String where T : A, T : B = t.foo().toString() + t.bar()

fun localBound(): Double {
    class Box<T : Number>(val x: T) { fun d() = x.toDouble() }
    return Box(3).d()
}

fun main() {
    val s: String? = null
    println(s + "!")
    println(s.plus(1))
    val f = String?::plus
    println(f(null, 2))
    println(f("x", null))
    val i: Int? = null
    if (i == null) println(i.b())
    val r = String?::a
    println(r(null))
    println(null.a())
    println(both(AB()))
    println(localBound())
}
