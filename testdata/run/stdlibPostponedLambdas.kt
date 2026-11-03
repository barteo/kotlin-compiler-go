fun <T> id(x: T) = x
fun <A : (Int) -> Int> take(x: A, y: A): (Int) -> Int = { x(y(it)) }

fun main() {
    // A lambda passed for a variable takes the function type that
    // variable is below through others: listOf's E, Pair's B, mapOf's V.
    val fs: List<(String) -> Int> = listOf(id { s -> s.length })
    println(fs[0]("abc"))
    val m: Map<String, (String, Int) -> String> = mapOf("a" to { s, n -> s.repeat(n) })
    println(m["a"]!!("ab", 3))
    // Or its declared bound.
    println(take({ a -> a + 1 }, { b -> b * 2 })(5))
}
