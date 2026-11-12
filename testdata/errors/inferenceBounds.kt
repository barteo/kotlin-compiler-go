// A variable inferred from upper constraints alone is their intersection
// with its declared bounds, which a reified variable can't be.

interface Bound
interface Other
inline fun <reified F : Bound> reified(): F? = null

fun main() {
    val o: Other? = reified() // ERROR type argument for reified type parameter 'F' was inferred to the intersection of ['Bound' & 'Other']
}

// A Unit a lambda returns isn't within the bound of a variable that
// nothing else gives.
fun <N : Number> materializeNumber(): N = throw Exception()
fun <R> myRun(f: () -> R): R = f()
fun unitResult(): Unit = myRun {
    materializeNumber() // ERROR cannot infer type for type parameter 'N'. Specify it explicitly.
}

// A builder lambda that doesn't check with the variable unfixed, for a
// reason other than an ambiguous call, leaves it uninferred: the other
// lambda doesn't give it a type for a second check.
class Container<A> {
    fun consume(arg: A) {}
}
fun <B> build(funcA: (Container<B>) -> ((B) -> Unit), funcB: (Container<B>) -> Unit) {}
fun twoLambdas() {
    build( // ERROR cannot infer type for type parameter 'B'
        { container -> { arg -> arg.length } },
        { container -> container.consume("") },
    )
}
