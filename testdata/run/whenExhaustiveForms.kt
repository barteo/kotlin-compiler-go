// A when without else is exhaustive in the forms K2 accepts beyond is
// checks of each sealed subclass: an enum's entries below a sealed
// interface, !is checks, a subject smart cast to an intersection, is
// checks of the subject's own type, and a Nothing subject.

sealed interface Base
interface A : Base
enum class C : Base { X, Y }
object D : Base

class AImpl : A

fun entries(b: Base): String = when (b) {
    is A -> "A"
    C.X -> "C.X"
    C.Y -> "C.Y"
    D -> "D"
}

sealed class S(val n: Int) {
    object First : S(1)
    open class NonFirst(n: Int) : S(n) {
        object Second : NonFirst(2)
    }
    object Last : S(3)
}

fun negated(s: S): String = when (s) {
    is S.First -> "first"
    !is S.First -> "not first " + s.n
}

interface Named {
    val name: String
}

sealed class Shape
class Circle : Shape(), Named {
    override val name = "circle"
}
open class Square : Shape()

fun intersection(x: Shape): String {
    if (x !is Named) return "unnamed"
    return when (x) {
        is Circle -> "c " + x.name
        is Square -> "s"
    }
}

fun ownType(i: Int?): String = when (i) {
    null -> "null"
    is Int -> "int " + (i + 1)
}

fun nullableIs(x: Any?): String = when (x) {
    is CharSequence? -> "char sequence? " + x
    is Number -> "number"
    else -> "other"
}

fun main() {
    println(entries(AImpl()))
    println(entries(C.X))
    println(entries(C.Y))
    println(entries(D))
    println(negated(S.First))
    println(negated(S.NonFirst.Second))
    println(intersection(Circle()))
    println(intersection(Square()))
    println(ownType(null))
    println(ownType(41))
    println(nullableIs(null))
    println(nullableIs("x"))
}
