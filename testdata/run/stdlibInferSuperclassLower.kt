// A type variable with a lower bound from a generic call without type
// arguments, as List<E> of emptyList(), and a known one of a subclass, as
// MutableList<Item>, is their common supertype, List<Item>, as in kotlinc.

class Item(val n: Int) {
    override fun toString() = "Item($n)"
}

fun <T> same(expected: T, actual: T): Boolean = expected == actual
fun <T> pick(first: T, second: T): T = second

fun main() {
    val built = mutableListOf<Item>()
    println(same(emptyList(), built))
    println(same(built, emptyList()))
    println(same(listOf(), built))
    built += Item(1)
    println(same(listOf(Item(1)).map { it.n }, built.map { it.n }))
    val list: List<Item> = pick(emptyList(), built)
    println(list)
    val set: Set<Item> = pick(emptySet(), hashSetOf(Item(2)))
    println(set)
    val map: Map<String, Item> = pick(emptyMap(), linkedMapOf("a" to Item(3)))
    println(map)
}
