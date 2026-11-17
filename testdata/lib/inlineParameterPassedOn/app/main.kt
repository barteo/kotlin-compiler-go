import lib.Base

class Model : Base<Any>("ready") {
    inline fun <reified T> runWhenState(crossinline block: (T) -> Unit) = whenState<T>(block)

    inline fun both(block: () -> Unit) = repeated("both", block)
}

fun first(model: Model): String {
    model.both { return "returned" }
    return "not returned"
}

fun main() {
    val model = Model()
    model.runWhenState<String> { println("string $it") }
    model.runWhenState<Int> { println("int $it") }
    model.whenState<String> { println("direct $it") }
    var n = 0
    model.both { n++ }
    println(n)
    println(first(model))
}
