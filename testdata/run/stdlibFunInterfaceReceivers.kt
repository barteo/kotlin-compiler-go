// Fun interfaces whose method has a receiver, as Compose's MeasurePolicy:
// a lambda converted to one has the receiver as this.
class Scope(val name: String) {
    fun scale(x: Int) = x * 2
}

fun interface Policy {
    fun Scope.measure(items: List<Int>, max: Int): String
}

fun interface Handler {
    suspend fun Scope.handle(): Int
}

fun measure(p: Policy) = with(p) { Scope("s").measure(listOf(1, 2), 10) }

suspend fun handle(h: Handler) = with(h) { Scope("t").handle() }

val P = Policy { items, max -> name + items.map { scale(it) } + max }

suspend fun main() {
    println(measure { items, max -> "$name ${items.sum()} $max ${scale(3)}" })
    println(measure(P))
    println(handle { name.length + scale(1) })
    val h = Handler { scale(name.length) }
    println(handle(h))
}
