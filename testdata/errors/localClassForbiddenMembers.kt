// A local class or an object expression's class with a nested class
// Kotlin doesn't allow there is still checked, as kotlinc checks it, with
// the nested class as the class it would be; and cycles among local
// classes are found as among the file's.

interface P<U, Y>

class A<T> {
    class B {
        fun test() {
            class C<W>() : P<W, T> { // ERROR unresolved or unsupported type: T
                companion object : P<Int, Int> // ERROR modifier 'companion' is not applicable inside 'local class'
                inner class D : P<W, T> // ERROR unresolved or unsupported type: T
            }
        }
    }
}

val oo = object {
    class Nested // ERROR 'Class' is prohibited here
    fun f1() = Nested(11) // ERROR takes 1 arguments
}

fun cycle() {
    open class Local {
        open inner class A : C() { // ERROR cycle in the inheritance hierarchy
            abstract inner class Inner
        }
        abstract inner class C : A.Inner() // ERROR cycle in the inheritance hierarchy
    }
}

fun <T> sees() {
    class C {
        class D { // ERROR 'Class' is prohibited here
            fun c(): C = C()
        }
    }
}
