import b.checkThat
import kotlin.coroutines.Continuation
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.startCoroutine

class Src {
    var n = 0

    suspend fun next(): Int {
        n++
        return n
    }
}

fun run(block: suspend Src.() -> Unit) {
    block.startCoroutine(Src(), Continuation(EmptyCoroutineContext) { it.getOrThrow() })
}

fun main() {
    println(checkThat(2, "two").transform { it * 10 })
    println(checkThat(3).transform("x") { error("boom $it") })
    run {
        repeat(3) {
            val item = next()
            checkThat(item, "item").transform { it * 10 }
            println(checkThat(item, "item").transform { it + 1 })
        }
    }
}
