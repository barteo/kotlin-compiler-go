// References to inner classes' constructors, local functions and the
// builtin types' members, and inner classes constructed on implicit and
// nullable receivers.

abstract class Base {
    inner class Inner(val s: String) {
        fun outer() = this@Base
    }
}

class Child : Base()

fun Child.make() = Inner("extension")

fun maybe(c: Child?) = c?.Inner("safe")?.s

fun main() {
    val unbound = Base::Inner
    println(unbound(Child(), "unbound").s)
    val child = Child()
    val bound = child::Inner
    println(bound("bound").outer() === child)
    println(Child().make().s)
    println(maybe(null))
    println(maybe(Child()))

    var calls = 0
    fun count(n: Int): Int {
        calls++
        return if (n == 0) calls else count(n - 1)
    }
    val r = ::count
    println(r(3))
    println(::count == ::count)
    println(::count.hashCode() == ::count.hashCode())
    println(::count.name)
    fun outer(): String {
        fun inner() = "nested"
        return (::inner)()
    }
    println(outer())

    println((Boolean::not)(true))
    println((Int::dec)(10))
    println((String::plus)("a", 1))
    println(("x"::plus)(2))
}
