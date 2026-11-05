// What a when over a value can leave out only holds while nothing else
// can have happened to it.
enum class E { A, B, C }

fun reassigned(e0: E, e1: E): Int {
    var e = e0
    if (e == E.A) return 1
    e = e1
    return when (e) { // ERROR 'when' expression must be exhaustive, add necessary 'else' branch
        E.B -> 2
        E.C -> 3
    }
}

fun notAlone(e: E, c: Boolean): Int {
    if (e == E.A && c) return 1
    return when (e) { // ERROR 'when' expression must be exhaustive, add necessary 'else' branch
        E.B -> 2
        E.C -> 3
    }
}

fun noJump(e: E): Int {
    if (e == E.A) println()
    return when (e) { // ERROR 'when' expression must be exhaustive, add necessary 'else' branch
        E.B -> 2
        E.C -> 3
    }
}

fun eitherOne(e: E, c: Boolean): Int {
    if (c || e == E.A) return 1
    if (e == E.B) return 2
    return when (e) {
        E.C -> 3
    }
}

fun eitherSide(e: E): Int {
    require(e == E.A || e == E.B)
    return when (e) { // ERROR 'when' expression must be exhaustive, add necessary 'else' branch
        E.A -> 1
    }
}
