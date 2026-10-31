// Arrays and varargs through overrides, generics, nullable arrays and Java.

interface Summer {
    fun sum(vararg xs: Int): Int
}

open class Plain : Summer {
    override fun sum(vararg xs: Int): Int {
        var s = 0
        for (x in xs) s += x
        return s
    }
}

class Doubled : Plain() {
    override fun sum(vararg xs: Int): Int = 2 * super.sum(*xs)
}

fun named(vararg xs: String, sep: String = ","): String {
    var s = ""
    for (x in xs) s += x + sep
    return s
}

fun <T : Comparable<T>> maxOf2(xs: Array<T>): T {
    var m = xs[0]
    for (x in xs) if (x > m) m = x
    return m
}

fun maybe(b: Boolean): IntArray? = if (b) intArrayOf(7, 8) else null

class Holder<T>(val items: Array<T>) {
    fun first(): T = items[0]
    fun put(i: Int, v: T) { items[i] = v }
}

data class Pixels(val raw: IntArray, val tags: Array<String>)

fun main() {
    val px = Pixels(intArrayOf(1, 2), arrayOf("p"))
    println(px)
    println(px == Pixels(intArrayOf(1, 2), arrayOf("p")))
    println(px.hashCode() == Pixels(intArrayOf(1, 2), arrayOf("p")).hashCode())
    val s: Summer = Doubled()
    println(s.sum(1, 2, 3))
    println(Plain().sum())
    val arr = arrayOf("a", "b")
    println(named(xs = *arr))
    println(named(*arr, sep = ";"))
    println(maxOf2(arrayOf(3, 9, 4)))
    println(maxOf2(arrayOf("x", "z", "y")))
    println(maybe(true)?.size)
    println(maybe(false)?.size)
    println(maybe(true)?.get(1))
    val ids = java.util.TimeZone.getAvailableIDs()
    println(ids.size > 0)
    println(java.util.Arrays.asList(*ids).size == ids.size)
    val h = Holder(arrayOf(1, 2))
    h.put(1, 5)
    println(h.first() + h.items[1])
    val grid = Array(2) { r -> Array(2) { c -> r * 2 + c } }
    println(grid[1][1])
    grid[1][0] += 10
    println(grid[1][0])
    var total = 0L
    val ls = LongArray(3) { it.toLong() * 3000000000L }
    for (l in ls) total += l
    println(total)
    val strs = Array<String?>(2) { null }
    println(strs[0] == null)
    val objs: Array<Any?> = arrayOfNulls(1)
    println(objs.size)
    println(intArrayOf(1, 2) == intArrayOf(1, 2))
    val same = intArrayOf(1)
    println(same == same)
    var grown = ByteArray(1) // a var of a primitive array type that a loop changes
    while (grown.size < 4) grown = ByteArray(grown.size * 2)
    println(grown.size)
}
