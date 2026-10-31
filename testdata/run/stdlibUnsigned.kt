import kotlin.random.Random
import kotlin.random.nextULong
import kotlin.time.Duration

// The standard library's value classes: unsigned integers and Duration.
// Their inline-only members are copied into the callers.

fun describe(u: UInt): String = "u=" + u + " (" + u.toLong() + ")"

fun sum(xs: List<UInt>): UInt {
    var total = 0u
    for (x in xs) {
        total += x
    }
    return total
}

class Counter(var count: ULong = 0uL) {
    fun inc() {
        count++
    }
}

fun main() {
    val a: UInt = 5u
    val b = 7u
    println(a + b)
    println(a < b)
    println(b - a)
    println((a * 3u).toString())
    val big = 4000000000u
    println(big)
    println(big.toLong())
    println(big / 3u)
    println(big % 7u)
    val l: ULong = 1uL shl 63
    println(l)
    println(Random(1).nextULong() > 0uL)
    println(listOf(1u, 2u).map { it * 2u })
    println(sum(listOf(1u, 2u, 4000000000u)))
    println(255.toUByte())
    println((-1).toUInt())
    println(3u.toInt() + 1)
    println(describe(9u))
    val x: UShort = 65535u
    println(x)
    println(x.toInt())
    val m = mapOf(1u to "one", 2u to "two")
    println(m[2u])
    val c = Counter()
    c.inc()
    c.inc()
    println(c.count)
    val u: Any = 42u
    println(u is UInt)
    println(u == 42u)
    val d = Duration.parse("1m 30s")
    println(d)
    println(d.inWholeSeconds)
    println(d + Duration.parse("30s"))
    println(d > Duration.parse("1m"))
}
