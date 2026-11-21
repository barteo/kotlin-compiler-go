// The right side of false && b, or of true || b, never runs: it
// smart-casts nothing.
class A { fun foo() {} }

fun f1(x: Any) { if (false && x is A) { x.foo() } } // ERROR unresolved reference: foo
fun f2(x: Any) { if (!(true || x !is A)) { x.foo() } } // ERROR unresolved reference: foo
fun f3(x: Any) { if (x is A && true) { x.foo() } }
fun f4(x: Any) { if (x !is A || false) { } else { x.foo() } }

fun main() {}
