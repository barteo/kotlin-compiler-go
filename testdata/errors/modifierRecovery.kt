// Modifiers that only type parameters or parameters take, and modifiers on
// init blocks and catch parameters, are errors that keep their
// declaration, so that the rest of it is checked, as kotlinc does. So is
// an enum class written without its class keyword.

fun broken(x: Int { // ERROR Expecting comma or ')'
    return x
}

class A(
    in // ERROR modifier 'in' is not applicable here
    private // ERROR modifier 'private' is not applicable to 'value parameter'
    a: Int
) {
    fun f(): String = 1 // ERROR type mismatch: inferred type is Int but String was expected
}

out // ERROR modifier 'out' is not applicable here
vararg // ERROR modifier 'vararg' is only allowed on parameters
class B {
    reified // ERROR modifier 'reified' is not applicable here
    fun g(): String = 2 // ERROR type mismatch: inferred type is Int but String was expected

    public init {} // ERROR modifier 'public' is not applicable to 'initializer'
    vararg init {} // ERROR modifier 'vararg' is only allowed on parameters

    fun h() {
        try {} catch (public e: Exception) {} // ERROR modifier 'public' is not applicable to 'value parameter'
        val s: String = 3 // ERROR type mismatch: inferred type is Int but String was expected
    }
}

class D {
    inner // ERROR modifier 'inner' is only applicable to classes
    enum
    open // ERROR 'class' keyword is expected after 'enum'
    constructor() {}

    fun f(): String = 4 // ERROR type mismatch: inferred type is Int but String was expected
}

enum Color { RED } // ERROR 'class' keyword is expected after 'enum'
val c: Color = Color.RED
val d: String = Color.RED // ERROR type mismatch: inferred type is Color but String was expected
