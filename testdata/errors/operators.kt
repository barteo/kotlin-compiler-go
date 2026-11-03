// Operator conventions: missing operator modifiers, operators that don't
// apply, and wrong operator declarations.

class V(val x: Int) {
    fun plus(o: V) = V(x + o.x)
    operator fun minus(o: V) = V(x - o.x)
    operator fun compareTo(o: V): String = "no" // ERROR 'operator' modifier is inapplicable on this function: must return Int
    operator fun contains(o: V): Int = 0 // ERROR must return Boolean
    operator fun frobnicate() = 1 // ERROR 'operator' modifier is inapplicable on this function: illegal function name
    operator fun unaryMinus(o: V) = this // ERROR 'operator' modifier is inapplicable on this function: must have no value parameters
    operator fun get() = 0 // ERROR must have at least a single value parameter
}

operator fun topLevel(a: Int) = a // ERROR 'operator' modifier is inapplicable on this function: illegal function name
operator fun plus(a: Int) = a // ERROR 'operator' modifier is inapplicable on this function: must be a member or an extension function

class Both(var n: Int) {
    operator fun plus(k: Int) = Both(n + k)
    operator fun plusAssign(k: Int) {
        n += k
    }
}

class OnlyPlus(val n: Int) {
    operator fun plus(k: Int) = OnlyPlus(n + k)
    operator fun times(k: Int) = "not a OnlyPlus"
}

fun main() {
    val a = V(1)
    println(a + a) // ERROR 'operator' modifier is required on 'plus'
    println(a - a)
    println(a * a) // ERROR unresolved reference: times
    println(-a) // ERROR no function 'unaryMinus' takes 0 arguments
    println(a < a) // ERROR 'compareTo' must return Int, not String
    println(a in a) // ERROR 'contains' must return Boolean
    println(a(1)) // ERROR expression 'a' of type V cannot be invoked as a function
    var both = Both(1)
    both += 1 // ERROR assignment operators ambiguity
    val bothVal = Both(1)
    bothVal += 1
    val p = OnlyPlus(1)
    p += 1 // ERROR val cannot be reassigned
    var q = OnlyPlus(1)
    q += 1
    q *= 2 // ERROR type mismatch
    var v = V(1)
    v++ // ERROR unresolved reference: inc
}

class NullableIt {
    operator fun iterator(): Steps? = null
}

class Steps {
    operator fun hasNext() = false
    operator fun next() = 1
}

interface Left
interface Right

class Both2 : Left, Right

class Ambiguous {
    operator fun iterator() = Both2()
}

operator fun Left.hasNext() = false
operator fun Right.hasNext() = false
operator fun Both2.next() = 1

class Counter

class Scope {
    operator fun Counter.iterator() = Steps()

    fun loop() {
        for (x in Counter()) {
        }
    }
}

fun loops() {
    for (x in NullableIt()) { // ERROR the iterator of the for-loop range has no 'hasNext()' or 'next()' method
    }
    for (x in Ambiguous()) { // ERROR ambiguous call
    }
    for (x in Counter()) { // ERROR for-loop range must have an 'iterator()' method
    }
}
