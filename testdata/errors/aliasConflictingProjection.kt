// A projection that conflicts with the variance in a typealias's
// expansion makes the argument a star.
class A { fun foo() {} }
class In<in T> { fun take(x: T) {} }
class Out<out T> { fun value(): T = null!! }

typealias In1<X> = In<X>
typealias Out1<X> = Out<X>

fun test(a: A,
    in3: In1<out A>, // ERROR conflicting projection
    out2: Out1<in A>, // ERROR conflicting projection
) {
    in3.take(a) // ERROR Nothing was expected
    val v: A = out2.value() // ERROR type mismatch
}

fun main() {}
