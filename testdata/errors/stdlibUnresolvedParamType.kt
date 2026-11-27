// A parameter of a type that couldn't be resolved takes any argument, as
// K2's error type does: its function is a candidate of the calls.

fun g(x: Unresolved) {} // ERROR unresolved or unsupported type: Unresolved
fun g(x: Int) {}
fun h(x: Unresolved) {} // ERROR unresolved or unsupported type: Unresolved

fun <T> f1(l: List1<T>): T = TODO() // ERROR unresolved or unsupported type: List1
fun <T> f1(c: Collection<T>): T = TODO()

fun <T> test(l: List<T>) {
    h(1)
    g("")
    g(1) // ERROR ambiguous call
    f1(l) // ERROR ambiguous call
}

inline var property: () -> Unit
    get() = { }
    set(noinline value) {
        keep(value)
    }

fun keep(f: () -> Unit) = f
