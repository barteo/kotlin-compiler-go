// Inner classes: instances hold their outer class's, which their code
// uses as this@Outer and through the outer class's members.

class Bag<T>(private vararg val items: T) : Iterable<T> {
    private var reads = 0

    inner class Cursor(private var i: Int = 0) : Iterator<T> {
        override fun hasNext() = i < items.size
        override fun next(): T {
            reads++
            return items[i++]
        }
        fun bag(): Bag<T> = this@Bag
    }

    override fun iterator(): Iterator<T> = Cursor()
    fun from(i: Int) = Cursor(i)
    fun reads() = reads
}

open class Tree(val name: String) {
    inner class Node(val label: String) {
        constructor(n: Int) : this("#$n")
        fun path() = "$name/$label"
        inner class Leaf(val x: Int) {
            fun path() = this@Node.path() + "/" + x + " in " + name
        }
        fun leaf(x: Int) = Leaf(x)
        fun later(): () -> String = { path() + "!" }
        fun asObject(): Any = object {
            override fun toString() = "object of " + this@Tree.name
        }
    }
    fun root() = Node("root")
}

class Forest(name: String) : Tree(name) {
    fun child(n: Int) = Node(n)
}

fun main() {
    val b = Bag("a", "b", "c")
    println(b.joinToString())
    val c = b.from(1)
    println(c.next() + c.next() + " " + c.hasNext())
    println(b.reads().toString() + " " + (c.bag() === b))
    val t = Tree("oak")
    println(t.root().path())
    val n = t.Node("branch")
    println(n.path())
    println(n.leaf(3).path())
    println(n.Leaf(4).path())
    println(n.later()())
    println(n.asObject())
    println(Forest("pine").child(7).path())
    val typed: Bag<Int>.Cursor = Bag(1, 2).from(0)
    println(typed.next() + 10)
}
