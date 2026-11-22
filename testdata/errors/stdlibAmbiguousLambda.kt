import kotlin.experimental.ExperimentalTypeInference

// The lambda of an ambiguous call is checked as kotlinc checks it,
// without an expected type: no candidate gives it a receiver or
// parameter types, so a member of the receiver is unresolved in it.
@OptIn(ExperimentalTypeInference::class)
@OverloadResolutionByLambdaReturnType
fun <E1> buildL(x: MutableList<E1>.() -> Int): Int = 1

@OptIn(ExperimentalTypeInference::class)
@OverloadResolutionByLambdaReturnType
fun <E2> buildL(x: MutableList<E2>.() -> String): String = ""

fun builder() {
    buildL { // ERROR ambiguous call
        add("") // ERROR unresolved reference: add
        ""
    }
}

fun take(a: Int, f: String.() -> Unit) {}
fun take(a: Long, f: Int.() -> Unit) {}

fun plain(x: Any) {
    take(x) { // ERROR none of the 2 candidates
        length // ERROR unresolved reference: length
    }
}
