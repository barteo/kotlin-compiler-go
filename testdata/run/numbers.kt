// Float, Byte and Short: literals, arithmetic, conversions and arrays.

data class Sample(val f: Float, val c: Char, val b: Byte, val s: Short)

fun half(x: Float): Float = x / 2

fun widen(b: Byte, s: Short): Long = b + s + 1L

fun takesByte(b: Byte): Int = b * 2

fun takesShort(s: Short): Short = s

fun main() {
    val f = 1.5f
    val g = 2F
    println(f)
    println(f + g)
    println(1f / 3)
    println(10f % 3f)
    println(-f)
    println(half(5f))
    println(f * 2 + 1)
    println(f < g)
    println(f == 1.5f)
    println(1e3f)
    println(0.1f + 0.2f)
    println(f.toDouble())
    println(f.toInt())
    println(f.toLong())
    println((2.7f).toInt())
    println((-2.7f).toInt())
    println(3.toFloat())
    println(3000000000L.toFloat())
    println(1.1.toFloat())
    println(f + 1.0)
    println(f + 1L)
    var acc = 0f
    for (i in 1..4) acc += i
    println(acc)
    acc++
    println(acc)
    val nf: Float? = if (f > 1) f else null
    println(nf)
    println(nf == 1.5f)

    val b: Byte = 10
    val nb: Byte = -128
    val s: Short = 300
    val ns: Short = -1
    println(b)
    println(nb)
    println(s)
    println(ns)
    println(b + b)
    println(b * s)
    println(-b)
    println(b < s)
    println(b == 10.toByte())
    println(200.toByte())
    println(70000.toShort())
    println((-1).toByte())
    println(300.toByte().toInt())
    println(65.toByte().toInt().toChar())
    println(b.toInt().toChar())
    println(s.toLong())
    println(b.toFloat())
    println(widen(b, s))
    println(takesByte(7))
    println(takesShort(-3))
    var counter: Byte = 127
    counter++
    println(counter)
    var sh: Short = 1
    sh--
    sh--
    println(sh)
    val nn: Byte? = if (b > 0) b else null
    println(nn)

    val fa = FloatArray(3) { it * 0.5f }
    println(fa[2])
    val ba = byteArrayOf(1, 2, 3)
    ba[0] = -5
    var sum = 0
    for (x in ba) sum += x
    println(sum)
    val sa = ShortArray(2)
    sa[1] = 7
    sa[1]++
    println(sa[1])
    println(floatArrayOf(1f, 2f).size + shortArrayOf(1).size)
    println(java.util.Arrays.toString(fa))
    println(java.util.Arrays.toString(ba))

    val p = Sample(1.25f, 'k', 3, 4)
    println(p)
    println(p == Sample(1.25f, 'k', 3, 4))
    println(p.hashCode() == Sample(1.25f, 'k', 3, 4).hashCode())
    println(p.copy(c = 'z'))
    val floats = java.util.Arrays.asList(1.5f, 2.5f)
    var t = 0f
    for (x in floats) t += x
    println(t)
    val any: Any = 2.5f
    if (any is Float) println(any * 2)
    println(java.lang.Float.compare(1f, 2f))
    println(Math.max(1.5f, 2.5f))
    println(Math.abs(-3.5f))
    println("$f|$b|$s")
}
