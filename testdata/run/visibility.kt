// Calls resolve as if declarations the code can't see weren't there: a
// private member gives way to an extension or another overload, and a
// superclass's private members, which a subclass doesn't inherit, are
// the enclosing class's in an object expression or inner class of it.

class Greeter {
    private fun greet() = "member"
    private val name = "member"
}

fun Greeter.greet() = "extension"
val Greeter.name: String get() = "extension"

open class Base {
    private fun f(x: Int) = "Base.f(Int)"
}

class Derived : Base() {
    fun f(x: Any) = "Derived.f(Any)"
}

open class Node(val label: String) {
    private fun tag() = label
    private val id = label

    fun anonymous(): Node = object : Node("anonymous") {
        override fun toString() = tag() + " " + id
    }

    inner class Child : Node("child") {
        fun show() = tag() + " " + id
    }
}

// Interfaces' members win over a superclass's private one of the name.
interface Named {
    val title: String get() = "Named.title"
}

open class Hidden {
    private val title = "Hidden.title"
}

class Both : Hidden(), Named

// A protected member is visible where the member it overrides is.
open class Shape(protected open val sides: Int) {
    protected open fun kind() = "shape"
    fun describe(other: Shape): String {
        if (other is Square) {
            return other.kind() + " " + other.sides
        }
        return other.kind()
    }
}

class Square : Shape(4) {
    override val sides = 4
    override fun kind() = "square"
}

// A class's code calls the protected members its companion inherits.
abstract class Registry {
    protected fun lookup() = "registry"
}

class Service {
    companion object : Registry()

    fun find() = lookup()
}

class Counter {
    var count = 0
        private set

    fun increment() {
        count++
    }

    companion object {
        private const val START = "start"
        fun start() = START
    }
}

fun main() {
    println(Greeter().greet())
    println(Greeter().name)
    println(Derived().f(1))
    val n = Node("outer")
    println(n.anonymous().toString())
    println(n.Child().show())
    println(Both().title)
    println(Shape(3).describe(Square()))
    println(Service().find())
    val c = Counter()
    c.increment()
    println(c.count)
    println(Counter.start())
}
