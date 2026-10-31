// Classes that implement Kotlin's mutable collection types, directly or
// through the standard library's AbstractMutableList and the like, used
// from Kotlin and from Java's collections.

class Buffer<E> : AbstractMutableList<E>() {
    private var items = arrayOfNulls<Any?>(2)
    override var size = 0
        private set

    @Suppress("UNCHECKED_CAST")
    override fun get(index: Int): E {
        if (index < 0 || index >= size) throw IndexOutOfBoundsException("index $index")
        return items[index] as E
    }

    override fun add(index: Int, element: E) {
        if (size == items.size) items = items.copyOf(size * 2)
        for (i in size downTo index + 1) items[i] = items[i - 1]
        items[index] = element
        size++
        modCount++
    }

    @Suppress("UNCHECKED_CAST")
    override fun removeAt(index: Int): E {
        val e = get(index)
        for (i in index until size - 1) items[i] = items[i + 1]
        size--
        modCount++
        return e
    }

    @Suppress("UNCHECKED_CAST")
    override fun set(index: Int, element: E): E {
        val old = get(index)
        items[index] = element
        return old
    }
}

class Ints : AbstractMutableList<Int>() {
    private val xs = ArrayList<Int>()
    override val size: Int get() = xs.size
    override fun get(index: Int): Int = xs[index]
    override fun add(index: Int, element: Int) = xs.add(index, element)
    override fun removeAt(index: Int): Int = xs.removeAt(index)
    override fun set(index: Int, element: Int): Int = xs.set(index, element)
}

class Registry<K, V> : AbstractMutableMap<K, V>() {
    private val m = LinkedHashMap<K, V>()
    override val entries: MutableSet<MutableMap.MutableEntry<K, V>> get() = m.entries
    override fun put(key: K, value: V): V? = m.put(key, value)
}

class Tags : AbstractMutableSet<String>() {
    private val s = LinkedHashSet<String>()
    override val size get() = s.size
    override fun iterator() = s.iterator()
    override fun add(element: String) = s.add(element.lowercase())
}

class Counter(private val names: MutableList<String>) : MutableIterable<String> {
    override fun iterator(): MutableIterator<String> = object : MutableIterator<String> {
        var i = 0
        override fun hasNext() = i < names.size
        override fun next() = names[i++]
        override fun remove() {
            names.removeAt(--i)
        }
    }
}

fun main() {
    val b = Buffer<String>()
    b.add("pear")
    b.add("apple")
    b += "fig"
    b.add(0, "kiwi")
    println(b.toString() + " " + b.size)
    java.util.Collections.sort(b)
    println(b)
    println(b.removeAt(1) + " " + b.remove("fig") + " " + b)
    val it = b.iterator()
    it.next()
    it.remove()
    println(b)
    try {
        val i = b.iterator()
        b.add("x")
        i.next()
    } catch (e: java.util.ConcurrentModificationException) {
        println("concurrent modification")
    }

    val ints = Ints()
    ints.addAll(listOf(5, 3, 9, 3))
    println(ints.remove(3).toString() + " " + ints + " " + ints.removeAt(0) + " " + ints)
    println(ints.contains(9).toString() + " " + ints.indexOf(3) + " " + (ints as List<Any?>).contains("9"))
    ints.sortDescending()
    println(ints)

    val r = Registry<String, Int>()
    r["a"] = 1
    r["b"] = 2
    r.getOrPut("c") { 3 }
    println(r.toString() + " " + r.remove("a") + " " + r)
    val copy = HashMap<String, Int>()
    copy.putAll(r)
    println(copy.toSortedMap())
    r.entries.first().setValue(20)
    println(r)

    val t = Tags()
    t.add("Kotlin")
    t.add("KOTLIN")
    t.addAll(listOf("Go", "go"))
    println(t.toString() + " " + t.size + " " + t.contains("go"))

    val names = mutableListOf("x", "y", "z")
    val cnt = Counter(names)
    val ci = cnt.iterator()
    while (ci.hasNext()) if (ci.next() == "y") ci.remove()
    println(names)
}
