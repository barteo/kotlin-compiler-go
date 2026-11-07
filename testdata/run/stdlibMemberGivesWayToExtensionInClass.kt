// putAll(values) in a map's subclass: the member putAll(Map) can't take
// an Array of pairs, so this's extension putAll(Array<out Pair>) is called.
class Prefilled(vararg values: Pair<String, String>) : HashMap<String, String>() {
    init {
        putAll(values)
        putAll(mapOf("m" to "n"))
    }
}

fun main() {
    println(Prefilled("a" to "b", "c" to "d").toSortedMap())
}
