// The classes in a destructuring declaration's value are named after the
// function, as kotlinc names them, not after the hidden variable.
fun f(): List<String> {
    val (a, b) = Pair(object {}, run { { 2 } })
    val c = object {}
    return listOf(a.javaClass.name, c.javaClass.name, b().toString())
}

fun main() {
    println(f())
}
