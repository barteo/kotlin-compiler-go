// Extensions with the standard library: nullable receivers called without
// ?., the program's extensions on its types, and calls between the two.

fun <T> List<T>.secondOrNull(): T? = if (size > 1) this[1] else null
fun List<Int>.total(): Int = fold(0) { acc, x -> acc + x }
fun String.wordList(): List<String> = split(" ").filter { it.isNotEmpty() }
fun Map<String, Int>.describe(): String = entries.joinToString(", ") { "${it.key}=${it.value}" }
fun <T> MutableList<T>.swap(i: Int, j: Int) {
    val t = this[i]
    this[i] = this[j]
    this[j] = t
}
fun String?.isBlankOrNull(): Boolean = this == null || isBlank()

// The program's own extension comes before the standard library's.
fun String.trimIndent(): String = "own trimIndent"

fun main() {
    val s: String? = null
    println(s.isNullOrEmpty())
    println(s.orEmpty().length)
    val t: String? = "ab"
    println(t.isNullOrEmpty())
    println(t.orEmpty().length)
    val xs: List<Int>? = null
    println(xs.orEmpty().size)

    println(listOf(1, 2, 3).secondOrNull())
    println(listOf("a").secondOrNull())
    println(listOf(1, 2, 3).total())
    println("  the quick  fox ".wordList())
    println(mapOf("a" to 1, "b" to 2).describe())
    val ys = mutableListOf(1, 2, 3)
    ys.swap(0, 2)
    println(ys)
    println(s.isBlankOrNull())
    println("  ".isBlankOrNull())
    println("x".isBlankOrNull())
    println("  a".trimIndent())
}
