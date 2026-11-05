fun <K> nullableK(f: () -> K?): K? = f()
fun <K> k(f: () -> K): K = f()

fun branches(b: Boolean, c: Int): Unit? {
    val x = if (b) Unit else null
    val y = when (c) { 0 -> Unit; else -> null }
    println(x)
    println(y)
    return y
}

fun main() {
    println(nullableK<Unit> { null })
    println(nullableK { null })
    println(k<Unit?> { null })
    println(branches(true, 0))
    println(branches(false, 1))
    var hit = false
    val z = k l@{
        if (!hit) return@l
        5
    }
    println(z)
}
