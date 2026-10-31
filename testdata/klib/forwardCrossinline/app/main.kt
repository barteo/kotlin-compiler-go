import lib.*

inline fun <reified F> fixture(crossinline configuration: Builder.() -> Unit = { }): String = common<F>(configuration)

inline fun labels(prefix: String, crossinline label: (Int) -> String) = twice(label).map { prefix + it }

inline fun named(crossinline configuration: Builder.() -> Unit) = common<Long>(configuration = configuration)

fun main() {
    println(fixture<String> { add("x") })
    println(fixture<Int>())
    var n = 0
    println(labels("#") { n += it; "n$it" })
    println(n)
    println(named { add("named") })
}
