// Arguments of a library class's constructor whose inlined code has
// exception handlers: kc evaluates them before the object is created.
val lock = Any()

fun main() {
    val sb = StringBuilder(synchronized(lock) { "locked" })
    println(sb)
    val p = Pair(1, runCatching { error("x") }.isFailure)
    println(p)
    val t = Triple("a", buildString { append(try { "t" } finally { println("finally") }) }, 3)
    println(t)
}
