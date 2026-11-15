// As in K2, the static scope of a class body is that of the class and of
// its superclasses: a field of a Java interface the class implements
// directly isn't in it, but one a Java superclass inherits is. The
// companions of superclasses are implicit receivers for extensions too.
class Direct : I {
    fun f() = a // ERROR unresolved reference: a
}

class ViaJava : C() {
    fun f() = a + b
}

// A public static of a package-private class, through a public subclass.
class Impl : p.Derived() {
    fun g() = baz()
}

open class A {
    companion object
}

fun A.Companion.foo() = 1

class B : A() {
    fun g(): Int = foo()
}

// In a delegation call, the nested object C comes before the companion's
// val C.
class Q {
    companion object {
        val C = 3
    }
    object C {}

    constructor(x: Int)
    constructor() : this(C) // ERROR type mismatch
}
