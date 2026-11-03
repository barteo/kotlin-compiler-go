import kotlin.reflect.KProperty

interface Codec<T> {
    fun encode(value: T): String
    fun decode(): T?
}

fun <T> codec(encode: (T) -> String, decode: () -> T?): Codec<T> = object : Codec<T> {
    override fun encode(value: T) = encode(value)
    override fun decode(): T? = decode()
}

fun interface ReadOnly<in T, out V> {
    operator fun getValue(thisRef: T, property: KProperty<*>): V
}

fun interface Foo<A, B> {
    fun invoke(a: A): B
}

typealias TA<K> = Foo<Int, K>

class Bar<X, Y>(val y: Y) {
    fun foo(x: X): Y = y
}

typealias TB<M> = Bar<Int, M>

fun main() {
    // The second lambda gives T, which the first's it has.
    val c = codec({ it.length.toString() }, { "abc" })
    println(c.encode(c.decode()!!))
    val name by ReadOnly { _, p -> p.name }
    println(name)
    println(TA { "s$it" }.invoke(20) + TB("3").foo(3))
}
