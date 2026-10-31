package lib

@JvmInline
value class Dp(val value: Float)

@JvmInline
value class Color(val value: ULong)

class Solid(val color: Color) {
    override fun toString() = "Solid(${color.value})"
}

class Options(val size: Dp = Dp(1f), val count: Int = 2, val name: String? = null) {
    constructor(name: String, size: Dp) : this(size, 0, name)

    override fun toString() = "Options(${size.value}, $count, $name)"
}

open class Shape(val corner: Dp) {
    override fun toString() = "${javaClass.simpleName}(${corner.value})"
}
