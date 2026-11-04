// Inlined loops with values that the caller pushed before the call: a
// receiver, earlier arguments, a binary operator's left operand, an
// object under construction. kc moves them to locals around the inlined
// code, as kotlinc does, so that no loop's backward branch has values on
// the operand stack (TestRun checks the classes), which HotSpot needs to
// compile the loop on stack replacement.

inline fun sumTo(n: Int, f: (Int) -> Int): Int {
    var s = 0
    for (i in 0 until n) s += f(i)
    return s
}

inline fun repeatSum(n: Int, f: (Int) -> Long): Long {
    var s = 0L
    var i = 0
    while (i < n) {
        s += f(i)
        i++
    }
    return s
}

inline fun <T> pass(x: T, f: (T) -> T): T = f(x)

inline fun firstOver(limit: Int, f: (Int) -> Int): Int {
    var i = 0
    do {
        if (f(i) > limit) return i
        i++
    } while (true)
}

inline fun guarded(n: Int, f: (Int) -> Int): Int =
    try {
        sumTo(n, f)
    } catch (e: ArithmeticException) {
        -1
    }

class Pair2(val a: String, val b: Int) {
    override fun toString() = "Pair2($a, $b)"
}

class Box(val x: Long, val y: Double, val z: Int) {
    override fun toString() = "Box($x, $y, $z)"
}

enum class Level(val n: Int) {
    LOW(sumTo(4) { it }),
    HIGH(sumTo(10) { it * 2 }),
}

fun three(a: Long, b: Double, c: Int) = "$a $b $c"

// Array(n) { ... } is a loop: in a field's initializer, under this, and
// as a constructor's argument.
class Grid(val w: Int) {
    val cells = Array(w) { IntArray(w) { i -> i * it } }
    override fun toString(): String {
        var s = ""
        for (row in cells) for (c in row) s += c
        return s
    }
}

class Wrap(val xs: IntArray, val tag: String) {
    override fun toString(): String {
        var s = tag + ":"
        for (x in xs) s += x
        return s
    }
}

open class Base {
    open fun total(xs: IntArray): Int {
        var t = 0
        for (x in xs) t += x
        return t
    }
}

class Derived : Base() {
    override fun total(xs: IntArray) = 1 + super.total(IntArray(xs.size) { xs[it] * 2 })
}

fun main() {
    val sb = StringBuilder()
    // A receiver.
    sb.append(sumTo(5) { it * it })
    println(sb)
    // Earlier arguments, of each size.
    println(three(7L, 1.5, sumTo(4) { it + 1 }))
    println(three(repeatSum(3) { it.toLong() * 10 }, 2.5, sumTo(3) { it }))
    // A binary operator's left operand.
    val k = 100
    println(k + sumTo(10) { it })
    println(2.0 * repeatSum(4) { it.toLong() })
    println("x" + sumTo(3) { it })
    // Objects under construction.
    println(Pair2("p", sumTo(6) { it }))
    println(Box(3L, 0.5, sumTo(3) { sumTo(it) { j -> j } }))
    println(Level.LOW.n + Level.HIGH.n)
    // Nested inline calls, with a loop in the lambda.
    println(k + pass(5) { x -> x + sumTo(x) { i -> sumTo(i) { it } } })
    println(sb.append('-').append(pass(3) { var t = 0; for (i in 0..it) t += i; t }))
    // A return from the loop, and a try around it.
    println(k * firstOver(50) { it * it })
    println(k - guarded(4) { 10 / (it + 1) })
    println(k - guarded(4) { 10 / (it - 2) })
    // A loop in a when's branch that is an argument.
    val n = 4
    println(three(1L, 2.0, when {
        n > 3 -> {
            var t = 0
            for (i in 0 until n) t += i
            t
        }
        else -> 0
    }))
    // A break from a lambda's loop to an outer loop.
    var total = 0
    for (round in 1..3) {
        total += pass(round) {
            var t = 0
            while (true) {
                if (t > it) break
                t++
            }
            t
        }
    }
    println(total)
    println(Grid(3))
    println(Wrap(IntArray(4) { it * it }, "sq"))
    println(Derived().total(intArrayOf(1, 2, 3)))
    println("n=" + Array(3) { "x$it" }.size)
}
