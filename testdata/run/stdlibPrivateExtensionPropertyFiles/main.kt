class Box(val data: Map<String, String>?)

private val Box.stamp: String
    get() {
        val n = data?.size ?: 0
        return "main:$n"
    }

private var Box.note: String
    get() = "main note"
    set(value) = println("main note = $value")

fun Box.second(): String? = data
    ?.takeIf { it.isNotEmpty() }
    ?.let {
        note = "b"
        "second $stamp ${this.stamp} $note"
    }

fun main() {
    val box = Box(mapOf("k" to "v"))
    println(box.first())
    println(box.second())
}
