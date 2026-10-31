// Object expressions: anonymous classes that implement interfaces and
// extend classes, capture locals, parameters and the enclosing this, and
// nest.
interface Greeter {
    fun greet(name: String): String
}

abstract class Shape(val name: String) {
    abstract fun area(): Double
    open fun describe(): String = name + " with area " + area()
}

class Counter(val start: Int) {
    private var hits = 0

    fun greeter(suffix: String): Greeter {
        var calls = 0
        return object : Greeter {
            override fun greet(name: String): String {
                calls++
                hits++
                return name + suffix + " #" + calls + " from " + this@Counter.start + ", hits " + hits
            }
        }
    }

    fun hits() = hits
}

class Pool(val prefix: String) {
    val done = java.util.ArrayList<String>()

    // The superclass's constructor takes captured values.
    fun worker(id: Int): Thread {
        val name = prefix + "-" + id
        return object : Thread(name) {
            private val square = id * id

            override fun run() {
                done.add(getName() + " " + square + " " + this@Pool.prefix)
            }
        }
    }
}

fun square(side: Double): Shape = object : Shape("square") {
    override fun area() = side * side
    override fun describe() = "a " + super.describe()
}

fun main() {
    val g = object : Greeter {
        override fun greet(name: String) = "Hi " + name
    }
    println(g.greet("Ann"))

    val c = Counter(10)
    val cg = c.greeter("!")
    println(cg.greet("Bob"))
    println(cg.greet("Cy"))
    println(c.hits())

    println(square(3.0).describe())

    var total = 0
    val adder = object {
        val base = 100
        fun add(x: Int): Int {
            total += x
            return base + total
        }
    }
    println(adder.add(1))
    println(adder.add(2))
    println(total)

    val list = java.util.ArrayList<Int>()
    list.add(3)
    list.add(1)
    list.add(2)
    java.util.Collections.sort(list, object : java.util.Comparator<Int> {
        override fun compare(a: Int, b: Int) = b - a
    })
    println(list)

    val r = object : Runnable {
        var n = 0

        init {
            n = 5
        }

        override fun run() {
            n++
            println("run " + n)
        }
    }
    r.run()
    val t = Thread(r)
    t.start()
    t.join()

    val nested = object : Greeter {
        override fun greet(name: String): String {
            val inner = object : Greeter {
                override fun greet(name: String) = "inner " + name + " " + list.size
            }
            return "outer(" + inner.greet(name) + ")"
        }
    }
    println(nested.greet("Dee"))

    val pool = Pool("w")
    for (i in 1..2) {
        val w = pool.worker(i)
        w.start()
        w.join()
    }
    println(pool.done)
}
