// A lambda whose parameter is the builder's variable itself is checked
// once the builder's lambda has inferred the variable, whatever their
// order.
class Entry<T>(val key: T, val content: (T) -> String) {
    constructor(entry: Entry<T>, content: (T) -> String) : this(entry.key, content)
}

fun <T> first(fallback: (T) -> Entry<T>, builder: MutableList<T>.() -> Unit): Entry<T> =
    fallback(buildList(builder).first())

fun <T> second(builder: MutableList<T>.() -> Unit, fallback: (T) -> Entry<T>): Entry<T> =
    fallback(buildList(builder).first())

fun show(x: Int) = "int $x"
fun show(x: String) = "string $x"

fun main() {
    val a = first({ u -> Entry(u) { show(it) } }) { add("a") }
    println(a.content(a.key))
    val b = second({ add(1) }) { u -> println(show(u)); Entry(u) { show(it + 1) } }
    println(b.content(b.key))
}
