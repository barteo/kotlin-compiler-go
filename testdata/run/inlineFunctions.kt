// The program's inline functions, whose bodies and lambda arguments are
// copied into their callers.

inline fun twice(action: () -> Unit) {
    action()
    action()
}

inline fun <T> measure(label: String, block: () -> T): T {
    val result = block()
    println(label + ": " + result)
    return result
}

inline fun Int.timesDo(f: (Int) -> Unit) {
    var i = 0
    while (i < this) {
        f(i)
        i++
    }
}

inline fun StringBuilder.line(text: String = "", build: StringBuilder.() -> Unit = {}): StringBuilder {
    append(text)
    build()
    append('\n')
    return this
}

inline fun firstOrDefault(xs: IntArray, default: Int, predicate: (Int) -> Boolean): Int {
    for (x in xs) {
        if (predicate(x)) {
            return x
        }
    }
    return default
}

inline fun keep(value: Int, noinline f: (Int) -> Int): () -> Int = { f(value) }

inline fun guarded(block: () -> Int): Int {
    try {
        return block()
    } catch (e: IllegalStateException) {
        return -1
    } finally {
        println("guarded done")
    }
}

inline fun <R> nested(x: Int, f: (Int) -> R): R = f(x + 1)

fun find(xs: IntArray): String {
    // A non-local return from the lambda returns from find.
    firstOrDefault(xs, 0) { x ->
        if (x < 0) {
            return "negative " + x
        }
        x > 10
    }
    return "none"
}

fun main() {
    var n = 0
    twice { n++ }
    println(n)
    val v = measure("sum") { 1 + 2 }
    println(v * 2)
    3.timesDo { i -> print(i) }
    println()
    val sb = StringBuilder()
    sb.line("a").line { append("b") }.line()
    print(sb)
    println(firstOrDefault(intArrayOf(1, 5, 12, 20), -1) { it > 10 })
    println(firstOrDefault(intArrayOf(1, 5), -1) { it > 10 })
    println(find(intArrayOf(1, -3, 20)))
    println(find(intArrayOf(1, 2)))
    val k = keep(20) { it * 2 }
    println(k())
    println(guarded { 7 })
    println(guarded { throw IllegalStateException() })
    // A lambda passed to a copy contains another copy.
    println(nested(1) { a -> nested(a * 10) { b -> a + b } })
    var total = 0
    for (i in 1..3) {
        twice {
            if (i == 2) {
                total += 100
            } else {
                total += i
            }
        }
    }
    println(total)
}
