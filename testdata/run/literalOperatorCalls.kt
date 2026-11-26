// An operator function of integer literals called by name, 2.plus(1) or
// (2 + 2 * 3).inv(), is a Long where one is expected, as 2 + 1 is,
// computed as Ints, overflowing as they do, and widened.
fun takeLong(l: Long): String = "Long $l"

class Holder<T>(val t: T)

val l1: Long = (2 + 2 * 3).inv()
val l2: Long = (2 + 2 * 3).unaryMinus()
val l3: Long = 3000000000.plus(1)
const val c1: Long = 2000000000.plus(2000000000)

fun main() {
    println(takeLong(2.plus(1)))
    println(takeLong(2.minus(5)))
    println(takeLong(4.times(3)))
    println(takeLong(7.div(2)))
    println(takeLong(7.rem(2)))
    println(takeLong(2.unaryPlus()))
    println(takeLong(2.unaryMinus()))
    println(takeLong(2.inv()))
    println(takeLong(1.shl(40)))
    println(takeLong(1 shl 40))
    println(takeLong(2000000000.plus(2000000000)))
    println(takeLong(2000000000.times(3)))
    println(takeLong((-2147483647 - 1).unaryMinus()))
    println(takeLong((2000000000 + 2000000000).inv()))
    println(takeLong((-1).ushr(28)))
    println(takeLong((1 + 1).times(1000000000)))
    val h: Holder<Long> = Holder(1.plus(2))
    println(h.t + 1)
    println("$l1 $l2 $l3 $c1")
}
