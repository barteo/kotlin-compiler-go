// Overloads chosen in a builder's lambda, before its variables are fixed:
// an inapplicable member gives way to an extension, and what a rejected
// candidate would have constrained doesn't count.

class Maker<T> {
    val got = mutableListOf<String>()
    fun make(vararg values: T): Maker<T> { got += "vararg " + values.toList(); return this }
    fun make(it: Iterable<T>): Maker<T> { got += "iterable " + it.toList(); return this }
}

inline fun <T : Any> maker(init: Maker<T>.() -> Unit): Maker<T> = Maker<T>().apply(init)

fun main() {
    println(buildList { addAll(arrayOf(1, 2)) })
    println(buildList { add(0); addAll(arrayOf(1, 2)); addAll(listOf(3)) })
    println(buildSet { addAll(arrayOf("a", "b", "a")) })
    val m = maker { make(listOf("a", "b")) }
    val first: String = m.got.first()
    println(first)
    println(maker { make("x", "y") }.got)
}
