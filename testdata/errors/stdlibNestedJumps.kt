// A jump that completes the expression it is in, or that may not run,
// doesn't make the function's body end in one.

fun returnFromLambda(): Int {
    run { return@run 1 }
} // ERROR missing return statement

fun inElvis(s: String?): Int {
    s?.length ?: return 0
} // ERROR missing return statement

fun inAnd(b: Boolean): Int {
    b && return 1
} // ERROR missing return statement

fun inSafeCall(s: String?): Int {
    s?.plus(return 1)
} // ERROR missing return statement

fun notCalledInPlace(): Int {
    lazy { throw IllegalStateException() }
} // ERROR missing return statement

fun continued(): Int {
    var i = 0
    do {
        i++
        if (i < 3) continue
        return i
    } while (i < 2)
} // ERROR missing return statement
