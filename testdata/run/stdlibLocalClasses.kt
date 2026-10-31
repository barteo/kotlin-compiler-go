// Classes declared in blocks: they capture the locals and the this they
// use, which each construction passes to their constructors.

class Shop(val name: String) {
    fun receipt(items: List<Int>): String {
        var total = 0
        class Line(val price: Int) {
            init {
                total += price
            }
            override fun toString() = "$name:$price"
        }
        val lines = items.map { Line(it) }
        return lines.joinToString() + " = " + total
    }
}

fun tree(depth: Int): String {
    val leaf = "leaf"
    class Node(val level: Int) {
        val children: List<Node> = if (level < depth) listOf(Node(level + 1), Node(level + 1)) else emptyList()
        fun count(): Int = 1 + children.sumOf { it.count() }
        override fun toString() = if (children.isEmpty()) leaf else "(" + children.joinToString(" ") + ")"
    }
    val root = Node(0)
    return "$root ${root.count()}"
}

fun main() {
    data class Point(val x: Int, val y: Int)
    val p = Point(1, 2)
    println(p.toString() + " " + (p == Point(1, 2)) + " " + p.copy(y = 5))
    val (x, y) = p
    println(x + y)

    var calls = 0
    val greeting = "hi"
    open class Greeter(val who: String) {
        constructor() : this("you")
        open fun greet(): String {
            calls++
            return "$greeting, $who"
        }
    }
    class Loud(who: String) : Greeter(who) {
        override fun greet() = super.greet().uppercase()
    }
    println(Greeter("ann").greet() + " / " + Greeter().greet() + " / " + Loud("bob").greet())
    println(calls)
    val makers = listOf("x", "y").map { Greeter(it) }
    println(makers.map { it.greet() } + calls)

    println(Shop("s").receipt(listOf(3, 4)))
    println(tree(2))
    println(Greeter::class.java.simpleName + " " + Greeter::class.java.isLocalClass)
}
