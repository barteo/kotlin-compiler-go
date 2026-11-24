// An interface's static methods are public, so @JvmStatic in an
// interface's companion object needs a public member, and a public
// setter (JVM_STATIC_ON_NON_PUBLIC_MEMBER), reported at the annotation.
interface B {
    companion object {
        @JvmStatic fun a1() {}
        @JvmStatic private fun a2() {} // ERROR only public members in interface companion objects
        @JvmStatic internal fun a4() {} // ERROR only public members in interface companion objects

        @JvmStatic
        var foo = 1

        @JvmStatic // ERROR only public members in interface companion objects
        var foo1 = 1
            protected set

        @JvmStatic // ERROR only public members in interface companion objects
        private var foo3 = 1

        val foo7 = 1
            @JvmStatic get

        public var foo9 = 1
            @JvmStatic private set // ERROR only public members in interface companion objects
    }
}

object O {
    @JvmStatic private fun p() {}
    @JvmStatic private var v = 1
}
