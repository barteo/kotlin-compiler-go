// As K2, a call on a variable smart-cast to a subclass leaves out the
// subclass's protected overrides that the code can't call on it, and has
// the declared type's members of their parameters instead.
open class Base
class Derived : Base()

interface M1
interface M2
interface M3
interface M3Sub : M3

open class A {
    public fun foo(a1: Base, a2: Base): M1 = TODO()
    protected open fun foo(a1: Base, a2: Derived): M2 = TODO()
    protected open fun foo(a1: Derived, a2: Derived): M3 = TODO()
}

open class B : A() {
    fun f(a: A, b: B, c: C, d: Derived) {
        val m1: M1 = a.foo(d, d)
        val m3: M3 = b.foo(d, d)
        val m2: M2 = c.foo(d, d)
        if (b is C) {
            val n3: M3 = b.foo(d, d)
            val n3sub: M3Sub = b.foo(d, d) // ERROR type mismatch
        }
        if (a is C) {
            val n2: M2 = a.foo(d, d)
        }
    }
}

class C : B() {
    override fun foo(a1: Derived, a2: Derived): M3Sub = TODO()
}
