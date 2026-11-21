// The warnings kotlinc 2.4.21 reports of the types below, inferred to
// name a private class of a.kt, as TestWarningsInferredInvisible reads
// them: each line ends with a comment naming the warnings it reports, if
// any. kc doesn't judge an if or when an expected type gives a type,
// which kotlinc may warn of: val i2: Any = if (c) O1 else O2.
package a

fun <T> sel(x: T, y: T): T = x
fun <T> lst(x: T, y: T): List<T> = listOf(x, y)
fun <T> va(vararg xs: T) = xs.size
inline fun <reified T> re(x: T, y: T) = arrayOf(x, y).size
fun <T> arr(x: Array<T>) = x.size

fun test(c: Boolean) {
    val s1 = sel(O1, O2) // INFERRED_INVISIBLE_RETURN_TYPE_WARNING
    val s2 = sel<_>(O1, O2) // INFERRED_INVISIBLE_RETURN_TYPE_WARNING
    val s3 = sel<Any>(O1, O2)
    val s4: Any = sel(O1, O2) // INFERRED_INVISIBLE_RETURN_TYPE_WARNING
    val l1 = lst(O1, O2) // INFERRED_INVISIBLE_RETURN_TYPE_WARNING
    val l2: List<Any> = lst(O1, O2) // INFERRED_INVISIBLE_RETURN_TYPE_WARNING
    va(O1, O2) // INFERRED_INVISIBLE_VARARG_TYPE_ARGUMENT_WARNING
    va<Any>(O1, O2)
    re(O1, O2) // INFERRED_INVISIBLE_REIFIED_TYPE_ARGUMENT_WARNING
    re<Any>(O1, O2)
    arr(arrayOf(O1, O2)) // INFERRED_INVISIBLE_REIFIED_TYPE_ARGUMENT_WARNING
    val i1 = if (c) O1 else O2 // INFERRED_INVISIBLE_WHEN_TYPE_WARNING
    if (c) O1 else O2 // INFERRED_INVISIBLE_WHEN_TYPE_WARNING UNUSED_EXPRESSION
    if (c) O1 else O2.let {} // UNUSED_EXPRESSION
    val w1 = when { c -> O1; else -> O2 } // INFERRED_INVISIBLE_WHEN_TYPE_WARNING
    when { c -> lst(O1, O1); else -> listOf(O2) } // INFERRED_INVISIBLE_WHEN_TYPE_WARNING
    println(sel(O1, O2)) // INFERRED_INVISIBLE_RETURN_TYPE_WARNING
    sel(O1, O2).toString() // INFERRED_INVISIBLE_RETURN_TYPE_WARNING
    val m = makeA()
    val ok = sel(O1, O1)
}
