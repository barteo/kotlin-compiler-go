// Functions whose block body ends in a statement that always jumps,
// though its type isn't Nothing: a jump in what the statement always
// evaluates, or a break that a finally block's jump overrides.

fun pair(a: Any, b: Any) {}

fun inArgument(): String {
    var s = "unset"
    pair(if (s == "unset") { s = "argument"; 1L } else "", return s)
}

fun inInitializer(n: Int): Int {
    val x: Array<Int> = Array(n, return n * 2)
}

fun inWhenInitializer(c: Int): Int {
    val nothing = when (c) {
        1 -> return 10
        2 -> return 20
        else -> return 30
    }
}

fun inRange(): String {
    for (i in 1..return "range") {}
}

fun inAssignment(): String {
    var s = ""
    s = "a" + return "assignment"
}

fun inTemplate(): String {
    println("${return "template"}")
}

fun breakOverridden(): String {
    var r = ""
    while (true) {
        try {
            r += "try "
            if (r.length > 0) break
        } finally {
            return r + "finally"
        }
    }
}

fun labeledBreakOverridden(): String {
    outer@ while (true) {
        var x = ""
        try {
            do {
                x = x + break@outer
            } while (true)
        } finally {
            return "labeled"
        }
    }
}

fun doWhileJumps(): String {
    var x: Any = 1
    do {
        try {
            throw IllegalStateException()
        } catch (e: IllegalStateException) {
            x = "do-while"
            break
        } finally {
            return x.toString()
        }
    } while (false)
}

class Runner {
    inline operator fun <T> invoke(f: () -> T) = f()
}

fun inInvokedLambda(): String {
    val run = Runner()
    val x = run { return "invoke" }
}

fun main() {
    println(inArgument())
    println(inInitializer(21))
    println(inWhenInitializer(2))
    println(inRange())
    println(inAssignment())
    println(inTemplate())
    println(breakOverridden())
    println(labeledBreakOverridden())
    println(doWhileJumps())
    println(inInvokedLambda())
}
