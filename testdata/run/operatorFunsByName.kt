// Operator functions of numbers and Char called by name, and Any's
// members of primitives and function values.

fun safePlus(i: Int?) = i?.plus(1)
fun equalsDouble(a: Double, b: Double) = a.equals(b)

fun main() {
    println(1.toByte().plus(1))
    println(1L.plus(1))
    println(1.0.plus(1))
    println('A'.plus(1))
    println('B'.minus('A'))
    println(7.div(2))
    println(7.rem(3))
    println(3L.times(2))
    println(5.unaryMinus())
    println(2.toShort().unaryPlus())
    val b: Byte = 127
    println(b.inc())
    println('a'.inc())
    println(0.0.compareTo(-0.0))
    println(1.compareTo(2L))
    println('a'.compareTo('b'))
    println(safePlus(null))
    println(safePlus(2))

    println(equalsDouble(-0.0, 0.0))
    println((0.0 / 0.0).equals(0.0 / 0.0))
    println(1.equals(null))
    println(230.equals(9.toByte()))
    println(true.equals(true))
    println('z'.hashCode())
    println(1.5.hashCode())
    val f: (Any) -> String = { it.toString() }
    println(f.equals(f))
    println(f.hashCode() == f.hashCode())
}
