// A jump that completes the expression it is in, or that may not run,
// doesn't make the function's body end in one.

fun returnFromLambda(): Int { // ERROR missing return statement
    run { return@run 1 }
}

fun inElvis(s: String?): Int { // ERROR missing return statement
    s?.length ?: return 0
}

fun inAnd(b: Boolean): Int { // ERROR missing return statement
    b && return 1
}

fun inSafeCall(s: String?): Int { // ERROR missing return statement
    s?.plus(return 1)
}

fun notCalledInPlace(): Int { // ERROR missing return statement
    lazy { throw IllegalStateException() }
}

fun continued(): Int { // ERROR missing return statement
    var i = 0
    do {
        i++
        if (i < 3) continue
        return i
    } while (i < 2)
}
