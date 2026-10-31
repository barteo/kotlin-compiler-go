// Classes by the qualified names of the library's typealiases of them.

fun first(xs: List<Int>): Int {
    if (xs.isEmpty()) throw kotlin.NoSuchElementException("empty")
    return xs[0]
}

fun main() {
    try {
        first(listOf())
    } catch (e: kotlin.NoSuchElementException) {
        println("caught " + e.message)
    }
    println(first(listOf(3, 4)))
    val e: Exception = kotlin.IllegalStateException("state")
    println(e.message)
    val sb = kotlin.text.StringBuilder("a")
    sb.append("b")
    println(sb)
    val list = kotlin.collections.ArrayList<String>()
    list.add("x")
    println(list)
}
