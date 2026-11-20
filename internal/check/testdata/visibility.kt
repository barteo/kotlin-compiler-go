// REDUNDANT_VISIBILITY_MODIFIER, with -Wextra: a visibility modifier that
// gives a declaration the visibility it has without one, or that the
// private or local class it is in hides.
fun f(): Any {
    class LocalClass {
        internal var foo = 0 // REDUNDANT_VISIBILITY_MODIFIER
        public fun bar() {} // REDUNDANT_VISIBILITY_MODIFIER
        private fun baz() {}
    }
    LocalClass().foo = 1
    return object {
        internal fun x() {} // REDUNDANT_VISIBILITY_MODIFIER
    }
}
public class C(public val a: Int, internal val b: Int) { // REDUNDANT_VISIBILITY_MODIFIER
    public val foo: Int = 0 // REDUNDANT_VISIBILITY_MODIFIER
    public fun bar() {} // REDUNDANT_VISIBILITY_MODIFIER
    internal fun baz() {}
    public class N // REDUNDANT_VISIBILITY_MODIFIER
    public interface NI // REDUNDANT_VISIBILITY_MODIFIER
    public object O // REDUNDANT_VISIBILITY_MODIFIER
    public companion object // REDUNDANT_VISIBILITY_MODIFIER
}
private class P {
    internal fun f() {} // REDUNDANT_VISIBILITY_MODIFIER
    public fun g() {} // REDUNDANT_VISIBILITY_MODIFIER
    internal class Q { // REDUNDANT_VISIBILITY_MODIFIER
        internal fun h() {} // REDUNDANT_VISIBILITY_MODIFIER
    }
    protected fun pr() {}
}
internal class I {
    public fun f() {} // REDUNDANT_VISIBILITY_MODIFIER
    internal fun g() {}
}
open class D {
    protected open fun a() {}
    protected open fun b() {}
    public open fun c() {} // REDUNDANT_VISIBILITY_MODIFIER
    internal open fun d() {}
}
class E : D() {
    protected override fun a() {} // REDUNDANT_VISIBILITY_MODIFIER
    public override fun b() {}
    public override fun c() {} // REDUNDANT_VISIBILITY_MODIFIER
    internal override fun d() {} // REDUNDANT_VISIBILITY_MODIFIER
}
enum class F private constructor(val x: Int) { A(1) } // REDUNDANT_VISIBILITY_MODIFIER
sealed class G protected constructor() { // REDUNDANT_VISIBILITY_MODIFIER
    protected constructor(x: Int) : this() // REDUNDANT_VISIBILITY_MODIFIER
    private constructor(x: Double) : this()
}
class H public constructor() // REDUNDANT_VISIBILITY_MODIFIER
interface II {
    public fun i() // REDUNDANT_VISIBILITY_MODIFIER
    private fun j() {}
}
public typealias TA = Int // REDUNDANT_VISIBILITY_MODIFIER
public var top = 0 // REDUNDANT_VISIBILITY_MODIFIER
    public get // REDUNDANT_VISIBILITY_MODIFIER
    private set
var top2 = 0
    public set // REDUNDANT_VISIBILITY_MODIFIER
open class J {
    protected var x = 0
        protected set // REDUNDANT_VISIBILITY_MODIFIER
    protected val y = 0
        protected get // REDUNDANT_VISIBILITY_MODIFIER
}
open class Owner {
    open class Base {
        protected open fun foo() {}
        internal open fun bar() {}
    }
    protected class ProtectedDerived : Base() {
        public override fun foo() {}
        public override fun bar() {}
    }
    internal class InternalDerived : Base() {
        public override fun foo() {}
        public override fun bar() {} // REDUNDANT_VISIBILITY_MODIFIER
    }
    private class PrivateDerived : Base() {
        public override fun foo() {}
        public override fun bar() {} // REDUNDANT_VISIBILITY_MODIFIER
    }
}
@Suppress("REDUNDANT_VISIBILITY_MODIFIER") public fun suppressed() {}
