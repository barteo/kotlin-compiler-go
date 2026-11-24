fun <T> par(a: MutableList<T>.() -> Unit, b: MutableList<T>.() -> Unit): List<T> {
    val l = mutableListOf<T>()
    l.a()
    l.b()
    return l
}

class Sink<T> { val got = mutableListOf<T>(); fun take(t: T) { got.add(t) }; fun last(): T = got.last() }
fun <T> both(first: Sink<T>.() -> Unit, second: (Sink<T>) -> Unit): Sink<T> = Sink<T>().also { it.first(); second(it) }

fun main() {
    // Two lambdas of one call, checked together.
    println(par({ add(1) }, { add(2); println(get(0) + 1) }))
    val s = both({ take("a") }) { it.take(it.last() + "b"); println(it.last().length) }
    println(s.got)

    // A builder in a branch.
    val c = System.currentTimeMillis() > 0
    val x = if (c) buildList { add("a"); println(get(0).length) } else emptyList()
    println(x)
    val y = when {
        c -> buildSet { add(1.5); println(first() * 2) }
        else -> setOf(2.5)
    }
    println(y)
    val z = if (!c) listOf(1L) else buildList { add(2L); add(get(0) + 1) }
    println(z)
}
