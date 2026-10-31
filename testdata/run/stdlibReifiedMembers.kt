// Inline member functions with reified type parameters, which are copied
// into their callers with the call's receiver as this, as kotlinc does.

class Registry {
    private val items = mutableMapOf<String, Any>()
    var lookups = 0

    fun register(key: String, value: Any) {
        items[key] = value
    }

    inline fun <reified T> get(key: String): T? {
        lookups++
        val v = find(key)
        return if (v is T) v else null
    }

    inline fun <reified T : Any> all(): List<T> = values().filterIsInstance<T>()

    inline fun <reified T> describe(): String = T::class.java.simpleName + "@" + lookups

    inline fun <R> withLookups(f: (Int) -> R): R = f(lookups)

    fun find(key: String): Any? = items[key]
    fun values(): Collection<Any> = items.values

    fun selfCheck(): String? = get<String>("name")
}

class Scope(val registry: Registry) {
    inline fun <reified T> inject(key: String): T = registry.get<T>(key) ?: error("no " + T::class.java.simpleName)
}

object Module {
    inline fun <reified T> typeName(): String = T::class.java.name
}

fun main() {
    val r = Registry()
    r.register("name", "kc")
    r.register("count", 3)
    r.register("ratio", 0.5)
    println(r.get<String>("name"))
    println(r.get<Int>("name"))
    println(r.get<Int>("count"))
    println(r.all<Number>())
    println(r.describe<String>())
    println(r.withLookups { it * 10 })
    println(r.selfCheck() + " " + r.lookups)
    val s = Scope(r)
    println(s.inject<Double>("ratio"))
    try {
        s.inject<Long>("count")
    } catch (e: IllegalStateException) {
        println(e.message)
    }
    println(Module.typeName<Registry>())
    val maybe: Registry? = null
    println(maybe?.get<String>("name"))
}
