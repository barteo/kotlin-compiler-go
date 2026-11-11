package demo.model

class Item(val id: Int) {
    override fun toString() = "Item($id)"
}

fun <T> holder(vararg xs: T): List<T> = xs.toList()

fun main() {
    val empty = emptyList<
        demo
        .model.Item,
        >()
    println(empty)
    val one = holder<
        demo.model
            .Item,
        >(Item(1))
    println(one)
    val m: Map<String, demo
        .model.Item> = mapOf("a" to Item(2))
    println(m)
}
