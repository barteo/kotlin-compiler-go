// Suspend function values called on nullable receivers, block?.invoke(),
// in the arguments of other suspend calls: the values below the call on
// the stack are spilled, and put back on both paths of the null check.
// In flow { }, builder inference sees the type of emit's argument.
import kotlinx.coroutines.flow.*
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.yield

interface Sink {
    suspend fun put(a: Any?, b: Any?, c: Any?)
}

fun completingFlow(block: (suspend () -> Unit)? = null) = flow {
    emit(block?.invoke() ?: Unit)
}

fun lengths(f: (suspend (String) -> Int)?) = flow {
    emit(f?.invoke("abc") ?: -1)
}

fun direct(f: suspend () -> Int) = flow {
    emit(f())
    emit(f.invoke() + 1)
}

suspend fun three(s: Sink, f: (suspend (String) -> Long)?, g: (suspend () -> String)?) {
    s.put(1L, f?.invoke("x"), g?.invoke())
}

suspend fun call(b: suspend () -> Int) = b.invoke()

fun main() = runBlocking {
    println(completingFlow { println("block") }.toList())
    println(completingFlow().toList())
    println(lengths { it.length }.toList() + lengths(null).toList())
    println(direct { yield(); 5 }.toList())
    val sink = object : Sink {
        override suspend fun put(a: Any?, b: Any?, c: Any?) = println(listOf(a, b, c))
    }
    three(sink, { it.length.toLong() + 10 }, { yield(); "g" })
    three(sink, null, null)
    println(call { 7 })
}
