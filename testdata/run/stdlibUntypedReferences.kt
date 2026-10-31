// Without an expected function type, a reference to a function is a
// kotlin.reflect KFunction, with a class of its own, as with kotlinc.

typealias Reg = (Any, String, Any?) -> Unit

open class Chan<E> {
    protected open fun register(select: String, element: Any?) { println("register $select $element") }
    @Suppress("UNCHECKED_CAST")
    fun clause(): Reg = Chan<*>::register as Reg
    fun twice(x: Int) = x * 2
}

fun top(x: Int) = x + 1
class Box(val v: Int)

@Suppress("UNCHECKED_CAST")
fun main() {
    val c = Chan<Int>()
    c.clause()(c, "s", 3)
    val r = ::top
    println(r(1))
    println(r.name)
    println(r == ::top)
    val b = c::twice
    println(b(4))
    println(b.name)
    val any: Any = ::Box
    println((any as (Int) -> Box)(5).v)
    println(::top.javaClass.name)
    println(r.javaClass.name)
    println(b.javaClass.name)
    println(any.javaClass.name)
    println(c.clause().javaClass.name)
    println(c.clause() == c.clause())
}
