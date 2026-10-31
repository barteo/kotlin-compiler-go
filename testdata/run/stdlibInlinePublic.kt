// The standard library's public inline functions, which kc inlines as
// kotlinc does, with their lambdas.

fun firstBig(xs: List<Int>): Int {
    xs.forEach { if (it > 10) return it }
    return -1
}

fun firstUpper(s: String): Char? {
    s.forEach { if (it.isUpperCase()) return it }
    return null
}

class Inventory(val items: List<String>) {
    // A lambda inlined in a method, using this's members.
    fun longNames(min: Int): List<String> = items.filter { it.length >= min && it != items[0] }
}

fun main() {
    val xs = listOf(1, 5, 12, 7)
    println(xs.filter { it > 3 }.map { it * 2 })
    println(xs.fold(0) { acc, x -> acc + x })
    println(xs.any { it > 10 } && !xs.all { it > 10 } && xs.none { it < 0 })
    println(xs.count { it % 2 == 1 })
    println(listOf("a", null, "b").mapNotNull { it?.uppercase() })
    println(firstBig(xs) + firstBig(listOf(1)))
    println("" + firstUpper("abCd") + " " + firstUpper("x"))
    xs.forEach { if (it == 5) return@forEach; print(it) }
    println()
    xs.forEachIndexed { i, x -> print("$i:$x ") }
    println()
    println(xs.mapIndexed { i, x -> i * x })
    println(xs.first { it > 4 } + (xs.firstOrNull { it > 100 } ?: 0))
    println(xs.groupBy { it % 2 })
    println(xs.associate { it to it * it })
    println(xs.partition { it > 6 })
    println(xs.flatMap { listOf(it, -it) }.takeWhile { it != 12 })
    println(xs.sumOf { it * 2 } + xs.maxOf { it })
    println(xs.map { x -> xs.filter { it < x }.size })

    // A var assigned in the lambdas stays a plain local.
    var total = 0
    xs.forEach { total += it }
    println(total)

    // Arrays and strings.
    val arr = intArrayOf(3, 4)
    println(arr.map { it + 1 }.toString() + arr.filter { it > 3 })
    println("Hello".filter { it.isLowerCase() } + "abc".count { it != 'b' })

    // Java collections, and Map.forEach with two parameters, which is
    // java.util.Map.forEach(BiConsumer).
    val al = java.util.ArrayList<Int>()
    al.add(9)
    al.forEach { println("al $it") }
    mapOf("k" to 1).forEach { k, v -> println(k + v) }
    mapOf("m" to 2).forEach { e -> println(e.key + e.value) }
    val sorted = mutableListOf(3, 1, 2)
    sorted.sort()
    println(sorted)

    println(Inventory(listOf("a", "bb", "ccc")).longNames(2))

    // Sequences' map and filter aren't inline: their lambdas are objects.
    println(xs.asSequence().map { it + 1 }.filter { it > 6 }.toList())
}
