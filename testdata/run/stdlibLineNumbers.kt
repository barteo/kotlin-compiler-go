// Line numbers around the standard library's inline functions: their
// lambdas have their own lines, and the code after them the caller's.

fun here(): Int = Throwable().stackTrace[1].lineNumber

inline fun twice(f: () -> Unit) {
    f()
    f()
}

fun main() {
    val x = 1.let {
        it + here()
    }
    println(x)
    listOf(1).forEach {
        println(here())
    }
    println(here())
    twice {
        println(here())
    }
    println(listOf(1, 2).map { it + here() })
    println(here())
}
