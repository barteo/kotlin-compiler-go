// a?.b == v, with a of a nullable value class or primitive type, calls b
// on a's unboxed value: Compose's parsed?.toArgb() != color.toArgb().

@JvmInline
value class Argb(val value: Long) {
    fun alpha(): Int = (value ushr 24).toInt()

    val red: Int get() = ((value shr 16) and 0xFF).toInt()
}

fun Argb.toInt(): Int = value.toInt()

fun parse(s: String): Argb? = s.toLongOrNull(16)?.let { Argb(it) }

fun changed(text: String, color: Argb): Boolean = parse(text)?.toInt() != color.toInt()

fun opaque(c: Argb?): Boolean = c?.alpha() == 0xFF

fun red(c: Argb?): Boolean = 0x12 == c?.red

fun next(n: Int?): Boolean = n?.inc() == 3

fun main() {
    val color = Argb(0xFF123456)
    println(changed("FF123456", color))
    println(changed("FF000000", color))
    println(changed("zz", color))
    println(opaque(color))
    println(opaque(Argb(0x00123456)))
    println(opaque(null))
    println(red(color))
    println(red(null))
    println(next(2))
    println(next(null))
}
