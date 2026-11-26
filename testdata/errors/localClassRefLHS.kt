// On a callable reference's left side, kotlinc 2.4 wants type arguments
// for a local class's own type parameters and for those it captures of
// the classes around it, not of the functions.

class C<U, V> {
    fun <T> foo() {
        class L
        val a = L::toString // ERROR 2 type arguments expected for class C.foo.L
        val b = L<Int>::toString // ERROR 2 type arguments expected for class C.foo.L
        val c = L<Int, Long>::toString
    }
    fun bar() {
        class M
        val f = M<Int, Int>::toString
    }
}

fun <T> top() {
    class N
    val g = N<Int>::toString // ERROR no type arguments expected for class
    val h = N::toString
}
