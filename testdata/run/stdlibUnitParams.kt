// Parameters of type Unit, whose values kc keeps off the stack: in
// lambdas, fun interface lambdas, local and generic functions.
fun interface Sink<T> {
    fun put(value: T)
}

class Box<T>(val value: T)

fun <T> identity(t: T): T = t

fun main() {
    val f: (Unit) -> Unit = { println(it) }
    f(Unit)
    val s = Sink<Unit> { println(it) }
    s.put(Unit)
    fun g(u: Unit) = println(u)
    g(Unit)
    val h = { u: Unit -> u }
    println(h(Unit))
    val keep: (Unit) -> Any = { val v = it; v }
    println(keep(Unit))
    println(listOf(Unit, Unit).map { it }.size)
    println(Box(Unit).value)
    println(identity(Unit) == Unit)
}
