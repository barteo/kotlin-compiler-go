// Outer instances and captures of inner and local classes and object
// expressions, delegates' fields, inline functions with reified type
// parameters, local functions and the classes of the lambdas of inline
// functions' default values (TestSignatures).
package l

import kotlin.properties.ReadOnlyProperty
import kotlin.reflect.KProperty

open class Foo
class Fin

class Outer<T>(val t: T) {
    inner class In {
        fun get(): T = t
        inner class Deep { fun get(): T = t }
        inner class DeepG<X>(val x: X)
    }
    inner class InG<U>(val u: U)
    fun local(): Any {
        class Loc { fun get(): T = t }
        return Loc()
    }
    fun obj(): Any = object { fun get(): T = t }
    fun <Q> localG(q: Q): Any {
        class Loc(val i: Int) { fun get(): T = t; fun q(): Q = q }
        return Loc(1)
    }
}
class Plain {
    inner class In
    inner class G<X> { inner class D }
    fun local(): Any {
        class Loc(val list: List<Foo>) { fun get() = this@Plain }
        class Loc2(val i: Int) { fun get() = this@Plain }
        return listOf(Loc(listOf()), Loc2(1))
    }
}
class Two<A, B> {
    inner class I { inner class J { fun a(): A? = null } }
    class Nested<C> { inner class K }
}

fun <T> ctors(t: T, s: String, foos: List<Foo>, mutable: MutableList<Foo>, fn: (Foo) -> Foo): Any {
    class L(val v: T, val list: List<Fin>)
    class Cap(val list: List<Foo>) { fun get() = s + t + foos + mutable + fn }
    class CapOnly { fun get() = s; fun t() = t }
    class NoGen(val x: Int) { fun get() = s }
    class Sec(val x: Int) { constructor(l: List<T>) : this(l.size); fun get() = s }
    class Gen<X>(val x: X, val l: List<X>)
    val o = object { fun get() = foos.size + mutable.size }
    return listOf(L(t, listOf()), Cap(listOf()), CapOnly(), NoGen(1), Sec(2), Gen(1, listOf()), o)
}
fun ctorsPlain(): Any {
    class M(val list: List<Fin>)
    return M(listOf())
}
enum class E {
    A;
    fun local(): Any {
        class Loc(val l: List<Foo>)
        return Loc(listOf())
    }
}

class RO<T>(val t: T) : ReadOnlyProperty<Any?, T> {
    override fun getValue(thisRef: Any?, property: KProperty<*>): T = t
}
val topLazy: List<Foo> by lazy { listOf() }
val topRO: List<Foo> by RO(listOf())
class Delegates<T>(t: T, m: Map<String, Any?>) {
    val lazy: List<Foo> by lazy { listOf() }
    val ro: List<Foo> by RO(listOf())
    val gen: T by RO(t)
    val map: String by m
    val plain: Lazy<T> = lazy { t }
    companion object {
        val inCompanion: List<Foo> by lazy { listOf() }
        val roInCompanion: List<Foo> by RO(listOf())
    }
}
object DelegatesObj {
    val lazy: List<Foo> by lazy { listOf() }
    @JvmStatic val static: List<Foo> by lazy { listOf() }
}
interface DelegatesItf {
    companion object { val lazy: List<Foo> by lazy { listOf() } }
}

inline fun <reified T> isA(x: Any): Boolean = x is T
inline fun <reified T : Any> listOfT(x: Any): List<T> = listOf(x as T)
inline fun <reified T> noGeneric(x: Int): Int = x
inline fun <reified T> List<T>.ext(x: T): Array<T> = arrayOf(x)
@PublishedApi internal inline fun <reified T> published(x: List<T>) = x
internal inline fun <reified T> intern(x: List<T>) = x
private inline fun <reified T> priv(x: List<T>) = x
inline val <reified T> List<T>.reifiedVal: Int get() = size
inline var <reified T> MutableList<T>.reifiedVar: T
    get() = this[0]
    set(v) { this[0] = v }
@Deprecated("", level = DeprecationLevel.HIDDEN) inline fun <reified T> hiddenReified(l: List<T>): List<T> = l
@Deprecated("", level = DeprecationLevel.HIDDEN) fun <T> hidden(l: List<T>): List<T> = l
@JvmSynthetic fun <T> synthetic(l: List<T>): List<T> = l

fun <T> make(): List<T> = listOf()
class K<A, Z>(val a: A) {
    inline fun <reified T> member(x: List<T>, a: A): Map<A, T> = mapOf()
    protected inline fun <reified T> prot(x: List<T>) = x
    fun <B, C> locals(b: B, s: String): Int {
        fun <U> own(u: U): Map<U, B> = mapOf()
        fun order(x: Map<C, B>, y: Map<Z, A>) = x.size + y.size
        fun classParam(x: Map<Z, B>) = x.size
        fun <D> nested(x: Map<D, B>): Int {
            fun inner(d: List<D>, c: List<C>, z: List<Z>) = d.size + c.size + z.size
            return inner(listOf(), listOf(), listOf())
        }
        fun captures(l: List<C>) = l.size + s.length + b.hashCode()
        fun usesThis(l: List<Fin>) = l.size + a.hashCode()
        fun inBody(): Int {
            val x: List<C> = listOf()
            return x.size
        }
        fun inCall() = make<C>().size
        fun inferred(): Int {
            val q = make<A>()
            return q.size
        }
        fun lambda() = { x: List<C> -> x.size }
        fun cast(x: Any) = x as? B
        fun none(l: List<Fin>) = l.size
        fun String.ext(l: List<Foo>) = l.size + length
        fun defaults(l: List<Foo> = listOf()) = l.size
        return own(1).size + order(mapOf(), mapOf()) + classParam(mapOf()) + nested<Int>(mapOf()) + captures(listOf()) +
            usesThis(listOf()) + inBody() + inCall() + inferred() + lambda()(listOf()) + cast(1).hashCode() + none(listOf()) +
            "".ext(listOf()) + defaults()
    }
    fun <B : Comparable<B>> bound(b: B): Int {
        fun bounded(l: List<B>) = l.size
        return bounded(listOf())
    }
}
fun localsPlain(foos: List<Foo>): Int {
    fun plain(s: List<Fin>) = s.size
    fun captured(s: List<Foo>): List<Foo> = s + foos
    suspend fun suspending(s: List<Fin>) = s.size
    fun deep(): Int {
        fun inner(l: List<Int>) = l.size
        return inner(listOf())
    }
    val inLambda = {
        fun inner(l: List<Int>) = l.size
        inner(listOf())
    }
    return plain(listOf()) + captured(listOf()).size + deep() + inLambda()
}
fun <T> localsInObject(t: T): Any = object {
    fun m(): Int {
        fun inner(l: List<T>) = l.size
        return inner(listOf())
    }
}
interface LocalsItf {
    fun d(): Int {
        fun inner(l: List<Fin>) = l.size
        return inner(listOf())
    }
}

fun takes(f: () -> Int) = f()
inline fun dflt(f: () -> Int = { 2 }) = f()
inline fun dfltList(f: (String) -> List<Foo> = { listOf() }) = f("")
inline fun dfltParams(f: (List<Foo>, MutableList<Foo>) -> List<Foo> = { a, _ -> a }) = f(listOf(), mutableListOf())
inline fun dfltUnit(f: (Int) -> Unit = { }) = f(1)
inline fun <T> dfltGeneric(t: T, f: (T) -> T = { it }) = f(t)
inline fun dfltExt(f: String.(Int) -> Long = { 2L }) = "".f(1)
inline fun dfltNullable(f: (Int?) -> Pair<Int, Fin>? = { null }) = f(1)
inline fun dfltNothing(f: () -> Nothing? = { null }) = f()
inline fun dfltNoinline(noinline f: () -> Int = { 2 }) = f()
internal inline fun dfltInternal(f: () -> Int = { 2 }) = f()
class Dflt {
    inline fun dfltMember(x: Int, f: () -> Int = { x }) = f()
}
