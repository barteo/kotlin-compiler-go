// Byte's and Short's bitwise functions, which kotlin.experimental
// declares as inline functions, and the standard library's bit functions.

import kotlin.experimental.and
import kotlin.experimental.inv
import kotlin.experimental.or
import kotlin.experimental.xor

fun main() {
    val b: Byte = 0x0F
    val c: Byte = 0x3C
    println(b and c)
    println(b or c)
    println(b xor c)
    println(b.inv())
    val s: Short = 0x0FF0
    println(s and 0x00FF.toShort())
    println(s.inv())
    println(0b1011.countOneBits())
    println(Long.MAX_VALUE.countLeadingZeroBits())
    println(Int.MIN_VALUE.takeHighestOneBit() ushr 31)
    println(listOf(1, 2, 4).fold(0) { acc, x -> acc or x })
}
