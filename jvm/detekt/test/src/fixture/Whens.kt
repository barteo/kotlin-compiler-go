package fixture

// when exhaustiveness and subtyping.

enum class Color { RED, GREEN, BLUE }

sealed interface Shape
class Circle(val r: Double) : Shape
class Square(val side: Double) : Shape

fun name(c: Color): String = when (c) {
    Color.RED -> "red"
    Color.GREEN -> "green"
    else -> "blue"
}

fun redundantElse(c: Color): String = when (c) {
    Color.RED -> "red"
    Color.GREEN -> "green"
    Color.BLUE -> "blue"
    else -> "none"
}

fun area(s: Shape): Double = when (s) {
    is Circle -> 3.14 * s.r * s.r
    is Square -> s.side * s.side
}

fun used(c: Color) {
    when (c) {
        Color.RED -> println("r")
        else -> {}
    }
}

open class Base {
    open fun f(): Int = 1
}

class Derived : Base() {
    override fun f(): Int = super.f()
}

fun subtype(b: Base): Boolean = b is Derived

abstract class NoAbstract {
    fun g() = 1
}

fun exceptions() {
    try {
        println()
    } catch (e: Exception) {
        throw RuntimeException(e.message)
    }
}

