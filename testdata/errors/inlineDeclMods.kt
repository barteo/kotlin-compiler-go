inline fun run0(f: () -> Unit) = f()

inline fun okMods(noinline f: () -> Unit, crossinline g: () -> Unit, noinline h: (() -> Unit)? = null) {
    run0 { g() }
    f()
}

inline fun okDefault(f: () -> Unit = {}, g: (Int) -> Int = { it }) {}

inline fun wrongType(noinline x: Int) {} // ERROR modifier 'noinline' is allowed only for function parameters of an inline function

fun notInline(crossinline f: () -> Unit) {} // ERROR modifier 'crossinline' is allowed only for function parameters of an inline function

inline fun nullable(f: (() -> Unit)?) {} // ERROR inline-parameter 'f' of 'nullable' must not be nullable. Add 'noinline' modifier to the parameter declaration or make its type not nullable

fun mk(): () -> Unit = {}

inline fun badDefault(f: () -> Unit = mk()) {} // ERROR invalid default value for an inline functional parameter: only lambdas, anonymous functions and callable references are supported

fun outer() {
    fun <reified T> local() {} // ERROR only type parameters of inline functions can be reified
}

inline fun cycleA() { cycleB() } // ERROR the call of 'cycleB' is a part of an inline call cycle
inline fun cycleB() { cycleA() } // ERROR the call of 'cycleA' is a part of an inline call cycle

inline fun viaObject() {
    object {
        fun g() = viaObject() // ERROR the call of 'viaObject' is a part of an inline call cycle
    }
}

val selfProp: Int
    inline get() = selfProp + 1 // ERROR the call of 'selfProp' is a part of an inline call cycle
