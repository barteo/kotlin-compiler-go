fun tap(n: Int) = println("tap $n")

class Box(val action: (() -> Unit)?)

fun pass(value: Any?) = value

fun main() {
    val skipped: String? = null
    val there: String? = "x"
    println(skipped?.let { tap(1) })
    println(there?.let { tap(2) })
    println(pass(skipped?.let { tap(3) }))
    println(listOf(skipped?.let { tap(4) }, there?.let { tap(5) }))
    println(mapOf("k" to skipped?.let { tap(6) }))
    println(skipped?.run { tap(7) })
    println(setOf(skipped?.also { tap(8) }))
    println("${skipped?.let { tap(9) }}")
    println(Box(null).action?.invoke())
    println(Box { tap(10) }.action?.invoke())
    skipped?.let { tap(11) } ?: println("fallback")
}
