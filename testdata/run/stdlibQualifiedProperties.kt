import kotlin.coroutines.*

// A library's top-level properties by their qualified names.

class Name(val n: String) : AbstractCoroutineContextElement(Name) {
    companion object Key : CoroutineContext.Key<Name>
}

suspend fun name() = kotlin.coroutines.coroutineContext[Name]?.n

suspend fun same() = kotlin.coroutines.coroutineContext === coroutineContext

fun main() {
    var r: String? = null
    suspend { name() + " " + same() }.startCoroutine(Continuation(Name("ctx")) { r = it.getOrThrow() })
    println(r)
    println(kotlin.io.DEFAULT_BUFFER_SIZE)
}
