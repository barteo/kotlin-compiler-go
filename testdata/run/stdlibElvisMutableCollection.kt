fun <T> build(block: MutableList<T>.() -> Unit): List<T> = mutableListOf<T>().apply(block)

fun sizes(m: java.util.HashMap<String, String>?): List<Int> = build {
    val values = m?.values ?: emptyList()
    add(values.size)
    val keys = m?.keys ?: emptySet()
    add(keys.size)
    val list: MutableList<String>? = m?.keys?.toMutableList()
    val all = list ?: mutableListOf()
    all.add("z")
    add(all.size)
}

fun main() {
    val m = java.util.HashMap<String, String>()
    m["a"] = "1"
    println(sizes(m))
    println(sizes(null))
    val c: Collection<String> = java.util.Collections.unmodifiableCollection(listOf("x")) ?: emptyList()
    println(c)
}
