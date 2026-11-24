// The left side of a callable reference can be the type of an inner class
// of a generic class, in parentheses too, and kotlinc takes its classes'
// type arguments where they don't fit, with a warning, the innermost
// class's first.
class W<T> {
    inner class A<K> {
        fun foo() {}
    }
}

class Outer<A> {
    inner class Middle<B> {
        inner class Inner<C>
    }
}

fun Outer<Int>.Middle<String>.Inner<Char>.foo() {}

fun test() {
    (W<String, Int>.A)::foo
    (W<String, Int>).A::foo
    (W.A<String, Int>)::foo
    (W).A<String, Int>::foo
    (W<String>).A<Int>::foo
    W<String>.A<Int>::foo

    Outer<Int>.Middle<String>.Inner<Char>::foo
    Outer<Int>.Middle.Inner<Char, String>::foo
    Outer.Middle.Inner<Char, String, Int>::foo
    Outer<Char, String, Int>.Middle.Inner::foo
    Outer.Middle<Int>.Inner<Char, String>::foo
    Outer<Int>.Middle<Int>.Inner<Char>::foo // ERROR type mismatch
}
