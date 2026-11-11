package kotlinx.collections.immutable

// The parts of kotlinx.collections.immutable that the plugin's code for
// its collections uses.
interface ImmutableList<out E> : List<E>

interface PersistentList<out E> : ImmutableList<E> {
    fun addAll(elements: Collection<@UnsafeVariance E>): PersistentList<E>
}

interface ImmutableSet<out E> : Set<E>

interface PersistentSet<out E> : ImmutableSet<E> {
    fun addAll(elements: Collection<@UnsafeVariance E>): PersistentSet<E>
}

interface ImmutableMap<K, out V> : Map<K, V>

interface PersistentMap<K, out V> : ImmutableMap<K, V> {
    fun putAll(m: Map<out K, @UnsafeVariance V>): PersistentMap<K, V>
}

internal class ListOf<E>(private val items: List<E>) : PersistentList<E>, List<E> by items {
    override fun addAll(elements: Collection<E>): PersistentList<E> = ListOf(items + elements)
    override fun equals(other: Any?) = items == other
    override fun hashCode() = items.hashCode()
    override fun toString() = "persistent$items"
}

internal class SetOf<E>(private val items: Set<E>) : PersistentSet<E>, Set<E> by items {
    override fun addAll(elements: Collection<E>): PersistentSet<E> = SetOf(items + elements)
    override fun equals(other: Any?) = items == other
    override fun hashCode() = items.hashCode()
    override fun toString() = "persistent$items"
}

internal class MapOf<K, V>(private val items: Map<K, V>) : PersistentMap<K, V>, Map<K, V> by items {
    override fun putAll(m: Map<out K, V>): PersistentMap<K, V> = MapOf(items + m)
    override fun equals(other: Any?) = items == other
    override fun hashCode() = items.hashCode()
    override fun toString() = "persistent$items"
}
