// Members declared operator and infix: indexing, iteration, destructuring
// and infix calls on the program's own classes.

class Grid(val w: Int, val h: Int) {
    val cells = IntArray(w * h)
    operator fun get(x: Int, y: Int): Int = cells[y * w + x]
    operator fun set(x: Int, y: Int, v: Int) {
        cells[y * w + x] = v
    }
}

class Countdown(val from: Int) {
    operator fun iterator(): Steps = Steps(from)
}

class Steps(var n: Int) {
    operator fun hasNext(): Boolean = n > 0
    operator fun next(): Int {
        n--
        return n + 1
    }
}

class Point(val x: Int, val y: Int) {
    operator fun component1() = x
    operator fun component2() = y
    infix fun dot(o: Point): Int = x * o.x + y * o.y
    infix fun shiftedBy(d: Int) = Point(x + d, y + d)
}

class Words(val ws: Array<String>) {
    operator fun get(i: Int) = ws[i]
    infix fun has(w: String): Boolean {
        for (x in ws) {
            if (x == w) return true
        }
        return false
    }
}

// Overrides of operator and infix functions are operator and infix too.
interface Cursor<T> {
    operator fun hasNext(): Boolean
    operator fun next(): T
}

class ArrayCursor<T>(val xs: Array<T>) : Cursor<T> {
    var i = 0
    override fun hasNext() = i < xs.size
    override fun next(): T {
        i += 1
        return xs[i - 1]
    }
}

class Bag<T>(val xs: Array<T>) {
    operator fun iterator() = ArrayCursor(xs)
}

open class Scale {
    open operator fun get(i: Int) = i
    open infix fun times(n: Int) = n
}

class Double2 : Scale() {
    override fun get(i: Int) = i * 2
    override fun times(n: Int) = n * 2
}

fun main() {
    val g = Grid(3, 2)
    g[1, 1] = 5
    g[2, 0] = 7
    println(g[1, 1] + g[2, 0])
    println(g[0, 0])

    for (i in Countdown(3)) {
        println(i)
    }

    val p = Point(2, 3)
    val (x, y) = p
    println("$x $y")
    println(p dot Point(4, 5))
    val q = p shiftedBy 10
    println("${q.x} ${q.y}")

    val w = Words(arrayOf("a", "b"))
    println(w[1])
    println(w has "a")
    println(w has "z")

    for (s in Bag(arrayOf("x", "yy", "zzz"))) {
        println(s.length)
    }
    val d: Scale = Double2()
    println(d[21])
    println(Double2()[5])
    println(Double2() times 4)
}
