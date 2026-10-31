// Suspend function types whose JVM interface, with the continuation, has
// more than 22 parameters: kotlin.jvm.functions.FunctionN. Their calls are
// tail calls: kotlinc's other calls of FunctionN's invoke don't suspend.

import kotlin.coroutines.*
import kotlin.coroutines.intrinsics.*

class A(val v: String)

typealias S<T> = suspend (A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, T) -> String

typealias R = suspend A.(A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A) -> Int

var pending: Continuation<Int>? = null

suspend fun pause(): Int = suspendCoroutineUninterceptedOrReturn { c ->
    pending = c
    COROUTINE_SUSPENDED
}

fun run(block: suspend () -> Unit) {
    block.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
    while (true) {
        val p = pending ?: break
        pending = null
        p.resume(1)
    }
}

suspend fun <T> call(f: S<T>, a: A, t: T): String = f(a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, t)

suspend fun narrow(f: suspend (Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, String) -> String) =
    f(1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, "21")

suspend fun extension(r: R, a: A, b: A) = a.r(b, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, b)

suspend fun nullable(h: R?, b: A) = h?.invoke(b, b, b, b, b, b, b, b, b, b, b, b, b, b, b, b, b, b, b, b, b, b, b, b)

fun main() = run {
    val a = A("a")
    val b = A("b")
    var count = 0
    val lambda: S<Char> = { p00, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, p20, c ->
        pause()
        count++
        p00.v + p20.v + c
    }
    println(call(lambda, a, 'x') + call(lambda, b, 'y') + count)
    println(narrow { _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, s -> s })

    val log = StringBuilder()
    val r: R = { x, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, y ->
        log.append(v + x.v + y.v)
        log.length + if (this === x) 0 else pause()
    }
    println(extension(r, a, b))
    println(nullable(if (count > 0) r else null, b))
    println(nullable(null, a))
    println(log)
    val any: Any = lambda
    println(any is Function23<*, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *>)
}
