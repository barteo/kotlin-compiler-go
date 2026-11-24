// A crossinline parameter passed on to a library's inline extension that
// captures it, as thenBy's selector: the parameter is a function object.

data class P(val a: Int, val b: Int)

inline fun <T> byBoth(crossinline f: (T) -> Int, crossinline g: (T) -> Int) = compareBy(f).thenBy(g)

inline fun <T> thenOnly(crossinline g: (T) -> Int): Comparator<T> = compareBy<T> { 0 }.thenBy(g)

inline fun <T> viaLocal(crossinline f: (T) -> Int, crossinline g: (T) -> Int): Comparator<T> {
    val c = compareBy(f)
    return c.thenBy(g)
}

fun main() {
    val ps = listOf(P(2, 1), P(1, 9), P(1, 3), P(2, 0))
    println(ps.sortedWith(byBoth({ it.a }, { it.b })))
    println(ps.sortedWith(thenOnly { -it.b }))
    println(ps.sortedWith(viaLocal({ it.b % 2 }, { it.a })))
}
