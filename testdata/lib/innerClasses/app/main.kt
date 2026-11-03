import second.Outer

fun Outer.testExt() = Inner("ext").show()

class Sub : Outer("sub")

fun main() {
    println(Outer("a").testExt())
    println(Outer("b").Inner("call").show())
    println(Sub().Inner("inherited").show())
    println(Outer("O").Middle("K").Inner2().foo())
    println(Outer("c").Middle("d").Inner2().name())
}
