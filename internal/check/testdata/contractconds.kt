@file:OptIn(kotlin.contracts.ExperimentalContracts::class)
import kotlin.contracts.*

fun c1(x: String?): Any? {
    contract {
        returnsNotNull() implies (x is String && x != null) // SENSELESS_COMPARISON
    }
    return x
}
fun c2(x: String?): Any? {
    contract {
        returnsNotNull() implies (x is String? || x is Any?) // USELESS_IS_CHECK
    }
    return x
}
fun c3(x: String?, y: Int): Boolean {
    contract {
        returns(true) implies (x != null)
        returns(false) implies (y != null) // SENSELESS_COMPARISON
    }
    return x != null
}
fun c4(x: Any?): Boolean {
    contract {
        returns(true) implies (x is String || x is CharSequence)
        returns(false) implies (x !is String && x is CharSequence)
    }
    return x is String
}
