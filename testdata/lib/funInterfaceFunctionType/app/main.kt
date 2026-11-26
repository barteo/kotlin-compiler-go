import lib.*

fun main() {
    val one = Source { 1 }
    println(one())
    println(read(Source { "a" }))
    println(read { "b" })
    println(Registry().put("x") { 10 }.put("y") { 20 }.names)
    println(same(Source { 3 })())
    val f: () -> Int = one
    println(f())
    val four = SuspendSource { two() + 2 }
    println(run { four() })
    println(run { four.map { it * 10 }() })
    println(run { two.map { it + 1 }.invoke() })
    val g: suspend () -> Int = four
    println(run { g() })
}
