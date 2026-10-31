package geo

/** A class in the same package as main, used without an import. */
@Suppress("unused")
class Local
@Deprecated("for the test") constructor(@Suppress("x") val n: Int) {
    @Volatile
    var calls = 0

    fun twice(): Int {
        calls++
        return n * 2
    }
}

fun local(): Int = Local(4).twice()
