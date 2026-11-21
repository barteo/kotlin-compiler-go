// A builder's variable bounded only from above where a nested call's
// lambda needs its type isn't fixed for it: the lambda's parameter takes
// what the bounds give, Any, and the builder's variable is what the code
// after gives it, String, as in K2.
class Controller<T> {
    fun yield(t: T): Boolean = true
    fun get(): T = null!!
}

fun <S> generate(g: Controller<S>.() -> Unit): S = null!!

fun bar(x: Any) {}
fun <R> myWith(r: R, b: (R) -> Unit) {}

fun unusedParameter(): Int = generate {
    bar(get())
    myWith(get()) { }
    yield("")
}.length

fun memberOfParameter(): Int = generate {
    bar(get())
    myWith(get()) {
        it.length // ERROR unresolved reference: length
    }
    yield("")
}.length
