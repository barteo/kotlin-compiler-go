// The warnings kotlinc 2.4.21 reports of the spread arguments below, as
// TestWarningsRedundantSpread reads them: each line ends with a comment
// naming the warnings it reports, if any.
fun f(vararg xs: String, n: Int = 0) = xs.size + n
fun <T> g(vararg ts: T) = ts.size
class K(vararg val ys: Int)
annotation class A(vararg val names: String)
annotation class B(val ints: IntArray)
annotation class C(vararg val ints: Int)
val arr = arrayOf("a")
fun use() {
    f(xs = *arr) // REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_FUNCTION
    f(*arr)
    f(xs = arrayOf("b"))
    f(n = 1, xs = *arrayOf("c")) // REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_FUNCTION
    g(ts = *arrayOf(1)) // REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_FUNCTION
    K(ys = *intArrayOf(1)) // REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_FUNCTION
    listOf(elements = *arrayOf(1)) // REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_FUNCTION
    @Suppress("REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_FUNCTION")
    val x = f(xs = *arr)
}
@A(names = *["a"]) fun a1() {} // REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_ANNOTATION
@A(*["a"]) fun a2() {}
@A(names = ["a"]) fun a3() {}
@B(ints = intArrayOf(elements = *intArrayOf(1))) fun a4() {} // REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_FUNCTION
@C(ints = *intArrayOf(1, 2)) fun a5() {} // REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_ANNOTATION
