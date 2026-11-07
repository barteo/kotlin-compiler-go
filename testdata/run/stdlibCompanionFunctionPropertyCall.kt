interface Message {
    val key: String

    data class Event(override val key: String = nextKey(), val type: String) : Message

    class Plain(val n: Int = counter()) {
        fun show() = "plain $n " + nextKey()
    }

    companion object {
        private var n = 0
        private val nextKey: () -> String = { "key" + (++n) }
        private fun counter() = 41 + n
    }
}

fun main() {
    println(Message.Event(type = "a"))
    println(Message.Event(type = "b"))
    println(Message.Plain().show())
}
