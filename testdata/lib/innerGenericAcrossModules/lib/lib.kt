// Inner classes of generic classes, which see their outer classes' type
// parameters: their metadata numbers those first, the outermost class's
// first.
class C4<X> {
    inner class D4<Y> {
        fun foo(x: X, y: Y) = "second $x $y"

        inner class E4<Z> {
            fun foo(x: X, y: Y, z: Z) = "third $x $y $z"
            inline fun bar(x: X, y: Y, z: Z, f: (X, Y, Z) -> String) = f(x, y, z)
        }
    }
}

open class G<T>(val t: T) {
    open inner class GI(val s: String = "d") {
        fun get(): T = t
    }
}
