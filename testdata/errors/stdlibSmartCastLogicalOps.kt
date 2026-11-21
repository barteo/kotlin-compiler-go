// As in K2, the right side of && and || may not run: an in-place
// lambda's assignment there ends the smart casts of the var it assigns,
// as one before it, r = q, gave. The right side's own smart casts don't
// hold after it either.
fun Boolean.accept() {}

fun alsoNull() {
    val q = true
    var r: Boolean?
    r = q
    if (q || r.also { r = null }!!) { // the warning: unnecessary non-null assertion
        r.accept() // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver
    } else {
        r.accept() // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver
    }
}

fun orRun(q: Boolean) {
    var r: Boolean?
    r = q
    if (!q || run { r = null; true }) r.accept() // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver
}

fun andRun(q: Boolean) {
    var r: Boolean?
    r = q
    if (q && run { r = null; true }) r.accept() // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver
}

fun sameType(q: Boolean) {
    var r: Boolean?
    r = q
    if (q || run { r = false; true }) r.accept() // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver
}

fun afterwards(q: Boolean) {
    var r: String?
    r = ""
    val ok = q || run { r = null; true }
    r.length // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver
}

fun controls(q: Boolean, s: String?) {
    var r: Any?
    r = "x"
    if (q && run { r = 1; true }) r.hashCode()
    if (q && s!!.isEmpty()) s.length
    if (q || s!!.isEmpty()) s.length // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver
    var t: String?
    t = ""
    if (q || run { println(); true }) t.length
}
