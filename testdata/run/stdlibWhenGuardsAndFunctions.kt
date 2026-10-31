// when guards, anonymous functions and local extension functions.

sealed interface Shape
data class Circle(val r: Double) : Shape
data class Rect(val w: Double, val h: Double) : Shape

fun describe(s: Shape): String = when (s) {
    is Circle if s.r > 10 -> "big circle"
    is Circle -> "circle " + s.r
    is Rect if s.w == s.h -> "square " + s.w
    is Rect -> "rect"
}

fun classify(n: Int?): String = when (n) {
    null -> "none"
    0 -> "zero"
    else if n < 0 -> "negative"
    else -> "positive"
}

fun apply(x: Int, f: (Int) -> Int) = f(x)

fun main() {
    println(listOf(Circle(20.0), Circle(1.0), Rect(2.0, 2.0), Rect(1.0, 3.0)).map { describe(it) })
    println(listOf(null, 0, -5, 7).map { classify(it) })

    val sq = fun(x: Int): Int { return x * x }
    println(sq(5))
    println(apply(4, fun(x) = x * 10))
    val countAll = fun(acc: Int, e: String): Int {
        if (e.isEmpty()) return acc
        return acc + e.length
    }
    println(listOf("ab", "", "c").fold(0, countAll))
    val ext = fun Int.(): String = "n$this"
    println(5.ext())
    val nullable = fun Any?.() = "r$this"
    println(null.nullable() + "x".nullable())
    // One without a receiver takes the receiver as its first parameter.
    val recv: String.(Int) -> Int = fun(s, n) = s.length + n
    println("abc".recv(1) + with("de") { recv(2) })
    println(buildString(fun(sb) { sb.append("built") }))
    var odd = 0
    listOf(1, 2, 3, 4).forEach(fun(v) {
        if (v % 2 == 0) return
        odd += v
    })
    println(odd)

    val unit = "u"
    fun Int.tag(): String = "$unit$this"
    fun String.shout(times: Int = 2) = uppercase().repeat(times)
    println(3.tag() + " " + "ab".shout() + " " + "c".shout(3))
    println(listOf(1, 2).map { it.tag() })
    fun List<Int>.total(): Int = if (isEmpty()) 0 else first() + drop(1).total()
    println(listOf(1, 2, 3).total())
}
