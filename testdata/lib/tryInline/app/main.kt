import lib.*

class Pair2(val a: Int, val b: Int)

fun main() {
    println(logged { 1 })
    println(orZero { 2 })
    println(orZero { error("x") })
    println(traced { "t" })
    println(listOf(1, traced { 2 }))
    println(Pair2(1, orZero { 3 }).b)
    println(firstLine("a\nb") { it.uppercase() })
    println(attempt { 4 }.getOrNull())
    println(attempt<Int> { error("y") }.isFailure)
}
