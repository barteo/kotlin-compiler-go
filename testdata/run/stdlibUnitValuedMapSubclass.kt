class Seen(private val max: Int) : LinkedHashMap<String, Unit>(4, 0.75f, true) {
    override fun removeEldestEntry(eldest: MutableMap.MutableEntry<String, Unit>) = size > max
}

fun firstSeen(seen: MutableMap<String, Unit>, id: String): Boolean {
    if (seen.containsKey(id)) return false
    seen[id] = Unit
    return true
}

fun main() {
    val seen = Seen(2)
    println(firstSeen(seen, "a"))
    println(firstSeen(seen, "a"))
    firstSeen(seen, "b")
    firstSeen(seen, "c")
    println(seen.keys)
    println(seen["c"])
    println(seen.get("z"))
    val any: Map<Any, Any?> = @Suppress("UNCHECKED_CAST") (seen as Map<Any, Any?>)
    println(any[1])
    println(seen.remove("b"))
    println(seen)
}
