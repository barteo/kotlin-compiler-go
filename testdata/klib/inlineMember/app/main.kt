import lib.*
import kotlin.coroutines.*

var next: Continuation<Unit>? = null

suspend fun pause(): Unit = suspendCoroutine { next = it }

// ktor's ByteReadChannel.read: a suspend call in the inlined lambda, and
// a local that only the lambda assigns.
suspend inline fun Buf.read(crossinline block: suspend (ByteArray, Int, Int) -> Int): Int {
    var result: Int
    Ops.readFromHead(this) { array, start, end ->
        result = block(array, start, end)
        result
    }
    return result
}

fun first(b: Buf): Int {
    var value: Int
    Ops.readFromHead(b) { a, s, _ ->
        value = a[s] * 10
        1
    }
    return value
}

fun main() {
    val b = Buf(byteArrayOf(1, 2, 3, 4))
    suspend {
        val n = b.read { a, s, e ->
            pause()
            println("read ${a[s]}..${a[e - 1]}")
            2
        }
        println("$n ${b.pos}")
        println(first(b))
        b.pos
    }.startCoroutine(Continuation(EmptyCoroutineContext) { println("done $it") })
    while (true) {
        val k = next ?: break
        next = null
        println("resume")
        k.resume(Unit)
    }
}
