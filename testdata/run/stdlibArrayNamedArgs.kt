// An array class's constructor called with named arguments in the other
// order, Array(init = ..., size = ...): the init lambda is inlined as
// always, and the arguments are evaluated in their order.

fun n(): Int { println("size"); return 3 }
fun f(): (Int) -> String { println("init"); return { "s$it" } }

fun main() {
    val a = Array(init = { it * 2 }, size = 3)
    println(a.toList())
    val b = Array(init = f(), size = n())
    println(b.toList())
    val c = IntArray(init = { it + 1 }, size = n())
    println(c.toList())
    val d = Array(size = n(), init = f())
    println(d.toList())
}
