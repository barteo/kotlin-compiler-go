// emptyList() next to a MutableList<Item> makes T the common supertype,
// List<Item>, not MutableList<Item>.

class Item

fun <T> pick(first: T, second: T): T = second

fun use(built: MutableList<Item>) {
    val list: List<Item> = pick(emptyList(), built)
    val mutable: MutableList<Item> = pick(emptyList(), built) // ERROR type mismatch: inferred type is List<Item> but MutableList<Item> was expected
}
