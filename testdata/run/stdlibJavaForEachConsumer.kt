import java.util.function.Consumer

fun main() {
    val names: Collection<String> = listOf("a", "bb")
    names.forEach(Consumer { n -> println("consumer $n") })
    val print = Consumer<String> { println("value $it") }
    listOf("c").forEach(print)
    val items: Iterable<Int> = setOf(1, 2)
    items.forEach(java.util.function.Consumer { println(it * 10) })
    mapOf("k" to 1).forEach(java.util.function.BiConsumer { k, v -> println("$k=$v") })
    names.forEach { println("lambda $it") }
    mapOf("m" to 2).forEach { (k, v) -> println("$k:$v") }
}
