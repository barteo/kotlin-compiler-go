// Arrays with the standard library: its extensions take arrays.

enum class Suit { CLUBS, HEARTS }

fun main() {
    val xs = intArrayOf(3, 1, 2)
    println(xs.toList())
    println(xs.joinToString())
    println(xs.sum())
    println(xs.sorted())
    for (i in xs.indices) print("$i:${xs[i]} ")
    println()
    val names = arrayOf("b", "a")
    println(names.joinToString("/"))
    println(names.toList())
    println(listOf(*names, "c"))
    val list = listOf("x", "y", "z")
    val arr = list.toTypedArray()
    println(arr.size)
    println(arr[2])
    println(Suit.values().size == Suit.entries.size)
    println(mutableListOf(*xs.toTypedArray()))
    println(names.contains("a"))
}
