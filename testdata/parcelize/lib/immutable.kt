// A stub of kotlinx.collections.immutable's interfaces and of the
// extensions that the parcelize plugin reads them with.
@file:JvmName("ExtensionsKt")

package kotlinx.collections.immutable

interface ImmutableList<out E> : List<E>
interface PersistentList<out E> : ImmutableList<E>
interface ImmutableSet<out E> : Set<E>
interface PersistentSet<out E> : ImmutableSet<E>
interface ImmutableMap<K, out V> : Map<K, V>
interface PersistentMap<K, out V> : ImmutableMap<K, V>

private class StubList<E>(val l: List<E>) : PersistentList<E>, List<E> by l {
    override fun toString() = "persistent$l"
    override fun equals(other: Any?) = l == other
    override fun hashCode() = l.hashCode()
}

private class StubSet<E>(val s: Set<E>) : PersistentSet<E>, Set<E> by s {
    override fun toString() = "persistent$s"
    override fun equals(other: Any?) = s == other
    override fun hashCode() = s.hashCode()
}

private class StubMap<K, V>(val m: Map<K, V>) : PersistentMap<K, V>, Map<K, V> by m {
    override fun toString() = "persistent$m"
    override fun equals(other: Any?) = m == other
    override fun hashCode() = m.hashCode()
}

fun <T> Iterable<T>.toPersistentList(): PersistentList<T> = StubList(toList())
fun <T> Iterable<T>.toPersistentSet(): PersistentSet<T> = StubSet(toSet())
fun <K, V> Map<out K, V>.toPersistentMap(): PersistentMap<K, V> = StubMap(toMap())
fun <T> persistentListOf(vararg elements: T): PersistentList<T> = elements.asIterable().toPersistentList()
fun <T> persistentSetOf(vararg elements: T): PersistentSet<T> = elements.asIterable().toPersistentSet()
fun <K, V> persistentMapOf(vararg pairs: Pair<K, V>): PersistentMap<K, V> = pairs.toMap().toPersistentMap()
