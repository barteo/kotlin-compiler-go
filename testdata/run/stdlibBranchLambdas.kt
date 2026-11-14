// Lambdas in branches take the other branches' function type, its
// receiver as their first parameter; a typed lambda for a bare variable
// has its own type; run { } in a class keeps the class's smart casts.

fun pick(b: Boolean): String.(String) -> String =
    if (b) {{ s, t -> s + "+" + t }} else {{ s: String, t: String -> t + "-" + s }}

fun op(n: Int): Int {
    val g = when (n) {
        0 -> {{ a, b -> a * b }}
        1 -> { a: Int, b: Int -> a + b }
        else -> {{ a, b -> a - b }}
    }
    return g(6, 3)
}

open class A<H : Function2<*, *, Function0<String>>>(val f: H)
class B : A<Function2<*, *, Function0<String>>>({ p1: List<*>, p2: String -> { "" + p1.size + p2 } })

class Box { fun m() = 1 }
operator fun Box.invoke() = 2
class Smart(val box: Box?) {
    fun f() = if (box != null) run { box.m() + box() } else 0
}

fun main() {
    println("a".(pick(true))("b"))
    println("a".(pick(false))("b"))
    println(listOf(op(0), op(1), op(2)))
    @Suppress("UNCHECKED_CAST")
    val f = B().f as (List<Int>, String) -> () -> String
    println(f(listOf(1, 2), "x")())
    val l = listOf({ x: Int -> x + 1 }, "s")
    println(l.size)
    println(Smart(Box()).f())
    println(Smart(null).f())
    val n = if (op(0) > 0) { { "lazy" } } else null
    println(n?.invoke())
}
