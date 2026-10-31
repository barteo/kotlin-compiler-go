// Lambdas passed to the standard library's inline-only functions, whose
// bodies kc generates where the inlined function invokes them.

import java.io.StringReader

class Counter(var count: Int) {
    // A lambda inlined in a method uses this and its members.
    fun bump(times: Int): Int {
        repeat(times) { count += it }
        return run { this.count * 2 }
    }
}

fun describe(n: Int?): String = n?.let { "n=" + it } ?: "none"

fun firstEven(xs: List<Int>): Int? = xs.find { it % 2 == 0 }

fun main() {
    // ?.let, and let and also nested.
    println(describe(4) + " " + describe(null))
    println("ab".let { s -> s.length.also { println("len " + it) } })
    println(5.let { it + 1 }.let { it * 2 })

    // A var assigned in repeat stays a plain local.
    var total = 0
    repeat(4) { total += it }
    println(total)

    // takeIf and takeUnless, with values on the stack below them.
    println("x" + 3.takeIf { it > 2 } + 7.takeUnless { it > 2 })
    println(1 + (10.takeIf { it % 2 == 0 } ?: 0))

    // run, also with return@run.
    val r = run {
        if (total > 3) return@run "big"
        "small"
    }
    println(r)

    // A lambda with a try, and one with a loop and break.
    val parsed = "12x".let {
        try {
            it.toInt()
        } catch (e: NumberFormatException) {
            -1
        } finally {
            println("parsed " + it)
        }
    }
    println(parsed)
    val found = run {
        var i = 0
        while (true) {
            if (i * i > 50) break
            i++
        }
        i
    }
    println(found)

    // maxOf invokes its selector at two sites.
    val words = listOf("kiwi", "banana", "fig")
    println(words.maxOf { it.length.toDouble() })
    println(words.minOf { it.length.toDouble() })

    // The overload is chosen by the lambda's result type, as kotlinc does
    // for sumOf and maxOf.
    println(words.sumOf { it.length } + words.sumOf { it.length * 0.5 })
    println(words.maxOf { it })

    // find, List and MutableList.
    println(firstEven(listOf(1, 3, 6, 8)))
    println(firstEven(listOf(1, 3)))
    println(List(3) { it * it })
    val ml = MutableList(2) { "m" + it }
    ml.add("z")
    println(ml)

    // use closes its resource, also when the lambda throws.
    println(StringReader("hello").use { it.read().toChar() })
    try {
        StringReader("q").use { throw IllegalStateException("boom") }
    } catch (e: IllegalStateException) {
        println("caught " + e.message)
    }

    // synchronized, with a monitor and a handler.
    val lock = StringBuilder()
    println(synchronized(lock) { total + 1 })

    // thenBy captures its lambda in a template object; the argument is a
    // lambda object.
    println(words.sortedWith(compareBy<String> { it.length }.thenBy { it }))
    val closer = AutoCloseable { println("closed") }
    closer.use { println("using") }

    // A lambda that isn't inlined, inside an inlined one.
    val adders = run { listOf(1, 2).map { x -> { y: Int -> x + y } } }
    println(adders[1](10))

    // A function value passed to let is invoked, not inlined.
    val twice = { n: Int -> n * 2 }
    println(21.let(twice))

    println(Counter(1).bump(3))
}
