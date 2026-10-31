// What compiling kotlinx.collections.immutable needed: bare types in is
// and as, an extension that a projected receiver's member leaves to,
// super calls, annotated blocks, progressions' step, arrays' iterator(),
// delegated collections and the members only the JVM's Map declares.


interface PMap<K, out V> : Map<K, V> {
    fun putting(key: K, value: @UnsafeVariance V): PMap<K, V>
    fun puttingAll(m: Map<out K, @UnsafeVariance V>): PMap<K, V> {
        var r: PMap<K, V> = this
        for ((k, v) in m) r = r.putting(k, v)
        return r
    }
}

// A member doesn't take the arguments on a receiver projected out K: the
// extension does.
fun <K, V> PMap<out K, V>.puttingAll(map: Map<out K, V>): PMap<K, V> = (this as PMap<K, V>).puttingAll(map)
fun <K, V> PMap<out K, V>.plus(map: Map<out K, V>): PMap<K, V> = puttingAll(map)

class SimpleMap<K, V>(private val m: Map<K, V>) : PMap<K, V>, Map<K, V> by m {
    override fun putting(key: K, value: V): PMap<K, V> = SimpleMap(m + (key to value))
    override fun toString() = m.toString()
}

class Buffer<out T>(private val items: Array<T>) {

    fun first(): T = items[0]
}

fun <E> sizeOf(xs: Iterable<E>): Int = if (xs is Collection) xs.size else xs.count()
fun <T> Iterable<T>.asList(): List<T>? = this as? List

interface Named<T> { fun name(): String = "named" }

abstract class Base<T> : Named<T> {
    override fun name() = "base " + super<Named>.name()
}

interface Built<E> : MutableSet<E> { fun built(): Set<E> }

class Builder<E> : AbstractMutableSet<E>(), Built<E> {
    private val inner = LinkedHashSet<E>()
    override val size get() = inner.size
    override fun iterator() = inner.iterator()
    override fun add(element: E) = inner.add(element)
    override fun addAll(elements: Collection<E>): Boolean = super.addAll(elements)
    override fun built(): Set<E> = inner
}

fun rootSize(n: Int) = n * 10

class Vector(val size: Int) {
    private fun rootSize(): Int = rootSize(size) // the top-level function
    fun root() = rootSize()
}

class Counts<K, V> : AbstractMutableMap<K, V>() {
    private val m = LinkedHashMap<K, V>()
    override val entries get() = m.entries
    override fun put(key: K, value: V): V? = m.put(key, value)
    fun remove(key: K, value: V): Boolean = m.remove(key, value)
}

class Node<K, V>(val keys: Array<Any?>) {
    fun update(x: Int): Node<K, V> {
        val n = when {
            x == 0 -> this
            else -> {
                val buf = arrayOfNulls<Any>(x)
                Node(buf)
            }
        }
        return n
    }
}

fun allPositive(xs: List<Any>): Boolean {
    xs.forEach { x ->
        when {
            x is Int -> @Suppress("UNCHECKED_CAST") {
                x > 0 || return false
            }
            else -> return false
        }
    }
    return true
}

interface Keys<K> : Set<K>
class KeySet<K>(s: Set<K>) : Keys<K>, Set<K> by s

class View<K, V>(private val m: Map<K, V>) : Map<K, V> by m {
    override val keys: Keys<K> = KeySet(m.keys)
}

fun main() {
    val pm = SimpleMap(mapOf(1 to "a"))
    println(pm.plus(mapOf(2 to "b")))
    println(Buffer(arrayOf("x", "y")).first())
    println(sizeOf(listOf(1, 2, 3)).toString() + " " + sizeOf(sequenceOf(1).asIterable()))
    println(setOf(1).asList())
    println(object : Base<Int>() {}.name())
    val b = Builder<Int>()
    b.addAll(listOf(3, 1, 3))
    println(b.built())
    println(Vector(4).root())
    val c = Counts<String, Int>()
    c.put("a", 1)
    println(c.remove("a", 2).toString() + " " + c.remove("a", 1) + " " + c.size)
    println(Node<String, Int>(arrayOf()).update(2).keys.size)
    println(allPositive(listOf(1, 2)).toString() + " " + allPositive(listOf(1, -2)) + " " + allPositive(listOf("x")))
    val arr = arrayOf(10, 20, 30, 40, 50)
    for (i in arr.indices step 2) print(arr[i])
    println()
    val it = arr.iterator()
    var sum = 0
    while (it.hasNext()) sum += it.next()
    val ii = intArrayOf(1, 2).iterator()
    while (ii.hasNext()) sum += ii.nextInt()
    println(sum)
    val v = View(mapOf("k" to 1))
    println(v.size.toString() + " " + v.keys.size + " " + v.getOrDefault("z", 9) + " " + v.entries)
}
