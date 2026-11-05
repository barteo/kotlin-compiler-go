class Bar : Foo(42) {
    inner class BarInner(val x: Int) : FooInner()
}

class H : G<String>("h") {
    inner class HI : GI()
    inner class HJ : GI("j")
}

fun main() {
    val o = Bar().BarInner(117)
    println("${o.x} ${o.foo()}")
    println(H().HI().get())
    println(H().HJ().get())
}
