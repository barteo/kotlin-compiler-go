// A local var that a closure may assign isn't smart cast: kotlinc reports
// the impossible smart cast, and goes on with the smart-cast type. Not of
// one that is checked once the closure assigns it: there is no smart cast.

fun run(f: () -> Unit) = f()

fun nullable(a: String?) {
    var x = a
    if (x != null) {
        run { x.length } // ERROR smart cast to 'String' is impossible, because 'x' is a local variable that is mutated in a capturing closure
        x = null
    }
}

fun any(a: Any) {
    var y = a
    if (y is String) {
        run { y.length } // ERROR smart cast to 'String' is impossible, because 'y' is a local variable that is mutated in a capturing closure
        run { y.get(0) } // ERROR smart cast to 'String' is impossible, because 'y' is a local variable that is mutated in a capturing closure
        // Nor are the libraries' extensions resolved on it, as in kotlinc.
        run { y.uppercase() } // ERROR unresolved reference: uppercase
        run { y.nope } // ERROR unresolved reference: nope
        y = 1
    }
}

fun checkedAfter(a: String?): Int {
    var x = a
    val f = { x = null }
    if (x != null) return x.length // ERROR only safe (?.) or non-null asserted
    f()
    return 0
}
