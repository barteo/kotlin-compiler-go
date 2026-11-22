// The errors of a builder's lambda are those of its one check, with the
// builder's variable unfixed: each is reported once, and names what the
// variable is inferred as afterwards.

fun ints() {
    buildList {
        add("3")
        val x: MutableList<Int> = this@buildList // ERROR inferred type is MutableList<String> but MutableList<Int> was expected
    }
}

fun strings() {
    buildList {
        val y: CharSequence = ""
        add(y)
        val x: MutableList<String> = this@buildList // ERROR inferred type is MutableList<CharSequence> but MutableList<String> was expected
    }
}

fun element() {
    buildList {
        add("")
        val x: StringBuilder = get(0) // ERROR but StringBuilder was expected
    }
}
