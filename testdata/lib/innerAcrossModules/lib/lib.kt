// Inner classes that another module's inner classes extend: their
// constructors take the outer instance, which the subclass passes.
open class Foo(val z: Int) {
    open inner class FooInner {
        fun foo() = z
    }
}

open class G<T>(val t: T) {
    open inner class GI(val s: String = "d") {
        fun get() = "$t$s"
    }
}
