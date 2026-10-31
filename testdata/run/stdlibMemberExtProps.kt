// Extension properties declared in interfaces, abstract or with bodies,
// which classes override and class delegation implements, and in value
// classes.

interface Named {
    val Int.label: String
    var StringBuilder.last: String
    val String.shout: String
        get() = uppercase() + suffix()

    fun suffix(): String
}

interface Box<T> {
    val T.boxed: String
    val <R> List<R>.second: R
        get() = this[1]
}

open class Impl : Named, Box<Int> {
    override val Int.label: String
        get() = "#$this"
    override var StringBuilder.last: String
        get() = if (isEmpty()) "" else this[length - 1].toString()
        set(value) {
            append(value)
        }
    override val Int.boxed: String
        get() = "[$this]"

    override fun suffix() = "!"
}

class Forward(n: Named) : Named by n

class Generic(b: Box<Int>) : Box<Int> by b

interface Ext<T> {
    fun T.ext(n: Int): String
    val T.prop: String
}

@JvmInline
value class Tag(val s: String) : Ext<Int> {
    val Int.s: String get() = "$this${this@Tag.s}"
    var StringBuilder.end: String
        get() = s
        set(value) {
            append(value).append(s)
        }

    fun Char.rep(n: Int): String = toString().repeat(n) + s

    override fun Int.ext(n: Int) = "${this + n}$s"
    override val Int.prop: String get() = "p$this"

    fun twice() = 2.s + 'c'.rep(2) + 'd'.rep(1) + run { 3.s + 'e'.rep(2) }

    fun fill(): String {
        val sb = StringBuilder()
        sb.end = "x"
        sb?.end = "y"
        return sb.toString() + sb.end
    }
}

fun <T> viaExt(e: Ext<T>, t: T) = with(e) { t.ext(1) + t.prop }

fun main() {
    with(Impl()) {
        val sb = StringBuilder("ab")
        sb.last = "c"
        println(listOf(1.label, sb.last, "hi".shout, 3.boxed, listOf("x", "y").second).joinToString(" "))
    }
    with(Forward(Impl())) {
        val sb = StringBuilder()
        sb.last = "z"
        println(listOf(7.label, sb.last, sb.toString(), "yo".shout).joinToString(" "))
    }
    with(Generic(Impl())) {
        println(5.boxed + " " + listOf(1, 2).second)
    }
    println(Tag("t").twice() + " " + Tag("u"))
    println(Tag("v").fill() + " " + viaExt(Tag("w"), 4) + " " + with(Tag("z")) { 5.s + 6.ext(2) + 'f'.rep(2) })
}
