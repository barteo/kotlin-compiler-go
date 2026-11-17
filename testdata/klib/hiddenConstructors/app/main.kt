import lib.*

fun main() {
    val s = Style(size = Size(5), name = "x")
    println("${s.size.v} ${s.name} ${s.weight} ${s.tag}")
    val t = Style()
    println("${t.size.v} ${t.name} ${t.weight} ${t.tag}")
    val o = Options(a = 9)
    println("${o.a} ${o.b}")
    println(Options().b)
}
