// What a call infers for a variable annotated @OnlyInputTypes must be
// the type of one of its inputs.

class Inv<T>
open class A
class B : A()

@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
fun <@kotlin.internal.OnlyInputTypes T> Inv<out T>.has(element: T): Boolean = false

@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
fun <@kotlin.internal.OnlyInputTypes T> same(a: T, b: T): Boolean = false

fun main() {
    val inv = Inv<A>()
    val b: B? = null
    inv.has(B())
    inv.has(A())
    inv.has<A?>(b)
    inv.has(b) // ERROR type inference failed. The value of the type parameter 'T' must be mentioned in input types
    same(A(), B())
    same("", inv) // ERROR type inference failed. The value of the type parameter 'T' must be mentioned in input types
}

// An integer literal's inputs are Int, and Long, Short and Byte where it
// fits; a projected type is compared as it is.
@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
fun <@kotlin.internal.OnlyInputTypes T> Inv<T>.get1(element: T): Boolean = false

fun literals(ints: Inv<Int>, longs: Inv<Long>, strings: Inv<String>, numbers: Inv<Inv<Number>>, anys: Inv<Inv<Any>>, x: Inv<out Number>) {
    ints.get1(42)
    longs.get1(42)
    longs.get1(42L)
    strings.get1(42) // ERROR type mismatch
    numbers.has(x)
    anys.has(x) // ERROR type inference failed. The value of the type parameter 'T' must be mentioned in input types
}

// A receiver's type argument annotated @Exact is the receiver's own, not
// a supertype of it: V is Int for an Out<Int>, which V1 : V? bounds.
class ExactOut<out T>

@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
fun <V, V1 : V?> ExactOut<@kotlin.internal.Exact V>.orDefault(defaultValue: V1): V1 = defaultValue

fun exactReceiver(o: ExactOut<Int>) {
    val r1: Int? = o.orDefault(null)
    val r2: Int? = o.orDefault(null as Int?)
    o.orDefault(1)
    o.orDefault("string") // ERROR argument type mismatch: actual type is String but Int? was expected
}

// A type parameter in scope is an input as any type: has(1) of an
// Inv<T> infers Any, neither T nor Int.
fun <T> params(inv: Inv<T>, t: T) {
    inv.has(t)
    inv.has(1) // ERROR type inference failed. The value of the type parameter 'T' must be mentioned in input types
}

// The Unit of an empty lambda isn't an input, nor is Unit a Nothing's.
@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
fun <@kotlin.internal.OnlyInputTypes T> expect(expected: T, block: () -> T) {}

@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
fun <@kotlin.internal.OnlyInputTypes T> expect(block: () -> T) {}

fun nothing(): Nothing = null!!
fun anyValue(): Any = ""

fun emptyLambdas() {
    expect { Unit }
    expect {} // ERROR type inference failed. The value of the type parameter 'T' must be mentioned in input types
    expect(anyValue()) {}
    expect(nothing()) { Unit }
    expect(nothing()) {} // ERROR type inference failed. The value of the type parameter 'T' must be mentioned in input types
}
