import kotlin.coroutines.*
import kotlin.coroutines.intrinsics.*

// Suspends for real: another thread resumes the coroutine with v.
suspend fun <T> later(v: T): T = suspendCoroutineUninterceptedOrReturn { c ->
    Thread { c.resume(v) }.start()
    COROUTINE_SUSPENDED
}

// The JVM entry point passes main its args through the lambda it runs.
suspend fun main(args: Array<String>) {
    val n = later(args.size)
    println("args: $n")
    println(later(args.joinToString(prefix = "[", postfix = "]")))
    for (a in args) println(later(a))
    println("done")
}
