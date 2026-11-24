// kotlinc: -jvm-target 17
// From Java 17 on, a sealed class or interface lists its direct subclasses
// in its PermittedSubclasses attribute, by name, and the JVM loads only
// those as its subclasses.
sealed class Expr {
    data class Num(val v: Int) : Expr()
    object Zero : Expr()
    sealed class Bin : Expr() {
        class Add(val a: Expr, val b: Expr) : Bin()
    }
}

class Lit : Expr()

class Neg(val e: Expr) : Expr.Bin()

class Gen<T> : Expr()

sealed interface Shape

enum class Dir : Shape { N, S }

@JvmInline
value class Circle(val r: Int) : Shape

data class Sq(val s: Int) : Shape

sealed interface Corner : Shape

data object Point : Corner

fun interface Painter : Shape {
    fun paint()
}

abstract class Brush : Corner

sealed class Empty

open class Plain

fun describe(c: Class<*>) {
    val permitted = c.permittedSubclasses?.map { it.name }
    println("${c.name}: sealed=${c.isSealed} $permitted")
}

fun eval(e: Expr): Int = when (e) {
    is Expr.Num -> e.v
    Expr.Zero -> 0
    is Expr.Bin.Add -> eval(e.a) + eval(e.b)
    is Neg -> -eval(e.e)
    is Lit -> 1
    is Gen<*> -> 2
}

fun main() {
    for (c in listOf(Expr::class.java, Expr.Bin::class.java, Shape::class.java, Corner::class.java,
        Empty::class.java, Plain::class.java, Lit::class.java)) {
        describe(c)
    }
    println(eval(Expr.Bin.Add(Expr.Num(3), Neg(Lit()))) + eval(Gen<String>()) + eval(Expr.Zero))
    val shapes: List<Shape> = listOf(Dir.N, Circle(2), Sq(3), Point, Painter { }, object : Brush() {})
    println(shapes.map { it is Corner })
}
