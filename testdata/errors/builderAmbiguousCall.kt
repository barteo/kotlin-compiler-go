// An ambiguous call in a builder's lambda gives the builder's type
// variable nothing, and what the calls before it gave stays, as in kotlinc.
// Extensions of their own type parameters stay ambiguous where members of
// the builder's variable prefer the candidate without a vararg.
class Inv<E1>
interface Controller<T> {
    fun add(x: T)
    fun foo1(vararg values: T)
    fun foo1(x: Inv<T>)
    fun foo3(value: T)
    fun foo3(x: Inv<T>)
}
fun <T1> Controller<T1>.fooExt(vararg values: T1) {}
fun <T2> Controller<T2>.fooExt(x: Inv<T2>) {}
fun <E> generate(f: Controller<E>.() -> Unit): E = null!!

fun test(inv: Inv<String>) {
    generate { foo1(inv) }
    generate { // ERROR cannot infer type for type parameter 'E'
        foo3(inv) // ERROR ambiguous call: 2 candidates for 'foo3'
    }
    generate { // ERROR cannot infer type for type parameter 'E'
        fooExt(inv) // ERROR ambiguous call: 2 candidates for 'fooExt'
    }
    val a = generate {
        add("")
        foo3(inv) // ERROR ambiguous call: 2 candidates for 'foo3'
    }
    val a1: Int = a // ERROR type mismatch: inferred type is String but Int was expected
    val b = generate {
        add("")
        fooExt(inv) // ERROR ambiguous call: 2 candidates for 'fooExt'
    }
    val b1: Int = b // ERROR type mismatch: inferred type is String but Int was expected
    val c = generate {
        add(inv)
        fooExt(inv)
    }
    val c1: Int = c // ERROR type mismatch: inferred type is Inv<String> but Int was expected
}
