// return in an expression body with an explicit type
// (AllowReturnInExpressionBodyWithExplicitType, on since Kotlin 2.3).
fun direct(): String = return "direct"
fun sign(x: Int): String = if (x > 0) return "pos" else "neg"
fun doubled(x: Int): Int = x.also { if (it < 0) return -1 } * 2
fun firstBig(l: List<Int>): Int = l.firstOrNull { it > 2 } ?: return 0

fun main() {
    println(direct())
    println("${sign(1)} ${sign(-1)}")
    println("${doubled(3)} ${doubled(-3)}")
    println("${firstBig(listOf(1, 5))} ${firstBig(listOf())}")
}
