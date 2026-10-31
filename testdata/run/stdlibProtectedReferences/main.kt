// References to a superclass's protected properties of another package
// are KProperty objects of classes of their own, which call the accessors
// through accessors of the subclass.
import base.Base

class Sub : Base() {
    fun readLabel(): String {
        val label = ::label
        return label.get() + " " + label.name
    }

    fun increment(): Int {
        val count = this::count
        count.set(count.get() + 1)
        return count()
    }

    fun setGuarded(): String {
        val guarded = ::guarded
        guarded.set("set")
        return guarded.get()
    }
}

fun main() {
    val s = Sub()
    println(s.readLabel())
    println(s.increment())
    println(s.setGuarded() + " " + s.guarded)
}
