package lib

import kotlin.coroutines.Continuation
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.startCoroutine

fun interface Source<T> : () -> T {
    override operator fun invoke(): T
}

fun interface SuspendSource<T> : suspend () -> T {
    override suspend operator fun invoke(): T
}

fun <T> read(s: Source<T>): T = s()

fun <T, S : Source<T>> same(s: S): S = s

class Registry {
    val names = mutableListOf<String>()

    fun put(name: String, s: Source<Int>): Registry {
        names += "$name=${s()}"
        return this
    }
}

fun <T> SuspendSource<T>.map(f: (T) -> T): SuspendSource<T> = SuspendSource { f(invoke()) }

val two: SuspendSource<Int> = SuspendSource { 2 }

fun <T> run(block: suspend () -> T): T {
    var out: Result<T>? = null
    block.startCoroutine(Continuation(EmptyCoroutineContext) { out = it })
    return out!!.getOrThrow()
}
