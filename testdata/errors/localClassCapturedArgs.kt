// A local class's type arguments on a callable reference's left side are
// those of the type parameters it captures after its own, the rest stars,
// as in K2. An inner class of a local class that captures a function's
// type parameter can't be checked for, as it is erased.

class C<U> {
    fun <T> foo() {
        class L

        val x = L::toString // ERROR one type argument expected
        val z = L<Int, Long>::toString // ERROR one type argument expected

        x(L())
        z(L()) // ERROR type mismatch
    }
}

fun <E> bar(y: Any): Any {
    class Outer<F> {
        inner class Inner
    }
    if (y is Outer<*>.Inner) { // ERROR cannot check for instance of erased type
        return y
    }
    return y
}

fun noParams(y: Any): Any {
    class Outer<F> {
        inner class Inner
    }
    if (y is Outer<*>.Inner) {
        return y
    }
    return y
}
