// Constructs the parser rejected in the wide survey (milestone 51).

interface Shape {
    val sides: Int
}

class Square : Shape {
    override val sides: Int
        @JvmName("sideCount")
        get() = 4
}

class Named(private val name: String) {
    var label = name
        private set
    val shout
        @Suppress("unused")
        get() = name.uppercase()
}

object Delegating : Shape
by Square()

fun <A, B
: Comparable<B>> maxOf2(a: A, b: B, c: B)
        : Pair<A, B> = a to if (b > c) b else c

fun <
    T,
    R,
> pairUp(t: T, r: R): Pair<T, R> = t to r

fun twice(f: (Int)
        -> Int): (Int) -> Int = { f(f(it)) }

fun describe(x: Int): String =
    @Suppress("MagicNumber")
    when (x) {
        1,
        2,
        -> "small"
        else -> "big"
    }

fun pick(flag: Boolean): (Int) -> Int =
    if (flag) { n -> n + 1 }
    else { n -> n * 2 }

fun choose(x: Int): (String) -> String = when (x) {
    0 -> { s -> s.reversed() }
    else -> { s -> s + s }
}

fun branchReturn(x: Int?): Int {
    when (x) {
        null -> if (x == null) return -1
        else -> return x
    }
    return 0
}

fun <T> apply2(t: T, f: (T, T) -> T): T = f(t, t)

fun <T> build(f: () -> T): T = f()

fun withReceiver(f: (StringBuilder).() -> Unit): String = StringBuilder().apply(f).toString()

class Grid {
    operator fun get(r: Int, c: Int): Int = r * 10 + c
}

val lazyText: String
    by lazy { "lazy" }

fun <@Suppress("unused") T> identity(t: T): T = t

fun main() {
    println(Square().sides)
    val n = Named("ann")
    println(n.label + " " + n.shout)
    println(Delegating.sides)
    println(maxOf2("a", 3, 2))
    println(pairUp(1, "one"))
    println(twice { it + 3 }(1))
    println(describe(2) + " " + describe(7))
    println(pick(true)(5).toString() + " " + pick(false)(5))
    println(choose(0)("abc") + " " + choose(1)("ab"))
    println(branchReturn(null) + branchReturn(4))
    val big = if
        (n.label.length > 2) "long" else "short"
    println(big)
    println(apply2(3) { a, b
        -> a * b })
    println(build<String>
        { "built" })
    val trailing = build()
    { "next line" }
    println(trailing)
    println(listOf("a", "b").map(String::uppercase))
    println(withReceiver { append("rx") })
    println(Grid()[
        1,
        2,
    ])
    println(lazyText)
    println(identity(9))
    var i = 0
    while (++i < 3);
    println(i)
    try {
        error("boom")
    } catch (@Suppress("SwallowedException") e: IllegalStateException) {
        println(e.message)
    }
}
