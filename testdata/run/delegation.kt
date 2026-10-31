import java.util.Comparator

interface Shape {
    val name: String
    var size: Int
    fun area(): Double
    fun describe(): String = "shape " + name + " of size " + size
}

class Square(override var size: Int) : Shape {
    override val name: String get() = "square"
    override fun area(): Double = (size * size).toDouble()
}

// Delegating to a constructor property reads its field.
class Framed(val inner: Shape) : Shape by inner

// To a parameter, with an explicit override that wins over the delegate.
class Doubled(s: Shape) : Shape by s {
    override fun area(): Double = 2 * 1.0
}

interface Named {
    fun id(): String
    fun greet(greeting: String = "hello"): String = greeting + ", " + id()
}

class Person(val first: String) : Named {
    override fun id(): String = first
}

fun traced(s: String): String {
    println("initializing " + s)
    return s
}

fun makeShape(log: String): Shape {
    println("making shape for " + log)
    return Square(3)
}

// Two delegations, and a delegate that is an arbitrary expression.
class Both(n: Named) : Named by n, Shape by makeShape("both") {
    val tag = traced("tag")
    init {
        println("init of Both")
    }
}

interface Source<T> {
    fun next(): T
    fun peek(): T
}

class Counter(var n: Int) : Source<Int> {
    override fun next(): Int {
        n += 1
        return n - 1
    }
    override fun peek(): Int = n
}

class Logged<T>(private val src: Source<T>) : Source<T> by src {
    override fun next(): T {
        val v = src.next()
        println("next: " + v)
        return v
    }
}

class IntSource(src: Source<Int>) : Source<Int> by src

open class Base {
    open fun id(): String = "base"
}

// Delegation takes precedence over the superclass's implementation.
class Sub(n: Named) : Base(), Named by n

class Task(r: Runnable) : Runnable by r

class ByLength(c: Comparator<String>) : Comparator<String> by c

fun total(s: Source<Int>, k: Int): Int {
    var sum = 0
    for (i in 0 until k) {
        sum += s.next()
    }
    return sum
}

// A superclass overrides the delegated member, with an inferred return
// type; the delegation's bridge needs it.
interface Tested {
    fun test(): String = "Tested"
}

class TestedDelegate : Tested {
    override fun test() = "delegate"
}

abstract class Middle : Tested {
    override fun test() = "Middle"
}

abstract class Derived : Middle()

class Hiding : Derived(), Tested by TestedDelegate()

fun main() {
    val sq = Square(2)
    val f = Framed(sq)
    println(f.name)
    println(f.area())
    f.size = 5
    println(sq.size)
    println(f.describe())
    val shape: Shape = f
    println(shape.size)

    val d = Doubled(Square(4))
    println(d.area())
    println(d.describe())

    val b = Both(Person("ann"))
    println(b.id())
    println(b.greet())
    println(b.greet("hi"))
    println(b.area())
    println(b.tag)
    val named: Named = b
    println(named.greet())

    val logged = Logged(Counter(10))
    println(logged.next())
    println(logged.peek())
    println(total(logged, 3))

    val ints = IntSource(Counter(1))
    println(total(ints, 4))
    println(ints.peek() + 1)

    println(Sub(Person("bob")).id())
    val base: Base = Sub(Person("carl"))
    println(base.id())

    Task(Runnable { println("running") }).run()
    val cmp = ByLength(Comparator { a, b -> a.length - b.length })
    println(cmp.compare("aaa", "b"))
    val words = java.util.ArrayList<String>()
    words.add("ccc")
    words.add("a")
    words.add("bb")
    java.util.Collections.sort(words, cmp)
    println(words)
    val tested: Tested = Hiding()
    println(Hiding().test() + " " + tested.test())
}
