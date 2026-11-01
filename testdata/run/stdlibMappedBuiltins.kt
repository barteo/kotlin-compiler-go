// The null handling of the overrides of special collection members whose
// parameters are Objects on the JVM, as kotlinc writes them, and their
// bridges with Java's descriptors.

private object AnyMap : MutableMap<Any, Any> {
    override fun containsKey(key: Any): Boolean = true
    override fun containsValue(value: Any): Boolean = true
    override fun get(key: Any): Any? = "v"
    override fun remove(key: Any): Any? = "r"
    override val size: Int get() = 0
    override fun isEmpty(): Boolean = true
    override fun put(key: Any, value: Any): Any? = null
    override fun putAll(from: Map<out Any, Any>) {}
    override fun clear() {}
    override val entries: MutableSet<MutableMap.MutableEntry<Any, Any>> get() = mutableSetOf()
    override val keys: MutableSet<Any> get() = mutableSetOf()
    override val values: MutableCollection<Any> get() = mutableListOf()
}

private object IntMap : Map<Any, Int> {
    override fun containsKey(key: Any): Boolean = true
    override fun containsValue(value: Int): Boolean = true
    override fun get(key: Any): Int = 1
    override val size: Int get() = 0
    override fun isEmpty(): Boolean = true
    override val entries: Set<Map.Entry<Any, Int>> get() = setOf()
    override val keys: Set<Any> get() = setOf()
    override val values: Collection<Int> get() = listOf()
}

private object AnyList : AbstractList<Any>() {
    override val size: Int get() = 1
    override fun get(index: Int): Any = "x"
    override fun contains(element: Any): Boolean = true
    override fun indexOf(element: Any): Int = 0
    override fun lastIndexOf(element: Any): Int = 0
}

fun main() {
    val m = AnyMap as MutableMap<Any?, Any?>
    println(listOf(m.get(null), m.containsKey(null), m.containsValue(null), m.remove(null)))
    println(listOf(m.get(""), m.containsKey(""), m.containsValue(""), m.remove("")))

    val im = IntMap as Map<Any?, Any?>
    println(listOf(im.get(null), im.containsKey(null), im.containsValue(null), im.containsValue("x")))
    println(listOf(im.get(""), im.containsKey(""), im.containsValue(3)))

    val l = AnyList as List<Any?>
    println(listOf(l.contains(null), l.indexOf(null), l.lastIndexOf(null)))
    println(listOf(l.contains(""), l.indexOf(""), l.lastIndexOf("")))
}
