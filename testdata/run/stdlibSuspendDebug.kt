// kotlinc: -Xdebug
import kotlin.coroutines.*
import kotlin.coroutines.intrinsics.*

// Compiled as with kotlinc's -Xdebug: a suspension point saves the dead
// variables' values, not null.

var saved: Continuation<Unit>? = null

suspend fun pause() = suspendCoroutineUninterceptedOrReturn<Unit> { c ->
    saved = c
    COROUTINE_SUSPENDED
}

// The spilled references of the continuation that saved is, by field.
fun spilled(): String {
    val c = saved!!
    val fields = c.javaClass.declaredFields.filter { it.name.startsWith("L$") }.sortedBy { it.name }
    return fields.joinToString { f ->
        f.isAccessible = true
        f.name + "=" + f.get(c)
    }
}

suspend fun work(name: String): Int {
    val dead = "dead " + name
    println(dead)
    pause()
    val n = name.length
    pause()
    return n
}

fun lambda(): suspend (String) -> Int = { s ->
    val gone = s + "!"
    println(gone)
    pause()
    s.length
}

fun main() {
    suspend { work("x") }.startCoroutine(Continuation(EmptyCoroutineContext) { println("work: " + it.getOrThrow()) })
    println(spilled())
    saved!!.resume(Unit)
    println(spilled())
    saved!!.resume(Unit)
    lambda().startCoroutine("y", Continuation(EmptyCoroutineContext) { println("lambda: " + it.getOrThrow()) })
    println(spilled())
    saved!!.resume(Unit)
}
