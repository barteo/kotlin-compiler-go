// Imports of classes and members of the default package: a nested class,
// an object's members, enum entries one by one and by *, and an imported
// enum entry invoked.
import Outer.Nested
import Host.greet
import Host.count
import Level.LOW
import Color.*

class Outer {
    class Nested(val v: Int)
}

object Host {
    var count = 0
    fun greet(name: String) = "hi $name"
}

enum class Level {
    LOW,
    HIGH;

    operator fun invoke(n: Int) = "$name $n"
}

enum class Color { RED, GREEN }

fun main() {
    println(Nested(3).v)
    println(greet("x"))
    count += 2
    count++
    println(count)
    println(LOW(5))
    println(GREEN)
    println(RED.ordinal)
}
