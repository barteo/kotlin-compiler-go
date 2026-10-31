// vararg parameters and the spread operator.

fun sum(vararg xs: Int): Int {
    var s = 0
    for (x in xs) s += x
    return s
}

fun join(sep: String, vararg parts: String): String {
    var s = ""
    for (i in 0 until parts.size) {
        if (i > 0) s += sep
        s += parts[i]
    }
    return s
}

fun <T> count(vararg items: T): Int = items.size

fun <T> firstOr(default: T, vararg items: T): T = if (items.size > 0) items[0] else default

fun tagged(vararg xs: Int, tag: String): String = tag + ":" + sum(*xs)

fun withDefault(vararg xs: Long, scale: Long = 10L): Long {
    var s = 0L
    for (x in xs) s += x * scale
    return s
}

fun apply(vararg xs: Int, f: (Int) -> Int): Int {
    var s = 0
    for (x in xs) s += f(x)
    return s
}

fun mutate(vararg xs: Int): Int {
    xs[0] = 100
    return xs[0]
}

fun nullable(vararg xs: String?): Int {
    var n = 0
    for (x in xs) if (x == null) n++
    return n
}

class Bag(vararg val items: String) {
    fun size(): Int = items.size
}

open class Base(vararg xs: Int) {
    val total = sum(*xs)
}

class Derived : Base(1, 2, 3)

fun main() {
    println(sum())
    println(sum(1))
    println(sum(1, 2, 3))
    val a = intArrayOf(4, 5)
    val b = intArrayOf(6)
    println(sum(*a))
    println(sum(1, *a, 2, *b))
    println(sum(*a, *b))
    println(join("-"))
    println(join("-", "x", "y", "z"))
    val words = arrayOf("p", "q")
    println(join("+", *words, "r"))
    println(count<Any>())
    println(count(1, "two", 3.0))
    println(count(*words))
    println(firstOr("none"))
    println(firstOr("none", "first", "second"))
    println(tagged(1, 2, tag = "t"))
    println(tagged(tag = "u"))
    println(withDefault(1L, 2L))
    println(withDefault(1L, scale = 2L))
    println(apply(1, 2, 3) { it * it })

    // A spread array is copied.
    val orig = intArrayOf(1, 2)
    println(mutate(*orig))
    println(orig[0])

    println(nullable("a", null, null))
    println(Bag("a", "b", "c").size())
    println(Bag().size())
    println(Derived().total)

    // Java varargs.
    println(java.lang.String.format("%s=%d", "n", 3))
    val args = arrayOf<Any>("m", 4)
    println(java.lang.String.format("%s=%d", *args))
    println(java.lang.String.join(",", *words))
    println(java.lang.String.join(",", "x", *words, "y"))
    println(java.util.Arrays.asList(*words).size)
}
