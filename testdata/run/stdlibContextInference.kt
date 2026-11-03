// Context arguments of generic context parameters, chosen with the
// call's other constraints, and contracts on context parameters.
@file:OptIn(ExperimentalContracts::class)

import kotlin.contracts.ExperimentalContracts
import kotlin.contracts.contract

interface Named
class Box<T>
class A : Named { override fun toString() = "A" }
class B : Named { override fun toString() = "B" }

context(t: T)
fun <T : Named> pick(b: Box<T>): String = t.toString()

@Suppress("INVISIBLE_REFERENCE")
context(t: @kotlin.internal.NoInfer T)
fun <T> pickNoInfer(b: Box<T>): String = t.toString()

interface Scope<V> { fun value(): V }
class ScopeOf<V>(val v: V) : Scope<V> { override fun value() = v }

context(s: Scope<V>)
fun <V> similar(other: V): V = s.value()

fun <T1, T2, R> context(with1: T1, with2: T2, block: context(T1, T2) () -> R): R = block(with1, with2)

context(a: String?)
fun validate() {
    contract { returns() implies (a != null) }
    if (a == null) throw IllegalArgumentException()
}

context(a: String?)
fun process(): Int {
    validate()
    return a.length
}

context(b: Any?)
fun isText() {
    contract { returns() implies (b is CharSequence) }
    if (b !is CharSequence) throw IllegalArgumentException()
}

context(b: Any?)
fun textLength(): Int {
    isText()
    return b.length
}

fun main() {
    // Both are of the nearest level: the argument's type chooses.
    println(context(A(), B()) { pick(Box<A>()) + pick(Box<B>()) })
    // The nearest that fits, with the argument's type.
    with(A()) { with(B()) { println(pick(Box<A>()) + pick(Box<B>())) } }
    println(context(A(), B()) { { pickNoInfer(Box<B>()) + pickNoInfer(Box<A>()) }() })
    with(A()) { with(B()) { println(pickNoInfer(Box<A>()) + listOf(1).map { pickNoInfer(Box<B>()) }) } }
    // The literal is an Int: the Scope<String> doesn't fit.
    with(ScopeOf(3) as Scope<Int>) {
        with(ScopeOf("string") as Scope<String>) {
            println(similar(1))
            println(similar("other"))
        }
    }
    with("text" as String?) { println(process()) }
    with("chars" as Any?) { println(textLength()) }
}
