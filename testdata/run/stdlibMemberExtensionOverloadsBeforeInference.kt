class State(val n: Int)

class Mapper {
    fun State.step() = whenStep { listOf(indicator(), indicator(2, true)) }

    private fun State.whenStep(block: State.() -> List<String>?): List<String>? = this.block()

    private fun State.indicator(extra: Int, flag: Boolean) = connected { "indicator $n $extra $flag" }

    private fun State.indicator() = connected { "indicator $n" }

    private fun State.connected(block: State.() -> String): String = block()
}

fun main() {
    with(Mapper()) {
        println(State(1).step())
    }
}
