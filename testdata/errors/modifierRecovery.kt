// Modifiers that only type parameters or parameters take, and modifiers on
// init blocks and catch parameters, are errors that keep their
// declaration, so that the rest of it is checked, as kotlinc does.

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
