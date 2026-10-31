package lib

// A base class whose protected members subclasses in other packages use
// from lambdas and objects, as a view model base class's are.
abstract class Base {
    private val log = StringBuilder()

    protected var state: String = "idle"
    protected val count: Int get() = log.length
    protected open var mode = 0

    protected fun emit(s: String) {
        log.append(s).append(';')
    }

    protected suspend fun emitLater(s: String): Int {
        emit("later $s")
        return log.length
    }

    protected fun <T> pick(a: T, b: T): T = if (state == "idle") a else b

    protected open fun describe(): String = "base"

    protected fun String.tagged(): String = "<$this>"

    protected val String.size2: Int get() = length * 2

    protected companion object {
        fun helper() = "helper"
        const val LIMIT = 3
    }

    fun dump() = log.toString()
}
