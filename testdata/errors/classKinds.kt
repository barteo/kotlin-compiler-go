sealed class Shape
class Circle : Shape()
class Square : Shape()
object Dot : Shape()

enum class Dir { UP, DOWN }

data class P(val x: Int, val y: Int = 0)

data class NoParams() // ERROR data class must have at least one primary constructor parameter

data class NotProps(val a: Int, b: Int) // ERROR data class primary constructor must only have property (val / var) parameters

fun area(s: Shape): Int = when (s) { // ERROR 'when' expression must be exhaustive
    is Circle -> 1
    is Square -> 2
}

fun dir(d: Dir): Int = when (d) { // ERROR 'when' expression must be exhaustive
    Dir.UP -> 1
}

fun statement(d: Dir) {
    when (d) { // ERROR 'when' expression must be exhaustive
        Dir.UP -> println("up")
    }
}

fun nullable(s: Shape?): Int = when (s) { // ERROR 'when' expression must be exhaustive
    is Circle, is Square, Dot -> 1
}

fun complete(s: Shape?): Int = when (s) {
    is Circle, is Square, Dot -> 1
    null -> 0
}

fun named() {
    val a = P(y = 1, z = 2) // ERROR cannot find a parameter with this name: z
    val b = P(x = 1, x = 2) // ERROR an argument is already passed for this parameter: x
    val c = P(y = 1, 2) // ERROR mixing named and positioned arguments is not allowed
    val d = P(1).copy(w = 3) // ERROR cannot find a parameter with this name: w
    val e = java.lang.Integer.valueOf(s = "1") // ERROR named arguments are not allowed for non-Kotlin functions
    val f = Dir() // ERROR enum types cannot be instantiated
    val g = Shape() // ERROR cannot create an instance of an abstract class: Shape
    val (x, y, z) = P(1) // ERROR unresolved reference: component3
}

open class Base {
    open fun f(x: Int = 1): Int = x
}

class Derived : Base() {
    override fun f(x: Int = 2): Int = x // ERROR an overriding function is not allowed to specify default values
}

enum class Abstract {
    DONE {
        override fun run() = 1
    },
    MISSING; // ERROR enum entry 'Abstract.MISSING' does not implement abstract member 'run'

    abstract fun run(): Int
}
