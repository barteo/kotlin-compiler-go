import defs.catching
import defs.catcher
import defs.evalCatch

fun main() {
    println(catching<IllegalStateException> { throw IllegalStateException("a") })
    println(catching<IllegalStateException> { })
    println(runCatching { catching<IllegalStateException> { throw IllegalArgumentException("b") } }.exceptionOrNull()?.message)
    println(evalCatch<IllegalArgumentException> { throw IllegalArgumentException("c") })
    println(evalCatch<IllegalArgumentException> { throw IllegalStateException("d") })
    println(evalCatch<RuntimeException> { throw IllegalStateException("e") })
    val c = catcher<IllegalStateException>()
    println(c { throw IllegalStateException("f") })
    println(runCatching { c { throw IllegalArgumentException("g") } }.exceptionOrNull()?.message)
}
