// A generic class's inner class written qualified needs the class's own
// type arguments even in its code; its outer classes' are in scope.
// A nested class that isn't inner hides a superclass's inner class of
// its name.

class G<T> {
    inner class A { inner class B }
    fun f(x: A.B, y: G.A, z: G<T>.A.B, w: A) {} // ERROR one type argument expected for class G
}

open class Base {
    inner class Inner
}
class Sub : Base()
class Hiding : Base() {
    class Inner
}

fun test() {
    Sub().Inner()
    Hiding().Inner() // ERROR unresolved reference: Inner
    Hiding.Inner()
}
