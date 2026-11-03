// Declarations that kotlinc tells apart by their Kotlin signatures,
// whatever their JVM ones.

fun f(x: Int): Int = x // ERROR conflicting overloads: f
fun f(x: Int): String = "" // ERROR conflicting overloads: f

class Funs {
    fun a(x: Int): Int = x // ERROR conflicting overloads: a
    fun a(x: Int) {} // ERROR conflicting overloads: a

    fun <T> b(t: T) where T : Comparable<T>, T : CharSequence {} // ERROR conflicting overloads: b
    fun <T> b(t: T): Int where T : CharSequence, T : Comparable<T> = 0 // ERROR conflicting overloads: b

    fun <T> c(t: T) {}
    fun <T : CharSequence> c(t: T) {}

    class Nested // ERROR conflicting overloads: Nested
    fun Nested() {} // ERROR conflicting overloads: Nested
    inner class Inner
    fun Inner() {}
}

class Ctor(val x: Int) // ERROR conflicting overloads: Ctor
fun Ctor(x: Int) = 0 // ERROR conflicting overloads: Ctor
fun Ctor(x: String) = 0

class Box<T>(val t: T)
typealias IntBox = Box<Int> // ERROR conflicting overloads: IntBox
fun IntBox(t: Int) = 0 // ERROR conflicting overloads: IntBox
fun IntBox(t: String) = 0

data class Pair2(val a: Int, val b: String) { // ERROR conflicting overloads
    fun component1(): Long = 0 // ERROR conflicting overloads: component1
    fun copy(a: Int, b: String): Int = 0 // ERROR conflicting overloads: copy
}

// One of them hidden: no conflict, unless one of them can be overridden.
class Final {
    fun h(x: Int): Int = x
    @Deprecated("", level = DeprecationLevel.HIDDEN) fun h(x: Int) {}
}

open class Open {
    open fun h(x: Int): Int = x // ERROR conflicting overloads: h
    @Deprecated("", level = DeprecationLevel.HIDDEN) fun h(x: Int) {} // ERROR conflicting overloads: h
}

@Deprecated("", level = DeprecationLevel.HIDDEN)
class HiddenClass() // ERROR conflicting overloads: HiddenClass
fun HiddenClass() {} // ERROR conflicting overloads: HiddenClass

// A function that takes the parameters of an inherited one, with other
// type parameters, which it doesn't override.
open class Base {
    fun <T> some(s: String): T = null!!
}

class Derived : Base() {
    fun some(text: String) = "" // ERROR conflicting overloads: some
}

// Properties and classifiers.
val Thing = 1 // ERROR redeclaration: Thing
object Thing // ERROR redeclaration: Thing

class Props {
    val Foo = 1 // ERROR redeclaration: Foo
    class Foo // ERROR redeclaration: Foo
    companion object {
        val G = 1 // ERROR redeclaration: G
        class G // ERROR redeclaration: G
    }
}

enum class E { // ERROR redeclaration: name
    name, // ERROR redeclaration: name
    other
}

// Classes with the same JVM name.
class A {
    class B // ERROR duplicate JVM class name 'A$B'
}
class `A$B` // ERROR duplicate JVM class name 'A$B'

class C {
    init {
        class D // ERROR duplicate JVM class name 'C$D'
    }
}
class `C$D` // ERROR duplicate JVM class name 'C$D'

// A vararg parameter isn't its array's, though the JVM tells them apart by neither.
fun v(vararg x: Int) {}
fun v(x: IntArray) {} // ERROR platform declaration clash
