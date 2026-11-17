// Variables are fixed in K2's readiness order: a variable with proper
// constraints before one with only its declared bounds, which then has
// no type to infer; and of equally ready ones, the first in the call.

interface A
fun <T : A, R : T> emptyStrangeMap(): Map<T, R> = TODO()
fun expected(): Map<A, A> = emptyStrangeMap()
fun none() = emptyStrangeMap() // ERROR cannot infer type for type parameter

interface Proxy<in D>
class Box<E : Any>(val left: E) : Proxy<E>

abstract class Api {
    abstract fun <T> magic(): T
    inline fun <reified P : Any> match(proxy: Proxy<P>): P = magic()
    inline fun <reified C : Any> g(x: C) = match(Box(x))
}

open class Expression<K>
class ModOp<T : Number?, S : Number?>(val expr1: Expression<T>, val expr2: Expression<S>)
class QueryParameter<Q> : Expression<Q>()

fun <K, R : K?> Expression<in R>.wrap(value: K): QueryParameter<K> = QueryParameter()
fun <M : Number?, Z : M> Expression<M>.rem(t: Z): ModOp<M, Z> = ModOp(this, wrap(t))

// A safe call's selector is completed before the call it's passed to.
fun safeCall(c: String?, x: Set<Int>) = c?.let { it.toInt() } in x
