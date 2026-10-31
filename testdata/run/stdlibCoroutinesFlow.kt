import kotlinx.coroutines.*
import kotlinx.coroutines.flow.*

fun numbers(n: Int): Flow<Int> = flow {
    for (i in 1..n) {
        delay(1)
        emit(i)
    }
}

suspend fun sumOf(f: Flow<Int>): Int {
    var total = 0
    f.collect { total += it }
    return total
}

fun main() = runBlocking {
    numbers(3).collect { println("value $it") }
    println(sumOf(numbers(4)))
    val doubled = numbers(5).map { it * 2 }.filter { it > 4 }.toList()
    println(doubled)
    val seen = ArrayList<String>()
    numbers(3).onEach { seen.add("saw $it") }.map { "#$it" }.collect { seen.add(it) }
    println(seen)
    println(numbers(10).take(3).toList())
    println(flowOf("a", "b", "c").first())
    println(listOf(1, 2, 3).asFlow().map { it * it }.reduce { a, b -> a + b })
    val state = MutableStateFlow(0)
    state.value = 5
    state.update { it + 1 }
    println(state.value)
    val transformed = numbers(3).transform { v ->
        emit(v)
        emit(v * 100)
    }.toList()
    println(transformed)
    var last = 0
    numbers(4).collectIndexed { index, value -> last = index * value }
    println(last)
}
