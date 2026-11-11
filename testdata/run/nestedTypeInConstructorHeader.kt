open class Base(val listener: Any)

class Primary : Base(object : Nested {
    override fun value() = 1
}) {
    interface Nested {
        fun value(): Int
    }
}

open class Consumer(val name: String, private val listener: OnUpdate) {
    constructor(name: String, onUpdate: (String) -> Unit) : this(
        name,
        object : OnUpdate {
            override fun onUpdate(value: String) {
                onUpdate(value)
            }
        },
    )

    fun send(v: String) = listener.onUpdate(v)

    interface OnUpdate {
        fun onUpdate(value: String)
    }
}

fun main() {
    println((Primary().listener as Primary.Nested).value())
    Consumer("c") { println("got " + it) }.send("x")
}
