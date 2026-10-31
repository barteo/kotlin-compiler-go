package rl

import kotlin.coroutines.Continuation
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.createCoroutine
import kotlin.coroutines.resume

// Public inline functions with reified type parameters whose lambdas
// call others: each call copies their classes with the call's
// type arguments, as with a flow of a stored value whose callbackFlow { }
// lambda reads the value with get<T>.

inline fun <reified T> describe(x: Any?): String = when (T::class) {
    String::class -> "string ${x is T}"
    Int::class -> "int ${x is T}"
    else -> "other ${x is T} ${T::class.simpleName}"
}

inline fun <reified T> later(x: Any?): () -> String = { describe<T>(x) }

fun runSuspend(block: suspend () -> Unit) {
    block.createCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() }).resume(Unit)
}

inline fun <reified T> collect(x: Any?, crossinline sink: (String) -> Unit) {
    runSuspend {
        sink("suspend " + describe<T>(x))
    }
}

inline fun <reified T> collectAll(x: Any?): List<String> {
    val out = mutableListOf<String>()
    runSuspend {
        out += "suspend " + describe<T>(x)
        val listener = java.util.function.Supplier { "listener " + describe<T>(x) }
        out += listener.get()
        val f = { "nested " + describe<T>(x) }
        out += f()
    }
    return out
}
