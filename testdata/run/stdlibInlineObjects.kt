// Object expressions in the program's inline functions: each copy of the
// function creates an instance of the object's class, with the copy's
// values.

interface Wrapper {
    fun run(): String
}

inline fun wrap(crossinline block: () -> String): Wrapper = object : Wrapper {
    override fun run() = "[" + block() + "]"
}

inline fun counter(start: Int): Wrapper {
    var n = start
    return object : Wrapper {
        override fun run(): String {
            n++
            return "n=" + n
        }
    }
}

abstract class Shape(val sides: Int) {
    abstract fun name(): String
}

inline fun shape(n: Int, crossinline name: (Int) -> String) = object : Shape(n) {
    override fun name() = name(sides)
}

class Host(val name: String) {
    inline fun greeter(crossinline suffix: () -> String): Wrapper = object : Wrapper {
        override fun run() = "hi " + name + suffix()
    }

    private inline fun secret(crossinline f: () -> Int) = object {
        fun value() = f() + name.length
    }.value()

    fun reveal() = secret { 10 }
}

fun main() {
    val local = "L"
    println(wrap { "a" + local }.run())
    val c = counter(5)
    c.run()
    println(c.run())
    println(shape(3) { "tri" + it }.name())
    println(Host("bob").greeter { "!" }.run())
    println(Host("alice").reveal())
    println(listOf("x", "y").map { s -> wrap { s + s }.run() })
}
