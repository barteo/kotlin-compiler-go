// An inner class constructed in a lambda inside another lambda takes the
// outer instance, which both lambdas capture, where the outer lambda's
// call is the receiver of another call.

fun <T> collect(fill: (MutableList<T>) -> Unit): List<T> {
    val list = mutableListOf<T>()
    fill(list)
    return list
}

fun each(block: () -> Unit) {
    block()
}

class Use(private val n: Int) {
    fun run(s: String): List<Int> =
        collect { out ->
                val length = length(s)
                each {
                    val session = Session(length, out)
                    session.on(1)
                    session.on(2)
                }
            }
            .reversed()

    private fun length(s: String): Int = s.length

    private inner class Session(private val length: Int, private val out: MutableList<Int>) {
        fun on(i: Int) {
            out.add(i + length + n)
        }
    }
}

fun main() {
    println(Use(10).run("abc"))
}
