package lib

@JvmInline
value class Size(val v: Int)

class Style(val size: Size, val name: String, val weight: Int, val tag: String) {
    @Deprecated("use the constructor with tag", level = DeprecationLevel.HIDDEN)
    constructor(size: Size = Size(1), name: String = "none", weight: Int = 0) : this(size, name, weight, "hidden")

    constructor(size: Size = Size(2), name: String = "some", weight: Int = 7, tag: String = "shown", extra: Int = 1) : this(size, name, weight + extra, tag)
}

class Options(val a: Int, val b: Int) {
    @Deprecated("gone", level = DeprecationLevel.HIDDEN)
    constructor(a: Int = 1) : this(a, -1)

    constructor(a: Int = 2, b: Int = 3, c: Long = 4) : this(a, b + c.toInt())
}
