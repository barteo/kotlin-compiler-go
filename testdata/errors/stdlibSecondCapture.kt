// Each projected argument for a variable in an invariant position is a
// capture of its own, which no other capture can be equal to or below:
// T can't be both of two MutableList<out Number>s' captures, even of one
// value, or of a Map<*, *>'s key and value, as K2 reports
// (ARGUMENT_TYPE_MISMATCH). kc used to approximate both by out Number,
// which let cp add a Double to a list of Ints.

class Inv<T>
fun <T> cp(a: MutableList<T>, b: MutableList<T>) { a.add(b[0]) }
fun <T> sw(a: Inv<T>, b: Inv<T>) {}
fun <T> one(a: MutableList<T>, b: List<T>) {}
fun <T> sz(a: Array<T>): Int = a.size
fun <T> foo2(m: Map<T, T>) = m
fun <T> pr(p: Pair<T, T>) {}
fun <T> first(l: MutableList<T>): T = l[0]

fun test(x: MutableList<out Number>, y: MutableList<out Number>, a: Inv<*>, b: Inv<*>, m: Map<*, *>, p: Pair<*, *>, arr: Array<*>, ints: List<Int>) {
    cp(x, y) // ERROR MutableList<CapturedType(out Number)> was expected
    cp(x, x) // ERROR MutableList<CapturedType(out Number)> was expected
    sw(a, b) // ERROR Inv<CapturedType(*)> was expected
    foo2(m) // ERROR Map<CapturedType(*), CapturedType(*)> was expected
    one(x, ints) // ERROR List<CapturedType(out Number)> was expected
    sz(arr)
    pr(p)
    val n: Number = first(x)
}
