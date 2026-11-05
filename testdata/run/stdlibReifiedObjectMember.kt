// An inline function's object with inline members of its own, whose
// reified type parameters the object's copies don't reify: the copy of f
// in main needs no class reification.
inline fun <reified E : Throwable> f(): String {
    return object {
        inline fun <reified T : Any> check(x: Any): String = if (x is T) "is" else "isn't"
        inline fun <reified T : Throwable> catching(): String =
            try {
                throw IllegalStateException()
            } catch (e: T) {
                "caught"
            }

        fun g(): String = check<String>("s") + " " + check<Int>("s") + " " + catching<IllegalStateException>()
    }.g() + " " + (AssertionError() is E)
}

fun main() {
    println(f<AssertionError>())
    println(f<IllegalStateException>())
}
