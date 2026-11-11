// An integer literal is a Long for the overload that takes Longs when it
// is more specific than the others: eq(Long, Long), not eq(Any?, Any?),
// for eq(5, aLong). Int still comes first where it fits as well.
fun eq(a: Long, b: Long) = "long ${a == b}"
fun eq(a: Any?, b: Any?) = "any ${a == b}"
fun eq(a: Double, b: Double) = "double ${a == b}"

fun one(a: Long) = "one long"
fun one(a: Any) = "one any"

fun two(a: Int) = "two int"
fun two(a: Long) = "two long"
fun two(a: Any) = "two any"

fun three(a: Short) = "three short"
fun three(a: Any) = "three any"

fun <T> gen(a: T, b: Long) = "gen long"
fun <T> gen(a: T, b: Any) = "gen any"

fun main() {
    val big = 123456789L
    println(eq(123456789, big))
    println(eq(5, "x"))
    println(one(7))
    println(two(7))
    println(three(7))
    println(gen("s", 3))
    println(listOf(1).size)
}
