// A when whose value is used, with a null branch that doesn't jump: the
// subject may still be null after it. A literal branch's type is only
// settled with the others', but it doesn't jump.

fun literals(x: CharSequence?): Int {
    val i = when (x) { is Any -> 1; null -> 2 }
    return x.length + i // ERROR only safe (?.) or non-null asserted
}

fun nullFirst(x: CharSequence?): Int {
    val i = when (x) { null -> 2; is Any -> 1 }
    return x.length + i // ERROR only safe (?.) or non-null asserted
}

fun compound(x: CharSequence?): Int {
    var i = 0
    i += when (x) { is CharSequence -> 1; null -> 2 }
    return x.length + i // ERROR only safe (?.) or non-null asserted
}

fun jumps(x: CharSequence?): Int {
    val i = when (x) { null -> return 2; else -> 1 }
    return x.length + i
}
