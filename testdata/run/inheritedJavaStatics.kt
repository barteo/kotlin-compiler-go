// A Java superclass's static fields and methods are in scope, unqualified,
// in its subclasses' code and in their nested classes': RESULT_OK in an
// Activity.
open class Base : Thread()

inline fun <R> run2(f: () -> R): R = f()

class Worker : Base() {
    val direct = MAX_PRIORITY
    val label = { x: Int ->
        when (x) {
            MAX_PRIORITY -> "max"
            else -> "other"
        }
    }

    fun lowest(): Int = run2 { MIN_PRIORITY }

    fun current() = currentThread() != null

    object Nested {
        val norm = NORM_PRIORITY
        fun interruptedNow() = interrupted()
    }
}

fun main() {
    val w = Worker()
    println(w.direct)
    println(w.label(10))
    println(w.label(3))
    println(w.lowest())
    println(w.current())
    println(Worker.Nested.norm)
    println(Worker.Nested.interruptedNow())
}
