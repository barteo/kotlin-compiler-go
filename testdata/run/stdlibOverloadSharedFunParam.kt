// Overloads that take the same function type for an argument give it to
// the argument as its expected type: the lambda that remember-like keep
// returns has the parameter's receiver, whose member extension it calls.
class Scope(val name: String) {
    fun String.at(x: Int) = println("$name: $this at $x")
}

fun chart(p: (Scope.(Long) -> Unit)? = null, step: ((Int, Double) -> Double)? = null) {
    println("two-argument step: ${step?.invoke(1, 2.0)}")
    p?.invoke(Scope("two"), 2L)
}

fun chart(p: (Scope.(Long) -> Unit)? = null, step: ((Int) -> Double)? = null) {
    println("one-argument step: ${step?.invoke(1)}")
    p?.invoke(Scope("one"), 1L)
}

fun <T> keep(key: Any?, calc: () -> T): T = calc()

fun use(ms: List<Pair<String, Int>>?) {
    chart(p = keep(ms) { { n -> ms?.forEach { (m, x) -> m.at(x + n.toInt()) } } }, step = { i -> i * 1.5 })
    chart(p = keep(ms) { { ms?.forEach { (m, x) -> m.at(x) } } }, step = { i, d -> i + d })
}

fun main() = use(listOf("a" to 1, "b" to 2))
