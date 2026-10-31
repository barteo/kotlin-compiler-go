// Extension functions and properties of the standard library, varargs,
// default arguments and infix calls. Inline functions such as map and
// filter are called, not inlined.

fun squares(n: Int): List<Int> {
    val out = mutableListOf<Int>()
    for (i in 1..n) {
        out.add(i * i)
    }
    return out
}

fun main() {
    // Varargs.
    val xs = listOf(1, 2, 3)
    println(xs)
    println(mutableListOf(5, 4, 3).sorted())
    println(setOf("x", "y", "x").size)
    println(listOf<String>().isEmpty())
    println(java.util.List.of(1, 2, 3))
    println(java.util.Arrays.asList("p", "q"))
    println(java.lang.String.format("%d-%s", 1, "x"))

    // Lambdas passed to inline functions, which run as calls.
    println(xs.map { it * 2 })
    println(xs.filter { it > 1 }.map { it * 10 })
    println(xs.fold(0) { acc, x -> acc + x })
    println(xs.first { it > 1 })
    println(xs.any { it > 2 })
    println(xs.all { it > 2 })
    println(xs.count { it % 2 == 1 })
    xs.forEach { print(it) }
    println()
    xs.forEach {
        if (it == 2) return@forEach
        print(it)
    }
    println()
    println(squares(4).filter { it % 2 == 0 })

    // Default arguments.
    println(xs.joinToString())
    println(xs.joinToString(", "))
    println(xs.joinToString("-", "[", "]"))
    println(xs.joinToString(" ") { "<" + it + ">" })
    println("abc".padStart(5) + "|")
    println("a,b,,c".split(","))

    // Extensions on String, Int and collections.
    println("  hi  ".trim())
    println("".isBlank())
    println("Hello".startsWith("He"))
    println("kotlin".reversed())
    println(xs.first())
    println(xs.last())
    println(xs.sorted().reversed())
    println(xs.lastIndex)
    for (i in xs.indices) {
        print(i)
    }
    println()
    println(xs.maxOrNull())
    println(listOf<Int>().maxOrNull())
    println(7.coerceAtMost(5))

    // Infix calls and maps.
    val p = 1 to "a"
    println(p.second)
    val m = mapOf(1 to "one", 2 to "two")
    println(m)
    println(m[2])
    println(m.getOrDefault(3, "none"))
    val counts = mutableMapOf<String, Int>()
    for (w in "a b a c a".split(" ")) {
        counts[w] = counts.getOrDefault(w, 0) + 1
    }
    println(counts)
    println(counts.keys.sorted())
    val inverse = HashMap<Int, String>()
    for (e in m.entries) {
        inverse[e.key * 10] = e.value
    }
    println(inverse)
}
