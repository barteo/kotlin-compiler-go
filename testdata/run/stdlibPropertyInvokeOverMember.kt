// A call of a name that is a member function and a property whose value
// has invoke() is the property's where only it takes the arguments, an
// argument too that is a property whose type is inferred.

class A

class State(val s: String = "abc")

class Use(val a: A, val n: Int) {
    operator fun invoke(state: State): Int = state.s.length + n
}

class Test {
    private val a = A()
    private val n = 2
    private val sut by lazy { sut(a = a) }

    private fun sut(a: A, n: Int = this.n) = Use(a, n)

    fun run() = sut(state) + sut(A(), 5)(state) + sut(a = a)(State())

    private val state =
        State(s = "abcd")
}

fun main() {
    println(Test().run())
}
