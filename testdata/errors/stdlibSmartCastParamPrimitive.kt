// A value of a type parameter's type smart cast to a primitive type is
// an intersection, Int & E, which is still an E: a call's fork over
// Generic<E, F> & Generic<Int, String> takes Generic<E, F> for an
// expected F, as K2 does.

interface Generic<K, V>

fun <X, Y> Generic<X, Y>.getValue(x: X): Y = TODO()

class MyPair<A, B>(a: A, b: B)

fun <E, F> pair(x: Generic<E, F>, e: E, c: Generic<Int, String>): MyPair<F, F> {
    if (c === x && e is Int) {
        val s: String = x.getValue(e)
        return MyPair(x.getValue(e), x.getValue(e))
    }
    return MyPair(x.getValue(e), x.getValue(e))
}

fun <E> keep(e: E): List<E> {
    if (e is Int) {
        val m: Map<E, E> = mapOf(e to e)
        return listOf(e + 1) // ERROR type mismatch: inferred type is List<Int> but List<E> was expected
    }
    return emptyList()
}

fun <T> range(arg: T): String {
    if (arg is Char) {
        return when (arg) {
            in 'A'..'Z' -> "upper"
            else -> "other"
        }
    }
    return "none"
}
