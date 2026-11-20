// When guards, anonymous functions and local extension functions.

sealed interface Shape
class Circle(val r: Int) : Shape
class Square(val s: Int) : Shape

fun noSubject(x: Int) = when {
    x > 0 if x < 10 -> 1 // ERROR guard conditions are only allowed in 'when' with a subject
    else -> 0
}

fun guardedNotExhaustive(s: Shape): Int = when (s) { // ERROR 'when' expression must be exhaustive
    is Circle if s.r > 0 -> 1
    is Square -> 2
}

fun guardType(s: Shape) = when (s) {
    is Circle if s.r -> 1 // ERROR type mismatch
    else -> 0
}

fun untyped() {
    val f = fun(x) = x // ERROR the parameters of an anonymous function need types
}

fun anonReturn(): Int {
    val f = fun(x: Int): String { return x } // ERROR type mismatch
    return 1
}

fun localExt() {
    fun Int.twice() = this * 2
    println(3.twice())
    println("a".twice()) // ERROR unresolved reference: twice
}

fun main() {}

fun commaGuard(s: Shape) = when (s) {
    is Circle, is Square if s.hashCode() > 0 -> 1 // ERROR a guard condition is not allowed after several conditions
    else -> 0
}

fun flag(): Boolean = true

fun guardAssigns(x: Any) {
    val v: Int
    when (x) {
        is String if flag() && { v = 1; true }() -> {} // ERROR captured values cannot be initialized
        else -> {}
    }
    val w: Int
    when (x) {
        is String if (if (flag()) { w = 1; true } else false) -> {}
        is Int if (if (flag()) { w = 2; true } else false) -> {} // ERROR val cannot be reassigned
        else -> {}
    }
}
