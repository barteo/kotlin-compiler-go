class Check<out T>(val actual: T)

fun <K, V> Check<Map<K, V>>.has(element: Pair<K, V>) = actual[element.first] == element.second

fun <V> Check<List<V>>.hasItem(item: Pair<String, V>) = item.second in actual

// An integer literal passed to a generic call that is an argument takes
// the Long, Short or Byte that the outer call's type parameter is known
// to be above, as with kotlinc: the Pair's Long equals the map's.
fun main() {
    val map = mapOf("a" to 1L)
    println(Check(map).has("a" to 1))
    println(Check(map).has(Pair("a", 1)))
    println(Check(listOf<Short>(2)).hasItem("b" to 2))
    println(Check(mapOf("c" to 3)).has("c" to 3))
    val pair = "a" to 1
    println(pair.second is Int)
}
