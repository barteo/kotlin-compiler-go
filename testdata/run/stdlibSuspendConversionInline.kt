// An inlined lambda that an inline function converts to a suspend one for
// another inline function is inlined there as it is: its return leaves
// the function it is written in. A function value converted for one is
// inlined as the conversion's lambda, whose invoke suspends nothing.
import kotlin.coroutines.*

suspend inline fun foo(f: suspend () -> String) = f()

suspend inline fun bar(f: () -> String) = foo(f)

suspend inline fun twice(n: Int, f: suspend (Int) -> Int) = f(f(n))

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
    val h: (Int) -> Int = { it * 3 }
    suspend { println(bar(g)); println(twice(2, h)) }.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
}
