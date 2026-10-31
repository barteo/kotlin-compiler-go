// Classes that implement Kotlin's read-only collection types, used from
// Kotlin and from Java's collections, which call the bridges, stubs and
// toArray kotlinc writes for them.

class Words(private val ws: Array<String>) : List<String> {
    override val size: Int get() = ws.size
    override fun get(index: Int): String = ws[index]
    override fun isEmpty(): Boolean = ws.isEmpty()
    override fun contains(element: String): Boolean = ws.contains(element)
    override fun containsAll(elements: Collection<String>): Boolean = elements.all { contains(it) }
    override fun indexOf(element: String): Int = ws.indexOf(element)
    override fun lastIndexOf(element: String): Int = ws.lastIndexOf(element)
    override fun iterator(): Iterator<String> = ws.toList().iterator()
    override fun listIterator(): ListIterator<String> = ws.toList().listIterator()
    override fun listIterator(index: Int): ListIterator<String> = ws.toList().listIterator(index)
    override fun subList(fromIndex: Int, toIndex: Int): List<String> = ws.toList().subList(fromIndex, toIndex)
}

class Evens(val n: Int) : AbstractList<Int>() {
    override val size: Int get() = n
    override fun get(index: Int): Int = index * 2
}

class Countdown(val from: Int) : Iterable<Int> {
    override fun iterator(): Iterator<Int> = object : Iterator<Int> {
        var i = from
        override fun hasNext() = i > 0
        override fun next() = i--
    }
}

class Pair2(override val key: String, override val value: Int) : Map.Entry<String, Int>

class Ages : AbstractMap<String, Int>() {
    override val entries: Set<Map.Entry<String, Int>> = setOf(Pair2("ann", 30), Pair2("bob", 25))
}

class Vowels : AbstractSet<Char>() {
    private val vs = "aeiou"
    override val size: Int get() = vs.length
    override fun iterator(): Iterator<Char> = vs.toList().iterator()
}

class Grid<T>(private val cells: List<T>) : Collection<T> {
    override val size get() = cells.size
    override fun isEmpty() = cells.isEmpty()
    override fun contains(element: T) = cells.contains(element)
    override fun containsAll(elements: Collection<T>) = cells.containsAll(elements)
    override fun iterator() = cells.iterator()
}

fun attempt(name: String, f: () -> Any?) {
    try {
        println(name + ": " + f())
    } catch (e: UnsupportedOperationException) {
        println(name + ": " + e.message)
    } catch (e: java.lang.reflect.InvocationTargetException) {
        println(name + ": " + e.cause?.message)
    }
}

// call calls a method by reflection, as Java code would.
fun call(x: Any, name: String, vararg args: Any?): Any? {
    val m = x.javaClass.methods.first { it.name == name && it.parameterCount == args.size }
    return m.invoke(x, *args)
}

fun main() {
    val w = Words(arrayOf("a", "b", "c", "b"))
    println(w.size.toString() + " " + w[1] + " " + w.indexOf("b") + " " + w.lastIndexOf("b"))
    println(w.joinToString("-"))
    println(ArrayList(w))
    println(java.util.Collections.unmodifiableList(w).size)
    val any: Collection<Any?> = w
    println(any.contains("c").toString() + " " + any.contains(3) + " " + any.contains(null))
    println((w as List<Any?>).indexOf(7))
    println(w.toTypedArray().size)
    println(w.stream().filter { it != "b" }.count())
    attempt("add") { call(w, "add", "d") }
    attempt("sort") { call(w, "sort", null) }
    attempt("clear") { call(w, "clear") }

    val e = Evens(5)
    println(e.toList())
    println(e.contains(4).toString() + " " + e.indexOf(6) + " " + (e as List<Any?>).contains("x"))
    println(java.util.Collections.frequency(e, 8))
    println(e.sum())

    println(Countdown(3).toList())
    println(Countdown(4).map { it * it })
    attempt("remove") { call(Countdown(1).iterator(), "remove") }

    val p = Pair2("k", 1)
    println(p.key + "=" + p.value)
    attempt("setValue") { call(p, "setValue", 2) }

    val ages = Ages()
    println(ages["ann"].toString() + " " + ages.size + " " + ages.keys + " " + ages.values)
    println(ages.containsKey("bob").toString() + " " + (ages as Map<Any?, Any?>).containsKey(5) + " " + ages.getOrDefault("zed", 0))
    val copy = HashMap<String, Int>()
    copy.putAll(ages)
    println(copy.toSortedMap())

    val v = Vowels()
    println(v.contains('e').toString() + " " + (v as Set<Any?>).contains("e") + " " + v.size)
    println(v.sorted().joinToString(""))

    val g = Grid(listOf(1, 2, 3))
    println(g.contains(2).toString() + " " + g.size + " " + g.toList() + " " + java.util.ArrayList(g))
    attempt("removeIf") { call(g, "removeIf", java.util.function.Predicate<Int> { it > 1 }) }
}
