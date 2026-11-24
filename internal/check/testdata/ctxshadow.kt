// CONTEXTUAL_OVERLOAD_SHADOWED: of two callables with the same signature
// but for their context parameters, the one that needs more.
interface Sup
interface Sub : Sup
interface X
context(c: X) fun a1() {} // CONTEXTUAL_OVERLOAD_SHADOWED
fun a1() {}
fun a2() {}
context(c: X) fun a2() {} // CONTEXTUAL_OVERLOAD_SHADOWED
context(c: Sup) fun a3() {}
context(c: Sub) fun a3() {} // CONTEXTUAL_OVERLOAD_SHADOWED
context(c: Sup) fun a5(x: Int) {}
context(c: Sub, d: X) fun a5(y: Int) {} // CONTEXTUAL_OVERLOAD_SHADOWED
context(c: X) fun a6(x: Int) {}
fun a6(x: String) {}
context(c: X) fun a7(x: Int): Int = 1 // CONTEXTUAL_OVERLOAD_SHADOWED
fun a7(x: Int): String = ""
fun <T> a8(x: T) {}
context(c: X) fun <U> a8(x: U) {} // CONTEXTUAL_OVERLOAD_SHADOWED
fun a9(x: Any) {}
context(c: X) fun a9(x: Int) {}
fun Int.e() {}
context(c: X) fun Int.e() {} // CONTEXTUAL_OVERLOAD_SHADOWED
context(c: X, d: Sup) fun a10() {}
context(d: Sub, c: X) fun a10() {} // CONTEXTUAL_OVERLOAD_SHADOWED
context(c: X) fun a11() {}
val a11: Int get() = 1
val p1: Int get() = 1
context(c: X) val p1: Int get() = 1 // CONTEXTUAL_OVERLOAD_SHADOWED
val String.p2: Int get() = 1
context(c: X) val String.p2: Int get() = 1 // CONTEXTUAL_OVERLOAD_SHADOWED
fun s1() {}
@Suppress("CONTEXTUAL_OVERLOAD_SHADOWED") context(c: X) fun s1() {}
class C {
    context(c: X) fun m1() {}
    fun m1() {}
    fun m2() {}
    context(c: X) fun m2() {} // CONTEXTUAL_OVERLOAD_SHADOWED
    context(c: X) val p1: Int get() = 1
    val p1: Int get() = 1
    val p2: Int get() = 1
    context(c: X) val p2: Int get() = 1 // CONTEXTUAL_OVERLOAD_SHADOWED
}
fun f() {
    context(c: X) fun l1() {}
    fun l1() {}
    fun l2() {}
    context(c: X) fun l2() {} // CONTEXTUAL_OVERLOAD_SHADOWED
}
