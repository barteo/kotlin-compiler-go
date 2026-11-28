// Errors kotlinc reports where kc used to report more after them: a call
// that resolves despite an error keeps its function's type.
import java.util.Date // ERROR conflicting import
import java.util.Date // ERROR conflicting import

class Box(val n: Int) {
    fun twice(): Int = n * 2
    fun named(a: Int): Int = a
}

class A

fun <T> id(t: T): T = t

fun unsafeCall(b: Box?) {
    val s: String = run { b.twice() } // ERROR only safe
}

fun unsafeProperty(b: Box?) {
    val s: String = b.n // ERROR only safe
}

fun arity(b: Box) {
    val x = run { b.named(1, 2) } // ERROR takes 2 arguments
    x.foo() // ERROR unresolved reference: foo
}

fun namedNotFound(b: Box) {
    val x = run { b.named(c = 1) } // ERROR cannot find a parameter
    x.foo() // ERROR unresolved reference: foo
}

fun errorReceiver() {
    val s: Int = run { unresolved.toString() } // ERROR unresolved reference: unresolved
}

fun implicitProperty(b: Box?) {
    val x = with(b) { n } // ERROR only safe
    x.foo() // ERROR unresolved reference: foo
}

fun implicitCall(b: Box?) {
    val s: String = with(b) { twice() } // ERROR only safe
}

fun Box.ext(): Int = n

fun implicitExtension(b: Box?) {
    val s: String = with(b) { ext() } // ERROR only safe
}

fun classAsValue() {
    val x = id(A) // ERROR cannot be used as a value
    val s: String = A // ERROR cannot be used as a value
}

fun sameImport(): Date = Date()

class Outer {
    fun member(): Int = 1
    fun String.memberExt(): Int = 2
    val prop = 3

    class Nested {
        fun f(s: String) {
            val a: String = member() // ERROR outer class 'Outer' of non-inner class
            val b: String = s.memberExt() // ERROR outer class 'Outer' of non-inner class
            val c = with(s) { memberExt() } // ERROR outer class 'Outer' of non-inner class
            val d: String = prop // ERROR outer class 'Outer' of non-inner class
            c.foo() // ERROR unresolved reference: foo
        }
    }
}

fun jumpInCondition(x: Any): String {
    when (x) {
        return "" -> {}
    }
}
