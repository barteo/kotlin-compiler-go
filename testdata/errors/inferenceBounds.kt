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
