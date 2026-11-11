fun codes(names: List<String>): List<Int> = names.flatMap { n ->
    try {
        if (n == "bad") error("bad")
        n.map { it.code }
    } catch (e: IllegalStateException) {
        emptyList()
    }
}

fun lengths(names: List<String>) = names.map { n ->
    try {
        n.toInt()
    } catch (e: NumberFormatException) {
        null
    } finally {
        println("checked $n")
    }
}

fun big(s: String): Long = try { s.toLong() } catch (e: NumberFormatException) { 0 }

fun main() {
    println(codes(listOf("ab", "bad", "c")))
    println(lengths(listOf("1", "x")))
    println(big("12") + big("z"))
    val m: Map<String, Int> = try { mapOf("a" to 1) } catch (e: Exception) { emptyMap() }
    println(m)
}
