// $default methods and constructors whose values were left off the stack:
// a Unit parameter's default value, a local class whose default values
// read what it captures, and an inline function's omitted Unit argument.

var log = ""

fun note(s: String) {
    log += s
}

fun unitFirst(x: Unit = note("a"), y: Int): Int = y

inline fun unitInline(u: Unit = note("b"), k: Int): Int = k + 1

inline fun unitPassed(u: Unit, k: Int): Int = k + 2

class Event(val now: String)

open class Base(val fn: () -> String)

class Test(x: String) : Base({
    class Local(val t: String = x)
    Local().t
})

fun main() {
    println(unitFirst(y = 1))
    println(unitFirst(note("c"), 2))
    println(unitInline(k = 3))
    println(unitPassed(note("d"), 4))
    println(log)

    val s = "captured"
    class A(val param: String = "OK") {
        val s2 = s + param
    }
    println(A().s2)
    println(A("Test").s2)

    val a: Event.() -> String = {
        class MyEvent(val result: String = now)
        MyEvent().result
    }
    println(Event("now").a())
    println(Test("lambda").fn())
}
