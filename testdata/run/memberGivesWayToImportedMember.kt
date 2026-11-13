package tools

import tools.Utils.group
import tools.Utils.label

object Utils {
    fun group(a: Int, b: Int, key: (Int) -> Int): Int = key(a) + key(b)

    fun label(n: Int) = "label $n"
}

// A method of the class that can't take a call's arguments gives way to
// the imported member of an object that can, as a test method named
// after the function it tests.
class UtilsTest {
    fun group() {
        println(group(1, 2, key = { it * 10 }))
        println(group(3, 4) { it + 1 })
    }

    fun label(): String = label(7) + "!"
}

fun main() {
    UtilsTest().group()
    println(UtilsTest().label())
}
