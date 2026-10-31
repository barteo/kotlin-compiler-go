// combine of more than five flows, whose inline body creates a suspend
// lambda that creates itself (its create()), copied with the call's
// reified type argument.
import kotlinx.coroutines.flow.*
import kotlinx.coroutines.runBlocking

@Suppress("UNCHECKED_CAST")
fun <T1, T2, T3, T4, T5, T6, R> combine6(
    flow1: Flow<T1>, flow2: Flow<T2>, flow3: Flow<T3>, flow4: Flow<T4>, flow5: Flow<T5>, flow6: Flow<T6>,
    transform: suspend (T1, T2, T3, T4, T5, T6) -> R,
): Flow<R> = combine(flow1, flow2, flow3, flow4, flow5, flow6) { args ->
    transform(args[0] as T1, args[1] as T2, args[2] as T3, args[3] as T4, args[4] as T5, args[5] as T6)
}

fun main() = runBlocking {
    val f = combine6(flowOf(1), flowOf("a"), flowOf(2L), flowOf('c'), flowOf(true), flowOf(3.0)) { a, b, c, d, e, g ->
        "$a $b $c $d $e $g"
    }
    println(f.toList())
    println(combine(listOf(flowOf("x"), flowOf("y"))) { it.joinToString("+") }.toList())
    val counts = MutableStateFlow(1)
    println(combine(counts, counts.map { it * 10 }, flowOf(5), flowOf(6), flowOf(7), flowOf(8)) { xs -> xs.sumOf { it } }.first())
}
