// Unsigned progressions with step in for loops, Nothing range bounds, the
// program's rangeTo and contains on built-in ranges, value class and
// primitive bounds of type parameters, ++ on Byte elements, Arrays as
// Cloneable, smart casts across &&, numbers' common supertype, Any's
// members over extensions, collection literals of primitive arrays and
// star projections written to with null.

operator fun Int.rangeTo(right: String): ClosedRange<Int> = this..this + 1
operator fun IntRange.contains(s: String): Boolean = s.length == 1

@JvmInline
value class V(val x: Int) {
    fun twice() = x * 2
}

fun <T : V> twice(v: T): Int = v.twice()
fun <T : Char> upper(c: T): Boolean = c in 'A'..'Z'
fun <T : UInt> sum(u: T): UInt {
    var s = 0u
    for (i in u..u + 2u) s += i
    return s
}

fun breakBound(): Int {
    var n = 0
    for (i in 0..1) {
        n++
        for (j in break downTo 1u) {}
    }
    return n
}

fun narrowed(a: Any?): Int = if (a is Int? && a != null) a + 1 else -1

fun cloneable(c: Cloneable) = c.javaClass.simpleName

fun number(d: Double?): Number = d ?: 0

annotation class Ints(val v: IntArray = [1, 2])

interface Box<T : Any> {
    fun put(f: () -> T?)
}

class StringBox : Box<String> {
    var last: String? = "x"
    override fun put(f: () -> String?) {
        last = f()
    }
}

fun Box<*>.clear() = put { null }

fun main() {
    val ul = mutableListOf<ULong>()
    for (i in 7uL downTo 1uL step 3L) ul.add(i)
    println(ul)
    val ui = mutableListOf<UInt>()
    for (i in 1u..9u step 4) ui.add(i)
    println(ui)

    println(0 in 1.."a")
    println("s" in 0..1)

    println(twice(V(21)))
    println(upper('Q').toString() + " " + upper('q'))
    println(sum(1u))
    println(breakBound())

    val bytes = arrayListOf<Byte>(1)
    bytes[0]++
    bytes[0]++
    println(bytes)

    println(narrowed(4))
    println(narrowed(null))
    println(cloneable(intArrayOf()) + " " + cloneable(arrayOf("")))
    println(number(null))
    println("a".equals(1))
    println(Ints().v.toList())

    val box = StringBox()
    box.clear()
    println(box.last)
}
