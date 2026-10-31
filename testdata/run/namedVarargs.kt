// A named argument of a vararg parameter passes the array itself, as
// *xs does: mapOf(pairs = arrayOf(...)).
fun f(vararg xs: Int) = xs.size
fun g(a: Int, vararg xs: String, b: Int = 0): String {
    var s = "$a"
    for (x in xs) s += " " + x
    return "$s $b"
}
class Box(vararg val items: String)
fun <T> h(vararg ts: T): Int = ts.size

fun main() {
    println(f(xs = intArrayOf(1, 2)))
    println(g(1, xs = arrayOf("a", "b"), b = 3))
    println(g(a = 1, xs = *arrayOf("c")))
    println(g(xs = arrayOf("d"), a = 2))
    println(Box(items = arrayOf("x", "y")).items.size)
    println(h(ts = arrayOf(1, 2, 3)))
    val arr = arrayOf("p", "q")
    println(g(5, xs = arr))
    arr[0] = "changed"
    println(g(5, xs = arr))
}
