// As K2, a value of a context function type with a receiver called on
// an explicit receiver with an argument for each context parameter isn't
// supported: the receiver would be the first context argument.
fun qux(f3: context(String, Int) Boolean.() -> Unit, f4: context(String, Int) String.() -> Unit) {
    "".f3(1, true) // ERROR unsupported contextual declaration call
    f3("", 1, true)
    with("") {
        with(1) {
            true.f3()
            "".f4()
        }
    }
}
