// Explicit backing fields: a property's field with a type of its own, a
// subtype of the property's, which reads in the property's private scope
// see.

class Box<T> {
    val items: List<T>
        field = mutableListOf()

    val count: Any
        field: Int = 7

    val label: CharSequence
        field: String

    init {
        label = "box"
    }

    fun add(x: T) {
        items.add(x)
    }

    fun size(): Int = count + items.size

    fun other(b: Box<T>): Int = b.count * 10

    inner class Inner {
        fun first(): T = items.removeAt(0)
    }

    class Nested {
        fun of(b: Box<String>): String = b.items.joinToString() + b.label.length
    }
}

val total: Number
    field = 40

fun bump(): Int = total + 2

object Registry {
    val names: Collection<String>
        field = linkedSetOf("a")

    fun register(n: String) = names.add(n)
}

fun main() {
    val b = Box<String>()
    b.add("x")
    b.add("y")
    println(b.items)
    println(b.count)
    println(b.size())
    println(b.other(b))
    println(b.label)
    println(Box.Nested().of(b))
    println(b.Inner().first())
    println(b.items)
    println(bump())
    println(total)
    println(Registry.register("b"))
    println(Registry.register("a"))
    println(Registry.names)
    val read: List<String> = b.items
    println(read is MutableList<*>)
}
