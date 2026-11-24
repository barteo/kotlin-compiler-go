// A member of an intersection whose components declare it with a Java
// method's platform result and a Kotlin result returns the Kotlin one,
// whichever component the value was cast to first, as in K2.

interface B { fun foo(): String? }
interface C { fun foo(): String }

fun f(x: Any?) {
    if (x is A && x is B) { val s: String = x.foo() } // ERROR type mismatch
    if (x is B && x is A) { val s: String = x.foo() } // ERROR type mismatch
    if (x is A && x is C) { val s: String = x.foo() }
    if (x is C && x is A) { val s: String = x.foo() }
    if (x is A) { val s: String = x.foo() }
}
