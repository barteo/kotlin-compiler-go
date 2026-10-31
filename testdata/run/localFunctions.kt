fun factorial(n: Int): Long {
    fun go(k: Int, acc: Long): Long = if (k <= 1) acc else go(k - 1, acc * k)
    return go(n, 1L)
}

fun counter(): Int {
    var count = 0
    fun bump(by: Int = 1) {
        count += by
    }
    bump()
    bump(5)
    bump(by = 10)
    return count
}

// A lambda that calls a local function captures what the function does.
fun viaLambda(): Int {
    var total = 0
    val base = 100
    fun add(x: Int) {
        total += x + base
    }
    val f = { x: Int -> add(x) }
    f(1)
    f(2)
    return total
}

// Nested local functions, and one that calls itself from a lambda.
fun nested(n: Int): String {
    val sb = StringBuilder()
    fun outer(depth: Int) {
        fun inner(s: String) {
            sb.append(s).append(depth)
        }
        inner("[")
        if (depth < n) {
            val again = { outer(depth + 1) }
            again()
        }
        inner("]")
    }
    outer(0)
    return sb.toString()
}

// Generic local functions and overloads.
fun generic(): String {
    fun <T : Any> twice(x: T): String = x.toString() + x.toString()
    fun describe(i: Int): String = "int " + i
    fun describe(s: String): String = "string " + s
    return twice(4) + " " + twice("ab") + " " + describe(1) + " " + describe("x")
}

fun shadow(): String = "top-level"

fun shadowing(): String {
    fun shadow(): String = "local"
    return shadow()
}

fun sameNames(flag: Boolean): Int {
    if (flag) {
        fun pick(): Int = 1
        return pick()
    } else {
        fun pick(): Int = 2
        return pick()
    }
}

class Account(val owner: String) {
    var balance = 0

    fun deposit(amounts: IntArray): String {
        fun log(what: String): String = owner + ": " + what + " -> " + balance
        val out = StringBuilder()
        for (a in amounts) {
            fun apply() {
                balance += a
            }
            apply()
            out.append(log("deposit " + a)).append("; ")
        }
        return out.toString()
    }

    init {
        fun greet() {
            println("opened for " + owner)
        }
        greet()
    }
}

interface Greeter {
    fun name(): String
    fun greet(): String {
        fun exclaim(s: String): String = s + "!"
        return exclaim("hello " + name())
    }
}

class English : Greeter {
    override fun name(): String = "world"
}

fun String.decorated(): String {
    fun wrap(left: String) = left + this + ")>"
    return wrap("<(")
}

fun earlyReturn(xs: IntArray): Int {
    fun firstNegative(): Int {
        for (x in xs) {
            if (x < 0) {
                return x
            }
        }
        return 0
    }
    return firstNegative() * 10
}

fun apply(f: (Int) -> Int, x: Int): Int = f(x)

fun references(): Int {
    val k = 3
    fun times(x: Int): Int = x * k
    return apply(::times, 5)
}

fun withVararg(): Int {
    fun sum(vararg xs: Int): Int {
        var s = 0
        for (x in xs) {
            s += x
        }
        return s
    }
    return sum(1, 2, 3) + sum()
}

class Temperature {
    var celsius = 0.0

    constructor(fahrenheit: Int) {
        fun convert(f: Int): Double = (f - 32) * 5.0 / 9.0
        celsius = convert(fahrenheit)
    }

    companion object {
        fun parse(s: String): Temperature {
            fun digits(): Int = Integer.parseInt(s.substring(0, s.length - 1))
            return Temperature(digits())
        }
    }
}

class Widget(val name: String) {
    fun render(times: Int): String {
        fun piece(i: Int): String = name + i
        val pieces = java.util.ArrayList<() -> String>()
        for (i in 0 until times) {
            pieces.add { piece(i) }
        }
        val sb = StringBuilder()
        for (p in pieces) {
            sb.append(p())
        }
        return sb.toString()
    }
}

fun smartCast(x: Any): Int {
    if (x is String) {
        fun len(): Int = x.length
        return len()
    }
    return -1
}

fun inTry(): String {
    var log = ""
    try {
        fun note(s: String) {
            log += s
        }
        note("a")
        throw IllegalStateException("b")
    } catch (e: IllegalStateException) {
        fun note2(s: String?) {
            log += s
        }
        note2(e.message)
    } finally {
        log += "c"
    }
    return log
}

fun main() {
    println(factorial(10))
    println(counter())
    println(viaLambda())
    println(nested(2))
    println(generic())
    println(shadowing())
    println(sameNames(true) + sameNames(false))
    val acc = Account("ann")
    println(acc.deposit(intArrayOf(5, 7)))
    println(English().greet())
    println("x".decorated())
    println(earlyReturn(intArrayOf(3, -4, -5)))
    println(references())
    println(withVararg())
    println(Temperature.parse("212F").celsius)
    println(Widget("w").render(3))
    println(smartCast("four") + smartCast(4))
    println(inTry())
}
