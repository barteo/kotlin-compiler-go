// A suspend lambda converted to a fun interface whose parameter is Unit:
// a Flow<Unit>'s collect { }. Its method takes a kotlin.Unit.
import kotlinx.coroutines.flow.*
import kotlinx.coroutines.runBlocking

fun interface Sink<T> {
    suspend fun put(value: T)
}

fun main() = runBlocking {
    var n = 0
    flowOf(Unit, Unit).collect { n++ }
    println(n)
    flowOf(Unit).onStart { println("start") }.collect {}
    flowOf(Unit).collect { println(it) }
    val sink = Sink<Unit> { print("put "); println(it) }
    sink.put(Unit)
    sink.put(Unit)
}
