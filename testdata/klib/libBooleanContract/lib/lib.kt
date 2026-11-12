package l

import kotlin.contracts.ExperimentalContracts
import kotlin.contracts.contract

@OptIn(ExperimentalContracts::class)
fun ensure(cond: Boolean, msg: String = "") {
    contract { returns() implies cond }
    if (!cond) throw IllegalStateException(msg)
}
