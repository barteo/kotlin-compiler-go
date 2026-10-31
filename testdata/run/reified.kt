// Reified type parameters of the program's inline functions: each copy
// knows its type arguments.

inline fun <reified T> isA(x: Any?): Boolean = x is T

inline fun <reified T> castOrNull(x: Any?): T? = x as? T

inline fun <reified T : Any> nameOf(): String = T::class.java.simpleName

inline fun <reified T> arrayOfTwo(a: T, b: T): Array<T> = arrayOf(a, b)

inline fun <reified T> filterType(xs: Array<Any?>): Int {
    var n = 0
    for (x in xs) {
        if (x is T) {
            n++
        }
    }
    return n
}

// A reified type parameter passed on to another reified function.
inline fun <reified T> describe(x: Any?): String = nameOf2<T>() + " " + isA<T>(x)

inline fun <reified T> nameOf2(): String = T::class.java.name

inline fun <reified E> classify(x: Any): String = when (x) {
    is E -> "match"
    else -> "other"
}

fun main() {
    println(isA<String>("s"))
    println(isA<String>(1))
    println(isA<Int>(1))
    println(castOrNull<String>("x"))
    println(castOrNull<String>(2))
    println(nameOf<java.util.ArrayList<String>>())
    println(nameOf<StringBuilder>())
    val arr = arrayOfTwo("a", "b")
    println(arr::class.java.simpleName + " " + arr.size)
    println(filterType<String>(arrayOf("a", 1, "b", null)))
    println(filterType<Int>(arrayOf("a", 1, "b", null)))
    println(describe<java.lang.Number>(5))
    println(classify<CharSequence>("x") + " " + classify<CharSequence>(1))
}
