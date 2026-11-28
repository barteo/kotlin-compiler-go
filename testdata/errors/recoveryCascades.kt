// Errors kotlinc reports in code with syntax errors, where kc used to
// report more after them: what recovery leaves out may have jumped.

data class Pair(val a: Int, val b: Int)

fun droppedReturn(a: Int?, b: Int?): Int {
    return a+ // ERROR Expecting an element
}

fun assignmentInReturn(): Boolean {
    var p = ""
    return (p = "x") && true // ERROR assignments are not expressions
}

fun emptyCondition(arg: Int): Int {
    when (arg) {
        0 -> return 0
        -> return 4 // ERROR Expecting an expression
        else -> return -1
    }
}

fun noJump(): Int {
    val x = 1 + // ERROR Expecting an element
} // ERROR missing return statement

fun destructuring(): Int {
    val (private a, public b) = Pair(1, 2) // ERROR modifier 'private' is not applicable to 'local variable'
    return a + b
}
