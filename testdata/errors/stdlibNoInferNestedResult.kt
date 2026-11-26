// As K2, a result annotated @NoInfer, as filterIsInstance's
// List<@NoInfer R>, doesn't take part in the inference of the call it is
// an argument of.
fun <X> bar6(x: List<X>, y: MutableList<in X>): X = TODO()
fun <X> bar7(x: List<X>): X = TODO()

fun test6(x: List<Any>, y: MutableList<String>) {
    bar6(x.filterIsInstance(), y) // ERROR cannot infer type for type parameter 'R'
    val t: String = bar7(x.filterIsInstance<String>())
    val u: List<String> = listOf(x.filterIsInstance<String>()).flatten()
}
