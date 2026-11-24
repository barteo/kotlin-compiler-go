// The program's classes take the place of the library's of their names
// in the library's signatures and supertypes, as in K2.
package p

class A {
    fun foo() {}
}

interface Base {
    fun bar()
    fun baz()
}

interface ToSubstitute : Intermediate // ERROR cycle in the inheritance hierarchy

fun test() {
    val a: A = B().a
    a.foo()
    B().a.foo()

    abstract class FooImpl : Foo() {
        fun usage() {
            bar()
        }

        override fun baz() {}
    }
}

class Impl : Foo() // ERROR does not implement abstract member
