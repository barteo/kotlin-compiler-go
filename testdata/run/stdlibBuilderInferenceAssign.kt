// Builder inference (PCLA) through assignments and non-null receivers:
// a generic call assigned to a property of the builder's type variable
// is inferred with it, and x!! of a variable of that type, also after
// emit(null), is non-null.
fun <E> execute(action: () -> E): E = action()

@Suppress("UNCHECKED_CAST")
fun <T> materialize(): T = "O" as T

interface Owner<P> {
    var producer1: () -> P
    var producer2: () -> P
    var value: P
}

fun <T> pcla(lambda: (Owner<T>) -> Unit): String {
    val o = object : Owner<T> {
        override var producer1: () -> T = { null!! }
        override var producer2: () -> T = { null!! }
        @Suppress("UNCHECKED_CAST")
        override var value: T = "" as T
    }
    lambda(o)
    return "" + o.producer1() + o.producer2() + o.value
}

interface Box<R> {
    fun emit(r: R)
    fun get(): R
}

fun <R> build(block: Box<R>.() -> Unit): List<R> {
    val items = mutableListOf<R>()
    object : Box<R> {
        override fun emit(r: R) { items += r }
        override fun get(): R = items.last()
    }.block()
    return items
}

fun main() {
    println(pcla { o ->
        o.producer1 = { materialize() }
        o.producer2 = { execute { "K" } }
    })
    println(pcla { o ->
        o.value = materialize()
        o.producer1 = { "A" }
        o.producer2 = { "B" }
    })
    println(build {
        emit(null)
        emit(1)
        val x = get()
        println(x!!.hashCode())
        if (x != null) println(x.toString())
    })
}
