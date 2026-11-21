// An ambiguous call in a builder's lambda adds nothing, and the builder's
// variable keeps what the other calls give it. Generic extensions whose
// variables the builder's bounds stay ambiguous, though members of the
// same shapes don't, as in K2.
class Inv<E1>
class In<E2>

interface Controller<T> {
    fun add(x: T)
    fun foo1(vararg values: T)
    fun foo1(x: Inv<T>)
    fun foo4(x: In<CharSequence>)
    fun foo4(x: In<T>)
}

fun <T1> Controller<T1>.fooExt(vararg values: T1) {}
fun <T2> Controller<T2>.fooExt(x: Inv<T2>) {}

fun <E> generate(f: Controller<E>.() -> Unit) {}

fun bar(inv: Inv<String>, i: In<CharSequence>, cs: CharSequence) {
    generate {
        foo1(inv)
    }
    generate {
        add(cs)
        foo4(i) // ERROR ambiguous call: 2 candidates for 'foo4'
    }
    generate { // ERROR cannot infer type for type parameter 'E'
        fooExt(inv) // ERROR ambiguous call: 2 candidates for 'fooExt'
    }
}
