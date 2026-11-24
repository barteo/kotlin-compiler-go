// As kotlinc's BUILDER_INFERENCE_STUB_RECEIVER, an element of a builder's
// type isn't fixed by an indexed compound assignment, as it is by an
// operator: this[0] += "K" is the error, and add("O") still gives
// buildList's E.
fun indexedCompound() {
    buildList {
        add("O")
        this[0] += "K" // ERROR the type of a receiver hasn't been inferred yet
    }
}

fun reassigned() = buildList {
    add("O")
    this[0] = this[0] + "K"
}

fun explicit() = buildList<String> {
    add("O")
    this[0] += "K"
}

// A member of Any, which a stub has, on an element whose type nothing
// has given yet is an error too, though add("ab") gives it later.
fun anyMembers() {
    buildList { if (size > 5) println(get(0).hashCode()); add("ab") } // ERROR the type of a receiver hasn't been inferred yet
    buildList { if (size > 5) println(get(0).toString()); add("ab") } // ERROR the type of a receiver hasn't been inferred yet
    buildList { if (size > 5) println(get(0)?.hashCode()); add("ab") } // ERROR the type of a receiver hasn't been inferred yet
}

// Not once something gives it, nor where no member is called.
fun anyMembersFine() {
    buildList { add("ab"); println(get(0).hashCode()) }
    buildList { if (size > 5) println(get(0) == null); add("ab") }
    buildList { if (size > 5) println("${get(0)}"); add("ab") }
}
