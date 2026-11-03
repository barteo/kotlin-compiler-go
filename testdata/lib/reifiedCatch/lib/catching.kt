package defs

// An inline function's catch block of a reified type parameter, which a
// caller's copy, inlined from the class file, catches the type argument of.
inline fun <reified E : Throwable> catching(block: () -> Unit): String {
    try {
        block()
    } catch (e: E) {
        return "caught " + e.message
    }
    return "none"
}

// And through another inline function's reified type parameter.
inline fun <reified T : Throwable> evalCatch(noinline block: () -> Unit): String =
    try {
        catching<T>(block)
    } catch (t: Throwable) {
        "escaped " + t.message
    }

// In a lambda, which a caller's copy of the class catches the type
// argument in.
inline fun <reified E : Throwable> catcher(): (() -> Unit) -> String = { block ->
    try {
        block()
        "none"
    } catch (e: E) {
        "deferred " + e.message
    }
}
