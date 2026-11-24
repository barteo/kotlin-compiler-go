// The errors of kotlinc's JVM backend: CONFLICTING_JVM_DECLARATIONS,
// ACCIDENTAL_OVERRIDE and DUPLICATE_CLASS_NAMES. kotlinc reports them only
// of a module its frontend reports no errors in, so this file has no others.

class Writer {
    fun emit(xs: java.util.ArrayList<String>) {} // ERROR platform declaration clash
    fun emit(xs: java.util.ArrayList<Int>) {} // ERROR platform declaration clash
}

fun top(xs: java.util.ArrayList<String>) {} // ERROR platform declaration clash
fun top(xs: java.util.ArrayList<Int>) {} // ERROR platform declaration clash

@JvmName("topNumbers")
fun top2(xs: java.util.ArrayList<Int>) {}
fun top2(xs: java.util.ArrayList<String>) {}

// A Kotlin property doesn't override a Java getter: its getter clashes.
abstract class Named : java.security.Principal {
    val name: String = "x" // ERROR accidental override: the following declarations have the same JVM signature (getName()Ljava/lang/String;)
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
fun v(vararg x: Int) {} // ERROR platform declaration clash
fun v(x: IntArray) {} // ERROR platform declaration clash

