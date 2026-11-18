// After a syntax error, kc checks the rest of the file, as kotlinc does:
// only the statement with the error is left out, not its function, and
// a lexer error doesn't stop the checking.
fun lexer() {
    val c = '\u12' // ERROR illegal escape
    val s: String = 1 // ERROR type mismatch
}

fun statement(x: Int) {
    val a = x +* 2 // ERROR Expecting an element
    val n: String = x // ERROR type mismatch
    println(a.length) // a was left out: no error of its own
}

fun labels() {
    while (true) a@ {
        break@a // ERROR unresolved label
    }
}

fun lambdas(f: () -> Unit) = f()

fun g() {
    lambdas() {} {} // ERROR only one lambda expression is allowed
    lambdas() {}
    {} // ERROR unexpected trailing lambda on a new line
}

val o = object<T> {} // ERROR type parameters are not allowed for objects
