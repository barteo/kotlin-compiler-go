// Objects, companion objects and nested classes.

interface Greeter {
    fun greet(name: String): String
}

object Polite : Greeter {
    var count = 0

    override fun greet(name: String): String {
        count++
        return "Good day, " + name
    }
}

object Registry {
    val names = java.util.ArrayList<String>()

    fun add(name: String): Int {
        names.add(name)
        return names.size
    }
}

class Point private constructor(val x: Int, val y: Int) {
    fun plus(other: Point): Point = create(x + other.x, y + other.y)

    fun describe(): String = "(" + x + ", " + y + ") of " + created

    fun scaled(k: Int): Point {
        val f = { p: Point -> create(p.x * k, p.y * k) }
        return f(this)
    }

    companion object {
        val origin = Point(0, 0)
        var created = 0

        fun create(x: Int, y: Int): Point {
            created++
            return Point(x, y)
        }
    }
}

class Temperature(val celsius: Double) {
    companion object Factory {
        const val ZERO = 273.15

        fun fromKelvin(k: Double): Temperature = Temperature(k - ZERO)
    }
}

class Outer(val name: String) {
    fun inner(): Nested = Nested(name.length)

    class Nested(val n: Int) {
        fun twice(): Int = n * 2
        fun deeper(): Deep = Deep(n)

        class Deep(val m: Int) {
            override fun toString(): String = "deep " + m
        }
    }

    object Constants {
        val answer = 42
    }

    interface Visitor {
        fun visit(n: Nested): String
    }
}

class Printer : Outer.Visitor {
    override fun visit(n: Outer.Nested): String = "visited " + n.n
}

interface Shape {
    fun area(): Double

    companion object {
        fun unit(): Shape = Square(1.0)
    }
}

class Square(val side: Double) : Shape {
    override fun area(): Double = side * side
}

fun greetAll(g: Greeter): String = g.greet("Ann") + "; " + g.greet("Bob")

fun main() {
    println(greetAll(Polite))
    println(Polite.count)
    val g: Greeter = Polite
    println(g.greet("Cy"))
    println(Polite.count)

    println(Registry.add("a") + Registry.add("b"))
    println(Registry.names)

    val p = Point.create(1, 2)
    val q = p.plus(Point.origin).plus(Point.create(3, 4))
    println(q.describe())
    println(p.scaled(3).describe())
    println(Point.created)
    Point.created = 100
    println(Point.Companion.created)

    println(Temperature.fromKelvin(300.0).celsius)
    println(Temperature.Factory.ZERO + Temperature.ZERO)

    val n = Outer("four").inner()
    println(n.twice())
    println(Outer.Nested(5).deeper())
    println(Outer.Nested.Deep(6))
    println(Outer.Constants.answer)
    println(Printer().visit(n))
    println(Shape.unit().area())
}
