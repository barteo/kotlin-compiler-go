// A syntax error doesn't hide the type errors in the rest of the file, as
// it doesn't with kotlinc: the declarations parsing recovers are checked.

fun ok(): Int = 1

fun broken(x: Int { // ERROR Expecting comma or ')'
    return x
}

fun typeError(): Int = "a" // ERROR type mismatch: inferred type is String but Int was expected

class C {
    fun alsoBroken( = 2 // ERROR Expecting ')'
    fun member(): String = 1 // ERROR type mismatch: inferred type is Int but String was expected
}

fun usesOk(): String = ok() // ERROR type mismatch: inferred type is Int but String was expected

// A for loop without its range is kept, as FIR keeps it, with a range of
// no type.
fun noRange(a: IntArray): Int {
    for // ERROR for-loop range must have an 'iterator()' method
    (n // ERROR Expecting 'in'
    ) {
        val s: String = 1 // ERROR type mismatch: inferred type is Int but String was expected
    }
} // ERROR missing return statement

// A primary constructor's modifiers without the constructor keyword and
// parameters are kept, as FIR keeps them.
class NoCtorKeyword
private // ERROR use the 'constructor' keyword after the modifiers of the primary constructor
@Suppress("x") { // ERROR Expecting 'constructor' keyword
    fun f(): String = 1 // ERROR type mismatch: inferred type is Int but String was expected
}
