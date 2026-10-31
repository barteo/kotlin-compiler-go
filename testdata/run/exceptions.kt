// throw, try/catch/finally, exceptions declared in Kotlin, and the
// preconditions require, check, error and TODO.

open class AppError(val code: Int) : Exception("app error " + code)

class NotFound(val key: String) : AppError(404) {
    override val message: String
        get() = "not found: " + key
}

class Invalid(reason: String) : IllegalArgumentException(reason)

val log = StringBuilder()

fun find(key: String): String {
    if (key.isEmpty()) throw Invalid("empty key")
    if (key == "x") throw NotFound(key)
    if (key == "boom") throw AppError(500)
    return key.uppercase()
}

fun lookup(key: String): String = try {
    find(key)
} catch (e: NotFound) {
    "missing " + e.key + " (" + e.code + ", " + e.message + ")"
} catch (e: AppError) {
    "failed " + e.code
} catch (e: IllegalArgumentException) {
    "invalid: " + e.message
} finally {
    log.append(key).append(";")
}

fun parse(s: String): Int = try {
    Integer.parseInt(s)
} catch (e: NumberFormatException) {
    -1
}

fun nested(n: Int): String {
    try {
        try {
            if (n == 0) return "zero"
            if (n == 1) throw IllegalStateException("one")
            return "many"
        } finally {
            log.append("inner;")
        }
    } catch (e: IllegalStateException) {
        return "caught " + e.message
    } finally {
        log.append("outer;")
    }
}

fun loop(): Int {
    var sum = 0
    for (i in 0 until 10) {
        try {
            if (i == 2) continue
            if (i == 5) break
            sum += i
        } finally {
            log.append(i).append(",")
        }
    }
    return sum
}

fun rethrow(): String {
    try {
        try {
            find("x")
        } catch (e: AppError) {
            log.append("rethrowing;")
            throw e
        }
    } catch (e: NotFound) {
        return "outer got " + e.key
    }
    return "none"
}

fun sum(a: Int, b: Int, c: Int): Int = a + b + c

fun required(n: Int?): Int {
    val v = requireNotNull(n) { "n is required" }
    require(v > 0) { "n must be positive, was " + v }
    check(v < 100)
    return v * 2
}

fun safe(f: () -> Int): String = try {
    "ok " + f()
} catch (e: IllegalArgumentException) {
    "IAE " + e.message
} catch (e: IllegalStateException) {
    "ISE " + e.message
}

fun first(s: String?): Char0 = Char0(s ?: throw IllegalArgumentException("no string"))

class Char0(val s: String)

class Config(text: String) {
    var value = 0
    var error = ""

    init {
        try {
            value = Integer.parseInt(text)
        } catch (e: NumberFormatException) {
            error = "bad " + text
        }
    }
}

fun twice(f: () -> Int): Int = f() + f()

fun withLambdaInFinally(n: Int): Int {
    var calls = 0
    try {
        if (n > 0) return n
    } finally {
        calls = twice { calls + 1 }
    }
    return calls
}

fun lambdaReturns(xs: java.util.ArrayList<Int>): Int = twice {
    try {
        if (xs.size > 1) return@twice xs.get(0)
        0
    } finally {
        log.append("lambda;")
    }
}

fun main() {
    println(lookup("abc"))
    println(lookup("x"))
    println(lookup("boom"))
    println(lookup(""))
    println(log)
    log.setLength(0)

    println(parse("42") + parse("4x2"))
    println(nested(0) + " " + nested(1) + " " + nested(2))
    println(log)
    log.setLength(0)
    println(loop())
    println(log)
    log.setLength(0)
    println(rethrow())
    println(log)

    println(sum(1, try { Integer.parseInt("x") } catch (e: Exception) { 10 }, 100))
    println("total " + try { find("ok") } finally { log.append("f") })

    println(safe { required(21) })
    println(safe { required(null) })
    println(safe { required(-1) })
    println(safe { required(500) })
    println(safe { error("bad state") })
    println(safe { check(false) { "checked " + 1 }; 0 })
    println(safe { require(false); 0 })
    println(safe { checkNotNull(null as String?); 0 })
    try {
        first(null)
    } catch (e: IllegalArgumentException) {
        println(e.message)
    }
    println(first("s").s)
    println(Config("12").value)
    println(Config("x").error)
    println(withLambdaInFinally(3) + withLambdaInFinally(0))
    val xs = java.util.ArrayList<Int>()
    xs.add(5)
    xs.add(6)
    log.setLength(0)
    println(lambdaReturns(xs))
    println(log)
    try {
        TODO("later")
    } catch (e: Throwable) {
        println(e.message)
    }
}
