import kotlin.coroutines.*
import kotlin.coroutines.intrinsics.*

suspend fun <T> later(v: T): T = suspendCoroutineUninterceptedOrReturn { c ->
    Thread { c.resume(v) }.start()
    COROUTINE_SUSPENDED
}

// Runs a suspend function on this thread, waiting for its result.
fun <T> block(f: suspend () -> T): T {
    val lock = Object()
    var done = false
    var value: Result<T>? = null
    f.startCoroutine(object : Continuation<T> {
        override val context: CoroutineContext get() = EmptyCoroutineContext
        override fun resumeWith(result: Result<T>) {
            synchronized(lock) {
                value = result
                done = true
                lock.notifyAll()
            }
        }
    })
    synchronized(lock) {
        while (!done) {
            lock.wait()
        }
    }
    return value!!.getOrThrow()
}

suspend fun twice(f: suspend (Int) -> Int, x: Int): Int = f(f(x))

suspend fun <R> withText(text: String, f: suspend String.() -> R): R = text.f()

class Counter(var count: Int) {
    fun incrementer(by: Int): suspend () -> Int = {
        count += later(by)
        count
    }
}

suspend fun loop(n: Int): suspend () -> List<Int> = {
    val out = ArrayList<Int>()
    for (i in 0 until n) {
        out.add(later(i) * 10)
    }
    out
}

fun main() {
    println(block { later(1) + later(2) })
    println(block { twice({ later(it + 1) }, 5) })
    var captured = 0
    val add: suspend (Int, Int) -> Int = { a, b ->
        captured += a
        later(a + b)
    }
    println(block { add(1, 2) + add(3, 4) })
    println(captured)
    val three: suspend (Int, String, Long) -> String = { a, b, c -> "$a$b$c" + later("!") }
    println(block { three(1, "two", 3L) })
    println(block { withText("hello") { length + later(1) } })
    val counter = Counter(10)
    val inc = counter.incrementer(5)
    println(block { inc() + inc() })
    println(counter.count)
    println(block { loop(4)() })
    val s = suspend { later("suspend block") }
    println(block(s))
    try {
        block<Int> {
            later(1)
            throw IllegalArgumentException("failed")
        }
    } catch (e: IllegalArgumentException) {
        println("caught " + e.message)
    }
    val nested: suspend () -> String = {
        val inner: suspend (String) -> String = { x -> later(x + "!") }
        inner("a") + inner("b")
    }
    println(block(nested))
}
