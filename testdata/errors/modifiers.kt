open open class R1 // ERROR repeated 'open'
class R2(private private val x: Int) // ERROR repeated 'private'
final sealed class S1 // ERROR modifier 'final' is incompatible with 'sealed'
class Outer {
    inner data class D(val x: Int) // ERROR is incompatible with
    companion data object C // ERROR modifier 'data' is not applicable
}
object O {
    protected fun f() {} // ERROR modifier 'protected' is not applicable inside 'object'
}
interface I {
    internal fun g() {} // ERROR modifier 'internal' is not applicable inside 'interface'
    final fun h() {} // ERROR modifier 'final' is not applicable inside 'interface'
    private abstract val p: Int // ERROR is incompatible with
    var q: Int
        get() = 1
        protected set(v) {} // ERROR modifier 'protected' is not applicable inside 'interface'
}
protected enum class PE { X } // ERROR modifier 'protected' is not applicable inside 'file'
open fun topOpen() {} // ERROR only allowed on members of classes
open object OO // ERROR not allowed on objects
annotation class AP(protected val x: Int) // ERROR modifier 'protected' is not applicable inside 'annotation class'
enum class EE {
    X {
        protected fun f() {} // ERROR modifier 'protected' is not applicable inside 'enum entry'
    }
}

fun locals() {
    private class L1 // ERROR modifier 'private' is not applicable to 'local class'
    sealed class L2 // ERROR modifier 'sealed' is not applicable to 'local class'
    const val c = 1 // ERROR modifier 'const' is not applicable to 'local variable'
    private fun lf() {} // ERROR modifier 'private' is not applicable to 'local function'
}

// Valid modifiers.
abstract class Base protected constructor() {
    protected abstract fun a()
    internal open val b: Int get() = 1
    private inner class In
    companion object {
        protected const val K = 1
    }
}
interface J {
    private fun p() {}
    companion object
}
fun interface F { fun run() }
data object DO
@JvmInline value class V(val x: Int)
enum class Dir {
    N { override fun turn() = S }, S { override fun turn() = N };
    abstract fun turn(): Dir
    inner class Ei
}
inline fun <reified T> inl(noinline f: () -> Unit, crossinline g: () -> Unit) {}
external fun nat(): Int
class Props {
    lateinit var s: String
    var v: Int = 0
        private set
    val i: Int
        inline get() = 1
}
fun localOk() {
    lateinit var x: String
    abstract class LA
    open class LO : LA()
    data class LD(val x: Int)
    val o = object { private fun f() {} }
}
