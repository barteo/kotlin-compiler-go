// A function value passed for a fun interface, as ktor-http's
// asFlow().collect(partHandler): kotlinc wraps it in a class of its own.

import kotlinx.coroutines.flow.*
import kotlinx.coroutines.runBlocking

fun interface Render { fun go(x: Int): String }
fun interface SuspendRender { suspend fun go(x: Int): String }
fun interface Combine { fun of(x: Long, y: Double): Int }
fun interface Sink { fun put(s: String) }
fun interface Op<T> { fun ap(x: T): T }

fun render(r: Render) = r.go(1)
suspend fun renderLater(r: SuspendRender) = r.go(2)
fun combine(c: Combine) = c.of(3L, 1.5)
fun sink(s: Sink) = s.put("hi")
fun same(a: Render, b: Render) = a == b && a.hashCode() == b.hashCode()
fun <T> twice(op: Op<T>, x: T) = op.ap(op.ap(x))
suspend fun <T> emitTo(c: FlowCollector<T>, v: T) = c.emit(v)

class Holder {
    fun go(f: (Int) -> String) = render(f)
}

suspend fun forEach(flow: Flow<Int>, handler: suspend (Int) -> Unit): Unit = flow.collect(handler)

fun main() {
    val f: (Int) -> String = { "n$it" }
    println(render(f))
    println(Holder().go(f))
    val g: suspend (Int) -> String = { "s$it" }
    println(runBlocking { renderLater(g) })
    val c: (Long, Double) -> Int = { x, y -> (x * y).toInt() }
    println(combine(c))
    val s: (String) -> Unit = { println("put $it") }
    sink(s)
    println(same(f, f))
    runBlocking { forEach(flowOf(1, 2)) { println("each $it") } }
    val triple: (Int) -> Int = { it * 3 }
    println(twice(triple, 2))
    val got: suspend (String) -> Unit = { println("got $it") }
    runBlocking { emitTo(got, "x") }
    // Last: kotlinc numbers this lambda among the suspend lambdas, kc
    // doesn't.
    println(same(f) { "other" })
}
