// The warnings kotlinc 2.4.21 reports with -Wextra of the modality
// modifiers below, as TestWarningsModality reads them: each line ends with
// a comment naming the warnings it reports, if any.
interface I {
    open val gav: Int get() = 42 // REDUNDANT_MODALITY_MODIFIER
    abstract fun foo() // REDUNDANT_MODALITY_MODIFIER
    open fun goo() {} // REDUNDANT_MODALITY_MODIFIER
    abstract var bar: Int // REDUNDANT_MODALITY_MODIFIER
    fun x()
    private fun p() {}
}
abstract class Base {
    final fun foo() {} // REDUNDANT_MODALITY_MODIFIER
    abstract fun bar()
    open val gav = 42
    override fun toString(): String = ""
}
class FinalDerived : Base() {
    override final fun bar() {} // REDUNDANT_MODALITY_MODIFIER
}
open class OpenDerived : Base() {
    override final fun bar() {}
    override open val gav = 13 // REDUNDANT_MODALITY_MODIFIER
    private final fun fan() {} // REDUNDANT_MODALITY_MODIFIER
}
final class Final // REDUNDANT_MODALITY_MODIFIER
abstract class AD1(override final val gav: Int) : I {
    override open fun foo() {} // REDUNDANT_MODALITY_MODIFIER
    override var bar = 1
    override fun x() {}
}
abstract interface AI // REDUNDANT_MODALITY_MODIFIER
final object FO // REDUNDANT_MODALITY_MODIFIER
sealed class S { abstract fun f(); open fun g() {} }
class SD : S() { override final fun f() {} } // REDUNDANT_MODALITY_MODIFIER
sealed class S2 : S() { override open fun f() {} ; override final fun g() {} } // REDUNDANT_MODALITY_MODIFIER
enum class E { A, B; final fun f() {}; open fun g() {} } // REDUNDANT_MODALITY_MODIFIER
enum class E2 : I { A { override fun foo() {} }; override final fun x() {}; override var bar = 1; override open fun goo() {} ; override fun foo() {} } // REDUNDANT_MODALITY_MODIFIER
object O { final fun f() {} } // REDUNDANT_MODALITY_MODIFIER
data class D(val a: Int) { final override fun toString() = "" } // REDUNDANT_MODALITY_MODIFIER
class C { companion object { final fun f() {} } ; final inner class In; final class N } // REDUNDANT_MODALITY_MODIFIER
annotation class An
fun loc() { open class L2 { final fun f() {} }; L2() } // REDUNDANT_MODALITY_MODIFIER
open class OC { final override fun hashCode() = 0; open fun h() {} }
class OC2 : OC() { final override fun h() {} } // REDUNDANT_MODALITY_MODIFIER
abstract class AC2 : OC() { open override fun h() {} } // REDUNDANT_MODALITY_MODIFIER
val anon = object : OC() { override final fun h() {} } // REDUNDANT_MODALITY_MODIFIER
open class OC3 : OC() { open override fun h() {} } // REDUNDANT_MODALITY_MODIFIER
