import lib.Point
import lib.Span
import lib.blend
import lib.check
import lib.each
import lib.holds
import lib.pause

fun main() {
    println(pause(5L))
    println(pause(Span(7)))
    println(blend(Point(1), Point(2), 0.5f))
    println(blend(Span(3), Span(4), 0.25f))
    println(blend(5L, 6L, 1f))
    val resting: Point? = Point(8)
    println(resting?.let { blend(it, Point(9), 0f) } ?: "none")
    listOf("a").each(Span(2)) { println(it) }
    listOf("b").each(3L) { println(it) }
    println(check(listOf("x", "y")).holds("x", "y"))
    println(check(arrayOf<Any?>("x")).holds("x"))
    println(check(mapOf("k" to 1)).holds("k" to 1))
}
