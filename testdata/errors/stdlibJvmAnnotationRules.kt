// Where kotlin.jvm's annotations can be written, and the names
// @JvmName gives.
import kotlin.reflect.KClass

interface I {
    @JvmOverloads fun f(s: String = "") // ERROR '@JvmOverloads' annotation cannot be used on interface methods
    @Synchronized fun g() = 1 // ERROR '@Synchronized' annotation cannot be used on interface members
}

abstract class Base {
    @JvmOverloads abstract fun f(s: String = "") // ERROR '@JvmOverloads' annotation cannot be used on abstract methods
    @Synchronized abstract fun g() // ERROR '@Synchronized' annotation cannot be used on abstract functions
    @Synchronized open fun h() = 1
    @JvmOverloads fun ok(s: String = "") = s
}

annotation class A @JvmOverloads constructor(val x: Int = 1) // ERROR '@JvmOverloads' annotation cannot be used on constructors of annotation classes
annotation class Projected(val k: Array<out KClass<*>>) // ERROR projections are not allowed in types of annotation members

class P(@JvmStatic val z: Int) { // ERROR only members in named objects and companion objects can be annotated with '@JvmStatic'
    @delegate:JvmSynthetic val d by lazy { 1 } // ERROR '@JvmSynthetic' annotation cannot be used on delegated properties
    @get:JvmSynthetic val e by lazy { 1 }
}

@Strictfp class S // ERROR '@Strictfp' annotation on classes is unsupported yet

@JvmName("with.dot") fun dotted() {} // ERROR illegal JVM name
@JvmName("") fun empty() {} // ERROR illegal JVM name
@JvmName("fine") fun named() {}

fun local() {
    @JvmOverloads fun f(s: String = "") = s // ERROR '@JvmOverloads' annotation cannot be used on local declarations
    @JvmStatic fun g() {} // ERROR only members in named objects and companion objects can be annotated with '@JvmStatic'
    class L {
        @JvmOverloads fun f(s: String = "") = s // ERROR '@JvmOverloads' annotation cannot be used on local declarations
    }
}
