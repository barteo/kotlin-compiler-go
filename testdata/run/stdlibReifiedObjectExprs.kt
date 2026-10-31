// Object expressions in inline functions with reified type parameters,
// copied from source: each copy creates an object of a copy of the class,
// with the call's type arguments. testdata/lib/reifiedObjects copies them
// from bytecode.

interface Check {
    fun test(x: Any?): Boolean
    fun name(): String
}

inline fun <reified T> typeCheck(): Check = object : Check {
    override fun test(x: Any?) = x is T
    override fun name() = T::class.simpleName ?: "?"
}

inline fun <reified T> tagged(tag: String): Check {
    var calls = 0
    return object : Check {
        override fun test(x: Any?): Boolean {
            calls++
            return x is T
        }
        override fun name() = tag + ":" + T::class.simpleName + ":" + calls
    }
}

inline fun <reified A, reified B> either(): Check = object : Check {
    override fun test(x: Any?) = typeCheck<A>().test(x) || typeCheck<B>().test(x)
    override fun name() = typeCheck<A>().name() + "|" + typeCheck<B>().name()
}

inline fun <reified T> filtered(xs: List<Any?>, crossinline keep: (T) -> Boolean): List<T> {
    val c = object {
        fun accept(x: Any?) = x is T && keep(x)
    }
    return xs.filter { c.accept(it) }.map { it as T }
}

class Box(val label: String) {
    inline fun <reified T> checker(): Check = object : Check {
        override fun test(x: Any?) = x is T
        override fun name() = label + "/" + T::class.simpleName
    }
}

inline fun <reified T> twice(): String = typeCheck<List<T>>().name() + either<T, String>().name()

fun main() {
    val xs = listOf(1, "a", 2.5, null, 'c', "b")
    val ints = typeCheck<Int>()
    val strings = typeCheck<String>()
    println(ints.name() + " " + xs.count { ints.test(it) })
    println(strings.name() + " " + xs.count { strings.test(it) })
    val t = tagged<Double>("t")
    xs.forEach { t.test(it) }
    println(t.name())
    val e = either<Char, Double>()
    println(e.name() + " " + xs.count { e.test(it) })
    println(filtered<String>(xs) { it < "b" })
    println(Box("box").checker<Number>().let { it.name() + " " + xs.count(it::test) })
    println(twice<Int>())
}
