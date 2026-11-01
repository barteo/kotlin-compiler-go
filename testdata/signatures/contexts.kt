// Generic signatures of functions with context parameters, which take
// them first, and of context function types (TestSignatures).
package p

interface Raise<in E>
interface Scope

context(r: Raise<E>)
fun <E> raised(e: E): Nothing = TODO()

context(r: Raise<List<E>>, _: Scope)
fun <E, A> List<A>.bound(f: (A) -> E): Map<E, A> = emptyMap()

context(c: Comparator<T>)
fun <T> T.atMost(other: T): Boolean = c.compare(this, other) <= 0

fun <E, A> scoped(block: context(Raise<E>) () -> A, f: context(Raise<E>, Scope) Foo.(A) -> List<E>): A = TODO()

context(r: Raise<E>)
suspend fun <E> suspended(e: E, block: suspend context(Scope) () -> E): List<E> = listOf(e)
