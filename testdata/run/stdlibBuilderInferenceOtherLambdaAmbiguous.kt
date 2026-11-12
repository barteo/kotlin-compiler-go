class Entry<T>(val key: T, val content: (T) -> String) {
    constructor(entry: Entry<T>, content: (T) -> String) : this(entry.key, content)
}

class Scope<T>(val fallback: (T) -> Entry<T>) {
    val entries = mutableListOf<Entry<T>>()
    fun add(key: T) { entries += Entry(key) { "added $it" } }
}

inline fun <T> provider(noinline fallback: (T) -> Entry<T>, builder: Scope<T>.() -> Unit): (T) -> String {
    val scope = Scope(fallback).apply(builder)
    return { key -> (scope.entries.firstOrNull { it.key == key } ?: scope.fallback(key)).let { it.content(it.key) } }
}

interface Key
data class Named(val name: String) : Key
typealias Installer = Scope<Key>.() -> Unit

fun main() {
    val installers: List<Installer> = listOf({ add(Named("a")) })
    val p = provider(fallback = { unknown -> Entry(unknown) { "unknown $it" } }) {
        installers.forEach { install -> install() }
    }
    println(p(Named("a")))
    println(p(Named("b")))
}
