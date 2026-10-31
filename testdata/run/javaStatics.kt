// Static methods of Java interfaces, which need class file version 52.

import java.util.ArrayList
import java.util.Collections
import java.util.Comparator
import java.util.function.Function

fun main() {
    val xs = java.util.List.of(3, 1, 2)
    println(xs)
    println(xs.size)
    println(java.util.Map.of("one", 1, "two", 2).get("two"))

    val words = ArrayList<String>()
    words.add("pear")
    words.add("fig")
    words.add("banana")
    Collections.sort(words, Comparator.naturalOrder<String>())
    println(words)
    Collections.sort(words, Comparator.reverseOrder<String>())
    println(words)
    Collections.sort(words, Comparator.comparingInt { it.length })
    println(words)

    val id = Function.identity<String>()
    println(id.apply("same"))
    println(Comparator.naturalOrder<Int>().compare(1, 2))
}
