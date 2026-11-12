import kotlin.coroutines.Continuation
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.startCoroutine

fun launch(block: suspend () -> Unit) {
    block.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
}

suspend fun <T> inner(block: suspend () -> T): T = block()

interface Callback {
    fun changed()
    fun moved(flag: Boolean)
    fun later(flag: Boolean)
}

class Source(private val prefix: String) {
    fun run(): String {
        val out = StringBuilder()
        launch {
            var count = 0
            fun update(reprobe: Boolean = false) {
                count++
                out.append("$prefix$count:$reprobe ")
            }
            val cb: Callback = inner {
                object : Callback {
                    override fun changed() = update()
                    override fun moved(flag: Boolean) = update(reprobe = flag)
                    override fun later(flag: Boolean) = listOf(flag).forEach { f -> Runnable { update(reprobe = f) }.run() }
                }
            }
            cb.changed()
            cb.moved(true)
            cb.later(true)
            update()
        }
        return out.toString().trim()
    }
}

fun main() {
    println(Source("s").run())
}
