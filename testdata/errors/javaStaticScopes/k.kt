// As K2's JavaClassStaticUseSiteScope: a Kotlin class has no statics of
// its Java supertypes, but a Java class below it does. A field that a
// Java class inherits from two supertypes is ambiguous, unless it is one
// field reached along two paths.
open class K : C()

interface KI : I

fun test() {
    val c = C.a + C.b
    C.bar()
    val k1 = K.a // ERROR unresolved reference: a
    val k2 = K.b // ERROR unresolved reference: b
    K.bar() // ERROR bar
    val ki = KI.a // ERROR unresolved reference: a
    val d = D.a + D.b
    D.bar()
    val e = E.a // ERROR overload resolution ambiguity
    val f = F.a
}
