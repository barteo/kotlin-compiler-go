inline fun inl(f: () -> Unit) = f()
inline fun cross(crossinline f: () -> Unit) = f()
fun plain(f: () -> Unit) = f()

inline fun allowed(p: () -> Unit, crossinline q: () -> Unit) {
    p()
    inl { p() }
    inl { inl { p() } }
    inl(p)
    cross { q() }
    val o = object { fun g() = q() }
    plain { q() }
    cross(q)
}

inline fun inObject(p: () -> Unit) {
    val o = object {
        fun g() = p() // ERROR can't inline 'p' here: it may contain non-local returns
    }
}

inline fun inCrossinline(p: () -> Unit) {
    cross { p() } // ERROR can't inline 'p' here: it may contain non-local returns
}

inline fun inLambda(p: () -> Unit) {
    val l = { p() } // ERROR can't inline 'p' here: it may contain non-local returns
    { p() }() // ERROR can't inline 'p' here: it may contain non-local returns
}

inline fun passedCrossinline(p: () -> Unit) {
    cross(p) // ERROR can't inline 'p' here: it may contain non-local returns
}

fun main() {}
