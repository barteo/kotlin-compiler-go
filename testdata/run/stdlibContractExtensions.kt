@file:OptIn(kotlin.contracts.ExperimentalContracts::class, kotlin.contracts.ExperimentalExtendedContracts::class)

import kotlin.contracts.*

// cond implies returnsNotNull()
fun decode(encoded: String?): String? {
    contract { (encoded != null) implies (returnsNotNull()) }
    if (encoded == null) return null
    return encoded + "!"
}

fun half(n: Any?): Int? {
    contract { (n is Int) implies returnsNotNull() }
    return if (n is Int) n / 2 else null
}

// cond holdsIn lambda
inline fun <R> withFlag(flag: Boolean, block: () -> R): R {
    contract {
        flag holdsIn block
        callsInPlace(block, InvocationKind.EXACTLY_ONCE)
    }
    return block()
}

inline fun <T> whenString(v: Any?, block: () -> T): T {
    contract { (v is String) holdsIn block }
    return block()
}

// contracts on property accessors
val Any?.isStr: Boolean
    get() {
        contract { returns(true) implies (this@isStr is String) }
        return this is String
    }

// contracts on operators
class Box(val v: Any?)

operator fun Box.contains(x: Any?): Boolean {
    contract { returns(true) implies (x is Int) }
    return x is Int && x == v
}

operator fun Any.iterator(): IntIterator {
    contract { returns() implies (this@iterator is IntRange) }
    return (this as IntRange).iterator()
}

// erased type check in a contract
fun <T> isListOf(x: Any?, probe: T): Boolean {
    contract { returns(true) implies (x is List<*>) }
    return x is List<*> && x.all { it?.javaClass == probe?.javaClass }
}

fun main() {
    println(decode("hi").length)
    val h: Int = half(10)
    println(h + 1)
    println(half(7) * 3)

    val any: Any? = "hello"
    val r = whenString(any) { any.length }
    println(r)
    val s: String? = "abc"
    println(withFlag(s != null) { s.length })

    val o: Any? = "prop"
    if (o.isStr) println(o.uppercase())

    val b = Box(3)
    val q: Any? = 3
    if (q in b) println(q + 10)

    val rng: Any = 1..3
    var sum = 0
    for (i in rng) sum += i
    println(sum + rng.last)

    val l: Any? = listOf(1, 2)
    if (isListOf(l, 0)) println(l.size)
}
