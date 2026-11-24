// Signatures of the DefaultImpls methods for the bodies an interface
// inherits: the members' as the interface sees them, with its type
// parameters and its type first (TestSignatures).
package i
open class Foo
interface Base<S : Foo> {
    fun plain(a: Foo, b: String): String = b
    fun self(): S? = null
    fun <R> conv(r: R, s: S): List<R> = listOf(r)
    fun defaults(a: Int = 1, s: List<S> = emptyList()): Int = a
    val prop: List<S> get() = emptyList()
    var both: Map<String, S>
        get() = emptyMap()
        set(value) {}
}
interface Same<S : Foo, X> : Base<S> { fun own(l: List<X>) = 1 }
interface Bound : Base<Foo> { fun own(l: List<String>) = 1 }
interface Deep<Z : Foo> : Same<Z, String>
interface Over<S : Foo> : Base<S> { override fun plain(a: Foo, b: String): String = "o" }
interface Plain {
    fun <T : Comparable<T>> pick(a: List<T>): T = a[0]
    fun map(a: List<String>): Map<String, Int> = emptyMap()
    fun none(a: String): String = a
    @JvmSuppressWildcards fun suppressed(a: List<Foo>): List<Foo> = a
    fun wild(a: List<@JvmWildcard String>, b: Map<String, Foo>): Int = 1
}
interface SubPlain : Plain
interface GenPlain<Q> : Plain, Base<Foo>
interface Element : kotlin.coroutines.CoroutineContext.Element
interface GenElement<T> : kotlin.coroutines.CoroutineContext.Element
