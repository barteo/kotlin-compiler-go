// A library's internal declarations, which its friends, as a module's unit
// tests, see (kotlinc's -Xfriend-paths).
package lib

internal class Secret(internal val code: Int) {
    internal var label = "plain"
    internal fun twice() = code * 2
    fun open() = "open $code"

    internal companion object {
        internal const val MAX = 9
        internal fun make() = Secret(MAX)
    }
}

internal fun hidden(x: Int) = x + 1

internal val hiddenVal = 5

internal var counter = 0

internal val String.shout get() = uppercase() + "!"

internal typealias Code = Secret

internal object Registry {
    internal val names = listOf("a", "b")
}

class Public internal constructor(internal val inner: Int) {
    constructor() : this(7)

    internal fun peek() = inner
}

abstract class Service {
    internal abstract fun name(): String

    fun describe() = "service ${name()}"
}
