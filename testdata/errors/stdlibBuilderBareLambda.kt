// As in K2, a lambda whose parameter is a type variable itself isn't
// checked with the variable unfixed: nothing in it infers the variable.
// A builder's lambda of the same call is checked first, and infers it.
fun takeS(x: String) {}

fun <T> one(f: (T) -> Unit) {}

fun <T> two(f: (T) -> Unit, g: MutableList<T>.() -> Unit) {}

fun main() {
    one { takeS(it) } // ERROR cannot infer type for type parameter 'T'
    one { x -> takeS(x) } // ERROR cannot infer type for type parameter 'T'
    two({ takeS(it) }) { } // ERROR cannot infer type for type parameter 'T'
    two({ println(it.length) }) { add("") }
}
