class Key<T>(val name: String)

class Store {
    private val map = mutableMapOf<Key<*>, Any?>()

    operator fun <T> set(key: Key<T>, value: T) {
        map[key] = value
    }

    @Suppress("UNCHECKED_CAST")
    fun <T> remove(key: Key<T>): T = map.remove(key) as T
}

fun intKey(n: Int) = Key<Int>("k$n")

fun edit(store: Store, block: (Store) -> Unit) = block(store)

fun attempt(name: String, block: () -> Unit) {
    try {
        block()
        println("$name ok")
    } catch (e: NullPointerException) {
        println("$name null")
    }
}

fun main() {
    val store = Store()
    store[intKey(0)] = 1
    attempt("statement") { store.remove(intKey(0)); println("removed") }
    attempt("last statement") { store.remove(intKey(1)) }
    attempt("forEach") { (0 until 3).forEach { store.remove(intKey(it)) } }
    attempt("nested") { edit(store) { s -> listOf(1, 2).forEach { s.remove(intKey(it)) } } }
    attempt("it") { edit(store) { it.remove(intKey(5)) } }
    attempt("for") { for (i in 0..1) store.remove(Key<Long>("l$i")) }
    attempt("function value") {
        val f: (Int) -> Unit = { store.remove(Key<Boolean>("b$it")) }
        f(1)
    }
    attempt("also") { store.also { it.remove(Key<Double>("d")) } }
    attempt("apply") { store.apply { remove(Key<Char>("c")) } }
    attempt("if") { if (store.hashCode() != 0) store.remove(intKey(10)) else println() }
    attempt("try") {
        try {
            store.remove(intKey(12))
        } catch (e: IllegalStateException) {
        }
    }
    attempt("used") { println(store.remove(intKey(15)) + 1) }
    attempt("nullable") { println(store.run { remove(Key<Int?>("n")) }) }
}
