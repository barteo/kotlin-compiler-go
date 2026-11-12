abstract class Receiver {
    abstract fun onValue(value: String)
}

class Producer<T> {
    val items = mutableListOf<T>()

    fun send(item: T) {
        items.add(item)
    }
}

fun <T> produce(block: Producer<T>.() -> Unit): List<T> = Producer<T>().apply(block).items

class Source(private val suffix: String) {
    private var count = 0

    fun values(): String =
        produce {
            val receiver = object : Receiver() {
                override fun onValue(value: String) {
                    value.let { send(parse(it)) }
                    count++
                }
            }
            receiver.onValue("a")
            receiver.onValue("b")
            send(parse("c"))
        }.joinToString()

    fun counted() = count

    private fun parse(value: String) = value + suffix
}

fun main() {
    val source = Source("!")
    println(source.values())
    println(source.counted())
}
