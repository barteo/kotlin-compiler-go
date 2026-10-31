class Span(override val start: Int, override val endInclusive: Int) : ClosedRange<Int>

class Window(r: ClosedRange<Int>) : ClosedRange<Int> by r

class Tagged(private val r: ClosedRange<Int>, val tag: String) : ClosedRange<Int> by r {
    override fun isEmpty(): Boolean = false
}

fun main() {
    val w = Window(IntRange(3, 7))
    println(w.start)
    println(w.endInclusive)
    println(5 in w)
    println(w.contains(9))
    println(w.isEmpty())
    val r: ClosedRange<Int> = w
    println(r.start + r.endInclusive)
    val t = Tagged(Span(5, 1), "t")
    println(t.isEmpty())
    println(t.start)
    println(t.tag)
}
