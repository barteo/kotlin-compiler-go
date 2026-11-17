// A lambda taking a builder's variable, next to the builder's lambda,
// calls a constructor whose overloads differ in the parameters left to
// their defaults: the builder's lambda gives the variable, and the key's
// type then chooses the constructor.

class Entry<T : Any>(val key: T, val id: Any = key.toString(), val content: (T) -> Unit) {
    constructor(entry: Entry<T>, content: (T) -> Unit) : this(entry.key, entry.id, content)
}

class Scope<T : Any> {
    val names = ArrayList<String>()
}

inline fun <T : Any> provider(noinline fallback: (unknown: T) -> Entry<T>, builder: Scope<T>.() -> Unit): (T) -> Entry<T> {
    println(Scope<T>().apply(builder).names)
    return fallback
}

interface Route

object Home : Route {
    override fun toString() = "Home"
}

fun use(installers: List<Scope<Route>.() -> Unit>) {
    val p = provider(fallback = { unknown -> Entry(unknown) { println("no entry for $it") } }) {
        installers.forEach { install -> install() }
    }
    val e = p(Home)
    e.content(e.key)
    println(e.id)
}

fun main() {
    use(listOf({ names.add("installed") }))
}
