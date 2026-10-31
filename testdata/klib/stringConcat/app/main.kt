import lib.*

fun main() {
    assertAs<String>("abc") { println("a string of ${it.length}") }
    try {
        assertAs<Int>("abc") { println("unreachable $it") }
    } catch (e: AssertionError) {
        println(e.message)
    }
    println(describe(1, 2L, 3.5, 'c', true, null) { "lambda" })
    println(describe(-1, Long.MAX_VALUE, -0.0, '\u0001', false, "s") { marked(7) })
    println(marked(3).map { it.code })
    println(twice(42) { "{$it}" })
    println(printer<Int>().print(5))
    println(printer<List<String>>().print(listOf("x")))
    println(published(9))
}
