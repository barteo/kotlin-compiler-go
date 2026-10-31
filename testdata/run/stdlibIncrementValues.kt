// x++, x--, ++x and --x as values through operator functions and the
// elements of lists, maps and get/set operators: x++ is the old value,
// read before the store, and ++x reads x again after it.

var log = ""

class Counter(val n: Int) {
    operator fun inc(): Counter {
        log += "inc(" + n + ");"
        return Counter(n + 1)
    }

    operator fun dec(): Counter = Counter(n - 1)

    override fun toString() = "Counter(" + n + ")"
}

class Holder {
    var reads = 0
    var c = Counter(0)
        get() {
            reads++
            return field
        }
}

object Grid {
    var cell = "a"

    operator fun get(vararg ix: Int): String {
        log += "get" + ix.size + ";"
        return cell
    }

    operator fun set(vararg ix: Int, value: String) {
        log += "set" + ix.size + ";"
        cell = value
    }
}

operator fun String.dec() = this + "-"
operator fun String?.inc() = this + "!"

open class Base
class Derived : Base()
operator fun Base.inc(): Derived = Derived()

class Maybe
operator fun Maybe?.inc(): Maybe? = if (this == null) Maybe() else null

fun index(i: Int): Int {
    log += "index(" + i + ");"
    return i
}

fun main() {
    var c = Counter(1)
    val old = c++
    val new = ++c
    println("" + old + " " + new + " " + c + " " + --c + " " + c--)
    println(log)

    log = ""
    val h = Holder()
    println("" + h.c++ + " " + h.reads + " " + ++h.c + " " + h.reads)

    val xs = mutableListOf(10, 20)
    println("" + xs[index(1)]++ + " " + ++xs[index(0)] + " " + xs)
    println(log)

    val m = mutableMapOf<String, String?>("k" to "v")
    println(m["k"]++ + " " + ++m["k"] + " " + m)

    log = ""
    val g1 = Grid[1, 2]++
    val g2 = --Grid[3]
    println(g1 + " " + g2 + " " + Grid.cell)
    println(log)

    var b: Base = Base()
    val before = b++
    println("" + (before is Derived) + " " + (b is Derived))

    var mb: Maybe? = Maybe()
    val after = ++mb
    println("" + (after == null) + " " + (mb == null) + " " + (mb++ == null) + " " + (mb != null))

    var u: UInt = 0u
    val uOld = u--
    println("" + uOld + " " + u + " " + ++u)

    var s = "s"
    val f: () -> String = { s++ }
    println(f() + " " + f() + " " + s)
}
