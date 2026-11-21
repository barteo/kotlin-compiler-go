// The ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING warnings
// kotlinc 2.4.21 reports of the calls below, as
// TestWarningsVarargNothing reads them: each line ends with a comment
// naming the warnings it reports, if any.
fun <T> f(vararg xs: T): Int = xs.size
fun <T : CharSequence?> g(vararg xs: T): Int = xs.size
fun <T : Any?> h(vararg xs: T): Int = xs.size
fun <A, T> two(a: A, vararg xs: T): Int = xs.size
fun <T> String.e(vararg xs: T): Int = xs.size
fun <T> id(x: T) = x
fun nothing(): Nothing? = null

object O {
    fun <T> f(vararg xs: T): Int = xs.size
}

fun test(s: String?) {
    f<Nothing>() // ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING
    f(nothing())
    f<Nothing?>()
    g(nothing()) // ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING
    g<Nothing?>(null) // ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING
    h(null)
    h<Nothing?>()
    two<Int, Nothing>(1) // ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING
    two<Nothing?, Int>(null)
    f(1, 2)
    f<String>()
    O.f<Nothing>() // ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING
    "a".e<Nothing>() // ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING
    s?.e<Nothing>() // ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING
    O
        .f<Nothing>() // ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING
    id(O.f<Nothing>()) // ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING
    O.f(*arrayOf<String>())
    listOf<Nothing>()
    mutableListOf<Nothing?>()
}

fun inferred(n: Nothing) {
    f(n) // ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING
}
