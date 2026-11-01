// Callable references whose types kc infers from their receivers, their
// overloads and their uses.

class Box<T>(val v: T) {
    fun get(): T = v
    inner class Inner(val n: Int) {
        fun show(): String = "$v/$n"
    }
}

fun <T> Box<T>.first(): T = v

class A {
    companion object {
        fun foo(): String = "A.Companion.foo"
    }
}

class B {
    companion object {
        fun foo(): String = "B.Companion.foo"
    }
}

fun B.foo(): String = "B.foo"

fun top(): Int = 1
fun Int.intExt(): Int = this

fun foo1() {}
fun foo2(i: Int) {}
fun bar(f: () -> Unit): String = "bar()"
fun bar(f: (Int) -> Unit): String = "bar(Int)"

fun <T> apply1(x: T, f: (T) -> String): String = f(x)

fun main() {
    // The receiver's type arguments are the reference's.
    val get = Box<String>::get
    println(get(Box("a")))
    val first = Box<Int>::first
    println(first(Box(2)) + 1)
    val inner = Box<String>::Inner
    println(inner(Box("o"), 3).show())

    // A bound reference to a builtin member isn't an extension's.
    var state = 0
    val f = state::toString
    state = 5
    println(f() + " " + state)

    // An extension wins over a companion's member.
    val a: () -> String = A::foo
    val b: (B) -> String = B::foo
    println(a())
    println(b(B()))

    // A reference compared is checked on its own.
    println(::top == Int::intExt)

    // The overload taking the reference's shape.
    println(bar(::foo1))
    println(bar(::foo2))

    // A reference inferred with the call's other arguments.
    println(apply1(7, Int::toString))
}
