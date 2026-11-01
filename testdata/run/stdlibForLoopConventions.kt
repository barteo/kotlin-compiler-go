// for loops whose iterator(), hasNext() and next() are member extensions,
// generic extensions or extensions of a nullable receiver, and ranges
// made by the members rangeTo and rangeUntil.

import kotlin.coroutines.Continuation
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.startCoroutine

class Countdown(val from: Int)

class Ticks(var left: Int)

class Clock {
    var ticks = 0

    operator fun Countdown.iterator() = Ticks(from)
    operator fun Ticks.hasNext(): Boolean {
        ticks++
        return left > 0
    }
    operator fun Ticks.next() = left--

    fun run(n: Int): String {
        var s = ""
        for (i in Countdown(n)) {
            if (i == 2) continue
            s += i
        }
        return "$s after $ticks ticks"
    }

    fun firstEven(n: Int): Int {
        for (i in Countdown(n)) {
            if (i % 2 == 0) return i
        }
        return -1
    }
}

class Cell<T>(val head: T, val tail: Cell<T>?)

operator fun <T> Cell<T>.iterator(): Iterator<T> = object : Iterator<T> {
    var cur: Cell<T>? = this@iterator
    override fun hasNext() = cur != null
    override fun next(): T {
        val c = cur!!
        cur = c.tail
        return c.head
    }
}

operator fun <T : Any> T?.iterator(): Iterator<T> = listOfNotNull(this).iterator()

class Pairs(val n: Int)

class PairIt(val n: Int) {
    var i = 0
}

operator fun Pairs.iterator() = PairIt(n)
operator fun <T : PairIt> T.hasNext() = i < n
operator fun PairIt.next(): Pair<Int, String> {
    i++
    return i to "#$i"
}

class Lazy(val n: Int)

class LazyIt(val n: Int) {
    var i = 0
    suspend operator fun hasNext(): Boolean = i < n
    operator fun next() = i++
}

class Source {
    operator fun Lazy.iterator() = LazyIt(n)

    suspend fun sum(l: Lazy): Int {
        var s = 0
        for (x in l) s += x
        return s
    }
}

fun Int.digitsUpto(end: Int): Int {
    var sum = 0
    for (i in rangeTo(end)) sum = sum * 10 + i
    for (i in rangeUntil(end)) sum += i
    return sum
}

fun Char.lettersTo(end: Char): String {
    var s = ""
    for (c in rangeTo(end)) s += c
    return s
}

fun main() {
    val clock = Clock()
    println(clock.run(4))
    println(Clock().firstEven(5))

    var s = ""
    for (x in Cell("a", Cell("b", Cell("c", null)))) s += x
    println(s)

    val lambdas = mutableListOf<() -> Int>()
    for (x in Cell(1, Cell(2, null))) lambdas += { x * 10 }
    println(lambdas.map { it() })

    val some: Int? = 7
    val none: Int? = null
    for (x in some) println("some $x")
    for (x in none) println("none $x")

    for ((i, name) in Pairs(3)) {
        if (i == 3) break
        println("$i $name")
    }

    var result = 0
    val block: suspend () -> Int = { Source().sum(Lazy(5)) }
    block.startCoroutine(Continuation(EmptyCoroutineContext) { result = it.getOrThrow() })
    println(result)

    println(1.digitsUpto(4))
    println('a'.lettersTo('e'))
    val r = 3L.rangeTo(6)
    println("$r ${r.javaClass.simpleName} ${2.rangeUntil(5)} ${'x'.rangeTo('z').last}")
    var t = 0
    for (i in 2.toByte().rangeTo(4.toShort())) t += i
    println(t)
}
