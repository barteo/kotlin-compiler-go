// Reified checks in the library's inline functions, with a nullable or platform type argument:
// null is of the type, as with kotlinc.

inline fun <reified T> List<Any?>.count(): Int = filterIsInstance<T>().size

fun main() {
    val xs = listOf("a", null, 1, null, 2.5)
    println(xs.filterIsInstance<String>())
    println(xs.filterIsInstance<String?>())
    println(xs.filterIsInstance<Int?>())
    println(xs.filterIsInstance<Any>())
    println(xs.filterIsInstance<Any?>())
    println(xs.count<Number>())
    println(xs.count<Number?>())
    println(xs.filterIsInstanceTo<CharSequence?, MutableList<CharSequence?>>(mutableListOf()))
    val platform = listOf(java.lang.String.valueOf(1))
    println(sameKind(xs, platform[0]))
    println(sameKind(xs, "b"))
}

inline fun <reified T> sameKind(xs: List<Any?>, like: T): List<T> = xs.filterIsInstance<T>()
