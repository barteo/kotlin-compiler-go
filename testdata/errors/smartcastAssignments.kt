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

// A local initialized with another value, but for a given type, is an
// alias of it until either is assigned: a check of one checks the other.

fun aliasOfParam(arg: Int?): Int {
    val y = arg
    if (y != null) return arg.inc()
    return 0
}

fun typedIsNoAlias(arg: Int?): Int {
    val y: Any? = arg
    if (y != null) return arg.inc() // ERROR only safe (?.) or non-null asserted
    return 0
}

fun deferredAlias(arg: Int?): Int {
    val y: Any?
    y = arg
    if (y != null) return arg.inc()
    return 0
}

fun aliasOfVar(s: String?): Int {
    var x = s
    val y = x
    if (y != null) return x.length
    return 0
}

fun aliasBothWays(s: Any) {
    var x = s
    val y = x
    if (x is String) y.length
}

fun aliasChain(s: String?): Int {
    val y = s
    val z = y
    if (z != null) return s.length
    return 0
}

fun aliasedVarAssigned(s: String?): Int {
    var x = s
    val y = x
    x = null
    if (y != null) return x.length // ERROR only safe (?.) or non-null asserted
    return 0
}

fun aliasedVarAssignedInBranch(s: String?): Int {
    var x = s
    val y = x
    if (cond()) x = null
    if (y != null) return x.length // ERROR only safe (?.) or non-null asserted
    return 0
}

fun aliasedVarAssignedInLoop(s: String?) {
    var x = s
    val y = x
    while (cond()) {
        if (y != null) x.length // ERROR only safe (?.) or non-null asserted
        x = null
    }
}

fun aliasAssignedInBranch(s: String?, t: String?): Int {
    val y: String?
    if (cond()) y = s else y = t
    if (y != null) return t.length // ERROR only safe (?.) or non-null asserted
    return 0
}

fun safeCallOfVar(s: String?): Int {
    var x = s
    val h = x?.length
    if (h != null) return x.length
    return 0
}

fun safeCallOfVarAssigned(s: String?): Int {
    var x = s
    val h = x?.length
    x = null
    if (h != null) return x.length // ERROR only safe (?.) or non-null asserted
    return 0
}

fun safeCallIntoVar(s: String?): Int {
    var h = s?.length
    if (h != null) return s.length // ERROR only safe (?.) or non-null asserted
    return 0
}

// An inlined lambda runs in place: assignments after it don't change
// what the var is in it.

inline fun inlined(f: () -> Unit) = f()

inline fun crossInlined(crossinline f: () -> Unit) = f()

fun escaping(f: () -> Unit) = f()

fun readInInlined(arg: Int?) {
    var x = arg
    if (x == null) return
    inlined { x.inc() }
    x = 42
}

fun readInCrossinline(arg: Int?) {
    var x = arg
    if (x == null) return
    crossInlined { x.inc() } // ERROR only safe (?.) or non-null asserted
    x = 42
}

fun readInEscaping(arg: Int?) {
    var x = arg
    if (x == null) return
    escaping { x.inc() } // ERROR only safe (?.) or non-null asserted
    x = 42
}

fun readThenAssignInInlined(arg: Int?) {
    // Without a contract, run may run again after it assigns x.
    var x = arg
    if (x == null) return
    run {
        x.inc() // ERROR only safe (?.) or non-null asserted
        x = null
    }
    if (x != null) x = 42
    x.inc() // ERROR only safe (?.) or non-null asserted
}

fun assignedNullInInlined(s: String?): Int {
    var x = s
    inlined { x = null }
    if (x != null) return x.length
    return 0
}

fun assignedNullInEscaping(s: String?): Int {
    var x = s
    val f = { x = null }
    if (x != null) return x.length // ERROR only safe (?.) or non-null asserted
    f()
    return 0
}

// A property of a var that a branch or a loop assigns loses its smart
// cast.

class Box(val x: String?)

fun propertyAfterBranch(p: Boolean): Int {
    var b = Box("a")
    if (b.x == null) return 0
    if (p) b = Box(null)
    return b.x.length // ERROR only safe (?.) or non-null asserted
}

fun propertyInLoop() {
    var b = Box("a")
    if (b.x == null) return
    while (cond()) {
        b.x.length // ERROR only safe (?.) or non-null asserted
        b = Box(null)
    }
}

fun propertyAliasInLoop() {
    var b = Box("a")
    val x = b.x
    if (x == null) return
    while (cond()) {
        b.x.length // ERROR only safe (?.) or non-null asserted
        b = Box(null)
    }
}

// A check of a var that the condition then assigns says nothing of it.

fun assignedInCondition(): Int {
    var x: String? = "a"
    if (x?.let { x = null; 1 } != null) return x.length // ERROR only safe (?.) or non-null asserted
    return 0
}

// The lambdas passed to one call may run in any order.

inline fun both(f: () -> Unit, g: () -> Unit) {
    g()
    f()
}

fun assignedBySibling() {
    var s: String? = "a"
    both({ s.length }, { s = null }) // ERROR only safe (?.) or non-null asserted
    s = "b"
    both({ s.length }, { s.length })
}

// A lambda that may run again while it runs keeps a smart cast only
// if what it assigns has the type.

fun assignedOtherInEscaping(x: Any) {
    var y: Any = x
    escaping {
        y = 10
        y = "a"
        y.length // ERROR unresolved reference: length
    }
}

fun assignedSameInEscaping(x: Any) {
    var y: Any = x
    escaping {
        y = "a"
        y.length
    }
}

// A lambda passed in an argument assigns before the arguments after it
// are evaluated.

inline fun <T> pick(a: T, b: T) = a ?: b

inline fun <T> make(f: (T) -> Unit): T = null!!

fun assignedInEarlierArgument() {
    var x: Int? = 5
    pick(make<Int?> { x = null }, x.inc()) // ERROR only safe (?.) or non-null asserted
}

// In the initializers, a constructor parameter that declares a property
// is the property, which may change if it is a var or open.

class VarProp(var x: Int?) {
    init {
        if (x != null) x.inc() // ERROR only safe (?.) or non-null asserted
    }
}

class ValProp(val x: Int?) {
    init {
        if (x != null) x.inc()
    }
}

open class OpenProp(open val x: Int?) {
    val y = if (x != null) x.inc() else 0 // ERROR only safe (?.) or non-null asserted
}

class FunProp(var f: (() -> Unit)?) {
    init {
        if (f != null) f() // ERROR use explicit
    }
}

class PlainParam(x: Int?) {
    init {
        if (x != null) x.inc()
    }
}
