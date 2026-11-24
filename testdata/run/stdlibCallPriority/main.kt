// Which candidate a call resolves to where several have its name, as K2
// decides by levels of scope, and by specificity within one.
package app

class A {
    fun foo() = "A.foo"
}

class B {
    fun foo() = "B.foo"
}

// A member of a nearer implicit receiver comes before a variable of an
// outer scope that could be invoked with it.
fun nearerMember(foo: A.() -> String): String {
    val out = StringBuilder()
    with(A()) {
        out.append(foo()).append(' ')
        with(B()) { out.append(foo()) }
    }
    return out.toString()
}

// An object's invoke operator, and a companion function named as a
// nested object, which can't be invoked.
class E {
    object f {
        operator fun invoke() = "object f"
    }
    companion object {
        val f: () -> String = { "companion val f" }
    }
}

sealed class Loc {
    object None : Loc() {
        override fun toString() = "None"
    }
    companion object {
        fun None(description: String): Loc = None.also { println("companion None($description)") }
    }
}

// A class and a function of its name at one level: the more specific.
class Foo(val str: String) {
    override fun toString() = "class Foo($str)"
}
fun Foo(string: String?) = "fun Foo($string)"

class Bx(val x: String) {
    override fun toString() = "class Bx($x)"
}
private typealias A3 = Bx
fun A3(x: Any) = "fun A3($x)"

// Factory functions that call the constructor, which is more specific.
class Made(val b: String, val flag: Boolean = true) {
    override fun toString() = "Made($b, $flag)"
}
fun Made(b: String?, flag: Boolean = true): Made = Made(b!! + "!", flag)

// A companion's nested class comes before its functions, outside it.
class Owner {
    companion object {
        fun Bar(string: String) = "fun Bar($string)"
        class Bar(val str: String?) {
            override fun toString() = "class Bar($str)"
        }
        val inside = Bar("in")
    }
    val outside = Bar("out")
}

// super.x(y) calls the supertype's property, not this one.
open class Base {
    protected open val check: (String) -> String = { "base $it" }
}
class Derived : Base() {
    override val check = { y: String -> "derived " + super.check(y) }
    fun run() = check("y")
}

// A companion's property comes before a package of its name.
class WithC {
    companion object {
        val c: String = "companion c"
    }
    fun len() = c.length
}

// A member that doesn't take the argument's type gives way to a
// top-level function that does.
interface Shape
class Square : Shape
val String.shape: Shape get() = Square()
class Taker {
    fun take(arg: Square) = "member take"
    fun test(s: String) = take(s.shape)
}
fun take(arg: Any) = "top-level take"

fun main() {
    println(nearerMember { "outer foo" })
    println(E.f())
    println(Loc.None("x"))
    println(Foo("1"))
    println(Foo(null))
    println(A3(""))
    println(A3(1))
    println(Made("b"))
    println(Made(null as String? ?: "n"))
    println(Owner().outside)
    println(Owner.inside)
    println(Derived().run())
    println(WithC().len())
    println(Taker().test(""))
}
