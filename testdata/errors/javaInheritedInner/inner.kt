// A Java source's simple name of an inner class of a generic class it
// is, or extends, has the outer class's type arguments, as javac's
// signature: BaseInner in Outer<H> is BaseOuter<H>.BaseInner.

fun use(x1: Outer<Int>, x2: Outer<Int>.Inner, b: BaseOuter<Long>) {
    val a: Int = x1.bar().foo1()
    val d: Double = x1.bar().foo2()
    val c: Int = x2.foo1()
    val l: Long = b.own().get()
    val s: String = x1.bar().foo1() // ERROR type mismatch
}
