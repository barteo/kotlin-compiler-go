package tasks

import kotlin.coroutines.*
import kotlin.coroutines.intrinsics.*

// Suspends for real: another thread resumes the coroutine with v.
suspend fun <T> later(v: T): T = suspendCoroutineUninterceptedOrReturn { c ->
    Thread { c.resume(v) }.start()
    COROUTINE_SUSPENDED
}

suspend fun sum(xs: List<Int>, start: Long = 0L): Long {
    var s = start
    for (x in xs) {
        s += later(x)
    }
    return s
}

suspend fun tail(x: Int): Int = later(x * 10)

open class Worker(val name: String) {
    open suspend fun work(n: Int): String {
        val done = later(n)
        return "$name did $done"
    }

    suspend fun twice(n: Int): String = work(n) + "; " + work(n + 1)
}

class Lazy(name: String) : Worker(name) {
    override suspend fun work(n: Int): String = "lazy " + super.work(later(n - 1))
}

interface Store<K, V> {
    suspend fun load(key: K): V

    suspend fun loadAll(keys: List<K>): List<V> {
        val out = ArrayList<V>()
        for (k in keys) {
            out.add(load(k))
        }
        return out
    }
}

class Squares : Store<Int, Int> {
    override suspend fun load(key: Int): Int = later(key * key)
}

object Clock {
    var ticks = 0
    suspend fun tick(): Int {
        later(Unit)
        ticks++
        return ticks
    }
}

@JvmInline
value class Id(val raw: String)

suspend fun lookup(id: Id): Id = Id(later(id.raw.uppercase()))

// The library drives its own coroutines too.
suspend fun selfTest(): String = Lazy("l").twice(3) + " " + sum(listOf(1, 2), 10L)

// Suspend lambdas, passed in and returned.
suspend fun <T> retry(times: Int, block: suspend (Int) -> T?): T? {
    for (i in 1..times) {
        val r = block(i)
        if (r != null) {
            return r
        }
    }
    return null
}

fun adder(n: Int): suspend (Int) -> Int = { x -> later(x + n) }

suspend fun <R> Worker.using(f: suspend Worker.() -> R): R = f()

// An inline suspend function, which kotlinc inlines from its
// timed$$forInline method.
suspend inline fun <T> timed(label: String, block: () -> T): T {
    val tag = later(label)
    val r = block()
    println("timed $tag")
    return r
}

// An inline suspend member, which callers in other modules inline from
// its own sample$$forInline method.
class Sampler(val prefix: String) {
    suspend inline fun sample(n: Int): String {
        val v = later(n)
        return "$prefix$v " + later(prefix)
    }
}
