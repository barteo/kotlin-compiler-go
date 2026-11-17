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
