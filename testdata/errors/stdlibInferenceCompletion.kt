// Inference and call completion that K2 accepts: literal arithmetic, !!
// as a call, typealias constructors, anonymous object types, overloaded
// references and the order of postponed lambdas.

fun <T> pick(a: T, b: T): T = a
fun <T> dependOn(x: T): T = x
fun takeLong(l: Long) {}

fun literals(l: Long) {
    takeLong(pick(l, 10 * 10)) // 10 * 10 is a Long here
    val s: String = dependOn(dependOn("a")!!)
}

class Hr<A, B, C, D>(val a: A, val b: B)
class Foo<T>
class Bar<T>
typealias Test<A, B> = Hr<A, B, Foo<A>, Bar<B>>

fun aliasCtor() {
    val t: Hr<Int, String, Foo<Int>, Bar<String>> = Test(1, "")
}

open class Base<T>(val v: T)

private fun <T> wrap(x: T) = object : Base<T>(x) { val extra = 1 }

fun anonSuper(): String = wrap("s").v

class Holder {
    private val obj by lazy { object { val x = 42 } }
    fun x() = obj.x
}

fun foo(reason: String): String = reason
fun foo(r: Int) {}
fun <R, T> myLet(x: () -> (T) -> R): R? = null

fun refOverload() {
    val u: Unit? = myLet { ::foo }
    val bad = myLet { ::foo } // ERROR cannot infer
}

interface Scope<out S : Element>
class Wrapped(val scope: Scope<Sub>)
open class Element
class Sub : Element()

fun <F : Element> two(a: (Scope<F>.() -> Unit)?, b: (Scope<F>.() -> Unit)?) {}

fun returnedLambda(a: Scope<Sub>.() -> Unit, b: (Wrapped.() -> Unit)?) {
    two({ a.invoke(this) }, run { { Wrapped(this) } })
    two({ a.invoke(this) }, b?.let { { Wrapped(this) } })
}

class Prod<out A> {
    fun <B> product(other: Prod<(A) -> B>): Prod<B> = TODO()
    fun <B, C, R> deep(other1: Prod<B>, other2: Prod<C>, f: (A, B, C) -> R) {
        val x: Prod<R> = product<R>(other1.product(other2.product(of { c -> { b -> { a -> f(a, b, c) } } })))
    }
}
fun <A> of(x: A): Prod<A> = TODO()

class MyPair<out A, out B>
fun <X : Del<Y>, Y> mat(): MyPair<X, Del<Y>> = TODO()
operator fun <T> MyPair<T, T>.provideDelegate(a: Any?, b: Any?): T = TODO()
interface Del<V> {
    operator fun getValue(a: Any?, b: Any?): V = TODO()
}

fun provided() {
    val x: String by mat()
}

interface Self<out T : Self<T>> {
    fun copy(t: @UnsafeVariance T): Self<T>
}

fun selfStar(o: Self<*>, p: Self<out Self<*>>) {
    val a: Self<Self<*>> = o
    val b: Self<Self<*>> = o.copy(o)
    val c: Self<Self<*>> = p
}
