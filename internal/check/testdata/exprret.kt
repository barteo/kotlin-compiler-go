// RETURN_IN_FUNCTION_WITH_EXPRESSION_BODY_WARNING: a return through an
// inline lambda of a function with an expression body and no return type.
fun f1(b: Boolean) = run {
    if (b) return 1 // RETURN_IN_FUNCTION_WITH_EXPRESSION_BODY_WARNING
    2
}
fun f2(b: Boolean): Int = run {
    if (b) return 1
    2
}
fun f3(xs: List<Int>) = xs.forEach {
    if (it == 0) return // RETURN_IN_FUNCTION_WITH_EXPRESSION_BODY_WARNING
}
fun f4(b: Boolean) = run {
    run {
        if (b) return 1 // RETURN_IN_FUNCTION_WITH_EXPRESSION_BODY_WARNING
    }
    2
}
fun f5(b: Boolean) = run {
    @Suppress("RETURN_IN_FUNCTION_WITH_EXPRESSION_BODY_WARNING")
    if (b) return 1
    2
}
fun f6(b: Boolean) = run {
    val g = { -> 3 }
    if (b) return g() // RETURN_IN_FUNCTION_WITH_EXPRESSION_BODY_WARNING
    2
}
fun f7(b: Boolean) = run label@{
    if (b) return@label 1
    2
}
class C {
    fun m(b: Boolean) = run {
        if (b) return "x" // RETURN_IN_FUNCTION_WITH_EXPRESSION_BODY_WARNING
        "y"
    }
}
