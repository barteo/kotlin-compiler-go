// kotlinx.coroutines's internal functions, which ktor-utils calls where it
// suppresses INVISIBLE_MEMBER and INVISIBLE_REFERENCE.

import kotlinx.coroutines.internal.*
import kotlin.coroutines.*

fun bridge(e: Throwable, c: Continuation<*>): Throwable = try {
    @Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
    recoverStackTrace(e, c)
} catch (_: Throwable) {
    e
}

fun main() {
    val c = Continuation<Unit>(EmptyCoroutineContext) {}
    val e = IllegalStateException("x")
    println(bridge(e, c) === e)
}
