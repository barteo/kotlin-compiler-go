// In a builder's lambda, overloads whose parameter types are of the
// builder's variable have none more specific than the other, and the one
// without a vararg is chosen: Controller<T>'s members. Generic
// extensions, whose parameter types are of their own type parameters,
// are compared as anywhere else, and stay ambiguous (KT-67947), so the
// builder's variable isn't inferred either.
class Inv<E>

interface Controller<T> {
    fun member(vararg values: T)
    fun member(x: Inv<T>)
}

fun <T1> Controller<T1>.extension(vararg values: T1) {}
fun <T2> Controller<T2>.extension(x: Inv<T2>) {}

fun <E> generate(f: Controller<E>.() -> Unit) {}

fun members(inv: Inv<String>) {
    generate {
        member(inv)
    }
}

fun extensions(inv: Inv<String>) {
    generate { // ERROR cannot infer type for type parameter 'E'
        extension(inv) // ERROR ambiguous call
    }
}
