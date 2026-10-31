// Calls that invoke a value: an extension property of function type,
// x.f(y), an enum entry with an invoke operator, E.ENTRY(y); and enum
// entries named in other entries' arguments.

class A(val x: String)

val A.plain: (String) -> String
    get() = { y -> x + y }

val A.withRecv: A.(String) -> String
    get() = fun A.(y: String): String { return this@withRecv.x + this.x + y }

val Int.ext: () -> Int get() = { this * 2 }
val Long.ext: Long get() = 4.ext().toLong()

enum class Op {
    TWICE, THRICE;

    operator fun invoke(i: Int) = i * (ordinal + 2)
}

interface Callback {
    fun call(): String
}

enum class E(val x: String, val f: () -> String, val cb: Callback) {
    FOO("foo", { FOO.x + BAR.x }, object : Callback { override fun call() = BAR.x }),
    BAR("bar", { "" }, object : Callback { override fun call() = FOO.x }),
}

fun main() {
    println(A("1").plain("2"))
    println(A("1").withRecv(A("2"), "3"))
    println(5L.ext)
    println(Op.TWICE(10))
    println(Op.THRICE(10))
    println(E.FOO.f())
    println(E.FOO.cb.call() + E.BAR.cb.call())
}
