class Box<T>(val value: T)

fun <T> isT(x: Any): Boolean = x is T // ERROR cannot check for instance of erased type: T

fun erased(x: Any): Boolean = x is Box<String> // ERROR cannot check for instance of erased type: Box<String>

fun known(x: Box<String>): Boolean = x is Box<String>

fun star(x: Any): Boolean = x is Box<*>

fun noCast(x: Any): Int {
    if (x is String) {
        return x.length
    }
    return x.length // ERROR unresolved reference: length
}

fun varCast(): Int {
    var x: Any = "s"
    if (x is String) {
        x = 1
        return x.length // ERROR unresolved reference: length
    }
    return 0
}

fun whenNoSubject(x: Any): Int = when {
    is String -> 1 // ERROR 'is' conditions need a 'when' subject
    else -> 2
}

fun notExhaustive(b: Boolean): Int = when (b) { // ERROR 'when' expression must be exhaustive
    true -> 1
}

fun function(x: Any): Boolean = x is () -> Unit // ERROR cannot check for instance of erased type

fun String.joined(s: String) = this + s

// A cast narrows after the statement only where it always runs.
fun castsThatMayNotRun(x: Any, y: String?, z: Any, w: Any, b: Boolean) {
    val ok = b || (x as String).length > 0
    println(x.length) // ERROR unresolved reference: length
    y?.joined(z as String)
    println(z.length) // ERROR unresolved reference: length
    val v = y ?: (w as String)
    println(w.length) // ERROR unresolved reference: length
}
