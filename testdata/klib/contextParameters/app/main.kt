import kotlin.coroutines.Continuation
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.startCoroutine
import lib.*

class N(m: String) : M(m) {
    context(c: Ctx)
    override fun describe(): String = "N " + super.describe()
}

class Square(val side: Int) : Shape<Ctx>() {
    context(t: Ctx)
    override fun area(): Int = side * side + t.n.length
}

fun main() {
    println(over + " " + P(1).w)
    with(Ctx("x")) {
        println(P(2).v)
        println(f(false) + " " + f(true) + " " + f(true, "given"))
        println("s".ext())
        println(top + " " + 3.extProp)
        topVar = "a"
        topVar += "b"
        println(topVar)
        val m: M = N("n")
        println(m.mem + " " + m.describe())
        m.memVar = "c"
        m.memVar += "d"
        println(log)
        with(m) { println(4.memExt) }
        println(withLog { "inline" } + " " + log)
        val shape: Shape<Ctx> = Square(3)
        println(shape.area())
        val block: suspend () -> Unit = { println(fetch(1)) }
        block.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
    }
}
