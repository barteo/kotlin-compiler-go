// An override of a package-private Java method declared without a
// visibility is protected, as in K2, and on the JVM.
open class B : A() {
    override fun foo(s: String): Any? {
        return null
    }

    public override fun bar() {}

    fun own() = foo("")
}

class C : B() {
    override fun foo(s: String): Any? = null

    fun sub(b: B) = super.foo("")
}

fun test(a: A, b: B, c: C) {
    a.foo("")
    b.foo("") // ERROR it is protected in 'B'
    c.foo("") // ERROR it is protected in 'C'
    b.bar()
}
