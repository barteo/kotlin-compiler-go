// Function values passed for suspend function types: converted, as with
// kotlinc, to suspend lambdas that call them, also for function types
// with receivers, nullable ones and overloaded functions' arguments, and
// objects of classes that implement function types, which also convert
// to fun interfaces.

import kotlin.coroutines.*

fun run(c: suspend () -> Unit) {
    c.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
}

fun runInt(c: suspend Int.() -> Unit) {
    c.startCoroutine(7, Continuation(EmptyCoroutineContext) { it.getOrThrow() })
}

fun runMaybe(c: (suspend () -> Unit)?) {
    if (c == null) println("none") else run(c)
}

suspend fun apply(f: suspend (String) -> String) = f("O")

fun pick(f: () -> String, g: suspend () -> String) = f()
fun pick(f: () -> String, g: suspend () -> String, h: suspend () -> String) = f() + "3"

val append: (String) -> String = { it + "K" }

open class Fns : () -> String, (Int) -> String {
    override fun invoke() = "empty"
    override fun invoke(p: Int) = "int $p"
}

fun interface Action {
    fun act(): String
}

fun act(a: Action) = a.act()

suspend fun callInt(f: suspend (Int) -> String) = f(42)

fun main() {
    run { println(apply(append)) }
    val plain: (Int) -> Unit = { println("plain $it") }
    runInt(plain)
    val ext: Int.() -> Unit = { println("ext $this") }
    runInt(ext)
    val unit: (() -> Unit)? = { println("nullable") }
    runMaybe(unit)
    runMaybe(null)
    val s = { "s" }
    println(pick(s, s))
    println(pick(s, s, s))
    run { println(callInt(Fns())) }
    println(act(Fns()))
}
