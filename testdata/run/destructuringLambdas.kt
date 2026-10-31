// Destructured lambda parameters: { (a, b) -> ... }.

data class P(val a: Int, val b: String)

fun call(p: P, f: (P) -> String) = f(p)
inline fun callInline(p: P, f: (P) -> String) = f(p)
fun twice(i: Int, p: P, f: (Int, P) -> String) = f(i, p) + f(i + 1, p)
fun value(f: () -> Int) = f()

fun main() {
    println(call(P(1, "x")) { (a, b) -> b + a })
    println(callInline(P(2, "y")) { (a, b) -> b + a })
    println(call(P(3, "z")) { (_, b) -> b })
    println(twice(0, P(4, "w")) { i, (a, b) -> "$i$a$b " })
    val g = { (a, b): P -> b.length + a }
    println(g(P(5, "vv")))
    val h: (P) -> Int = { (a) ->
        a * 2
    }
    println(h(P(6, "u")))
    println(value { (1 + 2) * 3 })
    // init is a soft keyword, and a name outside classes' init blocks.
    println(call(P(7, "t")) { (type, init) -> init + type })
    val init = 8
    C()
    println(init)
}

class C {
    init {
        println("init block")
    }
}
