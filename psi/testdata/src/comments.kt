// leading comment of file

/* block before fun */
fun f() {} // trailing f

/** KDoc g */
/* plain after kdoc */
fun g() {}

// detached


// attached to h
fun h() {
    // inside body
    val x = 1 // trailing x
    /* before y */ val y = 2

    // dangling at end
}

class K {
    // first member
    fun m() {}

    /** doc p */
    val p = 1 /* trailing p */

    // last
}
/* unclosed? no */
