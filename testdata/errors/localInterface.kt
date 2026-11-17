// Kotlin has no local interfaces: the checker reports them, as kotlinc's
// LOCAL_INTERFACE_NOT_ALLOWED and NESTED_CLASS_NOT_ALLOWED, and goes on.
fun foo() {
    interface A {} // ERROR is an interface so it cannot be local
    val b = object {
        interface C {} // ERROR prohibited here
    }
    class D {
        interface E {} // ERROR prohibited here
    }
    val f = {
        interface G {} // ERROR is an interface so it cannot be local
    }
    fun interface H { fun h() } // ERROR is an interface so it cannot be local
    private fun interface I { fun i() } // ERROR is an interface so it cannot be local
    val x: Int = "" // ERROR type mismatch
}
