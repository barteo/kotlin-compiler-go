// As in K2, a smart-cast variable's protected override, which the code
// doesn't see by Kotlin's rule for protected members, gives way to the
// declared type's member: other.a of other: A smart-cast to B, in A's
// code, is A's a.
interface Base { fun baseFun() }
interface Derived : Base { fun derivedFun() }

abstract class A {
    abstract protected val a: Base

    fun fest1(other: A) {
        other.a.baseFun()
        if (other is B) {
            other.a.baseFun()
            other.a.derivedFun() // ERROR unresolved reference
        }
    }

    open class B(override val a: Derived) : A() {
        class Nested {
            fun fest3(other: A) {
                if (other is B) other.a.derivedFun()
                if (other is C) other.a.derivedFun() // ERROR unresolved reference
            }
        }
    }

    class C(override val a: Derived) : B(a) {
        fun fest4(other: A) {
            if (other is B) other.a.derivedFun() // ERROR unresolved reference
            if (other is C) other.a.derivedFun()
        }
    }
}
