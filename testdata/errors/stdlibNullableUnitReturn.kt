// A safe call of a Unit function is a Unit?, which a function that
// returns Unit can't return: kotlinc coerces only a lambda's last
// expression and a block body's statements to Unit.
class C { fun close() {} }

fun test(c: C?) {
    val f1 = fun() { c?.close() }
    val f2 = fun(): Unit = c?.close() // ERROR inferred type is Unit? but Unit was expected
    val f3 = fun() { return c?.close() } // ERROR inferred type is Unit? but Unit was expected
    val f4 = { c?.close() }
    val f5 = l@{ return@l c?.close() }
    val f6 = fun(): Unit { c?.close() }
    val f7: () -> Unit = l@{ return@l c?.close() } // ERROR inferred type is Unit? but Unit was expected
    val f8: () -> Unit = { c?.close() }
}

fun named1(c: C?): Unit = c?.close() // ERROR inferred type is Unit? but Unit was expected
fun named2(c: C?) { return c?.close() } // ERROR inferred type is Unit? but Unit was expected
fun named3(c: C?): Unit { return c?.close() } // ERROR inferred type is Unit? but Unit was expected
fun named4(c: C?) = c?.close()
fun named5(c: C?) { c?.close() }
fun named6(c: C?): Unit = run { c?.close() }
fun named7(c: C?): Unit = c?.close() ?: Unit
fun named8(c: C?): Unit { c?.let { return it.close() } }
fun named9(c: C?): Unit? = c?.close()

fun outer(c: C?) {
    fun local1(): Unit = c?.close() // ERROR inferred type is Unit? but Unit was expected
    fun local2() { c?.close() }
}
