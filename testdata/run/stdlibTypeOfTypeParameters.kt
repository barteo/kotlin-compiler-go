// typeOf of a type with type parameters that aren't reified: as with
// kotlinc, KTypeParameters with their containers, variance and bounds, of
// a class, a function, an extension property, an inner class and an
// inline function's. A class implementing a read-only collection gets no
// stub of what its interfaces implement.

import kotlin.reflect.*

class Container<T>

fun <X, Y, Z> chain() where X : Y?, Y : List<Z>, Z : Set<String> = typeOf<Container<X>>()

fun <X> multi() where X : CharSequence, X : Comparable<String> = typeOf<Container<X?>>()

class C<X, in I, out O> {
    fun x() = typeOf<Container<X>>()
    fun i() = typeOf<Container<I>>()
    fun <M : X> m() = typeOf<Container<M>>()
    inner class D<Y : X> {
        fun <Z : Y> z() = typeOf<Container<Z>>()
    }
}

val <P> P.prop get() = typeOf<Container<P>>()

inline fun <reified R, Y : R> reified() = typeOf<Container<Y>>()

inline fun <reified T> typeOfValue(x: T) = typeOf<T>()
fun <A> pair(a: A) = typeOfValue(Pair(a, 1))

fun describe(t: KType): String {
    val p = t.arguments.single().type!!.classifier as KTypeParameter
    return "$t ${p.variance} ${p.isReified} ${p.upperBounds}"
}

interface Sized {
    val size: Int
}

interface Items<out T> : Sized, Iterable<T> {
    operator fun get(index: Int): T
    override fun iterator(): Iterator<T> = object : Iterator<T> {
        var i = 0
        override fun hasNext() = i < size
        override fun next() = get(i++)
    }
}

class Letters : Items<Char> {
    override val size get() = 3
    override fun get(index: Int) = 'a' + index
}

fun main() {
    println(describe(chain<MutableList<Set<String>>?, MutableList<Set<String>>, Set<String>>()))
    println(describe(multi<String>()))
    val c = C<Any, Any, Any>()
    println(describe(c.x()))
    println(describe(c.i()))
    println(describe(c.m<Any>()))
    println(describe(c.D<Any>().z<Any>()))
    println(describe("".prop))
    println(describe(reified<Any, Any>()))
    println(pair("a"))
    println(c.x() == c.x())
    println(c.x() == c.m<Any>())
    println(Letters().joinToString())
}
