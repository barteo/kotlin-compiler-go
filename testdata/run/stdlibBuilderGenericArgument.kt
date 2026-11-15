fun <T> one(t: T): List<T> = listOf(t)

data class Event(val name: String, val args: List<String>)

fun main() {
    val nested = buildMap { put("a", one("b")) }
    println(nested)
    val lists = buildMap { put("a", listOf("b", "c")) }
    println(lists)
    val events = buildMap {
        put("onCreate", listOf(Event("fill", listOf("distance"))))
    }
    println(events)
    val sets = buildMap {
        put(1, setOf(one(2)))
        put(2, emptySet())
    }
    println(sets)
}
