import kotlin.coroutines.Continuation
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.startCoroutine

fun launch(block: suspend () -> Unit) {
    block.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
}

private lateinit var topLevel: String

class Holder {
    private lateinit var value: String
    lateinit var open: String

    fun check(): String {
        var out = ""
        launch {
            out += if (!(::value.isInitialized)) "unset" else "set $value"
            out += if (::open.isInitialized) " open $open" else " closed"
            out += if (::topLevel.isInitialized) " top $topLevel" else " no top"
        }
        val read = { runCatching { value }.exceptionOrNull()?.javaClass?.simpleName ?: value }
        return out + " " + read()
    }

    fun set(v: String) {
        value = v
        open = v
        topLevel = v
    }
}

fun main() {
    val h = Holder()
    println(h.check())
    h.set("x")
    println(h.check())
}
