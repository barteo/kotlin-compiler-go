import kotlinx.coroutines.*
import kotlinx.coroutines.flow.*
import store.*
import kotlin.coroutines.*

fun main() = runBlocking {
    val s = Store("s")
    coroutineScope {
        listOf("a", "b", "c").forEach { launch { s.add(it) } }
    }
    println(s.size())
    println(s.updates().toList().sorted())
    val n = s.transaction {
        addUnlocked("d")
        delay(1)
        count()
    }
    println(n)
    try {
        s.transaction {
            addUnlocked("e")
            throw IllegalStateException("rollback")
        }
    } catch (e: IllegalStateException) {
        println(e.message)
    }
    println(s.size())
    println(loadAll(listOf("x", "yy", "zzz")))
    println(ticker(4).map { it * it }.toList())

    println(written { delay(1); it.write("hello") })
    println(greeting())
    println(runCatching { written { delay(1); error("boom") } }.exceptionOrNull()?.message)
    println(twice { delay(1); it + 10 })
    println(later { it * 7 }())
    println(echoed { delay(1); println("echo $it") }.toList())
    println(halved(8))
    val got = mutableListOf<Int>()
    sourceOf(1, 2, 3).mapped { delay(1); it * 2 }.mapped { it + 1 }.collect { got += it }
    println(got)
    println(stepping { it * 5 }.next(2))
    println(flowOf(1, 2).mapTwice { delay(1); it * 3 }.toList())
    println(flowOf(1, 2).addAll { it * 10 }.toList())
    println(flowOf(1, 2, 3, 4).keep { delay(1); it % 2 == 0 }.toList())
    println(flowOf(1, 2, 3).peek { println("peek $it") }.toList())
    println(flowOf(1, 2).emitTwice { delay(1); it * 5 }.toList())
    println(flowOf(1, 2).retried { if (it > 1) error("x") else it }.toList())
    // Not copied: its own method.
    val m = Class.forName("store.InlineKt").getMethod("halved", Int::class.javaPrimitiveType, Continuation::class.java)
    println(runCatching { m.invoke(null, -1, Continuation<Any?>(EmptyCoroutineContext) {}) }.exceptionOrNull()?.cause)
}
