package a

// An inner class's public inline functions read the outer instance
// through access$getThis$0$p, as kotlinc's do (KT-66096).
class E(val x: String) {
    inner class Inner(val y: String) {
        inline fun foo(z: String) = x + y + z
        inline fun bar(f: (String) -> String) = f(x)
        fun baz() = foo("!")
    }
}
