import lib.Builder

class Text(val s: String) {
    override fun toString() = "Text($s)"
}

fun main() {
    val b = Builder()
    b.factory<Text> { Text(fixture()) }
    b.factory<Int> { fixture<Int>() * 10 }
    b.factory<String> { fixture { append("x") } }
    b.factory<Double> { fixture(listOf(2.5)) }
    println(b.made)
}
