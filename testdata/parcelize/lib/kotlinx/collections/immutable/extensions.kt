package kotlinx.collections.immutable

fun <E> persistentListOf(vararg elements: E): PersistentList<E> = ListOf(elements.toList())
fun <E> persistentSetOf(vararg elements: E): PersistentSet<E> = SetOf(elements.toSet())
fun <K, V> persistentMapOf(vararg pairs: Pair<K, V>): PersistentMap<K, V> = MapOf(pairs.toMap())

fun <E> Iterable<E>.toPersistentList(): PersistentList<E> = this as? PersistentList<E> ?: persistentListOf<E>().addAll(toList())
fun <E> Iterable<E>.toPersistentSet(): PersistentSet<E> = this as? PersistentSet<E> ?: persistentSetOf<E>().addAll(toSet())
fun <K, V> Map<K, V>.toPersistentMap(): PersistentMap<K, V> = this as? PersistentMap<K, V> ?: persistentMapOf<K, V>().putAll(this)
