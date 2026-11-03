import a.*

fun main() {
    val i = E("O").Inner("K")
    println(i.foo("?"))
    println(i.bar { it + "K" })
    println(i.baz())
}
