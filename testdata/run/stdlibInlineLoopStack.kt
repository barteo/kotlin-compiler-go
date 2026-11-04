// Library inline functions that loop, with values that the caller pushed
// before the call, as inlineLoopStack.kt's: TestRun checks that no loop's
// backward branch has values on the operand stack.

class Named(val name: String, val total: Int) {
    override fun toString() = "Named($name, $total)"
}

fun pair(a: Any, b: Any) = "$a/$b"

fun main() {
    val xs = listOf(3, 1, 4, 1, 5, 9, 2, 6)
    val sb = StringBuilder()
    sb.append(xs.sumOf { it * 2 })
    println(sb)
    println(pair("sum", xs.sumOf { it.toLong() }))
    println(10 + xs.count { it > 2 })
    println(Named("n", xs.sumOf { it }))
    println(Named("m", xs.map { it + 1 }.fold(0) { a, b -> a + b }))
    println(pair(xs.size, xs.let { l -> var t = 0; for (x in l) t += x; t }))
    println(pair(1.5, xs.filter { it % 2 == 1 }.joinToString()))
    println(Named("r", runCatching { xs.first { it > 4 } }.getOrDefault(0)))
    var acc = 0L
    acc += xs.sumOf { x -> xs.count { it < x }.toLong() }
    println(acc)
    println(buildString { for (x in xs) append(x) }.length + xs.maxOf { it })
}
