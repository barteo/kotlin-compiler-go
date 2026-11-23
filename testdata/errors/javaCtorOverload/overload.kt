// CONFLICTING_OVERLOADS: a function and a Java class's constructor of
// one package and signature, reported on the function only.
package pkg

fun J(s: String) {} // ERROR conflicting overloads: J
fun J(i: Int) {}
