package lib

class Check<out T>(val actual: T)

fun <T> check(actual: T) = Check(actual)

fun Check<List<*>>.holds(vararg items: Any?): String = "list ${actual == items.toList()}"

fun <K, V> Check<Map<K, V>>.holds(vararg items: Pair<K, V>): String = "map ${actual == mapOf(*items)}"
