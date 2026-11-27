// The members of an error type are reported unresolved, as in K2, but
// for the error type of a name that couldn't be resolved.

fun smartCast(f: Boolean) {
    val xmlText = getText() // ERROR unresolved reference: getText
    xmlText.value = 0
    if (xmlText == null) {
    } else {
        xmlText.value = 0 // ERROR unresolved reference: value
        xmlText.foo() // ERROR unresolved reference: foo
    }
    if (xmlText != null) {
        xmlText.length // ERROR unresolved reference: length
    }
}

class Owner {
    inner class Inner {
        fun baz() {}
    }
}

fun innerWithoutOuter(o: Owner) {
    val err = Owner.Inner() // ERROR can be called only with receiver of containing class
    err.baz() // ERROR unresolved reference: baz
    o.Inner().baz()
}

class Box {
    fun take(f: (Int) -> Int) = f(1)
}

// A lambda passed to a call without a candidate has no implicit parameter.
fun noCandidate(b: Box) {
    b.unresolved { // ERROR unresolved reference: unresolved
        it // ERROR unresolved reference: it
    }
    b.take { it + 1 }
}
