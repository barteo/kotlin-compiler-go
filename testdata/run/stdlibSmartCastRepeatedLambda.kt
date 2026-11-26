// Smart casts of local vars in lambdas that may run more than once, as
// K2 has them: such a lambda is a loop, which keeps the smart casts of
// the vars it only assigns with op=, ++ or --, and an assignment that may
// run while a lambda does keeps the lambda's smart cast if it assigns a
// value of the smart cast's type.
import kotlin.contracts.*

@OptIn(ExperimentalContracts::class)
inline fun atLeastOnce(times: Int, block: () -> Unit) {
    contract { callsInPlace(block, InvocationKind.AT_LEAST_ONCE) }
    var i = 0
    do {
        block()
        i++
    } while (i < times)
}

fun later(block: () -> Unit): () -> Unit = block

fun <R> materialize(value: R): R = value

fun opAssignInRepeatedLambda(): Any? {
    var x: Any? = materialize("a")
    require(x is String)
    atLeastOnce(3) {
        println(x.length)
        x += "b"
    }
    return x
}

fun incrementInRepeatedLambda(): Any? {
    var x: Any? = materialize(1)
    require(x is Int)
    atLeastOnce(2) {
        x++
        println(x.inc())
    }
    return x
}

fun assignedBeforeLambdaInLoop(): List<() -> Unit> {
    var x: Any? = materialize("c")
    val blocks = mutableListOf<() -> Unit>()
    for (i in 1..3) {
        require(x is String)
        x = "d".repeat(i)
        blocks += later { println(x.length) }
    }
    return blocks
}

fun incrementAfterLambda(): () -> Unit {
    var x: Any? = materialize(10)
    require(x is Int)
    val f = later { println(x.inc()) }
    x++
    return f
}

fun main() {
    println(opAssignInRepeatedLambda())
    println(incrementInRepeatedLambda())
    assignedBeforeLambdaInLoop().forEach { it() }
    incrementAfterLambda()()
}
