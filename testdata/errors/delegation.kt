interface I {
    fun f(): Int
}

interface J {
    fun f(): Int
    fun g(): String
}

open class Impl : I {
    override fun f(): Int = 1
}

class A(i: Impl) : Impl by i // ERROR delegation is supported only for interfaces

class B(s: String) : I by s // ERROR type mismatch

class C(i: I, j: J) : I by i, J by j // ERROR class 'C' must override 'f' because it inherits multiple implementations for it

class D(i: I, j: J) : I by i, J by j {
    override fun f(): Int = 2
}

class E(i: I) : I by i {
    val x = 1
}

class F : I by this // ERROR 'this' is not defined

interface K : I by Impl() // ERROR delegation cannot be used in interfaces

object O : I by Impl()

class G(i: I) : I by i {
    fun h(): Int = f() + y // ERROR unresolved reference: y
}
