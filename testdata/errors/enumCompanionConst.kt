// A constant of an enum's companion object is as uninitialized as its
// other members where the enum's entries are made, though its value is
// inlined: kotlinc reports it in an entry's arguments and in the
// initializers the entries run.
enum class E(val v: Int) {
    A(C), // ERROR companion object of enum class 'E' is uninitialized here
    B(E.C), // ERROR companion object of enum class 'E' is uninitialized here
    D(E.Companion.C), // ERROR companion object of enum class 'E' is uninitialized here
    F(1) {
        override fun f() = C
    };

    val g: Int = C * v // ERROR companion object of enum class 'E' is uninitialized here
    val lazily: Int get() = C

    open fun f() = C + 1

    companion object {
        const val C = 1
        val twice = C * 2
    }
}

const val TOP = 2

// A constant declared elsewhere is one.
enum class G(val v: Int) {
    A(TOP),
    B(E.C)
}
