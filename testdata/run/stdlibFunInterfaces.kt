import kotlin.coroutines.Continuation
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.startCoroutine

// A fun interface whose method is a suspend function, as kotlinx's
// FlowCollector.
fun interface Collector<in T> {
    suspend fun emit(value: T)
}

suspend fun produce(c: Collector<Int>) {
    for (i in 1..3) c.emit(i)
}

class Source(val values: List<String>) {
    suspend fun collect(collector: Collector<String>) {
        for (v in values) collector.emit(v)
    }
}

fun main() {
    var sum = 0
    val out = StringBuilder()
    suspend {
        produce { sum += it }
        Source(listOf("a", "b")).collect { v -> out.append(v).append(';') }
        produce(Collector { sum += it * 100 })
    }.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
    println(sum)
    println(out)
}
