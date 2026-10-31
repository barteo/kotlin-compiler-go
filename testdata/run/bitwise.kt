// Int's and Long's bitwise functions, which are JVM instructions.

const val FLAG_A = 1 shl 0
const val FLAG_B = 1 shl 4
const val MASK = FLAG_A or FLAG_B
const val HIGH = 1L shl 40
const val NOT_MASK = MASK.inv()
const val SHIFTED = -16 ushr 28
const val WIDE = 1 shl 33
const val LONG_MASK = 0xFFFFL and HIGH.inv()
const val TEXT = "mask=$MASK high=$HIGH"
const val YES = true
const val NO = YES.not()
const val EAGER = YES.and(NO).or(NO.not()).xor(NO)

class Flags(var bits: Int) {
    fun has(flag: Int) = bits and flag != 0
    fun set(flag: Int) {
        bits = bits or flag
    }
    fun clear(flag: Int) {
        bits = bits and flag.inv()
    }
}

fun hash(s: String): Int {
    var h = 0
    for (c in s) h = (h shl 5) - h xor c.code
    return h
}

fun main() {
    val a = 0b1100
    val b = 0b1010
    println(a and b)
    println(a or b)
    println(a xor b)
    println(a.inv())
    println(a shl 2)
    println(-a shr 1)
    println(-a ushr 1)
    println(1 shl 32)
    println(1 shl 33)
    println(-1 ushr 31)
    println(a.and(b) + a.or(b))
    val l = 0xF0F0L
    println(l and 0xFFL)
    println(l or 1L)
    println(l xor l)
    println(l.inv())
    println(l shl 40)
    println(-l shr 4)
    println(-l ushr 60)
    println(1L shl 64)
    println(1L shl 63)
    println(a + 1 shl 1)
    println(a and 4 == 4)
    println(a or b > 10)
    println(FLAG_A)
    println(FLAG_B)
    println(MASK)
    println(HIGH)
    println(NOT_MASK)
    println(SHIFTED)
    println(WIDE)
    println(LONG_MASK)
    println(TEXT)
    val f = Flags(0)
    f.set(FLAG_B)
    f.set(FLAG_A)
    println(f.bits.toString() + " " + f.has(FLAG_A))
    f.clear(FLAG_A)
    println(f.bits.toString() + " " + f.has(FLAG_A))
    println(hash("kotlin"))
    val n: Int? = 6
    println(n?.and(3))
    println(n?.shl(1))
    when (a and 0xF) {
        MASK -> println("mask")
        12 -> println("twelve")
        else -> println("other")
    }
    val c = 'A'
    println(c.code or 0x20)
    println("$NO $EAGER")
}
