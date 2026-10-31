// Extension functions declared in classes: two receivers, this for the
// extension receiver and this@Class for the class's; default arguments,
// safe calls, lambdas that capture both, and inherited ones.

class Formatter(val prefix: String, val width: Int) {
    fun Int.fmt(pad: Char = ' '): String {
        var s = toString()
        while (s.length < width) s = "$pad$s"
        return prefix + s
    }

    fun String.tagged() = "$prefix<$this>${this@Formatter.width}"
    fun Long.big(): Long = this * 1000L

    fun all(xs: Array<Int>): String {
        var out = ""
        for (x in xs) out += x.fmt() + " "
        return out
    }

    fun maybe(s: String?) = s?.tagged()
    fun lazy(): () -> String = { 7.fmt('0') + "x".tagged() }
    fun big() = 5L.big()
}

open class Base {
    fun String.twice() = this + this
}

class Derived : Base() {
    fun go() = "ab".twice()
}

object Units {
    val factor = 3
    fun Int.scaled() = this * factor
    fun run() = 7.scaled()
}

fun main() {
    val f = Formatter("#", 4)
    println(f.all(arrayOf(1, 22, 333)))
    println(f.maybe("q"))
    println(f.maybe(null))
    println(f.lazy()())
    println(f.big())
    println(Derived().go())
    println(Units.run())
}
