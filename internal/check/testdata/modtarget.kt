// The warnings kotlinc 2.4.21 reports of the modifiers below, which mean nothing where they are, as
// TestWarningsModifierTarget reads them: each line ends with a comment naming
// the warnings it reports, if any.
open interface I1 // REDUNDANT_MODIFIER_FOR_TARGET
public open interface I2 { open interface N } // REDUNDANT_MODIFIER_FOR_TARGET
abstract interface I3
open abstract interface I4 // REDUNDANT_MODIFIER REDUNDANT_MODIFIER_FOR_TARGET
abstract open interface I7 // REDUNDANT_MODIFIER REDUNDANT_MODIFIER_FOR_TARGET
open fun interface I5 { fun f() } // REDUNDANT_MODIFIER_FOR_TARGET
@Suppress("REDUNDANT_MODIFIER_FOR_TARGET")
open interface I6
open class C1 { open interface N } // REDUNDANT_MODIFIER_FOR_TARGET
final enum class E0 { X }
final object O
enum class E {
    external A {
        fun foo() {}
    },
    B,
    external C,
    @Suppress("DEPRECATED_MODIFIER_FOR_TARGET") external D,
    @Deprecated("") external F // DEPRECATED_MODIFIER_FOR_TARGET
}
class P(external val a: String, external var b: Int, val c: Int) // DEPRECATED_MODIFIER_FOR_TARGET
class P2(@Suppress("DEPRECATED_MODIFIER_FOR_TARGET") external val a: String)
data class P3(private external val a: String) // DEPRECATED_MODIFIER_FOR_TARGET
annotation class An(external val a: String) // DEPRECATED_MODIFIER_FOR_TARGET
enum class E2(external val x: Int) { A(1) } // DEPRECATED_MODIFIER_FOR_TARGET
enum class E5 {
    external A { // DEPRECATED_MODIFIER_FOR_TARGET
        fun foo() {}
    },
    B,
    external C(), // DEPRECATED_MODIFIER_FOR_TARGET
    @Deprecated("") external F; // DEPRECATED_MODIFIER_FOR_TARGET
}
