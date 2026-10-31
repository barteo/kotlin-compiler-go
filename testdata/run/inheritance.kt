// Interfaces, abstract and open classes, overrides, super calls, getters,
// bridges, and Kotlin classes that extend or implement Java ones.

interface Named {
    val name: String
    val greeting: String
        get() = "hello, " + name

    fun describe(): String = "named " + name
}

interface Shape : Named {
    fun area(): Double
    fun scaled(k: Double): Shape
}

abstract class Base(override val name: String, val sides: Int) : Shape {
    var resized = 0

    override fun describe(): String = super.describe() + " with " + sides + " sides"

    override fun scaled(k: Double): Shape {
        resized++
        return copy(k)
    }

    abstract fun copy(k: Double): Shape

    override fun toString(): String = name + "(" + area() + ")"
}

open class Rect(val w: Double, val h: Double) : Base("rect", 4) {
    override fun area(): Double = w * h
    override fun copy(k: Double): Shape = Rect(w * k, h * k)
    open fun kind(): String = "rectangle"
}

class Square(side: Double) : Rect(side, side) {
    override val name: String
        get() = "square"

    override fun kind(): String = "square, a " + super.kind()
    override fun copy(k: Double): Shape = Square(w * k)
}

class Circle(val r: Double) : Base(if (r > 1.0) "big circle" else "circle", 0) {
    override fun area(): Double = 3.0 * r * r
    override fun copy(k: Double): Shape = Circle(r * k)
    override fun describe(): String = "round " + super.describe()
}

// A generic base class and a subclass that fixes its type argument: the
// override of get needs a bridge.
open class Box<T>(val value: T) {
    open fun get(): T = value
    open fun put(v: T): String = "put " + v
}

class IntBox(v: Int) : Box<Int>(v) {
    override fun get(): Int = value + 1
    override fun put(v: Int): String = "int " + (v * 2)
}

// Comparable<Version>.compareTo(T) is compareTo(Object) on the JVM.
class Version(val major: Int, val minor: Int) : Comparable<Version> {
    override fun compareTo(other: Version): Int =
        if (major != other.major) major - other.major else minor - other.minor

    override fun toString(): String = "" + major + "." + minor
    override fun hashCode(): Int = major * 31 + minor
}

class Task(val label: String) : Runnable {
    var ran = false
    override fun run() {
        ran = true
    }
}

// A subclass of a Java class that calls inherited methods unqualified.
class Stack : java.util.ArrayList<String>() {
    fun push(s: String) {
        add(s)
    }

    fun top(): String = get(size - 1)
}

class AppError(val code: Int) : Exception("error " + code) {
    fun report(): String = "" + code + ": " + message
}

fun total(shapes: java.util.ArrayList<Shape>): Double {
    var sum = 0.0
    for (s in shapes) {
        sum += s.area()
    }
    return sum
}

fun pick(big: Boolean): Base = if (big) Rect(2.0, 3.0) else Circle(1.0)

fun main() {
    val shapes = java.util.ArrayList<Shape>()
    shapes.add(Rect(2.0, 3.0))
    shapes.add(Square(2.0))
    shapes.add(Circle(1.0))
    shapes.add(Circle(2.0))
    for (s in shapes) {
        println(s.name + ": " + s.area() + ", " + s.describe() + ", " + s.greeting)
    }
    println(total(shapes))
    val r = Rect(1.0, 1.0)
    val bigger = r.scaled(3.0)
    println(bigger.area())
    println(r.resized)
    println(Square(1.0).kind())
    println(Rect(1.0, 2.0).kind())
    println(pick(true).sides + pick(false).sides)
    println(Circle(1.5))
    val named: Named = Square(5.0)
    println(named.name + " " + named.describe())

    val b: Box<Int> = IntBox(4)
    println(b.get())
    println(b.put(5))
    println(Box("x").get() + " " + Box("y").put("z"))

    val versions = java.util.ArrayList<Version>()
    versions.add(Version(1, 10))
    versions.add(Version(1, 2))
    versions.add(Version(0, 9))
    java.util.Collections.sort(versions)
    println(versions)
    println(Version(2, 0) > Version(1, 9))
    println(Version(1, 0).hashCode())

    val task = Task("t")
    val thread = Thread(task)
    thread.start()
    thread.join()
    println(task.ran)

    val stack = Stack()
    stack.push("a")
    stack.push("b")
    println(stack.top() + " " + stack.size)

    val e = AppError(42)
    println(e.report())
    println(e.message)
}
