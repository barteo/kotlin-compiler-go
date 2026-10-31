// is, !is, as, as? and the smart casts they allow.

interface Expr

class Num(val value: Int) : Expr {
    override fun equals(other: Any?): Boolean = other is Num && other.value == value
    override fun hashCode(): Int = value
}

class Add(val left: Expr, val right: Expr) : Expr

class Neg(val operand: Expr) : Expr

fun eval(e: Expr): Int = when (e) {
    is Num -> e.value
    is Add -> eval(e.left) + eval(e.right)
    is Neg -> -eval(e.operand)
    else -> 0
}

fun show(e: Expr): String {
    if (e is Num) return "" + e.value
    if (e !is Add) {
        return if (e is Neg) "-" + show(e.operand) else "?"
    }
    return "(" + show(e.left) + " + " + show(e.right) + ")"
}

fun describe(x: Any?): String = when (x) {
    null -> "null"
    is Int -> "int " + (x + 1)
    is String -> "string of " + x.length
    is Long, is Double -> "number"
    is Num -> "num " + x.value
    else -> "other"
}

fun lengthOf(x: Any): Int {
    if (x !is String) {
        return -1
    }
    return x.length
}

fun both(a: Any, b: Any): Boolean = a is String && b is String && a.length == b.length

fun firstWord(x: Any?): String {
    if (x is String && x.length > 0 || x == null) {
        return "maybe"
    }
    return "no"
}

open class Shape {
    open fun area(): Double = 0.0
}

class Circle(val r: Double) : Shape() {
    override fun area(): Double = 3.0 * r * r
    fun diameter(): Double = 2 * r
}

class Square(val side: Double) : Shape() {
    override fun area(): Double = side * side
}

class Holder(val content: Any) {
    fun describe(): String {
        if (content is String) {
            return "text " + content.length
        }
        return "something"
    }
}

abstract class Node {
    fun size(): Int = when (this) {
        is Leaf -> 1
        is Branch -> 1 + left.size() + right.size()
        else -> 0
    }

    fun label(): String {
        if (this is Leaf) {
            return "leaf " + name
        }
        return "branch"
    }
}

class Leaf(val name: String) : Node()

class Branch(val left: Node, val right: Node) : Node()

fun main() {
    val e = Add(Num(1), Neg(Add(Num(2), Num(3))))
    println(eval(e))
    println(show(e))
    println(Num(3) == Num(3))
    println(Num(3) == Num(4))
    println(Num(3).equals("3"))

    val values = java.util.ArrayList<Any?>()
    values.add(1)
    values.add("hello")
    values.add(null)
    values.add(2L)
    values.add(1.5)
    values.add(Num(7))
    values.add(true)
    for (v in values) {
        println(describe(v))
    }
    println(lengthOf("four") + lengthOf(4))
    println(both("ab", "cd"))
    println(both("ab", 1))
    println(firstWord(null) + " " + firstWord("") + " " + firstWord("x"))

    val shapes = java.util.ArrayList<Shape>()
    shapes.add(Circle(1.0))
    shapes.add(Square(2.0))
    for (s in shapes) {
        val c = s as? Circle
        println(c?.diameter())
        if (s is Circle) {
            println("circle " + s.diameter())
        } else if (s is Square) {
            println("square " + s.side)
        }
    }
    val any: Any = Circle(2.0)
    val circle = any as Circle
    println(circle.r)
    println((any as Shape).area())
    val n: Any = 41
    println((n as Int) + 1)
    println(n as? String)
    println("text" as? String)

    try2()

    println(Holder("abc").describe() + ", " + Holder(1).describe())
    val tree = Branch(Leaf("a"), Branch(Leaf("b"), Leaf("c")))
    println(tree.size())
    println(Leaf("x").label() + " " + tree.label())

    val f: () -> Any = { "lambda" }
    val r = f()
    val g = { if (r is String) r.length else 0 }
    println(g())

    when (val k = values.size) {
        7 -> println("seven: " + k)
        else -> println("other: " + k)
    }
    val flag = values.size > 3
    val word = when (flag) {
        true -> "yes"
        false -> "no"
    }
    println(word)
}

fun try2() {
    val nothing: Any? = null
    val s = nothing as? String
    println(s == null)
}
