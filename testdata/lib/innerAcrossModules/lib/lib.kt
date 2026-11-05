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

// A public inline function of an inner class, inlined in another module,
// reads the outer instance through the class's access$getThis$0$p.
class Box(val v: String) {
    inner class View {
        inline fun show(f: (String) -> String) = f(v)
    }
}
