// Private members are visible in their class and the classes nested in
// it; protected ones also in its subclasses. A private constructor is only
// visible in its class.

class Secret private constructor(val x: Int) {
    fun twin() = Secret(x)

    class Nested {
        fun make() = Secret(1)
    }
}

fun leak() = Secret(1) // ERROR cannot access 'constructor': it is private in 'Secret'

open class Box {
    private val inner = 1
    private fun peek() = inner
    protected fun guarded() = 2
    protected val shielded = 3
    var count = 0
        private set
    private class Key
    protected class Lock

    fun ok(other: Box) = other.inner + other.peek() + Key().hashCode()
}

class Crate : Box() {
    fun ok() = guarded() + shielded + Lock().hashCode()
    fun bad(b: Box) = peek() // ERROR cannot access 'peek': it is private in 'Box'
    fun reset() {
        count = 0 // ERROR cannot assign to 'count': the setter is private in 'Box'
    }
}

fun members(b: Box) {
    b.inner // ERROR cannot access 'inner': it is private in 'Box'
    b.peek() // ERROR cannot access 'peek': it is private in 'Box'
    b.guarded() // ERROR cannot access 'guarded': it is protected in 'Box'
    b.shielded // ERROR cannot access 'shielded': it is protected in 'Box'
    b.count = 1 // ERROR cannot assign to 'count': the setter is private in 'Box'
    b.count++ // ERROR cannot assign to 'count': the setter is private in 'Box'
    val r = Box::peek // ERROR cannot access 'peek': it is private in 'Box'
}

fun classes() {
    val k: Box.Key? = null // ERROR cannot access 'Box.Key': it is private in 'Box'
    Box.Lock() // ERROR cannot access 'Box.Lock': it is protected in 'Box'
}

// A private member isn't inherited: b.peek() on a subclass's instance is
// an error even in the class.
abstract class Base {
    private fun peek() = 1
    fun read(d: Derived) = d.peek() // ERROR cannot access 'peek': it is private in 'Base'
}

abstract class Derived : Base()

// A sealed class's private constructor is for its nested classes.
sealed class Shape private constructor() {
    class Circle : Shape()
}

object Square : Shape() // ERROR cannot access 'constructor': it is private in 'Shape'

data class Pair2(private val a: Int, val b: Int)

fun destructure(p: Pair2) {
    val (a, b) = p // ERROR cannot access 'component1': it is private in 'Pair2'
}
