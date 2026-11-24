// Errors of the checks added while checking kotlinpoet.

interface A {
    fun f(): Int = 1
}

interface B {
    fun f(): Int = 2
}

class C : A, B {
    override fun f() = super.f() // ERROR many supertypes available
}

class D : A, B {
    override fun f() = super<A>.f() + super<B>.f()
}

class Hidden {
    @Deprecated("", level = DeprecationLevel.HIDDEN)
    fun old() {}
}

fun hidden() {
    Hidden().old() // ERROR unresolved reference: old
}

fun f(a: Int, b: Int = 0, c: Int = 0) {}

fun mixed() {
    f(a = 1, 2)
    f(b = 1, 2) // ERROR mixing named and positioned arguments is not allowed
}

fun branches(): Int = if (true) 1 else "two" // ERROR type mismatch: inferred type is String but Int was expected

fun noBody(): Int // ERROR function 'noBody' must have a body

// An unresolved receiver isn't left out of the signature.
fun Nope.unknownRecv(x: Int) {} // ERROR unresolved
fun unknownRecv(x: Int) {}
