fun <T> id(x: T) = x
fun <T> select(x: T, y: T) = x

fun main() {
    val f = select(id { this + 1 }, fun Int.() = this)
    println(5.f())
    val g = select(id { this + it }, fun Int.(x: Int) = this * x)
    println(3.g(4))
}
