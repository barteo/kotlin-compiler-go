// Reified checks against nullable types: true for null, as kotlinc's are.
// The marker's type may be nullable (filterIsInstance<T?>()), or the call's
// type argument (filterIsInstance<String?>()).

inline fun <reified T> Sequence<*>.orNulls(): List<T?> = filterIsInstance<T?>().toList()
inline fun <reified T> isT(x: Any?) = x is T
inline fun <reified T> Iterable<*>.typed(): List<T> = filterIsInstance<T>()
fun main() {
    val xs = sequenceOf<Any?>(1, "x", null)
    println(xs.filterIsInstance<String?>().toList())
    println(xs.filterIsInstance<String>().toList())
    println(xs.toList().filterIsInstance<String?>())
    println(xs.toList().filterIsInstance<String>())
    println(xs.orNulls<String>())
    println(xs.orNulls<Int?>())
    println(xs.toList().typed<Int?>())
    println("" + isT<String?>(null) + isT<String>(null) + isT<String?>("a") + isT<String?>(1))
    println(listOf<Any?>(1, null).filter { it is String? })
}
