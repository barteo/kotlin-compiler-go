// Extension properties: getters with expression and block bodies, inferred
// and declared types, generic and nullable receivers, and their use
// unqualified in extensions and in classes.

class Vec(val x: Int, val y: Int) {
    fun scaledLen() = len2 * 10 // an extension property of this
}

val String.lastChar: Char get() = this[length - 1]
val Vec.len2 get() = x * x + y * y
val <T> Array<T>.penultimate: T get() = this[size - 2]
val Int?.orZero: Int get() = this ?: 0
val Int.isOdd: Boolean
    get() {
        return this % 2 == 1
    }
val Vec.desc: String get() = "${this@desc.x},$y len2=$len2"
val Long.kilo get() = this * 1000L
val Double.half: Double get() = this / 2.0
val StringBuilder.size: Int get() = length

fun Vec.twiceLen() = len2 * 2
val Vec.flipped get() = Vec(y, x)

fun main() {
    println("abc".lastChar)
    println(Vec(3, 4).len2)
    println(Vec(3, 4).scaledLen())
    println(arrayOf(1, 2, 3).penultimate)
    println(arrayOf("a", "b").penultimate.length)
    val n: Int? = null
    println(n.orZero)
    println(5.orZero)
    println(7.isOdd)
    println(8.isOdd)
    println(Vec(1, 2).desc)
    println(Vec(1, 2).twiceLen())
    println(Vec(1, 2).flipped.desc)
    println(3L.kilo)
    println(5.0.half)
    println(StringBuilder("abcd").size)
    val s: String? = "xy"
    println(s?.lastChar)
    val none: String? = null
    println(none?.lastChar)
}
