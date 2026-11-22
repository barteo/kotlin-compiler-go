// The warnings kotlinc 2.4.21 reports of the @JvmOverloads and @param: annotations below, as
// TestWarningsUselessOverloads reads them: each line ends with a comment naming
// the warnings it reports, if any.
@Target(AnnotationTarget.VALUE_PARAMETER, AnnotationTarget.PROPERTY)
annotation class Ann
@Target(AnnotationTarget.VALUE_PARAMETER)
annotation class Second

class C {
    @JvmOverloads constructor() {} // OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS
    @JvmOverloads constructor(a: Long) {} // OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS
    @JvmOverloads constructor(a: Int, b: Int = 1) {}
    @JvmOverloads fun foo(s: String) {} // OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS
    @JvmOverloads fun foo2() {} // OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS
    @JvmOverloads fun foo3(s: String = "") {}
    @JvmOverloads private fun p1(s: String = "OK") {} // OVERLOADS_PRIVATE
    @JvmOverloads private fun p2(s: String) {} // OVERLOADS_PRIVATE
    @JvmOverloads protected fun p3(s: String = "OK") {}
    @JvmOverloads internal fun p4(s: String = "OK") {}
    @JvmOverloads public fun p5(s: String = "OK") {}
    @Suppress("OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS") @JvmOverloads fun sup(s: String) {}
    @JvmOverloads fun String.ext() {} // OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS
    @JvmOverloads fun vf(vararg s: String) {} // OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS
    companion object {
        @JvmOverloads @JvmStatic fun st(s: String) {} // OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS
    }
}
class D @JvmOverloads constructor() // OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS
class D2 @JvmOverloads constructor(a: Int) // OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS
class D3 @JvmOverloads constructor(val a: Int = 1)
class D4 @JvmOverloads private constructor(val a: Int = 1) // OVERLOADS_PRIVATE
class D5 @JvmOverloads protected constructor(val a: Int = 1) { @JvmOverloads private constructor(s: String = "", t: Long) : this(1) } // OVERLOADS_PRIVATE
private class D6 @JvmOverloads constructor(val a: Int = 1) { @JvmOverloads fun f(s: String = "") {} }
@JvmOverloads fun top(s: String) {} // OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS
@JvmOverloads private fun topP(s: String = "") {} // OVERLOADS_PRIVATE
@JvmOverloads internal fun topI(s: String) {} // OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS
object O { @JvmOverloads fun f(s: String) {} } // OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS
open class Base { open fun ov(s: String = "") {} }
class Der : Base() { @JvmOverloads override fun ov(s: String) {} } // OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS

class PrimaryConstructorClass(
        @param:Ann a: String, // REDUNDANT_ANNOTATION_TARGET
@param:Ann val c: String,
@Ann d: String,
@Suppress("REDUNDANT_ANNOTATION_TARGET") @param:Ann e: String,
@param:Ann vararg f: String) // REDUNDANT_ANNOTATION_TARGET
enum class En(@param:Ann a: Int) { X(1) } // REDUNDANT_ANNOTATION_TARGET
annotation class AC(@param:Ann val a: Int)
