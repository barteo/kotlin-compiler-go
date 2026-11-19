// kotlin.contracts.contract is @ExperimentalContracts: a contract needs
// the opt-in, as with kotlinc.
import kotlin.contracts.*

fun f1(x: Any?): Boolean {
    contract { returns(true) implies (x != null) } // ERROR this declaration needs opt-in
    return x != null
}

@OptIn(ExperimentalContracts::class)
fun f2(block: () -> Unit) {
    contract { callsInPlace(block, InvocationKind.EXACTLY_ONCE) }
    block()
}

@ExperimentalContracts
fun f3(x: Any?): Boolean {
    contract { returns(true) implies (x is String) }
    return x is String
}

class C {
    fun m(block: () -> Unit) {
        contract { callsInPlace(block) } // ERROR this declaration needs opt-in
        block()
    }
}
