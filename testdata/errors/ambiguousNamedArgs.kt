// A member inherited from several supertypes that name a parameter
// differently takes no named argument for it, as kotlinc.
interface A {
    fun foo(x: Int, z: Int)
}

interface B {
    fun foo(y: Int, z: Int)
}

interface C : A, B

interface D : C

abstract class E : A, B {
    override fun foo(x: Int, z: Int) {}
}

abstract class F : E()

interface G<T> {
    fun bar(a: T)
}

interface H {
    fun bar(b: Int)
}

interface I : G<Int>, H

interface Same1 {
    fun baz(p: Int)
}

interface Same2 {
    fun baz(p: Int)
}

interface Same : Same1, Same2

fun calls(c: C, d: D, f: F, i: I, s: Same) {
    c.foo(x = 0, z = 1) // ERROR named argument is prohibited for parameter with an ambiguous name
    // kc calls A's foo, which has no y: kotlinc's message is the one above.
    c.foo(0, y = 1) // ERROR cannot find a parameter with this name: y
    d.foo(y = 0, z = 1) // ERROR cannot find a parameter with this name: y
    d.foo(x = 0, z = 1) // ERROR named argument is prohibited for parameter with an ambiguous name
    i.bar(a = 1) // ERROR named argument is prohibited for parameter with an ambiguous name
    c.foo(0, z = 1)
    c.foo(0, 1)
    f.foo(x = 0, z = 1)
    s.baz(p = 1)
}

// Overloads of one class whose signatures clash once substituted aren't
// one member: each takes its own names.
open class Base<T> {
    fun qux(x: T) = "x:$x"
    fun qux(y: String) = "y:$y"
}

class Derived : Base<String>()

fun overloads(b: Base<String>, d: Derived) {
    println(b.qux(x = "O") + b.qux(y = "K"))
    println(d.qux(x = "O") + d.qux(y = "K"))
}
