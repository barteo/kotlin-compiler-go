package lib

import kotlin.properties.Delegates

// @JvmStatic extensions and delegated properties of an object and of a
// companion object, which another module calls.
object Texts {
    @JvmStatic fun String.wrap(left: String = "[", right: String = "]"): String = left + this + right

    @JvmStatic fun String?.orEmpty2(): String = this ?: ""

    @JvmStatic val String.vowels: Int get() = count { it in "aeiou" }

    @JvmStatic var StringBuilder.tail: Char
        get() = this[length - 1]
        set(value) {
            setCharAt(length - 1, value)
        }

    @JvmStatic val table: Map<String, Int> by lazy { mapOf("one" to 1, "two" to 2) }

    @JvmStatic var mode: String by Delegates.observable("a") { _, old, new -> println("mode $old -> $new") }
}

class Counter {
    companion object {
        @JvmStatic fun Int.times3(): Int = this * 3

        @JvmStatic val Int.even: Boolean get() = this % 2 == 0

        @JvmStatic val start: Int by lazy { 100 }
    }
}
