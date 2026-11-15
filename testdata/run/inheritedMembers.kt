// Overrides that kotlinc accepts and kc once rejected.

// A var may override a val of a supertype with a subtype.
abstract class A {
    abstract val a: CharSequence
}

abstract class B : A() {
    abstract override val a: String
}

class C : B() {
    override var a: String = "c"
        private set
}

// Generic member extension properties override by position.
class Inv<E>(val e: E)

abstract class P {
    open val <D> Inv<D>.unwrapped: D get() = e
}

class Q : P() {
    override val <T> Inv<T>.unwrapped: T get() = e
    fun get(x: Inv<String>) = x.unwrapped
}

fun main() {
    println(C().a)
    println(Q().get(Inv("q")))
    defaults()
}

// A default value inherited through another supertype is a deprecation
// warning only.
interface D1 {
    fun foo(a: String = "d1"): String
}
interface D2 : D1
interface D3 {
    fun foo(a: String = "d3"): String
}
class D : D2, D3 {
    override fun foo(a: String) = a
}

fun defaults() {
    println(D().foo("d"))
}
