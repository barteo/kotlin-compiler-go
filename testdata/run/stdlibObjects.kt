// Companion objects of classes from the standard library.

fun main() {
    println(kotlin.random.Random.nextInt(1))
    println(Regex.escape("a.b"))
    println(Regex.escapeReplacement("$1"))
}
