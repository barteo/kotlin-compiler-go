// As K2's synthetic call for an elvis, an if, a when or a try, a call
// whose variables nothing else gives takes them from the other side or
// branches: a literal, a jump's Nothing or a null's Nothing?.

@Suppress("UNCHECKED_CAST")
fun <T> make(): T = null as T
fun <T> nullable(): T? = null

fun branches(c: Boolean): String {
    val b = if (c) make() else null
    val w = when { c -> make(); else -> null }
    val t = try { make() } catch (e: Exception) { null }
    return "$b $w $t"
}

fun jump(c: Boolean): String {
    val n = if (c) return "jumped" else make()
    return "not $n"
}

fun main() {
    println(nullable() ?: "d")
    println(nullable() ?: 42)
    val s: String = nullable() ?: "e"
    println(s.length)
    println(branches(true))
    println(branches(false))
    println(jump(true))
    try {
        println(jump(false))
    } catch (e: Throwable) {
        println(e.javaClass.simpleName)
    }
}
