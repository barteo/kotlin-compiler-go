// @JvmStatic extension functions, extension properties and delegated
// properties in objects and companion objects.

import Strings.shout
import Strings.words
import java.lang.reflect.Modifier
import kotlin.properties.Delegates

object Strings {
    private val suffix = "!"
    var calls = 0

    @JvmStatic fun String.shout(times: Int = 1): String {
        calls++
        return uppercase() + suffix.repeat(times)
    }

    @JvmStatic fun String?.orNone(): String = this ?: "none"

    @JvmStatic fun <T> List<T>.second(): T = this[1]

    @JvmStatic val String.words: Int get() = split(" ").size

    @JvmStatic var StringBuilder.last: Char
        get() = this[length - 1]
        set(value) {
            setCharAt(length - 1, value)
        }

    @JvmStatic val greeting: String by lazy {
        println("init greeting")
        "hello"
    }

    @JvmStatic var level: Int by Delegates.observable(1) { _, old, new -> println("level $old -> $new") }

    fun inside() = "in".shout() + " " + "a b".words + " " + greeting
}

class Box(val n: Int) {
    fun bump() = n.plusOne() + n.half

    companion object {
        @JvmStatic fun Int.plusOne(by: Int = 1): Int = this + by

        @JvmStatic val Int.half: Int get() = this / 2

        @JvmStatic var StringBuilder.first: Char
            get() = this[0]
            set(value) {
                setCharAt(0, value)
            }

        @JvmStatic val shared: List<String> by lazy {
            println("init shared")
            listOf("x", "y")
        }

        @JvmStatic var count: Int by Delegates.notNull()
    }
}

fun isStatic(c: Class<*>, name: String, vararg params: Class<*>) =
    Modifier.isStatic(c.getDeclaredMethod(name, *params).modifiers)

fun main() {
    println("a".shout())
    println("b".shout(3))
    println("x y z".words)
    println(Strings.inside())
    println(Strings.calls)
    with(Strings) {
        val n: String? = null
        println(n.orNone() + " " + "s".orNone())
        println(listOf(1, 2, 3).second())
        val sb = StringBuilder("abc")
        sb.last = 'z'
        println(sb.toString() + " " + sb.last)
        sb.last++
        println(sb)
        val maybe: String? = if (sb.length > 1) "p q" else null
        println(maybe?.words)
        println(maybe?.shout())
    }
    println(Strings.greeting)
    println(Strings.greeting)
    println(Strings.level)
    Strings.level = 5
    Strings.level += 2
    println(Strings.level)

    println(Box(6).bump())
    with(Box) {
        println(4.plusOne())
        println(4.plusOne(10))
        println(9.half)
        val sb = StringBuilder("abc")
        sb.first = 'q'
        println(sb.toString() + " " + sb.first)
    }
    println(Box.shared)
    println(Box.shared)
    Box.count = 3
    Box.count++
    println(Box.count)

    val s = Strings::class.java
    val str = String::class.java
    val int = Int::class.javaPrimitiveType!!
    val char = Char::class.javaPrimitiveType!!
    val sbc = StringBuilder::class.java
    println(isStatic(s, "shout", str, int) && isStatic(s, "shout\$default", str, int, int, Any::class.java))
    println(isStatic(s, "getWords", str) && isStatic(s, "getLast", sbc) && isStatic(s, "setLast", sbc, char))
    println(isStatic(s, "getGreeting") && isStatic(s, "getLevel") && isStatic(s, "setLevel", int))
    println(s.getMethod("shout", str, int).invoke(null, "r", 2))
    println(s.getMethod("getWords", str).invoke(null, "one two"))
    val b = Box::class.java
    println(isStatic(b, "plusOne", int, int) && isStatic(b, "getHalf", int) && isStatic(b, "getFirst", sbc) && isStatic(b, "setFirst", sbc, char))
    println(isStatic(b, "getShared") && isStatic(b, "getCount") && isStatic(b, "setCount", int))
    println(b.getMethod("plusOne", int, int).invoke(null, 1, 2))
    println(b.getMethod("getHalf", int).invoke(null, 10))
    println(b.getMethod("getShared").invoke(null))
}
