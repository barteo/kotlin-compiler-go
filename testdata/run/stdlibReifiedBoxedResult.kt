// A generic call's boxed result, passed as a nullable of its primitive
// type, keeps its box, as with kotlinc, a reified function's too: a mock
// library tells its matchers apart by the box.
object Scope {
    val marker: Any = 123456789L as Any
    var seen: Any? = null

    @Suppress("UNCHECKED_CAST")
    inline fun <reified T : Any> any(): T = marker as T

    @Suppress("UNCHECKED_CAST")
    fun <T : Any> plain(): T = marker as T
}

fun take(id: Long?, name: String?) {
    Scope.seen = id
}

fun main() {
    take(Scope.any(), null)
    println(Scope.seen === Scope.marker)
    take(Scope.plain(), null)
    println(Scope.seen === Scope.marker)
    val x: Long = Scope.any()
    println(x + 1)
}
