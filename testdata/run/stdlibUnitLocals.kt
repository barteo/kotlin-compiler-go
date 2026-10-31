// Locals of type Unit, inferred from a call that returns Unit, as a test's
// val response = subject.open(id) followed by assertThat(response)
// .isEqualTo(Unit). They have no storage: their value is the Unit object.
import kotlin.coroutines.*

fun act(): Unit {
    println("act")
}

fun <T> same(a: T, b: Any?) = a == b

suspend fun go() = act()

fun run(block: suspend () -> Unit) {
    block.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
}

fun main() {
    val r = act()
    println(r)
    println(r == Unit)
    println(same(r, Unit))
    println(listOf(r, r))
    var u = act()
    u = act()
    println(u)
    val f = { println(r); r }
    println(f())
    val g: () -> Any = { u }
    println(g())
    val h = { u = act() }
    h()
    val x: Unit = act()
    val y: Any = x
    println(y)
    val z = if (r == Unit) act() else act()
    println(z.hashCode() == Unit.hashCode())
    val first = listOf(Unit).first()
    println(first)
    val m = mapOf(1 to act())
    val got = m.getValue(1)
    println(got)
    run {
        val s = go()
        println(s)
    }
}
