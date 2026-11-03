// A suspend local function with default values: its $default method takes
// the continuation before the mask, and returns an Object.
import kotlin.coroutines.*

var result = "Fail"

suspend fun doAction() {
    suspend fun run(
        a: String,
        f: suspend (String) -> Unit = { input -> result = input },
    ) {
        f(a)
    }
    run("OK")
}

fun main() {
    suspend { doAction() }.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
    println(result)
}
