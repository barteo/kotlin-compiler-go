// A lambda that may run more than once drops, at its start, the smart
// casts of the vars it assigns with =, as a loop does, and one that may
// run at any time ends the aliases of those it assigns with op= too, as
// in K2.
import kotlin.contracts.*

@OptIn(ExperimentalContracts::class)
fun atLeastOnce(block: () -> Unit) {
    contract { callsInPlace(block, InvocationKind.AT_LEAST_ONCE) }
    block()
}

fun later(block: () -> Unit) = block()

fun <R> materialize(): R = null!!

fun assignedInRepeatedLambda() {
    var x: Any? = materialize()
    require(x is String)
    atLeastOnce {
        x.length // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type Any?
        x = ""
    }
}

fun aliasEndedByOpAssign() {
    val x: Any? = materialize()
    var y = x
    later {
        require(y is String)
        y += "" // ERROR nullable receiver of type Any?
        x.length // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type Any?
    }
}
