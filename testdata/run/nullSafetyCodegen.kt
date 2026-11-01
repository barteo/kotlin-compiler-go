// Null checks kc emits where kotlinc does: safe calls on values typed
// non-null that are null at runtime, casts to non-null types, and the
// evaluation order of comparisons with null and with safe calls.

class Box(val v: Int)

class Late {
    val s: String = init()
    fun init(): String = s?.length?.toString() ?: "uninitialized"
}

@Suppress("UNCHECKED_CAST")
fun <T> unsafe(x: Any?): T = x as T

fun <T> id(x: T): T = x

var log = ""

fun side(n: Int): Int {
    log += "side$n "
    return n
}

fun foo(x: Int?): Int? {
    log += "foo($x) "
    return x
}

fun main() {
    // A safe call on a non-null String that is null at runtime.
    val s: String = unsafe(null)
    println(s?.length)
    println(Late().s)

    // A generic result used as Int? keeps its box.
    val n: Int? = id(1000)
    val m: Int? = id(1000)
    println(n === m || n == m)

    // A cast to a non-null type throws NullPointerException.
    val a: Any? = null
    try {
        println(a as String)
    } catch (e: NullPointerException) {
        println("NPE: ${e.message}")
    }
    println(a as String?)

    // Both sides of a comparison with null run.
    log = ""
    println(foo(null) == foo(null))
    println(log)

    // In a?.v == f(), f runs only if a isn't null.
    val b: Box? = null
    val c: Box? = Box(2)
    log = ""
    println(b?.v == side(1))
    println(b?.v != side(2))
    println(c?.v == side(2))
    println(0 == b?.v)
    println(log)
}
