// The captures of an object expression's class and of a local class are
// stored before the superclass's constructor runs, which may call the
// members that use them.

abstract class WaitFor {
    init {
        condition()
    }

    abstract fun condition(): Boolean
}

open class Named(val name: String) {
    init {
        println("init ${describe()}")
    }

    open fun describe() = name
}

fun anon(): String {
    var result = "fail"
    val prefix = "O"
    object : WaitFor() {
        override fun condition(): Boolean {
            result = prefix + "K"
            return result.length == 2
        }
    }
    return result
}

fun local(): String {
    val suffix = "!"
    var calls = 0
    class Loud(n: String) : Named(n) {
        constructor() : this("second")

        override fun describe(): String {
            calls++
            return name + suffix
        }
    }
    Loud("first")
    Loud()
    return "calls $calls"
}

fun main() {
    println(anon())
    println(local())
}
