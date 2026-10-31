@file:JvmMultifileClass
@file:JvmName("Text")

package mf

@Throws(IllegalStateException::class)
fun half(n: Int): Int {
    check(n % 2 == 0) { "odd: $n" }
    return n / 2
}

fun Int.digits(): Int = toString().length + shout("").length - 1

// Private members of the part, used by a public inline function through
// accessors that kotlinc's copies call too.
private val base = 10

private fun scaled(n: Int) = n * base

@PublishedApi
internal fun scale(n: Int) = scaled(n)

inline fun scaledSum(xs: List<Int>): Int = xs.sumOf { scale(it) }
