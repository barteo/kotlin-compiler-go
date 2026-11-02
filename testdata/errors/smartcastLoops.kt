// Smart casts through loops, when without else, and try/finally.

fun cond(): Boolean = true

fun whileTrueBreak(p: String?): Int {
    while (true) {
        if (p == null) continue
        break
    }
    return p.length // the only way out is the break, after the check
}

fun whileCondition(p: String?): Int {
    var s: String? = p
    while (s == null) {
        s = "x"
    }
    return s.length // the condition is false after the loop
}

fun doWhileBody(p: String?): Int {
    do {
        p!!.length
    } while (p.length > 0) // the body runs before the condition
    return p.length
}

fun nestedDoWhile(p: String?): Int {
    var s: String? = p
    do {
        do {
            s = "y"
        } while (cond())
    } while (s.length > 0)
    return s.length
}

fun earlyReturnInWhen(p: String?, k: Int): Int {
    when (k) {
        0 -> if (p == null) return 0
        else -> if (p == null) return 1
    }
    return p.length
}

fun whenWithoutElse(p: String?, k: Int): Int {
    when {
        p == null -> return 0
        k > 0 -> println(k)
    }
    return p.length // no branch matched: p != null
}

fun boolLiteral(p: String?): Int {
    if ((p != null) == true) return p.length
    return 0
}

fun boolLiteralFalse(p: String?): Int {
    if ((p == null) == false) return p.length
    return 0
}

fun continueSkips(p: String?) {
    do {
        if (cond()) continue
        p!!.length
    } while (p.length > 0) // ERROR only safe (?.) or non-null asserted
}

fun zeroIterations(p: String?): Int {
    while (cond()) {
        p!!.length
    }
    return p.length // ERROR only safe (?.) or non-null asserted
}

fun breakBefore(p: String?): Int {
    while (true) {
        if (cond()) break
        p!!.length
    }
    return p.length // ERROR only safe (?.) or non-null asserted
}

fun finallyAssigns(p: String?): Int {
    var s: String? = p
    while (true) {
        s!!.length
        try {
            if (cond()) break
        } finally {
            s = null
        }
    }
    return s.length // ERROR only safe (?.) or non-null asserted
}

fun doWhileAssigns(p: String?): Int {
    var s: String? = "a"
    do {
        s = p
    } while (cond())
    return s.length // ERROR only safe (?.) or non-null asserted
}

fun oneBranch(p: String?, q: Boolean): Int {
    if (q) { p!! } else { }
    return p.length // ERROR only safe (?.) or non-null asserted
}

fun whenNoMatch(p: String?, k: Int): Int {
    when {
        k > 0 -> p!!
    }
    return p.length // ERROR only safe (?.) or non-null asserted
}

inline fun run(f: () -> Unit) = f()

fun lambdaAssigns(): Int {
    var s: String? = "a"
    while (cond()) {
        run { if (cond()) return@run; s = null }
        if (s == null) break
    }
    return s.length // ERROR only safe (?.) or non-null asserted
}
