package store

import kotlinx.coroutines.*
import kotlinx.coroutines.flow.*
import java.io.PrintWriter
import java.io.StringWriter
import java.io.Writer

// Inline functions whose suspend lambdas, and fun interface lambdas with a
// suspend method, capture crossinline lambdas: kotlinc inlines them into
// copies of their classes.

suspend inline fun written(crossinline block: suspend (Writer) -> Unit): String {
    val out = StringWriter()
    withContext(Dispatchers.Default) {
        PrintWriter(out).use { block(it) }
    }
    return out.toString()
}

suspend inline fun twice(crossinline block: suspend (Int) -> Int): Int =
    withContext(Dispatchers.Default) {
        delay(1)
        block(block(1))
    }

inline fun later(crossinline block: (Int) -> Int): suspend () -> Int = {
    delay(1)
    block(2)
}

inline fun echoed(crossinline f: suspend (Int) -> Unit): Flow<Int> = flow {
    flowOf(1, 2).collect {
        f(it)
        emit(it * 10)
    }
}

// Its continuation class and its message's lambda class have different
// names.
suspend inline fun halved(n: Int): Int {
    require(n >= 0) { "negative: $n" }
    delay(1)
    return n / 2
}

// Object expressions whose suspend members capture crossinline lambdas:
// kotlinc inlines them into copies of their classes, from the members'
// name$$forInline methods.
interface Source<T> {
    suspend fun collect(sink: suspend (T) -> Unit)
}

fun <T> sourceOf(vararg xs: T): Source<T> = object : Source<T> {
    override suspend fun collect(sink: suspend (T) -> Unit) {
        for (x in xs) {
            yield()
            sink(x)
        }
    }
}

inline fun <T> Source<T>.mapped(crossinline f: suspend (T) -> T): Source<T> = object : Source<T> {
    override suspend fun collect(sink: suspend (T) -> Unit) {
        this@mapped.collect { sink(f(it)) }
    }
}

fun interface Step {
    suspend fun next(x: Int): Int
}

inline fun stepping(crossinline f: (Int) -> Int): Step = object : Step {
    override suspend fun next(x: Int): Int {
        delay(1)
        return f(x) + 1
    }
}

// Flow operators whose objects the library's class path has: kc copies
// them, with the anonymous classes nested in them, their continuations and
// collectors, as kotlinc does, so kotlinc's inliner can copy them again,
// and without their members' name$$forInline twins, so kotlinc copies the
// state machines that call the lambdas, which suspend.
inline fun <T, R> Flow<T>.mapTwice(crossinline f: suspend (T) -> R): Flow<R> =
    map { f(it) }.map { it }

inline fun Flow<Int>.addAll(crossinline f: (Int) -> Int): Flow<Int> = map { f(it) + 1 }

inline fun Flow<Int>.keep(crossinline p: suspend (Int) -> Boolean): Flow<Int> = filter { p(it) }

inline fun Flow<Int>.peek(crossinline f: (Int) -> Unit): Flow<Int> =
    onEach { f(it) }.mapNotNull { if (it > 2) null else it }

inline fun Flow<Int>.emitTwice(crossinline f: suspend (Int) -> Int): Flow<Int> =
    transform { emit(f(it)); emit(f(it)) }

inline fun Flow<Int>.retried(crossinline f: (Int) -> Int): Flow<Int> =
    map { f(it) }.retryWhen { _, attempt -> attempt < 1 }.catch { emit(99) }
