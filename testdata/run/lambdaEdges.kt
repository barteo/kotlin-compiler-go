import java.util.ArrayList

class Widget(val name: String, size: Int) {
    val describe: () -> String = { "$name:$size" }
    var handler: (Int) -> Int = { it + size }
    var doubled = 0

    init {
        val scale = { x: Int -> x * size }
        doubled = scale(2)
    }

    fun helper(): String = name.uppercase()

    fun shout(): () -> String = { helper() + "!" }

    fun later(): () -> () -> String = { { name + size() } }

    fun size(): Int = name.length
}

// Lambdas that write a property, one of them nested in another lambda.
class Tally(start: Int) {
    var total = start
    val bump: (Int) -> Unit = { n -> total += n }

    fun addAll(xs: ArrayList<Int>) {
        each(xs) { x ->
            val add = { total += x }
            add()
        }
    }
}

fun <T> each(xs: ArrayList<T>, f: (T) -> Unit) {
    for (i in 0 until xs.size) f(xs.get(i))
}

fun <T> firstOr(xs: ArrayList<T>, fallback: T, p: (T) -> Boolean): T {
    for (i in 0 until xs.size) {
        if (p(xs.get(i))) return xs.get(i)
    }
    return fallback
}

fun <T> repeatValue(value: T, n: Int): ArrayList<() -> T> {
    val out = ArrayList<() -> T>()
    for (i in 0 until n) out.add { value }
    return out
}

fun <T, R> run2(f: () -> T, g: (T) -> R): R = g(f())

fun <T, R> decide(x: T, f: (T) -> R): R = f(x)

fun <T> either(a: T, b: T, first: Boolean) = if (first) a else b

fun classify(n: Int): String = decide(n) { x ->
    if (x < 0) return@decide "negative"
    if (x == 0) return@decide "zero"
    "positive"
}

fun main() {
    val w = Widget("knob", 3)
    println(w.describe())
    println(w.handler(10))
    w.handler = { it * 100 }
    println(w.handler(2))
    println(w.doubled)
    println(w.shout()())
    println(w.later()()())

    val xs = ArrayList<Int>()
    for (i in 1..6) xs.add(i)
    each(xs) {
        if (it % 2 == 0) return@each
        print("$it ")
    }
    println()
    println(firstOr(xs, -1) { it > 4 })
    println(firstOr(xs, -1) { it > 40 })

    val tally = Tally(10)
    tally.bump(5)
    tally.addAll(xs)
    println(tally.total)

    println(classify(-2) + " " + classify(0) + " " + classify(9))

    var d = 1.5
    val e = 2.25
    val addD = { x: Double -> d += x + e }
    addD(1.0)
    addD(0.25)
    println(d)

    var s: String? = null
    val setS = { v: String? -> s = v }
    setS("set")
    println(s)
    setS(null)
    println(s == null)

    val fs = repeatValue("v", 3)
    println(fs.get(2)() + fs.size)

    println(run2({ 21 }) { it * 2 })
    println(run2({ "a" }, { x -> x + "b" }))

    val maybe = { n: Int -> if (n > 0) n else null }
    println(maybe(3))
    println(maybe(-3))

    val sink = ArrayList<Int>()
    each(xs) { sink.add(it * it) }
    println(sink)

    var calls = 0
    val counter: () -> Int = { calls++; calls }
    counter()
    counter()
    println(calls)

    val nested = { a: Int ->
        val inner = { b: Int ->
            val innermost = { c: Int -> a * 100 + b * 10 + c }
            innermost(3)
        }
        inner(2)
    }
    println(nested(1))

    val big: (Long) -> Long = { it * 3_000_000_000L }
    println(big(2L))
    val flags: (Boolean, Boolean) -> Boolean = { a, b -> a && !b }
    println(flags(true, false))

    // Lambdas without parameters for one variable, and in the branches of
    // an if or a when, each have their own type: the variable's, or the
    // branches', is above both.
    val pair = either({ 1 }, { 1.5f }, calls > 1)
    println(pair())
    val branch = if (calls > 1) { { 2 } } else { { 2.5f } }
    println(branch())
    val parens = if (calls > 5) ({ 3 }) else ({ 3.5f })
    println(parens())
    val chosen = when (calls) { 2 -> { { 4.5f } } else -> { { 4 } } }
    println(chosen())
    // One with parameters takes the other branch's.
    val unary = if (calls > 5) big else { n -> n + 1 }
    println(unary(5L))
}
