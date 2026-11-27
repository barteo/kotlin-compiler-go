// A call whose variable another call's uninferred variable alone bounds
// can't be inferred either, as in K2, which reports each.

fun <K> id(arg: K): K = arg
fun <M> materialize(): M = TODO()
fun <T : Number> materializeNumber(): T = TODO()
fun <R> myRun(block: () -> R): R = block()
fun <T, R> myWith(t: T, block: T.() -> R): R = t.block()

fun nested() {
    id(id(materialize())) // ERROR cannot infer type for type parameter 'K'
    val x = myRun { materialize() } // ERROR cannot infer type for type parameter
}

fun branches(b: Boolean) {
    id(if (b) { // ERROR cannot infer type for type parameter 'K'
        id(unresolved) // ERROR unresolved reference: unresolved
    } else {
        id( // ERROR cannot infer type for type parameter 'K'
            materialize() // ERROR cannot infer type for type parameter 'M'
        )
    })
}

class Ex

fun lambdas(x: Any) {
    myWith(x) { // ERROR cannot infer type for type parameter 'R'
        myWith(Ex()) { // ERROR cannot infer type for type parameter 'R'
            foo() // ERROR unresolved reference: foo
        }
    }
}

// A lambda coerced to the Unit expected of it, through the variable of a
// call it ends with, leaves its last call's variable that can't be a Unit
// uninferred.
fun coerced(): Unit = myRun {
    myRun {
        materializeNumber() // ERROR cannot infer type for type parameter 'T'
    }
}

fun coercedInfers(): Unit = myRun {
    myRun {
        materialize()
    }
}
