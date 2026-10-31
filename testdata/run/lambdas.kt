import java.util.ArrayList

class Counter(var count: Int) {
    val step = 2

    fun incrementer(): () -> Unit = { count += step }

    fun adder(): (Int) -> Int = { x -> x + count }

    fun repeat(n: Int, action: (Int) -> Unit) {
        for (i in 0 until n) action(i)
    }

    fun nested(): () -> Int = {
        val inner = { this.count * 10 + step }
        inner()
    }
}

class Holder<T>(val value: T, val transform: (T) -> T) {
    fun get(): T = transform(value)
}

class Button(val label: String) {
    var onClick: ((String) -> Unit)? = null

    fun click() {
        onClick?.invoke(label)
    }
}

fun apply(x: Int, f: (Int) -> Int): Int = f(x)

fun twice(f: (Int) -> Int): (Int) -> Int = { x -> f(f(x)) }

fun adder(n: Int) = { x: Int -> x + n }

fun compose(f: (Int) -> Int, g: (Int) -> Int): (Int) -> Int = { f(g(it)) }

fun <T, R> map(xs: ArrayList<T>, f: (T) -> R): ArrayList<R> {
    val out = ArrayList<R>()
    for (i in 0 until xs.size) out.add(f(xs.get(i)))
    return out
}

fun <T> filter(xs: ArrayList<T>, p: (T) -> Boolean): ArrayList<T> {
    val out = ArrayList<T>()
    for (i in 0 until xs.size) {
        val x = xs.get(i)
        if (p(x)) out.add(x)
    }
    return out
}

fun <T, R> fold(xs: ArrayList<T>, initial: R, f: (R, T) -> R): R {
    var acc = initial
    for (i in 0 until xs.size) acc = f(acc, xs.get(i))
    return acc
}

fun <T> run(f: () -> T): T = f()

fun times(n: Int, body: () -> Unit) {
    var i = 0
    while (i < n) {
        body()
        i++
    }
}

fun sum3(f: (Int, Long, Double) -> Double): Double = f(1, 2L, 0.5)

val square: (Int) -> Int = { it * it }
var onEvent: ((String) -> Unit)? = null

fun main() {
    println(apply(3) { it + 1 })
    println(apply(3, { x -> x * 10 }))
    println(twice { it * 3 }(2))
    val add5 = adder(5)
    println(add5(10))
    println(compose(square, add5)(1))
    println(square.invoke(7))
    println(sum3 { a, b, c -> a + b + c })

    var total = 0
    times(4) { total += 10 }
    println(total)

    var log = ""
    val append = { s: String ->
        log += s
        log.length
    }
    println(append("ab"))
    println(append("cde"))
    println(log)

    var counter = 0L
    val next = {
        counter++
        counter
    }
    next()
    println(next())
    println(counter)

    val c = Counter(1)
    val inc = c.incrementer()
    inc()
    inc()
    println(c.count)
    println(c.adder()(100))
    c.repeat(3) { i -> print("$i ") }
    println()
    println(c.nested()())

    val xs = ArrayList<Int>()
    for (i in 1..5) xs.add(i)
    println(map(xs) { "n$it" })
    println(map(xs) { it * 2L })
    println(fold(xs, 0) { acc, x -> acc + x })
    println(fold(xs, "") { acc, x -> acc + x })
    println(filter(xs) { it % 2 == 1 })

    val h = Holder("abc") { it.uppercase() }
    println(h.get())
    val r = run { 42 }
    println(r + 1)
    val l: Long = run { 7 }
    println(l)
    run { println("in run") }

    println(onEvent == null)
    onEvent = { println("event: $it") }
    onEvent?.invoke("click")
    val f: ((Int) -> Int)? = null
    println(f?.invoke(1))
    if (onEvent != null) println("set")

    val b = Button("ok")
    b.click()
    b.onClick = { println("clicked $it") }
    b.click()

    val outer = 10
    val mk = { a: Int -> { b: Int -> a + b + outer } }
    println(mk(1)(2))

    val classify: (Int) -> String = { n ->
        when {
            n < 0 -> "neg"
            n == 0 -> "zero"
            else -> "pos"
        }
    }
    println(classify(-3) + classify(0) + classify(5))
    val noop: () -> Unit = {}
    noop()
    val d: (Double, Double) -> Double = { a, b -> a * b }
    println(d(1.5, 2.0))
    val pred = { s: String? -> s != null && s.length > 2 }
    println(pred("abcd"))
    println(pred(null))
    val first: (Int, Int) -> Int = { a, _ -> a }
    println(first(1, 2))
    val nullable: (Int?) -> Int = { it ?: -1 }
    println(nullable(null) + nullable(4))

    val fs = ArrayList<() -> Int>()
    for (i in 0 until 3) fs.add { i * i }
    var acc = 0
    for (i in 0 until fs.size) acc += fs.get(i)()
    println(acc)

    var flag = false
    val toggle = { flag = !flag }
    toggle()
    println(flag)
    val immediate = { x: Int -> x + 1 }(41)
    println(immediate)
}
