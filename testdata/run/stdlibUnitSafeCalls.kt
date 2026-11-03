// A safe call of a function returning Unit is a Unit?: null if the
// receiver is, without an expected type too.
class A {
    fun bar() {}
    fun baz(): Unit = Unit
}

val Unit?.ext: String get() = if (this == null) "null" else "unit"

fun <T> show(x: T) = x.toString()

fun main() {
    val a: A? = A()
    val n: A? = null
    val r1 = a?.bar()
    val r2 = n?.bar()
    println("$r1 $r2")
    println(a?.bar().ext + " " + n?.bar().ext)
    println(a?.baz().ext + " " + n?.baz().ext)
    println(show(n?.bar()) + " " + show(a?.bar()))
    val xs = listOf(a, n).map { it?.bar() }
    println(xs)
}
