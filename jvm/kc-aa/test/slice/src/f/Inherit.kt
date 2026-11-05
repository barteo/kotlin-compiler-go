package f

interface Source<T> {
    fun get(): T
    val size: Int
    fun <R> map(f: (T) -> R): Source<R>
}

interface Named {
    fun name(): String
    fun get(): String
}

abstract class Base<T>(val items: List<T>) : Source<T> {
    override val size get() = items.size
    open fun first(): T = items[0]
    protected abstract fun hook()
    private fun secret() {}
}

class Strings : Base<String>(listOf()), Named {
    override fun get() = items[0]
    override fun name() = "s"
    override fun hook() {}
}

sealed interface Shape
class Circle : Shape
object Square : Shape

class Bag : java.util.AbstractList<String>() {
    override val size get() = 0
    override fun get(index: Int) = ""
}

/** Pairs of types, target then source, for isAssignableFrom. */
class Assign(
    val sourceString: Source<String>, val strings: Strings,
    val sourceAny: Source<Any>, val strings2: Strings,
    val listAny: List<Any>, val listString: List<String>,
    val comparableString: Comparable<String>, val string: String,
    val any: Any, val nullableString: String?,
    val collectionString: Collection<String>, val listString2: List<String>,
    val named: Named, val base: Base<String>,
)
