package paint

// A value class over a value class, as Compose's Color over ULong.
@JvmInline
value class Color(val value: ULong) {
    val alpha: Int get() = (value shr 56).toInt() and 0xff

    fun copy(alpha: Int): Color = Color((value and 0xffffffffffffffUL) or (alpha.toULong() shl 56))

    override fun toString(): String = "Color(" + value.toString(16) + ")"
}

fun Color(argb: Long): Color = Color(argb.toULong() shl 32)

fun mix(a: Color, b: Color): Color = Color((a.value + b.value) / 2UL)

object Colors {
    val Black = Color(0xff000000)
}
