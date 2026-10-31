// Errors of the checks added while checking kotlinpoet.

class Writer {
    fun emit(xs: java.util.ArrayList<String>) {}
    fun emit(xs: java.util.ArrayList<Int>) {} // ERROR platform declaration clash
}

fun top(xs: java.util.ArrayList<String>) {}
fun top(xs: java.util.ArrayList<Int>) {} // ERROR platform declaration clash

@JvmName("topNumbers")
fun top2(xs: java.util.ArrayList<Int>) {}
fun top2(xs: java.util.ArrayList<String>) {}

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
    Hidden().old() // ERROR no function 'old' takes 0 arguments
}

fun f(a: Int, b: Int = 0, c: Int = 0) {}

fun mixed() {
    f(a = 1, 2)
    f(b = 1, 2) // ERROR mixing named and positioned arguments is not allowed
}

fun branches(): Int = if (true) 1 else "two" // ERROR type mismatch: inferred type is String but Int was expected

fun noBody(): Int // ERROR function 'noBody' must have a body
