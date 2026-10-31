// kotlinc: -jvm-default=disable
// Interfaces compiled without JVM default methods: their bodies are in
// DefaultImpls classes, which classes that implement them call.

interface Shape {
    fun area(): Int
    fun describe(scale: Int = 2): String = "area " + area() * scale + helper()
    private fun helper() = "!"
    fun doubler(): () -> Int = { area() * 2 }
}

interface Square : Shape {
    val side: Int
    override fun area(): Int = side * side
    fun perimeter() = 4 * side
}

interface Fixed : Square {
    override fun describe(scale: Int): String
}

class Unit3 : Shape {
    override fun area() = 3
}

class Sq(override val side: Int) : Square {
    override fun describe(scale: Int) = "square " + super<Square>.describe(scale)
}

abstract class Base : Fixed

class Concrete : Base() {
    override val side = 5
    override fun describe(scale: Int) = "fixed " + scale
}

interface Stack<E> : Collection<E> {
    fun push(e: E): Stack<E>
    fun pushAll(es: List<E>): Stack<E> {
        var s = this
        for (e in es) s = s.push(e)
        return s
    }
}

class ListStack<E>(private val items: List<E>) : Stack<E>, Collection<E> by items {
    override fun push(e: E): Stack<E> = ListStack(items + e)
    override fun toString() = items.toString()
}

interface Named {
    val name: String
        get() = "named"
    var count: Int
        get() = 1
        set(value) { println("set " + value) }
    val String.twice: String
        get() = this + this + name
    val id: Int
}

interface SubNamed : Named {
    override val name: String
        get() = "sub " + super.name
}

interface SubNamed2 : SubNamed

class Plain : Named {
    override val id = 7
}

class Over : SubNamed2 {
    override val id = 8
    override var count: Int
        get() = super.count + 10
        set(value) { super.count = value * 2 }
}

open class Partial : Named {
    override val name get() = "partial"
    override val id = 9
}

interface Counted : Named {
    override var count: Int
        get() = 5
        set(value) { println("counted " + value) }
}

class Rest : Partial(), Counted

fun main() {
    println(Unit3().describe())
    println(Unit3().describe(5))
    println(Unit3().doubler()())
    val s: Shape = Sq(3)
    println(s.describe())
    println(Sq(2).perimeter())
    println(Concrete().describe() + " " + Concrete().area() + " " + Concrete().perimeter())
    val st = ListStack(listOf(1)).pushAll(listOf(2, 3))
    println(st.toString() + " " + st.size + " " + st.contains(2))
    val p = Plain()
    println(p.name + " " + p.count + " " + p.id)
    p.count = 3
    with(p) { println("ab".twice) }
    val o: Named = Over()
    println(o.name + " " + o.count)
    o.count = 4
    with(o) { println("c".twice) }
    val r: Named = Rest()
    r.count = 6
    println(r.name + " " + r.count + " " + r.id)
    with(r) { println("d".twice) }
}
