// Sealed classes and interfaces, whose PermittedSubclasses attributes
// TestPermittedSubclasses compares with kotlinc's.
package p

sealed class Expr {
    data class Num(val v: Int) : Expr()
    object Zero : Expr()
    sealed class Bin : Expr() {
        class Add(val a: Expr, val b: Expr) : Bin()
    }
}

class Lit : Expr()

class Neg(val e: Expr) : Expr.Bin()

sealed interface Shape

enum class Dir : Shape { N, S }

@JvmInline
value class Circle(val r: Int) : Shape

data class Sq(val s: Int) : Shape

sealed interface Sub : Shape

object Point : Sub

class Late : Expr()

sealed class Empty

sealed interface Marker

abstract class Abs : Marker

class Gen<T> : Expr()

fun interface Fi : Shape {
    fun g()
}

class Both : Expr(), Marker, Sub
