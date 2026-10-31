fun trace(s: String): Int {
    println("  " + s)
    return s.length
}

// A primary constructor, and secondary ones that call it, directly or
// through each other.
open class Point(val x: Int, val y: Int) {
    val sum = trace("sum of " + x + " and " + y)

    init {
        println("  init Point")
    }

    constructor(both: Int) : this(both, both) {
        println("  Point(both)")
    }

    constructor(s: String, scale: Int = 10) : this(s.length * scale) {
        println("  Point(s, scale)")
    }

    constructor() : this("") {
        println("  Point()")
    }

    override fun toString(): String = "(" + x + ", " + y + ")"
}

// No primary constructor: each secondary one calls the superclass's.
open class Shape {
    val id = trace("id")
    var label: String = "unnamed"

    constructor(label: String) {
        this.label = label
        println("  Shape(label)")
    }

    constructor(w: Int, h: Int) : this("rect " + w + "x" + h) {
        println("  Shape(w, h)")
    }

    constructor(vararg names: String) : super() {
        label = names.size.toString()
    }
}

class Circle : Shape {
    var r = 0.0

    constructor(r: Double) : super("circle") {
        this.r = r
    }

    constructor(r: Int) : this(r.toDouble()) {
        println("  Circle(Int)")
    }

    fun area(): Double = 3.0 * r * r
}

class Square(side: Int) : Shape(side, side)

// A class that extends a Java class without a primary constructor.
class AppError : Exception {
    var code = 0

    constructor(message: String) : super(message) {
        code = 0
    }

    constructor(message: String, code: Int) : super(message) {
        this.code = code
    }

    constructor(cause: Throwable) : super(cause) {
        code = -1
    }
}

class Box<T> {
    val items = java.util.ArrayList<T>()

    constructor(a: T) {
        items.add(a)
    }

    constructor(a: T, b: T) : this(a) {
        items.add(b)
    }
}

interface Named {
    fun name(): String
}

class Alias(n: Named) : Named by n {
    var calls = 0

    constructor(s: String) : this(Fixed(s)) {
        calls = 1
    }
}

class Fixed(val s: String) : Named {
    override fun name(): String = s
}

class Early {
    var v = 0

    constructor(x: Int) {
        if (x < 0) {
            println("  negative")
            return
        }
        v = x
    }
}

fun make(f: (Int) -> Point): Point = f(7)

open class Holder<T>(val value: T)

class IntHolder : Holder<Int> {
    constructor(x: Int) : super(x * 2)
    constructor(s: String) : super(s.length)
}

abstract class Animal {
    abstract fun sound(): String
    var name = "?"

    constructor(name: String) {
        this.name = name
    }

    constructor() : this("anonymous")
}

class Dog(name: String) : Animal(name) {
    override fun sound(): String = name + " says woof"
}

class Cat : Animal() {
    override fun sound(): String = name + " says meow"
}

data class Pair2(val a: Int, val b: Int) {
    constructor(both: Int) : this(both, both)
}

class Outer {
    class Inner {
        var v = ""

        constructor(x: Int) {
            v = "inner " + x
        }
    }

    companion object {
        fun make(): Outer.Inner = Outer.Inner(9)
    }
}

class Counter {
    var count = 0
    val actions = java.util.ArrayList<() -> Unit>()

    constructor(n: Int) {
        for (i in 1..n) {
            actions.add { count += i }
        }
        val run = { k: Int -> count = count * k }
        actions.add { run(10) }
    }
}

fun main() {
    println("Point(1, 2)")
    println(Point(1, 2))
    println("Point(3)")
    println(Point(3))
    println("Point(\"ab\")")
    println(Point("ab"))
    println("Point(\"ab\", scale = 2)")
    println(Point(scale = 2, s = "ab"))
    println("Point()")
    println(Point())

    println("Shape(\"s\")")
    println(Shape("s").label)
    println("Shape(2, 3)")
    println(Shape(2, 3).label)
    println("Shape(\"a\", \"b\", \"c\")")
    println(Shape("a", "b", "c").label)
    println("Circle(2)")
    val c = Circle(2)
    println(c.label + " " + c.area() + " " + c.id)
    println("Square(4)")
    println(Square(4).label)

    try {
        throw AppError("bad", 3)
    } catch (e: AppError) {
        println(e.message + " " + e.code)
    }
    val wrapped = AppError(IllegalStateException("inner"))
    println(wrapped.cause!!.message + " " + wrapped.code)
    println(AppError("plain").code)

    println(Box("x", "y").items)
    println(Box(1).items)

    val a = Alias("fixed")
    println(a.name() + " " + a.calls)

    println(Early(-1).v)
    println(Early(5).v)

    println(make(::Point))
    val fromString: (String) -> Shape = ::Shape
    println(fromString("ref").label)

    println(IntHolder(21).value + IntHolder("abc").value)
    val animals = java.util.ArrayList<Animal>()
    animals.add(Dog("rex"))
    animals.add(Cat())
    for (a in animals) {
        println(a.sound())
    }
    println(Pair2(4))
    println(Pair2(4) == Pair2(4, 4))
    println(Outer.make().v)
    val counter = Counter(3)
    for (act in counter.actions) {
        act()
    }
    println(counter.count)
}
