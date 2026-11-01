// Returns of tailrec functions from lambdas passed to the standard
// library's inline functions, whose bytecode is inlined: the jumps leave
// a loop, and run the finally blocks of synchronized and use.

val one = listOf(1)

tailrec fun viaRun(n: Int, acc: Long): Long {
    run {
        if (n == 0) return acc
        return viaRun(n - 1, acc + n)
    }
}

tailrec fun viaLet(n: Int, acc: Long): Long {
    n.let {
        if (it == 0) return acc
        return viaLet(it - 1, acc + 1)
    }
}

tailrec fun viaForEach(n: Int, acc: Long): Long {
    if (n == 0) return acc
    one.forEach { return viaForEach(n - 1, acc + it) }
    return -1
}

val lock = Any()

tailrec fun viaSynchronized(n: Int): Int {
    synchronized(lock) {
        if (n == 0) return 5
        return viaSynchronized(n - 1)
    }
}

var closes = 0

class Res : AutoCloseable {
    override fun close() {
        closes++
    }
}

tailrec fun viaUse(n: Int): Int {
    Res().use {
        if (n == 0) return 6
        return viaUse(n - 1)
    }
}

fun main() {
    println(viaRun(1_000_000, 0))
    println(viaLet(1_000_000, 0))
    println(viaForEach(1_000_000, 0))
    println(viaSynchronized(1_000_000))
    println(Thread.holdsLock(lock))
    println(viaUse(1_000_000))
    println(closes)
}
