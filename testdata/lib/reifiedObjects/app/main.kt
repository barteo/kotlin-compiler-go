import objs.*

inline fun <reified T> twice(): String = typeCheck<List<T>>().name() + either<T, String>().name()

fun main() {
    val xs = listOf(1, "a", 2.5, null, 'c', "b")
    val ints = typeCheck<Int>()
    val strings = typeCheck<String>()
    println(ints.name() + " " + xs.count { ints.test(it) })
    println(strings.name() + " " + xs.count { strings.test(it) })
    val t = tagged<Double>("t")
    xs.forEach { t.test(it) }
    println(t.name())
    val e = either<Char, Double>()
    println(e.name() + " " + xs.count { e.test(it) })
    println(filtered<String>(xs) { it < "b" })
    println(Box("box").checker<Number>().let { it.name() + " " + xs.count(it::test) })
    println(twice<Int>())
}
