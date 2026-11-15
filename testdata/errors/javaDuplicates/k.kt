// As in K2, two members of one Java class that differ only in their
// result are ambiguous, and so are two fields of one name. A reference
// to a Java class's method has the shapes of its static methods too.
fun test(a: A) {
    a.foo() // ERROR ambiguous call
    A.bar() // ERROR ambiguous call
    a.f // ERROR overload resolution ambiguity
}

fun foo1(x: (String) -> Int) {}

fun foo3(x: (A, CharSequence) -> Int) {}
fun foo3(x: (String) -> Int) {}

fun refs() {
    foo1(A::baz)
    foo3(A::baz) // ERROR ambiguous call
}
