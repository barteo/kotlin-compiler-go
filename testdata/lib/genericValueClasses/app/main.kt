import values.*

fun <F : Any> foo(arg: F): F {
    val a = A<F?>(arg)
    return a.x
}

fun main() {
    val a1 = A("O")
    val a2 = A<String?>("K")
    println(a1.x + a2.x + foo("!"))
    println(N(21).twice())
    println(N(1.5).n)
    println(Box(3).max(Box(7)).v)
    println(Box("b").max(Box("a")))
    val m = Mapper<String> { it.x + it.x.length }
    println(m.map(A("abc")))
    val list = listOf(A(1), A(2))
    println(list.sumOf { it.x })
}
