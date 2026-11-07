// A lambda that ends with a try, whose blocks end with ifs without else,
// as octi-server's Mutex.withLock { try { ... } catch (e: E) { ... } }:
// the ifs are statements, and return@locked makes T Unit.

inline fun <T> locked(action: () -> T): T = action()

fun f(x: Int) {
    locked {
        try {
            if (x < 0) return@locked
            println("try $x")
            if (x > 0) {
                println("positive")
                if (x > 2) println("big")
            }
        } catch (e: IllegalStateException) {
            if (x > 1) println("caught")
        }
    }
}

fun g(x: Int) = locked {
    try {
        check(x != 2)
        if (x > 0) println("g $x")
    } catch (e: IllegalStateException) {
        try {
            if (x == 2) println("g two")
        } finally {
            println("finally")
        }
    }
}

fun main() {
    f(-1)
    f(0)
    f(3)
    g(1)
    g(2)
}
