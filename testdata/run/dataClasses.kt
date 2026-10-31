// Data classes and data objects, destructuring declarations, default
// parameter values and named arguments.

data class Point(val x: Int, val y: Int) {
    fun plus(other: Point): Point = Point(x + other.x, y + other.y)
}

data class Person(val name: String, val nick: String?, var age: Int = 30, val score: Double = 1.5, val active: Boolean = true, val id: Long = 7L)

data class Wrapper<T>(val value: T, val label: String = "w")

data class Custom(val a: Int) {
    override fun toString(): String = "Custom<" + a + ">"
}

data object Empty

interface Greeter {
    fun greet(name: String, greeting: String = "Hello", times: Int = 1): String
}

class Repeater : Greeter {
    override fun greet(name: String, greeting: String, times: Int): String {
        val sb = StringBuilder()
        for (i in 0 until times) {
            if (i > 0) sb.append(" ")
            sb.append(greeting).append(", ").append(name).append(".")
        }
        return sb.toString()
    }
}

class Window(val title: String = "untitled", val width: Int = 640, val height: Int = width * 3 / 4) {
    fun area(scale: Int = 1): Int = width * height * scale
}

fun range(from: Int = 0, to: Int = 10, step: Int = 1): String = "" + from + ".." + to + " by " + step

fun minMax(a: Int, b: Int): Point = if (a < b) Point(a, b) else Point(b, a)

fun main() {
    val p = Point(1, 2)
    println(p)
    println(p == Point(1, 2))
    println(p == Point(2, 1))
    println(p.hashCode() == Point(1, 2).hashCode())
    println(p.hashCode())
    println(p.plus(Point(10, 20)))
    println(p.copy(y = 5))
    println(p.copy())
    println(p.component1() + p.component2())

    val ann = Person("Ann", null)
    println(ann)
    println(ann.hashCode())
    val older = ann.copy(age = 31, nick = "annie")
    println(older)
    println(older == ann)
    println(older.hashCode())
    older.age = 40
    println(older)

    val (x, y) = minMax(9, 3)
    println("" + x + " " + y)
    val (name, _, age) = ann
    println(name + " is " + age)

    val set = java.util.HashSet<Point>()
    set.add(Point(1, 1))
    set.add(Point(1, 1))
    set.add(Point(2, 2))
    println(set.size)

    val points = java.util.ArrayList<Point>()
    points.add(Point(3, 4))
    points.add(Point(5, 6))
    for ((a, b) in points) {
        println(a * b)
    }

    val w = Wrapper(42)
    println(w)
    println(w.copy(label = "x"))
    println(Wrapper("s", "t") == Wrapper("s", "t"))
    println(Custom(3))
    println(Custom(3) == Custom(3))
    println(Empty)
    println(Empty == Empty)
    println(Empty.hashCode())

    val g: Greeter = Repeater()
    println(g.greet("Bob"))
    println(g.greet("Cy", times = 2))
    println(g.greet(greeting = "Hi", name = "Di"))
    println(Window().area())
    println(Window(width = 100).height)
    println(Window("big", 1000, 10).area(scale = 2))
    println(range())
    println(range(to = 5))
    println(range(step = 2, from = 1))
}
