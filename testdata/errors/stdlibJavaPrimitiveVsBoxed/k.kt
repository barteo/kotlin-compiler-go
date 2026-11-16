// As in K2, a Java int parameter is more specific than an Integer, also
// one that an annotation makes non-null, and an int than a long. Of an
// Integer and a long, neither is: the literal fits both (KT-55548).
fun main() {
    val a: String = JavaUtils.foo1(1)
    val b: String = JavaUtils.foo2(1)
    val c: Int = JavaUtils.foo3(1)
    JavaUtils.foo4(1) // ERROR ambiguous call
    val e: Int = JavaUtils.foo5(1)
}

fun ints() {
    val i: Int = Ints.f(123)
}
