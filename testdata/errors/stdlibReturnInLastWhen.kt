// As K2 resolves it (KT-58232), a when with only an else branch that a
// lambda ends with is one of its results, Nothing, besides the value of
// the return@label it ends with: the lambda's result variable is fixed
// before a builder call there is inferred, which can't infer its own.

fun <U> myRun(calc: () -> U): U = calc()

fun direct() = myRun {
    return@myRun buildList {
        add(1)
    }
}

fun twoBranches(b: Boolean) = myRun {
    when {
        b -> return@myRun buildList { add(2) }
        else -> return@myRun buildList { add(3) }
    }
}

fun oneBranch() = myRun {
    when {
        else -> return@myRun buildList { // ERROR cannot infer type for type parameter 'E'. Specify it explicitly.
            add(4) // ERROR type mismatch: inferred type is Int but Nothing was expected
        }
    }
}
