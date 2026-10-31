// Fixes from checking kotlinpoet: super through interfaces, smart casts
// after `as`, ++ as a value, named arguments before positional ones,
// default values of vararg parameters, hidden declarations, @JvmName
// overloads, private properties, and the built-in String() and Unit.

interface Named {
    fun describe(): String = "named"
    fun tag(): String
}

interface Builder<T : Builder<T>> {
    fun add(s: String): T
    fun addAll(xs: Array<String>): T {
        for (x in xs) add(x)
        @Suppress("UNCHECKED_CAST")
        return this as T
    }
}

class ListBuilder : Builder<ListBuilder>, Named {
    val items = StringBuilder()
    override fun add(s: String): ListBuilder {
        items.append(s).append(';')
        return this
    }
    override fun addAll(xs: Array<String>): ListBuilder = super.addAll(Array(xs.size) { xs[xs.size - 1 - it] })
    override fun describe(): String = "list " + super.describe()
    override fun tag() = "lb"
}

open class Shape(val sides: Int) {
    override fun hashCode() = sides
    override fun equals(other: Any?): Boolean {
        if (other == null || other !is Shape) return false
        other as Shape
        return sides == other.sides
    }
}

class Square : Shape(4), Comparable<Square> {
    override fun compareTo(other: Square) = 0
    override fun hashCode(): Int = super.hashCode() * 10
}

class Point(val x: Int, val y: Int) {
    @Deprecated("", level = DeprecationLevel.HIDDEN)
    constructor(x: Int, y: Int, vararg rest: Int) : this(x + rest.size, y)

    constructor(name: String, y: Int, z: Int, w: Int) : this(x = name.length, y + z + w)

    override fun toString() = "($x, $y)"
}

class Writer {
    @JvmName("emitNames")
    fun emit(xs: java.util.ArrayList<String>) = "names " + xs.size + xs.get(0)

    @JvmName("emitNumbers")
    fun emit(xs: java.util.ArrayList<Int>) = "numbers " + xs.sum()

    private fun java.util.ArrayList<Int>.sum(): Int {
        var s = 0
        for (x in this) s += x
        return s
    }
}

fun format(prefix: String, vararg args: Any = arrayOf("none"), suffix: String = "."): String {
    val sb = StringBuilder(prefix)
    for (a in args) sb.append(a).append(',')
    return sb.append(suffix).toString()
}

class Counter(private var count: Int) {
    private val step = 2
    fun next(): Int {
        count += step
        return count
    }
    val doubled: () -> Int get() = { count * 2 }
    companion object {
        fun peek(c: Counter) = c.count
        fun reset(c: Counter) {
            c.count = 0
        }
    }
}

class Spec(val name: String, val kind: String) {
    constructor(name: String) : this(normalize(name), kind())

    companion object {
        fun normalize(s: String) = s.substring(1)
        fun kind() = "default"
    }
}

fun quote(raw: Boolean, c: Char): String {
    val sb = StringBuilder()
    sb.append(if (raw) c else "'$c'")
    return sb.toString()
}

fun <T : Comparable<T>> largest(a: T, b: T): T = if (a > b) a else b

private val EMPTY = String()

fun kindOf(n: Int): String {
    var out = "other"
    when (n) {
        0 -> Unit
        1 -> out = "one"
    }
    return out
}

fun describe(o: Any?): String = o.toString()

fun main() {
    val b = ListBuilder().add("a").addAll(arrayOf("b", "c"))
    println(b.items)
    println(b.describe() + " " + b.tag())
    println(Shape(3) == Shape(3))
    println(Square().hashCode())

    println(Point(1, 2))
    println(Point("abc", 1, 2, 3))

    val w = Writer()
    val names = java.util.ArrayList<String>()
    names.add("x")
    val numbers = java.util.ArrayList<Int>()
    numbers.add(1)
    numbers.add(2)
    println(w.emit(names))
    println(w.emit(numbers))

    println(format("a"))
    println(format("b", 1, 2))
    println(format("c", suffix = "!"))

    var i = 0
    val xs = IntArray(3)
    xs[i++] = 10
    xs[i++] = ++i
    println("${xs[0]} ${xs[1]} ${xs[2]} $i")
    var l = 5L
    println("${l--} ${--l} $l")
    val inc = { i++ }
    println("${inc()} ${inc()} $i")

    val c = Counter(1)
    println(c.next())
    println(Counter.peek(c))
    Counter.reset(c)
    println(c.next())
    println(c.doubled())

    println(Spec("_name").name + " " + Spec("x").kind)
    println(quote(true, 'q') + quote(false, 'q'))
    println(largest("pear", "apple"))
    println(EMPTY.isEmpty())
    println(kindOf(0) + " " + kindOf(1))
    println(describe(null) + " " + describe(42))
    val n: Int? = null
    println(n.toString())
}
