package defs

typealias Handler = (String) -> Int
typealias Pred<T> = (T) -> Boolean
typealias Builder<T> = StringBuilder.(T) -> Unit
typealias Registry = HashMap<String, Handler>
typealias StrMap<V> = Map<String, V>
typealias Pairs<A> = List<Pair<A, A>>

class Box<T>(val v: T) {
    typealias Mapper = (Int) -> Int
    fun <R> map(f: (T) -> R): Box<R> = Box(f(v))
}

typealias IntBox = Box<Int>

fun apply(h: Handler, s: String): Int = h(s)
fun <T> filter(xs: List<T>, p: Pred<T>): List<T> = xs.filter(p)
fun <T> build(t: T, b: Builder<T>): String = StringBuilder().apply { b(t) }.toString()
fun registry(): Registry = HashMap()
fun twice(m: Box.Mapper, x: Int) = m(m(x))
