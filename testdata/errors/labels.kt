fun <T> run(f: () -> T): T = f()
fun cond(): Boolean = false

fun main() {
    outer@ for (i in 0..3) {
        break@inner // ERROR unresolved label: inner
    }
    outer@ while (cond()) {
        run {
            continue@outer // ERROR 'continue' is not allowed outside a loop
        }
    }
    do {
        val y = 1
    } while (y > 1)
    println(y) // ERROR unresolved reference: y
    val v: Int
    do {
        v = 1 // ERROR val cannot be reassigned
    } while (cond())
    val f = lit@{ n: Int ->
        if (n > 0) return@run 1 // ERROR unresolved label: run
        n
    }
}
