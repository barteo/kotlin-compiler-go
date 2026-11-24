// A call in a branch of an if in a builder's lambda that only the
// builder's variable gives a type to, while nothing bounds it yet, is
// inferred with the other branches: their join bounds the variable,
// whatever the lambda does after.

fun first(arg: Boolean) = buildList {
    if (arg) {
        removeLast()
    } else {
        add(42) // ERROR type mismatch: inferred type is Int but Boolean was expected
    }
}

fun afterAdd(arg: Boolean): List<Int> = buildList {
    if (!arg) {
        add(42)
    } else {
        removeLast()
    }
}

fun bounded(arg: Boolean): List<Int> = buildList {
    add(1)
    if (arg) removeLast() else add(2)
}

fun value(arg: Boolean): List<Int> = buildList {
    val x = if (arg) removeLast() else 3
    add(x)
}
