// As kotlinc's BUILDER_INFERENCE_STUB_RECEIVER, an element of a builder's
// type isn't fixed by an indexed compound assignment, as it is by an
// operator: this[0] += "K" leaves buildList's E uninferred.
fun indexedCompound() {
    buildList { // ERROR cannot infer type for type parameter 'E'
        add("O")
        this[0] += "K"
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
