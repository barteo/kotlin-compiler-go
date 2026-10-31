// Sealed classes and interfaces, with data classes and objects, and
// exhaustive when over them.

sealed interface Expr

data class Num(val value: Int) : Expr

data class Add(val left: Expr, val right: Expr) : Expr

data class Mul(val left: Expr, val right: Expr) : Expr

data object Zero : Expr

sealed class Result {
    data class Ok(val value: Int) : Result()

    sealed class Failure : Result() {
        data class Error(val message: String) : Failure()
        object Timeout : Failure()
    }

    fun describe(): String = when (this) {
        is Ok -> "ok " + value
        is Failure.Error -> "error: " + message
        Failure.Timeout -> "timeout"
    }
}

fun eval(e: Expr): Int = when (e) {
    is Num -> e.value
    is Add -> eval(e.left) + eval(e.right)
    is Mul -> eval(e.left) * eval(e.right)
    Zero -> 0
}

fun show(e: Expr): String = when (e) {
    is Num -> "" + e.value
    is Add -> "(" + show(e.left) + " + " + show(e.right) + ")"
    is Mul -> show(e.left) + " * " + show(e.right)
    is Zero -> "0"
}

fun failed(r: Result): Boolean = when (r) {
    is Result.Ok -> false
    is Result.Failure -> true
}

fun count(r: Result?): String = when (r) {
    null -> "none"
    is Result.Ok -> "one"
    is Result.Failure -> "fail"
}

fun safeDiv(a: Int, b: Int): Result =
    if (b == 0) Result.Failure.Error("division by zero") else Result.Ok(a / b)

fun main() {
    val e = Add(Num(2), Mul(Num(3), Add(Num(4), Zero)))
    println(eval(e))
    println(show(e))
    println(e)
    println(safeDiv(10, 2).describe())
    println(safeDiv(1, 0).describe())
    println(Result.Failure.Timeout.describe())
    println(failed(Result.Ok(1)) || failed(Result.Failure.Timeout))
    println(count(null) + " " + count(Result.Ok(2)))
    val r: Result = Result.Ok(5)
    when (r) {
        is Result.Ok -> println("statement ok")
        is Result.Failure -> println("statement fail")
    }
}
