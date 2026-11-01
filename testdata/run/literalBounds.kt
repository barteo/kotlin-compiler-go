// Integer literals for a type parameter with a Long, Byte or Short bound
// take the bound's type. stdlibLiteralBounds has unsigned ones.

class IcLong<T : Long>(val l: T)

class Box<T : Short>(val s: T)

fun <T : Long> id(t: T): T = t
fun <T : Byte> byte(t: T): T = t
fun <T : Long> pair(a: T, b: T?): String = "$a $b"

fun main() {
    val x = IcLong(1)
    println(x.l + 1L)
    println(id(5) + 1L)
    println(id(-5))
    println(byte(7))
    println(Box(3).s)
    println(pair(1, 2))
    println(pair(1, null))
}
