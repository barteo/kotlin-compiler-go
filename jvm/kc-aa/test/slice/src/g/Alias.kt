package g

// Type aliases, top-level declarations and package listing.

typealias Names = List<String>
typealias Mapper<T> = (T) -> String
typealias Pairs<K> = Map<K, List<K>>
typealias Again = Names

fun top(x: Int) = x
val topProp = 1

class Holder {
    val names: Names = listOf()
    val mapper: Mapper<Int> = { "" }
    val pairs: Pairs<String>? = null
    val again: Again = listOf()
    val list: ArrayList<Int> = ArrayList()
    val nested: kotlin.collections.ArrayList<Names> = ArrayList()
    val plain: List<String> = listOf()
}
