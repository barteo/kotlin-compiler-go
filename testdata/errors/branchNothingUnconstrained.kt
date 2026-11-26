// A call in a branch takes its variables from the others only when one of
// them has a type of its own: a jump's Nothing or a null.

fun <T> make(): T = TODO()

fun f(c: Boolean) {
    val i = if (c) make() else make() // ERROR cannot infer type for type parameter 'T'
}
