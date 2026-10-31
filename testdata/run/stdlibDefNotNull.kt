// Definitely non-nullable types: T & Any.

interface ColumnType<T> {
    fun valueToString(value: T & Any): String
    fun nonNull(value: T): T & Any = value ?: error("null value")
}

class StringColumn : ColumnType<String?> {
    override fun valueToString(value: String) = "'$value'"
}

class Column<T>(val type: ColumnType<T>) {
    fun render(v: T): String = if (v == null) "NULL" else type.valueToString(v)
}

fun <T> T.orThrow(): T & Any = this ?: throw IllegalStateException("was null")

fun <T> firstNonNull(xs: List<T>): T & Any {
    for (x in xs) if (x != null) return x
    throw NoSuchElementException()
}

fun <T> lengthOf(x: T & Any): Int = x.toString().length


class Cache<K, V> {
    private val map = HashMap<K & Any, V & Any>()
    fun put(k: K & Any, v: V & Any) { map[k] = v }
    fun get(k: K & Any): V? = map[k]
    fun keys(): List<K & Any> = map.keys.toList()
}

fun main() {
    val col = Column(StringColumn())
    println(col.render("abc"))
    println(col.render(null))
    println(StringColumn().nonNull("x").length)
    val s: String? = "hi"
    println(s.orThrow().length)
    try {
        val n: Int? = null
        n.orThrow()
    } catch (e: IllegalStateException) {
        println(e.message)
    }
    println(firstNonNull(listOf(null, 3, 4)) + 1)
    println(lengthOf<String?>("four"))
    val c = Cache<String?, Int?>()
    c.put("a", 1)
    c.put("b", 2)
    println(c.get("a"))
    println(c.keys().map { it.length })
}
