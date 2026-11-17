// A file with syntax errors reports all of them, as kotlinc does: the
// recovering parser's, and what its grammar accepts but kotlinc rejects
// later, which kc's parser stops at.
fun a(x: Int) {
    val y = (1 + // ERROR Expecting an element
}
fun b( {} // ERROR Expecting ')'
class C private constructor { // ERROR Expecting primary constructor parameter list
}
fun d(v: Any?, b: Boolean) {
    var z = 0
    val w = while (b) {} // ERROR expected an expression, found a loop
    val u = z = 1 // ERROR assignments are not expressions
    val k = @Suppress("x") class K // ERROR expected an expression, found a declaration
    v as String & Any // ERROR Unexpected tokens
    when (b) {
        b && b -> {} // ERROR confusing branch condition
        (b || b) -> {}
        (z = 2) -> {} // ERROR assignments are not expressions
        else -> {}
    }
}
