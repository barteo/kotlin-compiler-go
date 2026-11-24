// Nothing is no reified type argument, inferred or given; Nothing? is.
// As in K2, a reified variable whose lower bounds are all Nothing is
// inferred from its upper bounds, declared ones too.

interface Bound

inline fun <reified T : Bound> takeReified(t: T?): T = t ?: TODO()
inline fun <reified T> takeReifiedUnbound(t: T?): T = t ?: TODO()
inline fun <reified M> materialize(): M = TODO()
inline fun <reified T> foo(block: () -> T): String = ""
fun <T> select(a: T, b: T): T = a

fun test(): Bound {
    takeReified(null)
    takeReifiedUnbound(null) // ERROR cannot use 'Nothing' as reified type parameter
    select(null, materialize())
    foo<Any> { TODO() }
    foo { TODO() } // ERROR cannot use 'Nothing' as reified type parameter
    materialize<Nothing>() // ERROR cannot use 'Nothing' as reified type parameter
    materialize<Nothing?>()
    return takeReifiedUnbound(null)
}
