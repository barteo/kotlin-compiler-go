// Integer literals for a type parameter with a Long, Byte, Short or
// unsigned bound take the bound's type.

@JvmInline
value class IcLong<T : Long>(val l: T)

class Box<T : Short>(val s: T)

fun <T : Long> id(t: T): T = t
fun <T : Byte> byte(t: T): T = t
fun <T : UByte> ubyte(t: T): T = t
fun <T : UShort> ushort(a: T?, b: T): String = "$a $b"
fun <T : ULong> ulong(t: T): T = t
fun <T : UInt> uint(t: T): T = t

fun <T : UByte> inRange(a: T?, b: T?, c: T): Boolean where T : Comparable<UByte> =
    a != null && b != null && c in a..b

fun main() {
    val x = IcLong(1)
    println(x.l + 1L)
    println(id(5) + 1L)
    println(byte(7))
    println(Box(3).s)
    println(ubyte(200u))
    println(ushort(1u, 60000u))
    println(ulong(3u))
    println(uint(4u))
    println(inRange(1u, 3u, 2u))
    println(inRange(1u, 3u, 5u))
}
