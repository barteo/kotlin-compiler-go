inline fun <R> runIt(block: () -> R): R = block()
fun <T> id(x: T): T = x
fun <T> nested(body: () -> T): T = body()
fun <T> materialize(): T = null as T

fun bare(b: Boolean) {
    val x = runIt l@{
        if (b) return@l
        return@l 1 // ERROR inferred type is Int but Unit was expected
    }
    runIt<Unit> { if (b) return@runIt id(5); Unit } // ERROR inferred type is Int but Unit was expected
}

fun ok(b: Boolean): Unit = runIt outer@{
    if (b) return@outer
    runIt inner@{ return@inner materialize() }
}

fun ok2(): Unit = runIt outer@{ runIt inner@{ return@outer materialize() } }
fun ok3() { runIt<Unit> { return@runIt nested { 5 } } }
fun afterBare(b: Boolean) = runIt l@{
    if (b) return@l
    5
}
