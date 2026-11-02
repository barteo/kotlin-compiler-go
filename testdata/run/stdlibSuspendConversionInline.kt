// An inlined lambda that an inline function converts to a suspend one for
// another inline function is inlined there as it is: its return leaves
// the function it is written in.
import kotlin.coroutines.*

suspend inline fun foo(f: suspend () -> String) = f()

suspend inline fun bar(f: () -> String) = foo(f)

suspend fun test(x: Int): String {
    bar { if (x > 0) return "early $x" else "lambda $x" }
    return "late $x"
}

fun run(x: Int) {
    ::test.startCoroutine(x, Continuation(EmptyCoroutineContext) { println(it.getOrThrow()) })
}

fun main() {
    run(1)
    run(0)
    val g: () -> String = { "value" }
    suspend { println(bar(g)) }.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
}
