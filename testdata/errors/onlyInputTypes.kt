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
