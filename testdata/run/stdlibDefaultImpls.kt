// Classes implementing standard library interfaces whose members have
// bodies, in ClosedRange$DefaultImpls: kc gives them methods that call
// those bodies.
class Span(override val start: Int, override val endInclusive: Int) : ClosedRange<Int>

class AlwaysEmpty(override val start: Int, override val endInclusive: Int) : ClosedRange<Int> {
    override fun isEmpty(): Boolean = true
}

open class Interval(override val start: Double, override val endInclusive: Double) : ClosedFloatingPointRange<Double> {
    override fun lessThanOrEquals(a: Double, b: Double): Boolean = a <= b
}

class Unit01 : Interval(0.0, 1.0)

abstract class Letters : ClosedRange<String>

class FirstHalf : Letters() {
    override val start = "a"
    override val endInclusive = "m"
}

fun describe(r: ClosedRange<Int>) = "" + (3 in r) + " " + r.isEmpty()

fun main() {
    val s = Span(1, 5)
    println(3 in s)
    println(s.contains(9))
    println(s.isEmpty())
    println(Span(5, 1).isEmpty())
    println(describe(s))
    println(describe(AlwaysEmpty(1, 5)))
    val i = Unit01()
    println(0.5 in i)
    println(i.contains(2.0))
    println(i.isEmpty())
    val f: ClosedRange<Double> = Interval(2.0, 1.0)
    println(f.isEmpty())
    println("k" in FirstHalf())
    println(FirstHalf().contains("z"))
}
