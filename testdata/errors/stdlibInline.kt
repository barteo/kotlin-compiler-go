// Errors of inlined functions of the standard library.

import kotlin.reflect.typeOf

class Holder(val n: Int)

fun main() {
    println(Holder(if ("a".isNotEmpty()) 1 else 2).n)
    typeOf<List<String>>()
}

// Non-local jumps.

fun fromObject(): Int {
    // AutoCloseable { } keeps its lambda in an object.
    val c = AutoCloseable { return } // ERROR 'return' is not allowed here
    for (i in 0..1) {
        compareBy<Int> { if (it > i) continue; it } // ERROR 'continue' is not allowed outside a loop
    }
    return 0
}

fun inferred() = run { return 1 } // ERROR 'return' needs an explicit return type

fun labels(xs: List<Int>): Int {
    xs.let { return@nope 1 } // ERROR unresolved label: nope
    val f = { x: Int -> x.let { return@labels 2 } } // ERROR 'return@labels' is not allowed here: it would leave the innermost lambda
    return 0
}
