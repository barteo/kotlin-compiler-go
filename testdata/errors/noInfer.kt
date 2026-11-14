// A @NoInfer parameter's argument adds no constraint: it is checked
// against the type the other arguments infer. A @NoInfer result type
// isn't constrained by the expected type.

interface In<in U>
interface Base
interface Derived : Base

@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
fun <T> pick(x: In<T>, y: @kotlin.internal.NoInfer T, d: T): T = d

@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
fun <T> make(fn: () -> T): @kotlin.internal.NoInfer T = fn()

fun test(x: In<Base>, base: Base, derived: Derived) {
    pick(x, derived, derived)
    pick(x, base, derived) // ERROR type mismatch: inferred type is Base but Derived was expected
    val u: Unit = make { }
    val i: Int = make { 42 }
    val v: Unit = make { 42 } // ERROR type mismatch: inferred type is Int but Unit was expected
}
