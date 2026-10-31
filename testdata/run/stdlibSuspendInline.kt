import kotlin.coroutines.*
import java.io.StringReader

// suspendCoroutine is inline: its suspension point is the caller's.
suspend fun <T> later(v: T): T = suspendCoroutine { c -> Thread { c.resume(v) }.start() }

suspend fun now(v: Int): Int = suspendCoroutine { c -> c.resume(v) }

suspend fun failing(): Int = suspendCoroutine { c -> Thread { c.resumeWithException(IllegalStateException("bad")) }.start() }

suspend fun notTail(v: Int): Int {
    val x = suspendCoroutine<Int> { c -> c.resume(v) }
    return x + suspendCoroutine<Int> { c -> Thread { c.resume(v) }.start() }
}

inline fun <T> measured(name: String, block: () -> T): T {
    println("start $name")
    val r = block()
    println("end $name")
    return r
}

suspend inline fun <T> twiceLater(v: T, f: (T) -> T): T = f(later(v))

suspend inline fun repeatSuspending(n: Int, action: (Int) -> Unit) {
    for (i in 0 until n) {
        action(later(i))
    }
}

suspend fun lambdas(): Int {
    var sum = 0
    repeat(3) { sum += later(it) }
    listOf(10, 20).forEach { sum += later(it) }
    val x = later(5).let { it + later(1) }
    val y = measured("m") { later(100) + 1 }
    val m = mapOf("a" to 1, "b" to 2).map { (k, v) -> k + later(v) }
    return sum + x + y + m.size
}

suspend fun inlineSuspend(): String {
    val a = twiceLater(7) { it * 2 }
    val sb = StringBuilder()
    repeatSuspending(3) { sb.append(it) }
    repeatSuspending(2) { i -> sb.append(now(i * 10)) }
    return "$a $sb"
}

suspend fun resources(): String {
    val text = StringReader("line").use { r -> later(r.read().toChar().toString()) }
    val failed = try {
        StringReader("x").use { failing() }
    } catch (e: IllegalStateException) {
        -1
    }
    return text + failed
}

suspend fun main() {
    println(lambdas())
    println(inlineSuspend())
    println(resources())
    println(notTail(4))
    println(now(3) + later(4))
}
