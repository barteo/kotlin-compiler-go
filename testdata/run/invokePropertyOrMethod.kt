class Logger(private val prefix: String) {
    operator fun invoke(n: Int) = println("$prefix $n")
}

fun repeat2(block: () -> Unit) {
    block()
    block()
}

class Worker(private val log: Logger) {
    init {
        log()
        repeat2 { log() }
    }

    private fun log() {
        for (n in 1..2) log(n)
    }
}

fun main() {
    Worker(Logger("logged"))
}
