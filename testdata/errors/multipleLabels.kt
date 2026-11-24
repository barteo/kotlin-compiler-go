@Target(AnnotationTarget.EXPRESSION)
@Retention(AnnotationRetention.SOURCE)
annotation class Ann

fun parenthesized() = lambda@ ({ return@lambda })
fun annotated() = lambda@ @Ann { return@lambda }
fun twice() = l1@ l2@ { return@l2 } // ERROR multiple labels are forbidden
fun thrice() = l1@ l2@ l3@ { return@l3 } // ERROR multiple labels are forbidden

fun statement() {
    l1@ l2@ { return@l2 } // ERROR multiple labels are forbidden
}

inline fun call(s: () -> Unit) = s()

inline fun argument(s: () -> Unit) {
    call((label@ s))
    call((label2@ label@ s)) // ERROR multiple labels are forbidden
}
