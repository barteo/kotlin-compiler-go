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

// Nor does one whose bound mentions a star-projected parameter of the
// interface in its arguments: F extends Comparable<F>, or F extends
// List<E>.
fun dependentBounds() {
    B.g { x -> } // ERROR cannot infer a type for this parameter
    B.h { x -> } // ERROR cannot infer a type for this parameter
}
