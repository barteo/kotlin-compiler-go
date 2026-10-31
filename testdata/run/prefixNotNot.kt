// !! before an operand is two prefix nots, not a not-null assertion.
class Flip(val n: Int)

operator fun Flip.not() = Flip(n + 1)

fun main() {
    val p = 2 < 1
    val s: String? = "ab"
    println(!!p)
    println(!!!p)
    println(!!!!!p)
    println(!!(s?.length == 2))
    println((!!!!!Flip(0)).n)
    println(s!!.length)
    val t = s!!
    println(!!t.isEmpty())
}
