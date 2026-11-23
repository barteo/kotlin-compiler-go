class Pair2<X, Y>(val x: X, val y: Y)
typealias PL<T> = Pair2<T, List<T>>

fun <T> put(m: MutableList<T>, a: T, b: T) {}

val pl = PL(1, null) // ERROR Nothing? but List<Int> was expected
val ok = PL(1, listOf(2))

fun f(m: MutableList<Int>, i: Int) {
    put(m, 1, null) // ERROR Nothing? but Int was expected
    put(m, i, null) // ERROR Nothing? but Int was expected
    put(m, i, 2)
}
