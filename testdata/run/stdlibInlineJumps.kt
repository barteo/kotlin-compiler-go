// return, break and continue in lambdas inlined into a function leave
// the function or its loops, as in Kotlin 2.2.

fun firstNegative(xs: List<Int>): Int {
    try {
        for (x in xs) {
            x.let { if (it < 0) return it }
        }
    } finally {
        println("searched " + xs)
    }
    return 0
}

fun countUntil(limit: Int): Int {
    var n = 0
    while (true) {
        repeat(10) {
            if (n >= limit) break
            n++
        }
    }
    return n
}

fun odds(xs: List<Int>): String {
    val sb = StringBuilder()
    for (x in xs) {
        sb.run {
            if (x % 2 == 0) continue
            append(x)
        }
    }
    return sb.toString()
}

// A return with values on the stack below it, and through nested lambdas.
fun sum(c: Boolean): Int = 1 + run {
    if (c) return 100
    2
}

fun nested(s: String?): String {
    s?.let { a -> a.also { if (it.isEmpty()) return "empty" } }
    return "got " + s
}

fun labeled(xs: List<Int>): Int {
    xs.let {
        for (x in it) {
            if (x > 2) return@labeled x
        }
    }
    return -1
}

// return@map leaves the lambda passed to map, from a lambda inlined in it.
fun clamp(xs: List<Int>): List<Int> = xs.map { x -> x.let { if (it > 2) return@map 2; it } }

fun main() {
    println(firstNegative(listOf(3, -4, 5)))
    println(firstNegative(listOf(1)))
    println(countUntil(25))
    println(odds(listOf(1, 2, 3, 4, 5)))
    println(sum(true) + sum(false))
    println(nested("") + " " + nested("x") + " " + nested(null))
    println(labeled(listOf(1, 3, 4)) + labeled(listOf(0)))
    println(clamp(listOf(1, 2, 3)))
    var found = -1
    for (i in 0..5) {
        with(i * i) {
            if (this > 10) {
                found = i
                break
            }
        }
    }
    println(found)
}
