package d

import a.Marker

@Marker
abstract class Repo<K : Comparable<K>, out V>(val items: Map<K, V>) : Iterable<V> {
    @Marker
    abstract fun find(key: K, vararg more: K): V?

    @Marker
    fun <R> map(f: (V) -> R, g: suspend String.(Int) -> Unit): List<R> = items.values.map(f)

    @Marker
    val size get() = items.size

    @Marker
    val String.twice: String get() = this + this

    @Marker
    fun star(xs: List<*>, ys: Array<in String>): Unit {}
}

@Marker
fun <T> nn(t: T & Any): T = t

// TODO: kotlinc infers String!, which KSP prints as kotlin.String; kc infers String?.
@Marker
fun platform() = System.getProperty("x")

@Marker
val inferred = listOf(1 to "a")

@Marker
fun broken(): Missing = TODO()
