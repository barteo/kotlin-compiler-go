// Enum entries with bodies, whose classes, Foo$FOO, aren't public: a
// program names the entries, not their classes.
enum class Foo {
    FOO() {
        private fun privateBar() = "bar"
        override fun bar(): String = privateBar()
        override var xxx: String
            get() = "xxx"
            set(value) {}
    };

    abstract fun bar(): String
    abstract var xxx: String
}

enum class A {
    X {
        val x = "OK"

        inner class Inner {
            val y = x
        }

        val z = Inner()
        override val test: String
            get() = z.y
    };

    abstract val test: String
}
