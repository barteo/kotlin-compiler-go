// A star projection for a bounded type parameter reads as its bound when
// overloads are inferred: Saver<T, *>, for Saver<Original, Saveable : Any>,
// is a Saver<T, out Any>, as Compose's rememberSaveable(saver = ...) takes.

interface Saver<Original, Saveable : Any> {
    fun save(value: Original): Saveable
}

class State(val n: Int)

@JvmName("rememberWithKey")
fun <T : Any> remember(vararg inputs: Any?, saver: Saver<T, out Any>? = null, key: String? = null, init: () -> T): T {
    println("with key ${saver != null} $key ${inputs.size}")
    return init()
}

fun <T : Any> remember(vararg inputs: Any?, saver: Saver<T, out Any>, init: () -> T): T {
    println("saver ${saver.save(init())} ${inputs.size}")
    return init()
}

val stateSaver: Saver<State, *> = object : Saver<State, Int> {
    override fun save(value: State): Int = value.n
}

fun main() {
    println(remember(saver = stateSaver) { State(1) }.n)
    println(remember(1, 2, saver = stateSaver) { State(2) }.n)
    println(remember(key = "k") { State(3) }.n)
}
