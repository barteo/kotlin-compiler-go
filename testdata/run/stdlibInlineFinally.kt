// Non-local jumps out of lambdas that inline functions with finally
// blocks invoke in a try: the jump runs the finally block, which kc
// copies from the function's bytecode. And lambdas that the program's
// inline functions pass on to other inline functions, okio's withLock.

import java.io.Closeable
import java.util.concurrent.locks.ReentrantLock
import kotlin.concurrent.withLock as jvmWithLock
import kotlin.contracts.ExperimentalContracts
import kotlin.contracts.InvocationKind
import kotlin.contracts.contract

class Res(val name: String) : Closeable {
    override fun close() = println("close $name")
}

val lock = ReentrantLock()

fun fromUse(x: Int): String {
    Res("a").use { r ->
        if (x > 0) return "early ${r.name}"
    }
    return "late"
}

fun fromSynchronized(xs: List<Int>): Int {
    var sum = 0
    for (x in xs) {
        synchronized(lock) {
            if (x == 2) continue
            if (x == 4) break
            sum += x
        }
    }
    return sum
}

fun fromWithLock(x: Int): String {
    lock.jvmWithLock {
        if (x > 0) return "held ${lock.isHeldByCurrentThread}"
    }
    return "not held"
}

// okio's: a contract, which kotlinc compiles to nothing, and a lambda
// passed on to the standard library's withLock.
@OptIn(ExperimentalContracts::class)
inline fun <T> ReentrantLock.withLock(action: () -> T): T {
    contract {
        callsInPlace(action, InvocationKind.EXACTLY_ONCE)
    }
    return jvmWithLock(action)
}

inline fun <T> twice(block: () -> T): T {
    block()
    return once(block)
}

inline fun <T> once(block: () -> T): T = block()

class Handle {
    var closed = false
    var open = 1

    fun close(): String {
        lock.withLock {
            if (closed) return "already"
            closed = true
            if (open != 0) return "open"
        }
        return "closed"
    }
}

fun assigned(b: Boolean): String {
    // A lambda called exactly once assigns what follows reads.
    val t: String
    lock.withLock {
        t = if (b) "yes" else "no"
    }
    var v: Int
    run { v = 3 }
    return "$t $v"
}

fun main() {
    println(fromUse(1))
    println(fromUse(0))
    println(fromSynchronized(listOf(1, 2, 3, 4, 5)))
    println(fromWithLock(1) + " " + lock.isHeldByCurrentThread)
    val h = Handle()
    println(h.close())
    println(h.close())
    h.closed = false
    h.open = 0
    println(h.close())
    var k = 0
    println(twice { k++; "k=$k" })
    println(assigned(true))
}
