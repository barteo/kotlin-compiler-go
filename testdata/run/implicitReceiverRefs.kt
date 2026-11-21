// A reference without a receiver to a generic extension of an implicit
// receiver takes its type arguments from the receiver, as K2 infers them:
// K := String of ::bar in a Collector<String>'s code, and in a builder's
// lambda, whose receiver's variable the builder then infers.

interface Collector<in T> {
    fun name(): String
}

fun <K> Collector<K>.bar(): String = "bar:" + name()
fun <K> Collector<K>.baz(): String = "baz:" + name()
fun <K> K.self(): K = this

fun <R> select(vararg x: R): R = x[0]

fun Collector<String>.refs(): String {
    val inv = select(::bar, ::baz)
    val s = select(::self, ::self)
    return inv() + "," + s().name()
}

class Flow<out R>(val block: Collector<R>.() -> String)

fun <L> flow(block: Collector<L>.() -> String): Flow<L> = Flow(block)

fun poll(): Flow<String> = flow {
    val inv = select(::bar, ::baz)
    inv()
}

fun main() {
    val c = object : Collector<String> {
        override fun name() = "c"
    }
    println(c.refs())
    println(poll().block(c))
}
