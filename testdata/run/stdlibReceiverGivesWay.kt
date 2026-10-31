// An implicit receiver's function that the arguments don't fit gives way
// to a top-level one: run(listOf(...)) in a class or a lambda with a
// receiver calls the package's run(List<String>), not the standard
// library's T.run(block). With a lambda, run { } is still this.run { }.

fun run(names: List<String>): String = names.joinToString("+")

fun let(n: Int): Int = n * 10

class Greeter(val prefix: String) {
    fun all() = run(listOf("a", "b"))
    fun scaled() = let(4)
    fun block() = run { prefix + "!" }
    fun mapped() = let { it.prefix.length }
}

fun withReceiver(): String = with(StringBuilder("x")) {
    append(run(listOf("c", "d")))
    toString()
}

fun main() {
    val g = Greeter("hi")
    println(g.all())
    println(g.scaled())
    println(g.block())
    println(g.mapped())
    println(withReceiver())
    println(run(listOf("top")))
}
