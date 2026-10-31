// Char, Float, Byte and Short through spills, defaults, varargs, generics and NaN.

fun scaled(x: Float = 1.5f, c: Char = '*', b: Byte = 2): String = "" + x * b + c

fun joined(vararg cs: Char): String = String(cs)

fun parse(s: String): Float = try {
    java.lang.Float.parseFloat(s)
} catch (e: NumberFormatException) {
    -1f
}

class Box<T>(val value: T)

fun grade(score: Float): Char = when {
    score >= 90f -> 'A'
    score >= 80f -> 'B'
    else -> 'C'
}

fun main() {
    println(scaled())
    println(scaled(2f))
    println(scaled(c = '#', b = 3))
    println(joined())
    println(joined('o', 'k'))
    val cs = charArrayOf('a', 'b')
    println(joined(*cs, 'c'))
    println(java.util.Arrays.asList('q', 'r').get(0))
    println(java.util.Arrays.asList(2.5f).get(0) * 2)
    println(1.5f + parse("2.25"))
    println(1.5f + parse("x"))
    println("" + 'c' + try { 'd' } finally { })
    val nan = 0f / 0f
    println(nan == nan)
    println(nan < 1f)
    println(nan > 1f)
    println(nan != nan)
    val inf = 1f / 0f
    println(inf)
    println(-inf)
    println(Box('z').value)
    println(Box(3.5f).value + 1)
    val bb: Box<Byte> = Box(5)
    println(bb.value + 1)
    println(grade(95f))
    println(grade(85.5f))
    println(grade(10f))
    val big = 3.4e38f
    println(big * 10)
    println(1.0f.toDouble() == 1.0)
    println((0.1f).toDouble())
    println(16777217.toFloat())
    println(9223372036854775807L.toFloat())
    println(127.toByte() + 1)
    println((-129).toByte())
    println('a'.code.toByte())
    println(65535.toChar().code)
    println(65536.toChar().code)
    val s: Short = -32768
    println(s - 1)
    println((s - 1).toShort())
    var total = 0
    for (c in "hello") total += c.code
    println(total)
    val dc = 'x'
    val msg = when (dc) {
        in 'a'..'m' -> "first half"
        in 'n'..'z' -> "second half"
        else -> "?"
    }
    println(msg)
}
