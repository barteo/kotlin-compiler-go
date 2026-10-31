expect fun withBody(): Int = 1 // ERROR 'expect' declaration cannot have a body
expect val withInit: Int = 2 // ERROR 'expect' property cannot have an initializer
expect fun noActual(): Int // ERROR the 'expect' declaration 'noActual' has no 'actual' declaration
expect class E {
    fun f(): Int = 1 // ERROR 'expect' declaration cannot have a body
    init { } // ERROR 'expect' declaration cannot have a body
}

class A {
    expect fun member(): Int // ERROR modifier 'expect' is not applicable to 'member function'
}

actual fun withBody(): Int = 1
actual val withInit: Int = 2
actual fun noExpect(): Int = 3 // ERROR 'noExpect' has no corresponding 'expect' declaration
actual class E {
    actual fun f(): Int = 1
}
