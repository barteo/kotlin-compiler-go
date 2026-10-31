// A var that a lambda passed to a suspend inline function uses is shared:
// the function may save the locals in the continuation before it invokes
// the lambda, as suspendCoroutine does, and the lambda's assignment must
// be seen when the code resumes. kotlinx.coroutines' withTimeoutOrNull
// depends on it.

import kotlin.coroutines.*
import kotlin.coroutines.intrinsics.*

class Tagged(val tag: Any?) : Exception()

var saved: Continuation<String>? = null
var lastTag: Any? = null

suspend fun orNull(): String? {
    var tag: Any? = null
    try {
        return suspendCoroutineUninterceptedOrReturn { u ->
            val t = Any()
            tag = t
            lastTag = t
            saved = u
            COROUTINE_SUSPENDED
        }
    } catch (e: Tagged) {
        if (e.tag === tag) return null
        throw e
    }
}

suspend fun safe(): String {
    var where = "before"
    val r = suspendCoroutine<String> { u -> where = "inside"; saved = u }
    return "$r $where"
}

suspend inline fun parked(crossinline f: (Continuation<String>) -> Unit): String =
    suspendCoroutineUninterceptedOrReturn { u -> f(u); COROUTINE_SUSPENDED }

suspend fun own(): String {
    var n = 0
    val r = parked { u -> n++; saved = u }
    return "$r $n"
}

// In tail form, as in kotlinc: parked's copy returns the result of
// suspendCoroutineUninterceptedOrReturn, and tail has no continuation
// class of its own.
suspend fun tail(): String = parked { u -> saved = u }

// Resumes the saved continuation with with, or else with Tagged(lastTag).
fun run(with: Result<String>?, block: suspend () -> Any?) {
    var r: Any? = "unset"
    block.startCoroutine(Continuation(EmptyCoroutineContext) { r = it })
    saved!!.resumeWith(with ?: Result.failure(Tagged(lastTag)))
    println(r)
}

fun main() {
    run(null) { orNull() }
    run(Result.failure(Tagged(null))) { orNull() }
    run(Result.success("ok")) { safe() }
    run(Result.success("ok")) { own() }
    run(Result.success("tail")) { tail() }
    println(Class.forName("StdlibSuspendSharedVarsKt").declaredMethods.filter { it.name == "tail" }.size)
    println(runCatching { Class.forName("StdlibSuspendSharedVarsKt\$tail\$1") }.isSuccess)
}
