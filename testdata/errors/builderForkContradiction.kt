// A call in a builder's lambda whose fork contradicts what the calls
// before it gave the builder's variable doesn't join its constraints: the
// variable keeps the earlier type, and the call is an error, as in K2.
// kc reports the receiver and the smart cast value, K2 the argument.
class Controller<T>
fun <S> generate(g: Controller<S>.() -> Unit): S = TODO()
interface A<E>
interface B : A<Int>
fun <F> Controller<F>.baz(a: A<F>, f: F) {}

fun <T> bar(a: A<T>, w: T) {
    generate {
        baz(a, w)
        if (a is B) {
            baz(a, w)
        }
    }
    generate {
        baz(a, w)
        if (a is B) {
            baz(a, 1) // ERROR type mismatch
        }
    }
}
