import kotlin.coroutines.Continuation
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.startCoroutine

fun interface Listener { fun on(v: Int) }

fun register(l: Listener) = l.on(1)

fun runIt(f: () -> Unit) = f()

fun <T> block(f: suspend () -> T): T {
    var r: Result<T>? = null
    f.startCoroutine(Continuation(EmptyCoroutineContext) { r = it })
    return r!!.getOrThrow()
}

inline fun lambdas(crossinline g: () -> Int) {
    runIt { println("lambda " + g()) }
    register { println("sam " + (it + g())) }
    val o = object : Runnable {
        override fun run() = println("object " + g())
    }
    o.run()
    val nested = { { g() + 1 } }
    println("nested " + nested()())
}

inline fun wrap(noinline f: (Int) -> Unit) = register(f)

inline fun suspending(crossinline g: () -> Int): Int = block { g() * 10 }

@PublishedApi
internal inline fun internalLambda(crossinline g: () -> String) = runIt { println(g()) }

inline fun callsInternal(crossinline g: () -> String) = internalLambda(g)

fun main() {
    lambdas { 1 }
    wrap { println("wrapped $it") }
    println(suspending { 2 })
    callsInternal { lib.greet("app") }
}
