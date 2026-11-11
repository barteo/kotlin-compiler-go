package text

import kotlin.properties.Delegates

// @JvmStatic extensions and delegated properties, which Java calls as
// static methods that take the receiver first.
object Strings {
    @JvmStatic fun String.shout(times: Int = 1): String = uppercase() + "!".repeat(times)

    @JvmStatic val String.words: Int get() = split(" ").size

    @JvmStatic var StringBuilder.last: Char
        get() = this[length - 1]
        set(value) {
            setCharAt(length - 1, value)
        }

    @JvmStatic val greeting: String by lazy { "hello" }

    @JvmStatic var level: Int by Delegates.observable(1) { _, old, new -> println("level $old -> $new") }
}

class Counter {
    companion object {
        @JvmStatic fun Int.times3(): Int = this * 3

        @JvmStatic val Int.even: Boolean get() = this % 2 == 0

        @JvmStatic var StringBuilder.first: Char
            get() = this[0]
            set(value) {
                setCharAt(0, value)
            }

        @JvmStatic val start: Int by lazy { 100 }
    }
}
