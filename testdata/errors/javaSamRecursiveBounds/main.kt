// A lambda converted to a star-projected Java interface whose type
// parameters' bounds are a cycle gets no type for its parameters, as in
// Kotlin's javaSamTypeWithRecursionInBounds.kt test.

fun named() {
    A.foo { a -> // ERROR cannot infer a type for this parameter
        null!!
    }
}

fun implicit() {
    A.foo { // ERROR cannot infer a type for this parameter
        null!!
    }
}
