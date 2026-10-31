// The constructs of ktor-io's suspend functions that its tests showed
// kc got wrong.

import kotlinx.coroutines.*

class Channel(text: String) {
    val buffer = StringBuilder(text)
    suspend fun awaitContent(): Boolean {
        yield()
        return true
    }
}

// The local is read after the if that throws when the suspension point's
// result is false: it is live there, so it is saved, not nulled out, as
// in ktor's readByte.
suspend fun first(channel: Channel): Char {
    val current = channel.buffer
    if (current.length < 5 && !channel.awaitContent()) {
        throw IllegalStateException("eof")
    }
    return current[0]
}

suspend fun firstOrNull(channel: Channel): Char? {
    val current = channel.buffer
    if (!channel.awaitContent()) return null
    return current.firstOrNull()
}

fun main() = runBlocking {
    println(first(Channel("abc")))
    println(firstOrNull(Channel("")))
    println(firstOrNull(Channel("z")))
}
