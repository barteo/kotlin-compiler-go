external fun withBody() {} // ERROR external declaration cannot have a body

external class K // ERROR modifier 'external' is not applicable to 'class'

external val p: Int = 1 // ERROR modifier 'external' is not applicable to 'property'

class Params(external q: String) { // ERROR modifier 'external' is not applicable to 'value parameter'
    external constructor() : this("") // ERROR modifier 'external' is not applicable to 'constructor'

    external fun f(external x: Int) // ERROR modifier 'external' is not applicable to 'value parameter'
}

class Primary external constructor() // ERROR modifier 'external' is not applicable to 'constructor'

abstract class A {
    abstract external fun f() // ERROR external declaration cannot be abstract

    abstract val x: Int // ERROR external declaration cannot be abstract
        external get

    inline external fun g() // ERROR inline functions cannot be external
}

interface I {
    external fun f() // ERROR members of interfaces cannot be external

    val x: Int
        external get // ERROR members of interfaces cannot be external

    companion object {
        external fun ok()
        @JvmStatic external fun f() // ERROR '@JvmStatic' annotation cannot be used on 'external' members of interface companions
    }
}

class Accessors {
    val ok: Int
        external get

    var both: Int
        external get
        external set

    var getter: Int // ERROR must be initialized
        external get

    val body: Int
        external get() = 1 // ERROR external declaration cannot have a body
}

fun local() {
    class L {
        external fun ok()
    }
    object {
        external fun ok()
    }
}
