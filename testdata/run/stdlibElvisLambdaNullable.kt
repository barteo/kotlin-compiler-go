// The left side's type is a hint for the right side of an elvis without
// an expected type: a lambda there may end in a nullable value, which
// makes the elvis nullable.

fun first(): String? = null
fun all(): List<String> = listOf("b", "c")
fun <T> id(t: T): T = t

fun main() {
    val a = first() ?: all().let { names -> names.find { it == "c" } ?: names.firstOrNull() }
    val b = first() ?: run { emptyList<String>().firstOrNull() }
    val c = id(first() ?: all().let { it.lastOrNull() })
    val d: String = first() ?: emptyList<String>().let { it.firstOrNull() } ?: "z"
    println(a)
    println(b == null)
    println(c?.length)
    println(d)
}
