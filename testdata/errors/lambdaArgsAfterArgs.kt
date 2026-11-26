// As in K2, a call's lambda arguments are analyzed after its other
// arguments, which may assign what they read: a var, or a val that an
// inlined lambda reads. A val that a lambda that isn't inlined reads
// must be assigned where the lambda is, as the lambda may run once the
// call has it (KT-70133).

inline fun inl(x: () -> Int, y: String) { x() }
inline fun noinl(noinline x: () -> Int, y: String) { x() }
fun later(x: () -> Int, y: String) {}

fun valInlined() {
    val x: String
    inl(x = { x.length }, y = if (true) { x = ""; "" } else { x = ""; "" })
}

fun valLater() {
    val x: String
    later(x = { x.length }, y = if (true) { x = ""; "" } else { x = ""; "" }) // ERROR variable 'x' must be initialized
}

fun valNoinline() {
    val x: String
    noinl({ x.length }, if (true) { x = ""; "" } else { x = ""; "" }) // ERROR variable 'x' must be initialized
}

fun varLater() {
    var x: String
    later({ x.length }, if (true) { x = ""; "" } else { x = ""; "" })
}

fun varAfter() {
    var x: String
    later({ x.length }, "") // ERROR variable 'x' must be initialized
    x = ""
    later({ x.length }, x)
}

fun arrayInit() {
    val x: String
    Array(init = { x.length }, size = if (true) { x = ""; 1 } else { x = ""; 1 })
}
