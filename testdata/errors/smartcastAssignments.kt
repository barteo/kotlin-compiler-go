// Smart casts that an assignment undoes: of a property of a reassigned
// var, and of a var a lambda assigns.

class My(val x: Int?)

inline fun run(f: () -> Unit) = f()

inline fun <T, R> T.let(f: (T) -> R): R = f(this)

fun cond(): Boolean = true

fun reassignedReceiver() {
    var y: My = My(42)
    if (y.x != null) {
        y = My(null)
        y.x.inc() // ERROR only safe (?.) or non-null asserted
    }
}

fun reassignedNullableReceiver() {
    var y: My? = My(42)
    if (y!!.x != null) {
        y = My(null)
        y!!.x.inc() // ERROR only safe (?.) or non-null asserted
    }
}

fun keptReceiver(): Int {
    var y: My = My(42)
    if (y.x != null) {
        return y.x.inc()
    }
    return 0
}

fun checkedAgain(): Int {
    var y: My = My(42)
    if (y.x != null) {
        y = My(null)
        if (y.x != null) return y.x.inc()
    }
    return 0
}

fun assignedInRun(): Int {
    var x: String? = null
    if (x != null) {
        run { x = "a" }
        return x.length // ERROR only safe (?.) or non-null asserted
    }
    return 0
}

fun assignedInLet(y: String): Int {
    var x: String? = null
    if (x != null) {
        y.let { x = it }
        return x.length // ERROR only safe (?.) or non-null asserted
    }
    return 0
}

fun assignedInSafeLet(y: String?): Int {
    var x: String? = null
    if (x != null) {
        y?.let { x = it }
        return x.length // ERROR only safe (?.) or non-null asserted
    }
    return 0
}

fun assignedInBranch(): Int {
    var x: String? = null
    if (x != null) {
        if (cond()) run { x = "a" }
        return x.length // ERROR only safe (?.) or non-null asserted
    }
    return 0
}

fun assignedWithoutLambda(): Int {
    var x: String? = null
    if (x != null) {
        if (cond()) x = "a"
        return x.length
    }
    return 0
}

fun checkedAfterLambda(): Int {
    var x: String? = null
    run { x = "a" }
    if (x != null) return x.length
    return 0
}
