// The line numbers that stack traces show: of statements, calls whose
// arguments are on other lines, constructors, initializers, members,
// local functions, lambdas and loops.

fun here(): Int = Throwable().getStackTrace()[1].getLineNumber()

fun twice(a: Int, b: Int): String = "" + a + " " + b

class Point(val x: Int) {
    val atInit = here()
    var inInit = 0

    init {
        inInit = here()
    }

    constructor(x: Int, y: Int) : this(x + y) {
        inInit = here()
    }

    fun member(): Int {
        val unused = x
        return here()
    }
}

fun local(): Int {
    fun inner() = here()
    return inner()
}

fun lambda(): Int {
    val f = {
        here()
    }
    return f()
}

fun main() {
    println(here())
    println(twice(
        here(),
        here()))
    val p = Point(1)
    println(p.atInit)
    println(p.inInit)
    println(Point(1, 2).inInit)
    println(p.member())
    println(local())
    println(lambda())
    var sum = 0
    for (i in 0 until 2) {
        sum += here()
    }
    println(sum)
    var n = 0
    while (n < 1) {
        n++
        println(here())
    }
    val s = "abc"
        .length
    println(s + here())
    try {
        val a: Any = "x"
        println(a as Int)
    } catch (e: ClassCastException) {
        println(e.getStackTrace()[0].getLineNumber())
    }
}
