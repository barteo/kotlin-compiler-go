@file:Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")

import kotlin.internal.OnlyInputTypes

// Common supertypes as K2's: an intersection joins component by
// component, type parameters keep a common bound, and a generic Java
// supertype counts. An imported @OnlyInputTypes is the annotation.

interface Base { fun base() {} }
interface Base3
interface Base2 : Base3
interface One : Base, Base2
interface Two : Base, Base2
interface Three : Base, Base3

fun <S : Base> intersect(vararg elements: S): S = TODO()
fun <S> intersectNoBound(vararg elements: S): S = TODO()
fun some(a: One, b: Two, c: Three) = intersectNoBound(intersect(a, b), c)

fun test(a: One, b: Two, c: Three) {
    intersectNoBound(intersect(a, b), c).base()
    val x: Base3 = intersectNoBound(intersect(a, b), c)
    some(a, b, c).base() // ERROR unresolved reference: base
}

open class Service<S : Service<S, A, M>, A : S, M : S>

fun <S : Service<S, A, M>, A : S, M : S> pick(app: A, module: M?): S = module ?: app

class InvB<I>(val v: I)
fun <@OnlyInputTypes T> strictSelect(arg1: T, arg2: T): T = arg1

fun only(first: String, any: Any) {
    strictSelect(InvB(first), InvB(first))
    strictSelect(InvB(first), InvB(any)) // ERROR must be mentioned in input types
}
