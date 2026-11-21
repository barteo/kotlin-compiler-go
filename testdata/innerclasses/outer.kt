// Nested classes that other classes name (TestInnerClasses).
package q

class Outer {
    class Nest {
        fun hi() = "hi"

        class Deeper
    }

    inner class Inner

    annotation class Mark

    @RequiresOptIn
    @Retention(AnnotationRetention.BINARY)
    annotation class Experimental

    enum class Kind { A, B }

    interface Callback {
        fun call()
    }

    companion object {
        const val NAME = "outer"
    }
}

class Generic<T> {
    inner class Part(val t: T)
}
