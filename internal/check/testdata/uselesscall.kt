// USELESS_CALL_ON_NOT_NULL (-Wextra): orEmpty, isNullOrEmpty and
// isNullOrBlank of the standard library on a receiver that isn't null.
val list1: List<Int> = listOf(1)
val l0 = list1.orEmpty() // USELESS_CALL_ON_NOT_NULL
val l1 = list1.orEmpty().map { "$it" } // USELESS_CALL_ON_NOT_NULL
val s = ""
val e0 = s.isNullOrEmpty() // USELESS_CALL_ON_NOT_NULL
val e1 = s.isNullOrBlank() // USELESS_CALL_ON_NOT_NULL
val s2: String? = ""
val e2 = s2?.isNullOrEmpty() // USELESS_CALL_ON_NOT_NULL
val e3 = s2.isNullOrEmpty()
val l2: List<Int>? = listOf(1)
val l3 = l2?.orEmpty() // USELESS_CALL_ON_NOT_NULL
val l4 = l2.orEmpty()

fun test(q: Sequence<Int>, a: Array<String>, m: Map<String, Int>, cs: CharSequence): Int {
    val x0 = q.orEmpty() // USELESS_CALL_ON_NOT_NULL
    val x1 = a.orEmpty() // USELESS_CALL_ON_NOT_NULL
    val x2 = m.orEmpty() // USELESS_CALL_ON_NOT_NULL
    val x3 = cs.isNullOrEmpty() // USELESS_CALL_ON_NOT_NULL
    val x4 = a.isNullOrEmpty()
    val x5 = m.isNullOrEmpty()
    with(s) { orEmpty() }
    val y0 = (s as String?).orEmpty()
    if (s2 != null) s2.orEmpty() // USELESS_CALL_ON_NOT_NULL
    @Suppress("USELESS_CALL_ON_NOT_NULL")
    val y1 = s.orEmpty()
    return listOf(x0, x1, x2, x3, x4, x5, y0, y1).size
}

fun <T : Any> g(t: List<T>) = t.orEmpty() // USELESS_CALL_ON_NOT_NULL
fun <T> h(t: T) where T : CharSequence? = t.isNullOrEmpty()
