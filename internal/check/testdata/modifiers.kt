// The warnings kotlinc 2.4.21 reports of the modifiers below, as
// TestWarningsModifiers reads them: each line ends with a comment naming
// the warnings it reports, if any.
open class Base { open fun b() {}; open val v: Int = 0 }
class F(open val p: Int, val q: Int) : Base() { // NON_FINAL_MEMBER_IN_FINAL_CLASS
    open fun a() {} // NON_FINAL_MEMBER_IN_FINAL_CLASS
    open override fun b() {} // NON_FINAL_MEMBER_IN_FINAL_CLASS
    override open val v: Int = 1 // NON_FINAL_MEMBER_IN_FINAL_CLASS
    @Suppress("NON_FINAL_MEMBER_IN_FINAL_CLASS")
    open fun s() {}
    internal open fun pr() {} // NON_FINAL_MEMBER_IN_FINAL_CLASS
    open var w: Int // NON_FINAL_MEMBER_IN_FINAL_CLASS
        get() = 0
        set(x) {}
}
object O { open val x = 1; open fun y() {} } // NON_FINAL_MEMBER_IN_OBJECT
class C { companion object { open fun z() {} } } // NON_FINAL_MEMBER_IN_OBJECT
enum class E {
    A { override fun e() {} },
    B { open fun g() {} }; // NON_FINAL_MEMBER_IN_FINAL_CLASS
    open fun e() {}
}
data class D(open val d: Int) // NON_FINAL_MEMBER_IN_FINAL_CLASS
@JvmInline value class V(val v: Int) { open fun f() {} } // NON_FINAL_MEMBER_IN_FINAL_CLASS
abstract class Abs { open fun f() {} }
sealed class S { open fun f() {} }
interface I { open fun f() {} }
annotation class Ann(val x: Int)
fun local() {
    class L { open fun f() {} } // NON_FINAL_MEMBER_IN_FINAL_CLASS
    val o = object { open fun f() {} } // NON_FINAL_MEMBER_IN_FINAL_CLASS
}
