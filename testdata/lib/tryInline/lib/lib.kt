package lib

import java.io.StringReader

inline fun <T> logged(block: () -> T): T {
    try {
        return block()
    } finally {
        println("finally")
    }
}

inline fun orZero(block: () -> Int): Int = try { block() } catch (e: IllegalStateException) { 0 }

inline fun <T> traced(crossinline block: () -> T): T = logged { block() }

inline fun firstLine(text: String, crossinline transform: (String) -> String): String =
    StringReader(text).buffered().use { transform(it.readLine()) }

inline fun <T> attempt(block: () -> T): Result<T> = runCatching { block() }
