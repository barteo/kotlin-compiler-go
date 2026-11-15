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

fun unitFun() {}
fun unitLambda(x: () -> Unit) {}

fun unitElvis(list: List<Int>) {
    val u: Unit = myLet { ::foo } ?: unitFun()
    unitLambda { myLet { ::foo } ?: unitFun() }
    unitLambda { list.firstOrNull() ?: 1 }
    unitLambda { list.firstOrNull() ?: return@unitLambda }
    unitLambda { myLet { ::foo } ?: 1 } // ERROR cannot infer
}

class LocalParams {
    private fun <E, F> make(e: E, f: F) = {
        class C<G>(val e: E, val f: F, val g: G)
        val c: C<F> = C<F>(e, f, f)
        c
    }

    fun use() {
        val c = make<CharSequence, Number>("", 1)()
        val e: CharSequence = c.e
        val g: Number = c.g
        val s: String = c.e // ERROR type mismatch
    }
}

fun <T> localRefs(t: T) {
    class A(val t: T)
    class B<I>
    val a = A::class
    val b = B::class
    val get: (A) -> T = A::t
}

fun returnInCatch(f: () -> String?) = run {
    val foo = try {
        f
    } catch (e: Exception) {
        return { "1" }
    }
    { "2" }
}

fun returnInLambda(b: Boolean) = run { if (b) return 1; 2 }
fun returnUnitInLambda(b: Boolean) = run { if (b) return; 2 } // ERROR return type mismatch
fun returnInBody(x: Int?) = x ?: return 0 // ERROR needs an explicit return type

enum class EnOne { A }
enum class EnTwo { D }
fun useEnOne(x: EnOne) = x
fun impossibleBoth(x: Any) {
    if (x is EnOne && x is EnTwo) {
        useEnOne(x)
    }
}

val receiverLambda: (String.() -> String)? = null
fun whenLambdaBranch(): String {
    val g = when {
        receiverLambda != null -> receiverLambda
        else -> { { this + "K" } }
    }
    return g("O")
}

interface HasVal { val n: Int }
interface HasVar { var n: Int }
fun assignVar(a: Any) {
    if (a is HasVal && a is HasVar) {
        a.n = 5
    }
}

interface BoundA { fun doA() }
interface BoundB { fun doB() }
class TwoBounds<T>(val value: T) where T : BoundA, T : BoundB
fun starBounds(c: TwoBounds<*>) {
    c.value.doA()
    c.value.doB()
}

fun funcIdentity(c: (Int) -> Number, b: (Any) -> Int) {
    if (c === b) {
        val x: Int = c("")
    }
}

sealed interface Status<out T : Any> {
    val value: T?
    data class Pending<out T : Any>(override val value: T? = null) : Status<T>
    sealed interface Done<out T : Any> : Status<T>
    data class Ok<out T : Any>(override val value: T) : Done<T>
}

@OptIn(kotlin.contracts.ExperimentalContracts::class)
fun <T : Any> Status<T>.isPending(): Boolean {
    kotlin.contracts.contract { returns(false) implies (this@isPending is Status.Done) }
    return this is Status.Pending
}

@OptIn(kotlin.contracts.ExperimentalContracts::class)
fun <T : Any> Status.Done<T>.isOk(): Boolean {
    kotlin.contracts.contract { returns(true) implies (this@isOk is Status.Ok) }
    return this is Status.Ok
}

fun contractArgs() {
    val s = Status.Pending<String>()
    if (!s.isPending() && s.isOk()) {
        val v: String = s.value
    }
}

interface WhenA { fun foo() }
interface WhenB { fun bar() }
fun whenAlias(x: Any?) {
    when (val y = x) {
        !is WhenA -> {}
        !is WhenB -> {
            x.foo()
            y.foo()
        }
        else -> x.bar()
    }
}
