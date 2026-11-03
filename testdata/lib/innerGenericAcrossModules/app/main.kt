class H : G<Int>(7) {
    inner class HI : GI()
}

fun main() {
    println(C4<String>().D4<Int>().foo("a", 1))
    val e4 = C4<Int>().D4<String>().E4<Int>()
    println(e4.foo(19, "b", 23))
    println(e4.bar(1, "c", 2) { x, y, z -> "${x + z}$y" })
    val i: Int = H().HI().get()
    println(i + 1)
}
