// Extension properties whose receiver is a function type, which is in
// parentheses: val (Int.() -> String).p.
val (Int.() -> String).one: String
    get() = this(1)

val <T> (Int.() -> T).two: T
    get() = this(2)

var log = ""

var ((String) -> Int).logged: String
    get() = "len " + this("abc")
    set(value) {
        log += value + this(value)
    }

val (() -> Unit)?.isSet: Boolean
    get() = this != null

class Box(val n: Int) {
    val ((Int) -> Int).applied: Int
        get() = this(n)

    fun show(f: (Int) -> Int) = f.applied
}

fun main() {
    val f: Int.() -> String = { "f" + this }
    println(f.one)
    println({ x: Int -> x * 10 }.two)
    val g: (String) -> Int = { it.length }
    println(g.logged)
    g.logged = "hello"
    println(log)
    val h: (() -> Unit)? = null
    println("${h.isSet} ${{ }.isSet}")
    println(Box(7).show { it + 1 })
}
