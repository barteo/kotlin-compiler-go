// The checks of inherited members: visibility, default values and
// interfaces that implement Any's methods.

interface Shape {
    val area: Int
    var name: String
    fun draw()
}

class Square : Shape {
    internal override val area: Int = 1 // ERROR cannot weaken access privilege 'public'
    override var name: String = ""
        private set // ERROR cannot weaken access privilege 'public'
    protected override fun draw() {} // ERROR cannot weaken access privilege 'public'
}

open class Base {
    protected open fun f() {}
    internal open fun g() {}
}

class Derived : Base() {
    internal override fun f() {} // ERROR cannot change access privilege 'protected'
    protected override fun g() {} // ERROR cannot change access privilege 'internal'
}

// An override without a visibility has the overridden member's.
open class Mid : Base() {
    override fun g() {}
}

class Leaf : Mid() {
    internal override fun g() {}
}

interface X {
    fun foo(a: Int = 1)
}

interface Y {
    fun foo(a: Int = 2)
}

class XY : X, Y {
    override fun foo(a: Int) {} // ERROR more than one overridden descriptor declares a default value for 'a'
}

interface WithAny {
    override fun toString(): String = "" // ERROR an interface may not implement a method of 'Any'
    override fun equals(other: Any?): Boolean // abstract: allowed
    fun hashCode(x: Int): Int = x // not Any's
}
